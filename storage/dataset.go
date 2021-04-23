package storage

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/marekgalovic/anndb/cluster"
	"github.com/marekgalovic/anndb/index"
	"github.com/marekgalovic/anndb/math"
	pb "github.com/marekgalovic/anndb/protobuf"
	"github.com/marekgalovic/anndb/storage/raft"
	"github.com/marekgalovic/anndb/utils"

	badger "github.com/dgraph-io/badger/v2"
	uuid "github.com/satori/go.uuid"
)

const maxBatchRequestSize int = 100

var (
	DimensionMissmatchErr    error = errors.New("Value dimension does not match dataset dimension")
	PartitionNotFoundErr     error = errors.New("Partition not found")
	PartitionNotOnNodeErr    error = errors.New("Partition is not loaded on the node")
	BatchRequestTooLargerErr error = errors.New("Batch request too large")
)

type partitionBatchResult map[uuid.UUID]error
type partitionBatchRequestRemoteFn func(pb.DataManagerClient, context.Context, *pb.PartitionBatchRequest) (*pb.BatchResponse, error)
type partitionBatchRequestLocalFn func(*partition, context.Context, []*pb.BatchItem) (map[uuid.UUID]error, error)

type Dataset struct {
	id          uuid.UUID
	len         int
	meta        *pb.Dataset
	clusterConn *cluster.Conn

	partitions    []*partition
	partitionsMap map[uuid.UUID]*partition
	partitionsMu  *sync.RWMutex

	searchClients        map[uint64]pb.SearchClient
	searchClientsMu      *sync.RWMutex
	dataManagerClients   map[uint64]pb.DataManagerClient
	dataManagerClientsMu *sync.RWMutex
}

func newDataset(id uuid.UUID, meta pb.Dataset, raftWalDB *badger.DB, raftTransport *raft.RaftTransport, clusterConn *cluster.Conn, datasetManager *DatasetManager) (*Dataset, error) {
	d := &Dataset{
		id:                   id,
		meta:                 &meta,
		clusterConn:          clusterConn,
		partitions:           make([]*partition, meta.GetPartitionCount()),
		partitionsMap:        make(map[uuid.UUID]*partition),
		partitionsMu:         &sync.RWMutex{},
		searchClients:        make(map[uint64]pb.SearchClient),
		searchClientsMu:      &sync.RWMutex{},
		dataManagerClients:   make(map[uint64]pb.DataManagerClient),
		dataManagerClientsMu: &sync.RWMutex{},
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

func (this *Dataset) Len(ctx context.Context) (uint64, error) {
	this.partitionsMu.RLock()
	defer this.partitionsMu.RUnlock()

	var result uint64 = 0
	wg := &sync.WaitGroup{}
	errorCh := make(chan error, len(this.partitions))
	for _, partition := range this.partitions {
		if partition.isOnNode(this.clusterConn.Id()) {
			atomic.AddUint64(&result, uint64(partition.len()))
			errorCh <- nil
		} else {
			wg.Add(1)
			go func(ctx context.Context, wg *sync.WaitGroup, errorCh chan error, result *uint64) {
				defer wg.Done()
				client, err := this.getDataManagerClient(ctx, partition.randomNodeId())
				if err != nil {
					errorCh <- err
					return
				}
				resp, err := client.PartitionLen(ctx, &pb.PartitionInfoRequest{
					DatasetId:   this.id.Bytes(),
					PartitionId: partition.id.Bytes(),
				})
				if err != nil {
					errorCh <- err
					return
				}
				atomic.AddUint64(result, resp.GetLen())
			}(ctx, wg, errorCh, &result)
		}
	}

	go func() {
		wg.Wait()
		close(errorCh)
	}()

	for i := 0; i < len(this.partitions); i++ {
		select {
		case err := <-errorCh:
			if err != nil {
				return 0, err
			}
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}

	return atomic.LoadUint64(&result), nil
}

func (this *Dataset) BytesSize(ctx context.Context) (uint64, error) {
	this.partitionsMu.RLock()
	defer this.partitionsMu.RUnlock()

	var result uint64 = 0
	wg := &sync.WaitGroup{}
	errorCh := make(chan error, len(this.partitions))
	for _, partition := range this.partitions {
		if partition.isOnNode(this.clusterConn.Id()) {
			atomic.AddUint64(&result, uint64(partition.bytesSize()))
			errorCh <- nil
		} else {
			wg.Add(1)
			go func(ctx context.Context, wg *sync.WaitGroup, errorCh chan error, result *uint64) {
				defer wg.Done()
				client, err := this.getDataManagerClient(ctx, partition.randomNodeId())
				if err != nil {
					errorCh <- err
					return
				}
				resp, err := client.PartitionBytesSize(ctx, &pb.PartitionInfoRequest{
					DatasetId:   this.id.Bytes(),
					PartitionId: partition.id.Bytes(),
				})
				if err != nil {
					errorCh <- err
					return
				}
				atomic.AddUint64(result, resp.GetBytesSize())
			}(ctx, wg, errorCh, &result)
		}
	}

	go func() {
		wg.Wait()
		close(errorCh)
	}()

	for i := 0; i < len(this.partitions); i++ {
		select {
		case err := <-errorCh:
			if err != nil {
				return 0, err
			}
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}

	return atomic.LoadUint64(&result), nil
}

func (this *Dataset) PartitionLen(ctx context.Context, partitionId uuid.UUID) (uint64, error) {
	partition, err := this.getPartition(partitionId)
	if err != nil {
		return 0, err
	}

	if !partition.isOnNode(this.clusterConn.Id()) {
		return 0, PartitionNotOnNodeErr
	}

	return uint64(partition.len()), nil
}

func (this *Dataset) PartitionBytesSize(ctx context.Context, partitionId uuid.UUID) (uint64, error) {
	partition, err := this.getPartition(partitionId)
	if err != nil {
		return 0, err
	}

	if !partition.isOnNode(this.clusterConn.Id()) {
		return 0, PartitionNotOnNodeErr
	}

	return uint64(partition.bytesSize()), nil
}

func (this *Dataset) Insert(ctx context.Context, id uuid.UUID, value math.Vector, metadata index.Metadata) error {
	if err := this.checkDimension(&value); err != nil {
		return err
	}

	partition := this.getPartitionForId(id)
	if !partition.isOnNode(this.clusterConn.Id()) {
		// Proxy the request to the partition leader
		client, err := this.getDataManagerClient(ctx, partition.randomNodeId())
		if err != nil {
			return nil
		}
		_, err = client.Insert(ctx, &pb.InsertRequest{
			DatasetId: this.id.Bytes(),
			Id:        id.Bytes(),
			Value:     value,
			Metadata:  metadata,
		})
		return err
	}

	return partition.insert(ctx, id, value, metadata)
}

func (this *Dataset) Update(ctx context.Context, id uuid.UUID, value math.Vector, metadata index.Metadata) error {
	if err := this.checkDimension(&value); err != nil {
		return err
	}

	partition := this.getPartitionForId(id)
	if !partition.isOnNode(this.clusterConn.Id()) {
		// Proxy the request to the partition leader
		client, err := this.getDataManagerClient(ctx, partition.randomNodeId())
		if err != nil {
			return nil
		}
		_, err = client.Update(ctx, &pb.UpdateRequest{
			DatasetId: this.id.Bytes(),
			Id:        id.Bytes(),
			Value:     value,
			Metadata:  metadata,
		})
		return err
	}

	return partition.update(ctx, id, value, metadata)
}

func (this *Dataset) Remove(ctx context.Context, id uuid.UUID) error {
	partition := this.getPartitionForId(id)
	if !partition.isOnNode(this.clusterConn.Id()) {
		// Proxy the request to the partition leader
		client, err := this.getDataManagerClient(ctx, partition.randomNodeId())
		if err != nil {
			return nil
		}
		_, err = client.Remove(ctx, &pb.RemoveRequest{
			DatasetId: this.id.Bytes(),
			Id:        id.Bytes(),
		})
		return err
	}

	return this.getPartitionForId(id).remove(ctx, id)
}

func (this *Dataset) BatchInsert(ctx context.Context, items []*pb.BatchItem) (map[uuid.UUID]error, error) {
	if len(items) > maxBatchRequestSize {
		return nil, BatchRequestTooLargerErr
	}

	errors := make(map[uuid.UUID]error)
	var checkedItems []*pb.BatchItem
	for _, item := range items {
		value := math.Vector(item.GetValue())
		if err := this.checkDimension(&value); err != nil {
			errors[uuid.FromBytesOrNil(item.GetId())] = err
		} else {
			checkedItems = append(checkedItems, item)
		}
	}

	errs, err := this.partitionsBatchRequest(
		ctx, checkedItems,
		func(client pb.DataManagerClient, ctx context.Context, req *pb.PartitionBatchRequest) (*pb.BatchResponse, error) {
			return client.PartitionBatchInsert(ctx, req)
		},
		func(partition *partition, ctx context.Context, items []*pb.BatchItem) (map[uuid.UUID]error, error) {
			return partition.batchInsert(ctx, items)
		},
	)
	if err != nil {
		return nil, err
	}
	for id, err := range errs {
		errors[id] = err
	}
	return errors, nil
}

func (this *Dataset) PartitionBatchInsert(ctx context.Context, partitionId uuid.UUID, items []*pb.BatchItem) (map[uuid.UUID]error, error) {
	partition, err := this.getPartition(partitionId)
	if err != nil {
		return nil, err
	}

	return partition.batchInsert(ctx, items)
}

func (this *Dataset) BatchUpdate(ctx context.Context, items []*pb.BatchItem) (map[uuid.UUID]error, error) {
	if len(items) > maxBatchRequestSize {
		return nil, BatchRequestTooLargerErr
	}

	errors := make(map[uuid.UUID]error)
	var checkedItems []*pb.BatchItem
	for _, item := range items {
		value := math.Vector(item.GetValue())
		if err := this.checkDimension(&value); err != nil {
			errors[uuid.FromBytesOrNil(item.GetId())] = err
		} else {
			checkedItems = append(checkedItems, item)
		}
	}

	errs, err := this.partitionsBatchRequest(
		ctx, checkedItems,
		func(client pb.DataManagerClient, ctx context.Context, req *pb.PartitionBatchRequest) (*pb.BatchResponse, error) {
			return client.PartitionBatchUpdate(ctx, req)
		},
		func(partition *partition, ctx context.Context, items []*pb.BatchItem) (map[uuid.UUID]error, error) {
			return partition.batchUpdate(ctx, items)
		},
	)
	if err != nil {
		return nil, err
	}
	for id, err := range errs {
		errors[id] = err
	}
	return errors, nil
}

func (this *Dataset) PartitionBatchUpdate(ctx context.Context, partitionId uuid.UUID, items []*pb.BatchItem) (map[uuid.UUID]error, error) {
	partition, err := this.getPartition(partitionId)
	if err != nil {
		return nil, err
	}

	return partition.batchUpdate(ctx, items)
}

func (this *Dataset) BatchRemove(ctx context.Context, items []*pb.BatchItem) (map[uuid.UUID]error, error) {
	if len(items) > maxBatchRequestSize {
		return nil, BatchRequestTooLargerErr
	}

	return this.partitionsBatchRequest(
		ctx, items,
		func(client pb.DataManagerClient, ctx context.Context, req *pb.PartitionBatchRequest) (*pb.BatchResponse, error) {
			return client.PartitionBatchRemove(ctx, req)
		},
		func(partition *partition, ctx context.Context, items []*pb.BatchItem) (map[uuid.UUID]error, error) {
			return partition.batchRemove(ctx, items)
		},
	)
}

func (this *Dataset) PartitionBatchRemove(ctx context.Context, partitionId uuid.UUID, items []*pb.BatchItem) (map[uuid.UUID]error, error) {
	partition, err := this.getPartition(partitionId)
	if err != nil {
		return nil, err
	}

	return partition.batchRemove(ctx, items)
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

	result := make(index.SearchResult, 0, int(k)*len(nodePartitions))
	for i := 0; i < len(nodePartitions); i++ {
		select {
		case items := <-resultCh:
			result = append(result, items...)
		case err := <-errorCh:
			return nil, err
		case <-ctx.Done():
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

	result := make(index.SearchResult, 0, int(k)*len(partitions))
	for i := 0; i < len(partitions); i++ {
		select {
		case items := <-resultCh:
			result = append(result, items...)
		case err := <-errorCh:
			return nil, err
		case <-ctx.Done():
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

func (this *Dataset) getPartitionForId(id uuid.UUID) *partition {
	this.partitionsMu.RLock()
	defer this.partitionsMu.RUnlock()

	return this.partitions[utils.UuidMod(id, uint64(this.Meta().GetPartitionCount()))]
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

	request := &pb.SearchPartitionsRequest{
		DatasetId:    this.Meta().GetId(),
		PartitionIds: partitionIdsBytes,
		Query:        query,
		K:            uint32(k),
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
		id, err := uuid.FromBytes(item.GetId())
		if err != nil {
			errorCh <- err
		}

		result = append(result, index.SearchResultItem{
			Id:       id,
			Metadata: item.GetMetadata(),
			Score:    item.GetScore(),
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

func (this *Dataset) groupBatchItemsByPartition(items []*pb.BatchItem) map[*partition][]*pb.BatchItem {
	result := make(map[*partition][]*pb.BatchItem)
	for _, item := range items {
		partition := this.getPartitionForId(uuid.Must(uuid.FromBytes(item.GetId())))
		if _, exists := result[partition]; !exists {
			result[partition] = make([]*pb.BatchItem, 0)
		}
		result[partition] = append(result[partition], item)
	}
	return result
}

func (this *Dataset) partitionsBatchRequest(ctx context.Context, items []*pb.BatchItem, remoteFn partitionBatchRequestRemoteFn, localFn partitionBatchRequestLocalFn) (map[uuid.UUID]error, error) {
	wg := &sync.WaitGroup{}
	resultCh := make(chan partitionBatchResult)

	partitionItems := this.groupBatchItemsByPartition(items)
	for partition, items := range partitionItems {
		wg.Add(1)
		go this.handlePartitionBatchRequest(ctx, partition, items, wg, resultCh, remoteFn, localFn)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	errors := make(map[uuid.UUID]error)
	for i := 0; i < len(partitionItems); i++ {
		select {
		case result := <-resultCh:
			for id, err := range result {
				errors[id] = err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return errors, nil
}

func (this *Dataset) handlePartitionBatchRequest(ctx context.Context, partition *partition, items []*pb.BatchItem, wg *sync.WaitGroup, resultCh chan partitionBatchResult, remoteFn partitionBatchRequestRemoteFn, localFn partitionBatchRequestLocalFn) {
	defer wg.Done()

	if !partition.isOnNode(this.clusterConn.Id()) {
		client, err := this.getDataManagerClient(ctx, partition.randomNodeId())
		if err != nil {
			resultCh <- this.errorToPartitionBatchResult(items, err)
			return
		}

		resp, err := remoteFn(client, ctx, &pb.PartitionBatchRequest{
			DatasetId:   this.id.Bytes(),
			PartitionId: partition.id.Bytes(),
			Items:       items,
		})
		if err != nil {
			resultCh <- this.errorToPartitionBatchResult(items, err)
			return
		}
		resultCh <- this.errorsResponseToPartitionBatchResult(resp.GetErrors())
		return
	}

	errors, err := localFn(partition, ctx, items)
	if err != nil {
		resultCh <- this.errorToPartitionBatchResult(items, err)
		return
	}

	resultCh <- partitionBatchResult(errors)
	return
}

func (this *Dataset) errorToPartitionBatchResult(items []*pb.BatchItem, err error) partitionBatchResult {
	result := make(partitionBatchResult)
	for _, item := range items {
		result[uuid.FromBytesOrNil(item.GetId())] = err
	}
	return result
}

func (this *Dataset) errorsResponseToPartitionBatchResult(errStrings map[string]string) partitionBatchResult {
	result := make(partitionBatchResult)
	for id, errString := range errStrings {
		result[uuid.Must(uuid.FromString(id))] = errors.New(errString)
	}
	return result
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

func (this *Dataset) getDataManagerClient(ctx context.Context, nodeId uint64) (pb.DataManagerClient, error) {
	client := this.getCachedDataManagerClient(nodeId)
	if client != nil {
		return client, nil
	}

	conn, err := this.clusterConn.Dial(nodeId)
	if err != nil {
		return nil, err
	}

	this.dataManagerClientsMu.Lock()
	defer this.dataManagerClientsMu.Unlock()

	this.dataManagerClients[nodeId] = pb.NewDataManagerClient(conn)
	return this.dataManagerClients[nodeId], nil
}

func (this *Dataset) getCachedDataManagerClient(nodeId uint64) pb.DataManagerClient {
	this.dataManagerClientsMu.RLock()
	defer this.dataManagerClientsMu.RUnlock()

	if client, exists := this.dataManagerClients[nodeId]; exists {
		return client
	}
	return nil
}
