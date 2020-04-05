package storage

import (
	"context";
	"time";
	"bytes";
	"errors";
	"sync";

	pb "github.com/marekgalovic/anndb/protobuf";
	"github.com/marekgalovic/anndb/math";
	"github.com/marekgalovic/anndb/index";
	"github.com/marekgalovic/anndb/index/space";
	"github.com/marekgalovic/anndb/storage/raft";
	"github.com/marekgalovic/anndb/storage/wal";
	"github.com/marekgalovic/anndb/utils";

	"github.com/satori/go.uuid";
	"github.com/golang/protobuf/proto";
	badger "github.com/dgraph-io/badger/v2";
	log "github.com/sirupsen/logrus";
)

var (
	RaftNotLoadedOnNodeErr error = errors.New("Raft is not loaded on this node")
)

type partition struct {
	id uuid.UUID
	meta *pb.Partition
	dataset *Dataset
	index *index.Hnsw

	raft *raft.RaftGroup
	wal wal.WAL
	raftTransport *raft.RaftTransport
	datasetManager *DatasetManager
	raftMu *sync.RWMutex

	insertNotifications *utils.Notificator
	deleteNotifications *utils.Notificator

	log *log.Entry
}

func newIndexFromDatasetProto(dataset *pb.Dataset) *index.Hnsw {
	var s space.Space
	switch dataset.GetSpace() {
	case pb.Space_Euclidean:
		s = space.NewEuclidean()
	case pb.Space_Manhattan:
		s = space.NewManhattan()
	case pb.Space_Cosine:
		s = space.NewCosine()
	}

	return index.NewHnsw(uint(dataset.GetDimension()), s) 
}

func newPartition(id uuid.UUID, meta *pb.Partition, dataset *Dataset, raftWalDB *badger.DB, raftTransport *raft.RaftTransport, datasetManager *DatasetManager) *partition {
	p := &partition {
		id: id,
		meta: meta,
		dataset: dataset,
		index: newIndexFromDatasetProto(dataset.Meta()),
		raft: nil,
		wal: wal.NewBadgerWAL(raftWalDB, id),
		raftTransport: raftTransport,
		datasetManager: datasetManager,
		raftMu: &sync.RWMutex{},
		insertNotifications: utils.NewNotificator(),
		deleteNotifications: utils.NewNotificator(),
		log: log.WithFields(log.Fields {
			"partition_id": id,
		}),
	}

	return p
}

func (this *partition) nodeIds() []uint64 {
	return this.meta.GetNodeIds()
}

func (this *partition) close() {
	this.raftMu.RLock()
	defer this.raftMu.RUnlock()

	if this.raft != nil {
		this.raft.Stop()
	}
}

func (this *partition) loadRaft(nodeIds []uint64) error {
	this.raftMu.Lock()
	defer this.raftMu.Unlock()

	var err error
	this.raft, err = raft.NewRaftGroup(this.id, nodeIds, this.wal, this.raftTransport)
	if err != nil {
		return err
	}
	if err := this.raft.RegisterProcessFn(this.process); err != nil {
		return err
	}
	if err := this.raft.RegisterProcessSnapshotFn(this.processSnapshot); err != nil {
		return err
	}
	if err := this.raft.RegisterSnapshotFn(this.snapshot); err != nil {
		return err
	}

	this.log.Info("Loaded Raft")
	return nil
}

func (this *partition) unloadRaft() error {
	this.raftMu.Lock()
	defer this.raftMu.Unlock()
	if this.raft == nil {
		return RaftNotLoadedOnNodeErr
	}

	this.raft.Stop()
	this.wal.DeleteGroup()
	this.raft = nil
	this.log.Info("Unloaded Raft")
	return nil
}

func (this *partition) insert(ctx context.Context, id uint64, value math.Vector) error {
	this.raftMu.RLock()
	defer this.raftMu.RUnlock()
	if this.raft == nil {
		return RaftNotLoadedOnNodeErr
	}

	ctx, cancelCtx := context.WithTimeout(ctx, 1 * time.Second)
	defer cancelCtx()

	notifC, notifId := this.insertNotifications.Create()
	defer func() { this.insertNotifications.Remove(notifId) }()

	proposal := &pb.PartitionChange {
		Type: pb.PartitionChangeType_InsertValue,
		NotificationId: notifId.Bytes(),
		Id: id,
		Value: value,
		Level: int32(this.index.RandomLevel()),
	}
	proposalData, err := proto.Marshal(proposal)
	if err != nil {
		return err
	}

	if err := this.raft.Propose(ctx, proposalData); err != nil {
		return err
	}

	select {
	case err := <- notifC:
		if err != nil {
			return err.(error)
		}
		return nil
	case <- ctx.Done():
		return ctx.Err()
	}
}

func (this *partition) remove(ctx context.Context, id uint64) error {
	ctx, cancelCtx := context.WithTimeout(ctx, 1 * time.Second)
	defer cancelCtx()

	notifC, notifId := this.insertNotifications.Create()
	defer func() { this.insertNotifications.Remove(notifId) }()

	proposal := &pb.PartitionChange {
		Type: pb.PartitionChangeType_DeleteValue,
		NotificationId: notifId.Bytes(),
		Id: id,
	}
	proposalData, err := proto.Marshal(proposal)
	if err != nil {
		return err
	}

	if err := this.raft.Propose(ctx, proposalData); err != nil {
		return err
	}

	select {
	case err := <- notifC:
		if err != nil {
			return err.(error)
		}
		return nil
	case <- ctx.Done():
		return ctx.Err()
	}
}

func (this *partition) search(ctx context.Context, query []float32, k uint) (index.SearchResult, error) {
	return this.index.Search(ctx, query, k)
}

func (this *partition) proposeAddNode(ctx context.Context, nodeId uint64) error {
	if err := this.datasetManager.addPartitionNode(ctx, this.dataset.id, this.id, nodeId); err != nil {
		return err
	}

	return this.raft.ProposeJoin(nodeId, "")
}

func (this *partition) addNode(nodeId uint64) {
	this.meta.NodeIds = append(this.meta.NodeIds, nodeId)

	if nodeId == this.raftTransport.NodeId() {
		this.loadRaft(nil)
	}
}

func (this *partition) proposeRemoveNode(ctx context.Context, nodeId uint64) error {
	if err := this.datasetManager.removePartitionNode(ctx, this.dataset.id, this.id, nodeId); err != nil {
		return err
	}

	return this.raft.ProposeLeave(nodeId)
}

func (this *partition) removeNode(nodeId uint64) {
	newNodeIds := make([]uint64, 0)
	for _, id := range this.meta.GetNodeIds() {
		if id != nodeId {
			newNodeIds = append(newNodeIds, id)
		}
	}
	this.meta.NodeIds = newNodeIds

	if nodeId == this.raftTransport.NodeId() {
		this.unloadRaft()
	}
}

func (this *partition) insertValue(notificationId uuid.UUID, id uint64, value []float32, level int) error {
	err := this.index.Insert(id, value, level);
	this.insertNotifications.Notify(notificationId, err)
	return nil
}

func (this *partition) deleteValue(notificationId uuid.UUID, id uint64) error {
	err := this.index.Remove(id);
	this.deleteNotifications.Notify(notificationId, err)
	return nil
}

func (this *partition) process(data []byte) error {
	var change pb.PartitionChange
	if err := proto.Unmarshal(data, &change); err != nil {
		return err
	}

	notificationId, err := uuid.FromBytes(change.GetNotificationId())
	if err != nil {
		return err
	}

	switch change.Type {
	case pb.PartitionChangeType_InsertValue:
		return this.insertValue(notificationId, change.GetId(), change.GetValue(), int(change.GetLevel()))
	case pb.PartitionChangeType_DeleteValue:
		return this.deleteValue(notificationId, change.GetId())
	}

	return nil
}

func (this *partition) processSnapshot(data []byte) error {
	return this.index.Load(bytes.NewBuffer(data), false)
}

func (this *partition) snapshot() ([]byte, error) {
	var buf bytes.Buffer
	if err := this.index.Save(&buf, false); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (this *partition) isUnderReplicated() bool {
	return len(this.nodeIds()) < int(this.dataset.Meta().GetReplicationFactor())
}