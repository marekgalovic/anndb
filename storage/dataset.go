package storage

import (
	"errors";
	"context";
	"sync";
	"math/rand";
	"sort";
	"io";

	pb "github.com/marekgalovic/anndb/protobuf";
	"github.com/marekgalovic/anndb/cluster";
	"github.com/marekgalovic/anndb/index";
	"github.com/marekgalovic/anndb/math";
	"github.com/marekgalovic/anndb/storage/raft";
	
	"github.com/satori/go.uuid";
	badger "github.com/dgraph-io/badger/v2";
)

var (
	DimensionMissmatchErr error = errors.New("Value dimension does not match dataset dimension")
	PartitionNotFoundErr error = errors.New("Partition not found")
)

type Dataset struct {
	id uuid.UUID
	meta *pb.Dataset
	clusterConn *cluster.Conn

	partitions []*partition
	partitionsMap map[uuid.UUID]*partition
	partitionsMu *sync.RWMutex

	searchClients map[uint64]pb.SearchClient
	searchClientsMu *sync.RWMutex
}

func newDataset(id uuid.UUID, meta pb.Dataset, raftWalDB *badger.DB, raftTransport *raft.RaftTransport, clusterConn *cluster.Conn, datasetManager *DatasetManager) (*Dataset, error) {
	d := &Dataset {
		id: id,
		meta: &meta,
		clusterConn: clusterConn,
		partitions: make([]*partition, meta.GetPartitionCount()),
		partitionsMap: make(map[uuid.UUID]*partition),
		partitionsMu: &sync.RWMutex{},
		searchClients: make(map[uint64]pb.SearchClient),
		searchClientsMu: &sync.RWMutex{},
	}

	for i := 0; i < int(meta.GetPartitionCount()); i++ {
		pid, err := uuid.FromBytes(meta.Partitions[i].GetId())
		if err != nil {
			return nil, err
		}
		partition := newPartition(pid, meta.Partitions[i], d, raftWalDB, raftTransport, datasetManager)
		d.partitions[i] = partition
		d.partitionsMap[pid] = partition
	}

	return d, nil
}

func (this *Dataset) close() {
	this.partitionsMu.Lock()
	defer this.partitionsMu.Unlock()

	for _, partition := range this.partitions {
		partition.close()
	}
}

func (this *Dataset) Meta() *pb.Dataset {
	return this.meta
}

func (this *Dataset) Insert(ctx context.Context, id uint64, value math.Vector) error {
	if err := this.checkDimension(&value); err != nil {
		return err
	}

	return this.getPartitionForId(id).insert(ctx, id, value)
}

func (this *Dataset) Remove(ctx context.Context, id uint64) error {
	return this.getPartitionForId(id).remove(ctx, id)
}

func (this *Dataset) Search(ctx context.Context, query math.Vector, k uint) (index.SearchResult, error) {
	if err := this.checkDimension(&query); err != nil {
		return nil, err
	}

	ctx, cancelCtx := context.WithCancel(ctx)
	defer cancelCtx()

	nodePartitions := this.getSearchQueryNodes()

	wg := &sync.WaitGroup{}
	resultCh := make(chan index.SearchResult, len(nodePartitions))
	errorCh := make(chan error, len(nodePartitions))

	for nodeId, partitionIds := range nodePartitions {
		wg.Add(1)
		go this.searchPartitionsOnNode(ctx, nodeId, partitionIds, query, k, wg, resultCh, errorCh)
	}

	go func() {
		wg.Wait()
		close(resultCh)
		close(errorCh)
	}()

	result := make(index.SearchResult, 0, int(k) * len(nodePartitions))
	for i := 0; i < len(nodePartitions); i++ {
		select {
		case items := <- resultCh:
			result = append(result, items...)
		case err := <- errorCh:
			return nil, err
		case <- ctx.Done():
			return nil, ctx.Err()
		}
	}

	sort.Sort(result)
	return result[:math.MinInt(int(k), len(result))], nil
}

func (this *Dataset) SearchPartitions(ctx context.Context, partitionIds []uuid.UUID, query math.Vector, k uint) (index.SearchResult, error) {
	var err error
	partitions := make([]*partition, len(partitionIds))
	for i, partitionId := range partitionIds {
		partitions[i], err = this.getPartition(partitionId)
		if err != nil {
			return nil, err
		}
	}

	ctx, cancelCtx := context.WithCancel(ctx)
	defer cancelCtx()

	wg := &sync.WaitGroup{}
	resultCh := make(chan index.SearchResult, len(partitions))	
	errorCh := make(chan error, len(partitions))

	for _, partition := range partitions {
		wg.Add(1)
		go this.searchPartition(ctx, partition, query, k, wg, resultCh, errorCh)
	}

	go func() {
		wg.Wait()
		close(resultCh)
		close(errorCh)
	}()

	result := make(index.SearchResult, 0, int(k) * len(partitions))
	for i := 0; i < len(partitions); i++ {
		select {
		case items := <- resultCh:
			result = append(result, items...)
		case err := <- errorCh:
			return nil, err
		case <- ctx.Done():
			return nil, ctx.Err()
		}
	}

	sort.Sort(result)
	return result[:math.MinInt(int(k), len(result))], nil
}

func (this *Dataset) getPartition(id uuid.UUID) (*partition, error) {
	this.partitionsMu.RLock()
	defer this.partitionsMu.RUnlock()

	if partition, exists := this.partitionsMap[id]; exists {
		return partition, nil
	}
	return nil, PartitionNotFoundErr
}

func (this *Dataset) getPartitionForId(id uint64) *partition {
	this.partitionsMu.RLock()
	defer this.partitionsMu.RUnlock()

	return this.partitions[id % uint64(this.Meta().GetPartitionCount())]
}

func (this *Dataset) checkDimension(value *math.Vector) error {
	if uint32(len(*value)) != this.Meta().GetDimension() {
		return DimensionMissmatchErr
	}
	return nil
}

func (this *Dataset) getSearchQueryNodes() map[uint64][]uuid.UUID {
	this.partitionsMu.RLock()
	defer this.partitionsMu.RUnlock()

	result := make(map[uint64][]uuid.UUID)
	for _, partition := range this.partitions {
		partitionNodeIds := partition.nodeIds()
		nodeId := partitionNodeIds[rand.Intn(len(partitionNodeIds))]
		if _, exists := result[nodeId]; !exists {
			result[nodeId] = make([]uuid.UUID, 0)
		}
		result[nodeId] = append(result[nodeId], partition.id)
	}
	return result
}

func (this *Dataset) searchPartitionsOnNode(ctx context.Context, nodeId uint64, partitionIds []uuid.UUID, query math.Vector, k uint, wg *sync.WaitGroup, resultCh chan index.SearchResult, errorCh chan error) {
	defer wg.Done()

	client, err := this.getNodeSearchClient(ctx, nodeId)
	if err != nil {
		errorCh <- err
		return
	}

	partitionIdsBytes := make([][]byte, len(partitionIds))
	for i, partitionId := range partitionIds {
		partitionIdsBytes[i] = partitionId.Bytes()
	}

	request := &pb.SearchPartitionsRequest {
		DatasetId: this.Meta().GetId(),
		PartitionIds: partitionIdsBytes,
		Query: query,
		K: uint32(k),
	}

	stream, err := client.SearchPartitions(ctx, request)
	if err != nil {
		errorCh <- err
		return
	}

	result := make(index.SearchResult, 0, k)
	for {
		item, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			errorCh <- err
			return
		}

		result = append(result, index.SearchResultItem {
			Id: item.GetId(),
			Score: item.GetScore(),
		})
	}

	resultCh <- result
}

func (this *Dataset) searchPartition(ctx context.Context, partition *partition, query math.Vector, k uint, wg *sync.WaitGroup, resultCh chan index.SearchResult, errorCh chan error) {
	defer wg.Done()

	result, err := partition.search(ctx, query, k)
	if err != nil {
		errorCh <- err
		return
	}
	resultCh <- result
}

func (this *Dataset) getNodeSearchClient(ctx context.Context, nodeId uint64) (pb.SearchClient, error) {
	client := this.getCachedNodeSearchClient(nodeId)
	if client != nil {
		return client, nil
	}

	conn, err := this.clusterConn.Dial(nodeId)
	if err != nil {
		return nil, err
	}

	this.searchClientsMu.Lock()
	defer this.searchClientsMu.Unlock()

	this.searchClients[nodeId] = pb.NewSearchClient(conn)
	return this.searchClients[nodeId], nil
}

func (this *Dataset) getCachedNodeSearchClient(nodeId uint64) pb.SearchClient {
	this.searchClientsMu.RLock()
	defer this.searchClientsMu.RUnlock()

	if client, exists := this.searchClients[nodeId]; exists {
		return client
	}
	return nil
}