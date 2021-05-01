package storage

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/marekgalovic/anndb/cluster"
	pb "github.com/marekgalovic/anndb/protobuf"
	"github.com/marekgalovic/anndb/storage/raft"
	"github.com/marekgalovic/anndb/utils"

	badger "github.com/dgraph-io/badger/v2"
	"github.com/golang/protobuf/proto"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

var (
	DatasetNotFoundErr      error = errors.New("Dataset not found")
	DatasetAlreadyExistsErr error = errors.New("Dataset already exists")
)

type DatasetManager struct {
	raft          raft.Group
	raftWalDB     *badger.DB
	raftTransport *raft.RaftTransport
	clusterConn   *cluster.Conn
	allocator     *Allocator

	datasets   map[uuid.UUID]*Dataset
	datasetsMu *sync.RWMutex

	notificator *utils.Notificator
}

func NewDatasetManager(raft raft.Group, raftWalDB *badger.DB, raftTransport *raft.RaftTransport, clusterConn *cluster.Conn, allocator *Allocator) (*DatasetManager, error) {
	dm := &DatasetManager{
		raft:          raft,
		raftWalDB:     raftWalDB,
		raftTransport: raftTransport,
		clusterConn:   clusterConn,
		allocator:     allocator,

		datasets:   make(map[uuid.UUID]*Dataset),
		datasetsMu: &sync.RWMutex{},

		notificator: utils.NewNotificator(),
	}

	if err := raft.RegisterProcessFn(dm.process); err != nil {
		return nil, err
	}
	if err := raft.RegisterProcessSnapshotFn(dm.processSnapshot); err != nil {
		return nil, err
	}
	if err := raft.RegisterSnapshotFn(dm.snapshot); err != nil {
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

func (this *DatasetManager) List(ctx context.Context, withSize bool) ([]*pb.Dataset, error) {
	this.datasetsMu.RLock()
	defer this.datasetsMu.RUnlock()

	i := 0
	result := make([]*pb.Dataset, len(this.datasets))
	for _, dataset := range this.datasets {
		result[i] = dataset.Meta()
		if withSize {
			size, err := dataset.Len(ctx)
			if err != nil {
				return nil, err
			}
			result[i].Size = size
		}
		i++
	}

	return result, nil
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
	ctx, cancelCtx := context.WithTimeout(ctx, 1*time.Second)
	defer cancelCtx()

	id := uuid.NewV4()
	dataset.Id = id.Bytes()
	dataset.Partitions = make([]*pb.Partition, dataset.GetPartitionCount())
	partitionsNodeIds := this.allocator.getPartitionsNodeIds(uint(dataset.GetPartitionCount()), uint(dataset.GetReplicationFactor()))
	for i := 0; i < int(dataset.GetPartitionCount()); i++ {
		dataset.Partitions[i] = &pb.Partition{
			Id:      uuid.NewV4().Bytes(),
			NodeIds: partitionsNodeIds[i],
		}
	}

	datasetData, err := proto.Marshal(dataset)
	if err != nil {
		return nil, err
	}

	notifC, notifId := this.notificator.Create(0)
	defer func() { this.notificator.Remove(notifId) }()

	proposal := &pb.DatasetManagerChange{
		Type:           pb.DatasetManagerChangeType_DatasetManagerCreateDataset,
		NotificationId: notifId.Bytes(),
		Data:           datasetData,
	}
	proposalData, err := proto.Marshal(proposal)
	if err != nil {
		return nil, err
	}

	if err := this.raft.Propose(ctx, proposalData); err != nil {
		return nil, err
	}

	select {
	case err := <-notifC:
		if err != nil {
			return nil, err.(error)
		}
		return this.Get(id)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (this *DatasetManager) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, cancelCtx := context.WithTimeout(ctx, 1*time.Second)
	defer cancelCtx()

	notifC, notifId := this.notificator.Create(0)
	defer func() { this.notificator.Remove(notifId) }()

	proposal := &pb.DatasetManagerChange{
		Type:           pb.DatasetManagerChangeType_DatasetManagerDeleteDataset,
		NotificationId: notifId.Bytes(),
		Data:           id.Bytes(),
	}
	proposalData, err := proto.Marshal(proposal)
	if err != nil {
		return err
	}

	if err := this.raft.Propose(ctx, proposalData); err != nil {
		return err
	}

	select {
	case err := <-notifC:
		if err != nil {
			return err.(error)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (this *DatasetManager) createDataset(notificationId uuid.UUID, data []byte) error {
	var dataset pb.Dataset
	if err := proto.Unmarshal(data, &dataset); err != nil {
		return err
	}
	id, err := uuid.FromBytes(dataset.GetId())
	if err != nil {
		return err
	}

	this.datasetsMu.Lock()
	defer this.datasetsMu.Unlock()
	if _, exists := this.datasets[id]; exists {
		this.notificator.Notify(notificationId, DatasetAlreadyExistsErr, false)
		return nil
	}

	this.datasets[id], err = newDataset(id, dataset, this.raftWalDB, this.raftTransport, this.clusterConn, this)
	if err != nil {
		this.notificator.Notify(notificationId, err, false)
		return nil
	}
	log.Infof("Create dataset: %s", id)
	for _, partition := range this.datasets[id].partitions {
		this.allocator.watch(partition)
	}
	this.notificator.Notify(notificationId, nil, false)
	return nil
}

func (this *DatasetManager) deleteDataset(notificationId uuid.UUID, data []byte) error {
	id, err := uuid.FromBytes(data)
	if err != nil {
		return err
	}

	this.datasetsMu.Lock()
	defer this.datasetsMu.Unlock()
	dataset, exists := this.datasets[id]

	if !exists {
		this.notificator.Notify(notificationId, DatasetNotFoundErr, false)
		return nil
	}

	for _, partition := range dataset.partitions {
		this.allocator.unwatch(partition.id)
	}

	delete(this.datasets, id)
	this.notificator.Notify(notificationId, nil, false)
	return nil
}

func (this *DatasetManager) process(data []byte) error {
	var change pb.DatasetManagerChange
	if err := proto.Unmarshal(data, &change); err != nil {
		return err
	}

	notificationId, err := uuid.FromBytes(change.GetNotificationId())
	if err != nil {
		return err
	}

	switch change.Type {
	case pb.DatasetManagerChangeType_DatasetManagerCreateDataset:
		return this.createDataset(notificationId, change.Data)
	case pb.DatasetManagerChangeType_DatasetManagerDeleteDataset:
		return this.deleteDataset(notificationId, change.Data)
	case pb.DatasetManagerChangeType_DatasetManagerUpdatePartitionNodes:
		return this.updatePartitionNodes(notificationId, change.Data)
	}
	return nil
}

func (this *DatasetManager) processSnapshot(data []byte) error {
	this.datasetsMu.Lock()
	defer this.datasetsMu.Unlock()

	var dmSnapshot pb.DatasetManagerSnapshot
	if err := proto.Unmarshal(data, &dmSnapshot); err != nil {
		return err
	}

	for _, dataset := range dmSnapshot.Datasets {
		id, err := uuid.FromBytes(dataset.GetId())
		if err != nil {
			return err
		}
		if _, exists := this.datasets[id]; !exists {
			this.datasets[id], err = newDataset(id, *dataset, this.raftWalDB, this.raftTransport, this.clusterConn, this)
			if err != nil {
				return err
			}
			for _, partition := range this.datasets[id].partitions {
				this.allocator.watch(partition)
			}
		}
	}
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

func (this *DatasetManager) addPartitionNode(ctx context.Context, datasetId uuid.UUID, partitionId uuid.UUID, nodeId uint64) error {
	return this.proposePartitionNodesChangeAndWaitForCommit(ctx, datasetId, partitionId, nodeId, pb.DatasetPartitionNodesChangeType_DatasetPartitionNodesChangeAddNode)
}

func (this *DatasetManager) removePartitionNode(ctx context.Context, datasetId uuid.UUID, partitionId uuid.UUID, nodeId uint64) error {
	return this.proposePartitionNodesChangeAndWaitForCommit(ctx, datasetId, partitionId, nodeId, pb.DatasetPartitionNodesChangeType_DatasetPartitionNodesChangeRemoveNode)
}

func (this *DatasetManager) proposePartitionNodesChangeAndWaitForCommit(ctx context.Context, datasetId uuid.UUID, partitionId uuid.UUID, nodeId uint64, changeType pb.DatasetPartitionNodesChangeType) error {
	change := &pb.DatasetPartitionNodesChange{
		Type:        changeType,
		DatasetId:   datasetId.Bytes(),
		PartitionId: partitionId.Bytes(),
		NodeId:      nodeId,
	}
	changeData, err := proto.Marshal(change)
	if err != nil {
		return err
	}

	notifC, notifId := this.notificator.Create(0)
	defer func() { this.notificator.Remove(notifId) }()

	proposal := &pb.DatasetManagerChange{
		Type:           pb.DatasetManagerChangeType_DatasetManagerUpdatePartitionNodes,
		NotificationId: notifId.Bytes(),
		Data:           changeData,
	}
	proposalData, err := proto.Marshal(proposal)
	if err != nil {
		return err
	}
	if err := this.raft.Propose(ctx, proposalData); err != nil {
		return err
	}

	select {
	case err := <-notifC:
		if err != nil {
			return err.(error)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (this *DatasetManager) updatePartitionNodes(notificationId uuid.UUID, data []byte) error {
	var change pb.DatasetPartitionNodesChange
	if err := proto.Unmarshal(data, &change); err != nil {
		return err
	}

	datasetId, err := uuid.FromBytes(change.GetDatasetId())
	if err != nil {
		this.notificator.Notify(notificationId, err, false)
		return nil
	}

	partitionId, err := uuid.FromBytes(change.GetPartitionId())
	if err != nil {
		this.notificator.Notify(notificationId, err, false)
		return nil
	}

	dataset, err := this.Get(datasetId)
	if err != nil {
		this.notificator.Notify(notificationId, err, false)
		return nil
	}

	partition, err := dataset.getPartition(partitionId)
	if err != nil {
		this.notificator.Notify(notificationId, err, false)
		return nil
	}

	switch change.Type {
	case pb.DatasetPartitionNodesChangeType_DatasetPartitionNodesChangeAddNode:
		partition.addNode(change.GetNodeId())
	case pb.DatasetPartitionNodesChangeType_DatasetPartitionNodesChangeRemoveNode:
		partition.removeNode(change.GetNodeId())
	}

	this.notificator.Notify(notificationId, nil, false)
	return nil
}
