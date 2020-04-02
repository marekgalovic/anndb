package storage

import (
	"errors";
	"context";
	"sync";
	"time";
	"math/rand"

	pb "github.com/marekgalovic/anndb/pkg/protobuf";
	"github.com/marekgalovic/anndb/pkg/cluster";
	"github.com/marekgalovic/anndb/pkg/storage/raft";
	"github.com/marekgalovic/anndb/pkg/utils";

	"github.com/satori/go.uuid";
	"github.com/golang/protobuf/proto";
	badger "github.com/dgraph-io/badger/v2";
)

var (
	DatasetNotFoundErr error = errors.New("Dataset not found")
	DatasetAlreadyExistsErr error = errors.New("Dataset already exists")
)

type DatasetManager struct {
	zeroGroup *raft.RaftGroup
	raftWalDB *badger.DB
	raftTransport *raft.RaftTransport
	clusterConn *cluster.Conn

	datasets map[uuid.UUID]*Dataset
	datasetsMu *sync.RWMutex

	createdNotifications *utils.Notificator
	deletedNotifications *utils.Notificator
}

func NewDatasetManager(zeroGroup *raft.RaftGroup, raftWalDB *badger.DB, raftTransport *raft.RaftTransport, clusterConn *cluster.Conn) (*DatasetManager, error) {
	dm := &DatasetManager {
		zeroGroup: zeroGroup,
		raftWalDB: raftWalDB,
		raftTransport: raftTransport,
		clusterConn: clusterConn,

		datasets: make(map[uuid.UUID]*Dataset),
		datasetsMu: &sync.RWMutex{},

		createdNotifications: utils.NewNotificator(),
		deletedNotifications: utils.NewNotificator(),
	}

	if err := zeroGroup.RegisterProcessFn(dm.process); err != nil {
		return nil, err
	}
	if err := zeroGroup.RegisterProcessSnapshotFn(dm.processSnapshot); err != nil {
		return nil, err
	}
	if err := zeroGroup.RegisterSnapshotFn(dm.snapshot); err != nil {
		return nil, err
	}

	return dm, nil
}

func (this *DatasetManager) Close() {
	this.datasetsMu.Lock()
	defer this.datasetsMu.Unlock()

	for _, dataset := range this.datasets {
		dataset.close()
	}
}

func (this *DatasetManager) List() []*pb.Dataset {
	this.datasetsMu.RLock()
	defer this.datasetsMu.RUnlock()

	i := 0
	result := make([]*pb.Dataset, len(this.datasets))
	for _, dataset := range this.datasets {
		result[i] = dataset.Meta()
		i++
	}

	return nil
}

func (this *DatasetManager) Get(id uuid.UUID) (*Dataset, error) {
	this.datasetsMu.RLock()
	defer this.datasetsMu.RUnlock()

	if dataset, exists := this.datasets[id]; exists {
		return dataset, nil
	}

	return nil, DatasetNotFoundErr
}

func (this *DatasetManager) Create(ctx context.Context, dataset *pb.Dataset) (*Dataset, error) {
	ctx, cancelCtx := context.WithTimeout(ctx, 1 * time.Second)
	defer cancelCtx()

	id := uuid.NewV4()
	dataset.Id = id.Bytes()
	dataset.Partitions = make([]*pb.Partition, dataset.GetPartitionCount())
	partitionsNodeIds := this.getPartitionsNodeIds(this.clusterConn.NodeIds(), uint(dataset.GetPartitionCount()), uint(dataset.GetReplicationFactor()))
	for i := 0; i < int(dataset.GetPartitionCount()); i++ {
		dataset.Partitions[i] = &pb.Partition {
			Id: uuid.NewV4().Bytes(),
			NodeIds: partitionsNodeIds[i],
		}
	}

	proposal := &pb.DatasetsChange {
		Type: pb.DatasetsChangeType_CreateDataset,
		Dataset: dataset,
	}
	proposalData, err := proto.Marshal(proposal)
	if err != nil {
		return nil, err
	}

	notif := this.createdNotifications.Create(id)
	defer func() { this.createdNotifications.Remove(id) }()

	if err := this.zeroGroup.Propose(ctx, proposalData); err != nil {
		return nil, err
	}

	select {
	case err := <- notif:
		if err != nil {
			return nil, err.(error)
		}
		return this.Get(id)
	case <- ctx.Done():
		return nil, ctx.Err()
	}
}

func (this *DatasetManager) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, cancelCtx := context.WithTimeout(ctx, 1 * time.Second)
	defer cancelCtx()

	proposal := &pb.DatasetsChange {
		Type: pb.DatasetsChangeType_DeleteDataset,
		Dataset: &pb.Dataset {Id: id.Bytes()},
	}
	proposalData, err := proto.Marshal(proposal)
	if err != nil {
		return err
	}

	notif := this.deletedNotifications.Create(id)
	defer func() { this.deletedNotifications.Remove(id) }()

	if err := this.zeroGroup.Propose(ctx, proposalData); err != nil {
		return err
	}

	select {
	case err := <- notif:
		if err != nil {
			return err.(error)
		}
		return nil
	case <- ctx.Done():
		return ctx.Err()
	}
}

func (this *DatasetManager) createDataset(dataset *pb.Dataset) error {
	id, err := uuid.FromBytes(dataset.GetId())
	if err != nil {
		return err
	}

	this.datasetsMu.Lock()
	defer this.datasetsMu.Unlock()
	if _, exists := this.datasets[id]; exists {
		this.createdNotifications.Notify(id, DatasetAlreadyExistsErr)
		return nil
	}

	this.datasets[id], err = newDataset(dataset, this.raftWalDB, this.raftTransport, this.clusterConn)
	if err != nil {
		this.createdNotifications.Notify(id, err)
		return nil
	}
	this.createdNotifications.Notify(id, nil)
	return nil
}

func (this *DatasetManager) deleteDataset(datasetProto *pb.Dataset) error {
	id, err := uuid.FromBytes(datasetProto.GetId())
	if err != nil {
		return err
	}

	this.datasetsMu.Lock()
	defer this.datasetsMu.Unlock()
	dataset, exists := this.datasets[id]

	if !exists {
		this.deletedNotifications.Notify(id, DatasetNotFoundErr)
		return nil
	}

	if err := dataset.deleteData(); err != nil {
		this.deletedNotifications.Notify(id, err)
		return nil
	}

	delete(this.datasets, id)
	this.deletedNotifications.Notify(id, nil)
	return nil
}

func (this *DatasetManager) getPartitionsNodeIds(nodeIds []uint64, partitionCount uint, replicationFactor uint) [][]uint64 {
	partitionsNodeIds := make([][]uint64, partitionCount)
	for i := 0; i < int(partitionCount); i++ {
		rand.Shuffle(len(nodeIds), func(i, j int) {
			nodeIds[i], nodeIds[j] = nodeIds[j], nodeIds[i]
		})
		
		partitionsNodeIds[i] = nodeIds[:replicationFactor]
	}

	return partitionsNodeIds
}

func (this *DatasetManager) process(data []byte) error {
	var change pb.DatasetsChange
	if err := proto.Unmarshal(data, &change); err != nil {
		return err
	}

	switch change.Type {
	case pb.DatasetsChangeType_CreateDataset:
		return this.createDataset(change.Dataset)
	case pb.DatasetsChangeType_DeleteDataset:
		return this.deleteDataset(change.Dataset)
	}
	return nil
}

func (this *DatasetManager) processSnapshot(data []byte) error {
	return nil
}

func (this *DatasetManager) snapshot() ([]byte, error) {
	this.datasetsMu.RLock()
	defer this.datasetsMu.RUnlock()

	i := 0
	datasets := make([]*pb.Dataset, len(this.datasets))
	for _, dataset := range this.datasets {
		datasets[i] = dataset.Meta()
		i++
	}

	return proto.Marshal(&pb.DatasetManagerSnapshot{Datasets: datasets})
}