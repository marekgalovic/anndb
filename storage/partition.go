package storage

import (
	"context";
	"time";
	"bytes";
	"errors";
	"sync";
	"math/rand";

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

const proposalTimeout time.Duration = 5 * time.Second

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

	notificator *utils.Notificator

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
		notificator: utils.NewNotificator(),
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

func (this *partition) len() int {
	return this.index.Len()
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

func (this *partition) insert(ctx context.Context, id uint64, value math.Vector, metadata index.Metadata) error {
	this.raftMu.RLock()
	defer this.raftMu.RUnlock()
	if this.raft == nil {
		return RaftNotLoadedOnNodeErr
	}

	proposal := &pb.PartitionChange {
		Type: pb.PartitionChangeType_PartitionChangeInsertValue,
		Id: id,
		Value: value,
		Metadata: metadata,
		Level: int32(this.index.RandomLevel()),
	}

	res, err := this.proposeAndWaitForCommit(ctx, proposal)
	if err != nil {
		return err
	}
	if res != nil {
		return res.(error)
	}
	return nil
}

func (this *partition) update(ctx context.Context, id uint64, value math.Vector, metadata index.Metadata) error {
	this.raftMu.RLock()
	defer this.raftMu.RUnlock()
	if this.raft == nil {
		return RaftNotLoadedOnNodeErr
	}

	proposal := &pb.PartitionChange {
		Type: pb.PartitionChangeType_PartitionChangeUpdateValue,
		Id: id,
		Value: value,
		Metadata: metadata,
	}

	res, err := this.proposeAndWaitForCommit(ctx, proposal)
	if err != nil {
		return err
	}
	if res != nil {
		return res.(error)
	}
	return nil
}

func (this *partition) remove(ctx context.Context, id uint64) error {
	this.raftMu.RLock()
	defer this.raftMu.RUnlock()
	if this.raft == nil {
		return RaftNotLoadedOnNodeErr
	}

	proposal := &pb.PartitionChange {
		Type: pb.PartitionChangeType_PartitionChangeDeleteValue,
		Id: id,
	}
	
	res, err := this.proposeAndWaitForCommit(ctx, proposal)
	if err != nil {
		return err
	}
	if res != nil {
		return res.(error)
	}
	return nil
}

func (this *partition) batchInsert(ctx context.Context, items []*pb.BatchItem) (map[uint64]error, error) {
	this.raftMu.RLock()
	defer this.raftMu.RUnlock()
	if this.raft == nil {
		return nil, RaftNotLoadedOnNodeErr
	}

	for _, item := range items {
		item.Level = int32(this.index.RandomLevel())
	}

	proposal := &pb.PartitionChange {
		Type: pb.PartitionChangeType_PartitionChangeBatchInsertValue,
		BatchItems: items,
	}

	res, err := this.proposeAndWaitForCommit(ctx, proposal)
	if err != nil {
		return nil, err
	}
	return res.(partitionBatchResult), nil
}

func (this *partition) batchUpdate(ctx context.Context, items []*pb.BatchItem) (map[uint64]error, error) {
	this.raftMu.RLock()
	defer this.raftMu.RUnlock()
	if this.raft == nil {
		return nil, RaftNotLoadedOnNodeErr
	}

	proposal := &pb.PartitionChange {
		Type: pb.PartitionChangeType_PartitionChangeBatchUpdateValue,
		BatchItems: items,
	}
	
	res, err := this.proposeAndWaitForCommit(ctx, proposal)
	if err != nil {
		return nil, err
	}
	return res.(partitionBatchResult), nil
}

func (this *partition) batchRemove(ctx context.Context, items []*pb.BatchItem) (map[uint64]error, error) {
	this.raftMu.RLock()
	defer this.raftMu.RUnlock()
	if this.raft == nil {
		return nil, RaftNotLoadedOnNodeErr
	}

	proposal := &pb.PartitionChange {
		Type: pb.PartitionChangeType_PartitionChangeBatchDeleteValue,
		BatchItems: items,
	}
	
	res, err := this.proposeAndWaitForCommit(ctx, proposal)
	if err != nil {
		return nil, err
	}
	return res.(partitionBatchResult), nil
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

func (this *partition) proposeAndWaitForCommit(ctx context.Context, proposal *pb.PartitionChange) (interface{}, error) {
	ctx, cancelCtx := context.WithTimeout(ctx, proposalTimeout)
	defer cancelCtx()

	notifC, notifId := this.notificator.Create()
	defer func() { this.notificator.Remove(notifId) }()

	proposal.NotificationId = notifId.Bytes()
	proposalData, err := proto.Marshal(proposal)
	if err != nil {
		return nil, err
	}

	if err := this.raft.Propose(ctx, proposalData); err != nil {
		return nil, err
	}

	select {
	case res := <- notifC:
		return res, nil
	case <- ctx.Done():
		return nil, ctx.Err()
	}
}

func (this *partition) insertValue(notificationId uuid.UUID, id uint64, value math.Vector, metadata index.Metadata, level int) error {
	err := this.index.Insert(id, value, metadata, level);
	this.notificator.Notify(notificationId, err)
	return nil
}

func (this *partition) updateValue(notificationId uuid.UUID, id uint64, value math.Vector, metadata index.Metadata) error {
	vertex, err := this.index.GetVertex(id)
	if err != nil {
		this.notificator.Notify(notificationId, err)
		return nil
	}
	if err := this.index.Remove(id); err != nil {
		this.notificator.Notify(notificationId, err)
		return nil
	}
	for k, v := range vertex.Metadata() {
		if _, exists := metadata[k]; !exists {
			metadata[k] = v
		}
	}
	err = this.index.Insert(id, value, metadata, vertex.Level())
	this.notificator.Notify(notificationId, err)
	return nil
}

func (this *partition) deleteValue(notificationId uuid.UUID, id uint64) error {
	err := this.index.Remove(id);
	this.notificator.Notify(notificationId, err)
	return nil
}

func (this *partition) batchInsertValue(notificationId uuid.UUID, items []*pb.BatchItem) error {
	errors := make(partitionBatchResult)
	for _, item := range items {
		if err := this.index.Insert(item.GetId(), item.GetValue(), item.GetMetadata(), int(item.GetLevel())); err != nil {
			errors[item.GetId()] = err
		}
	}
	this.notificator.Notify(notificationId, errors)
	return nil
}

func (this *partition) batchUpdateValue(notificationId uuid.UUID, items []*pb.BatchItem) error {
	errors := make(partitionBatchResult)
	for _, item := range items {
		vertex, err := this.index.GetVertex(item.GetId())
		if err != nil {
			errors[item.GetId()] = err
			continue
		}
		if err := this.index.Remove(item.GetId()); err != nil {
			errors[item.GetId()] = err
			continue
		}
		metadata := item.GetMetadata()
		for k, v := range vertex.Metadata() {
			if _, exists := metadata[k]; !exists {
				metadata[k] = v
			}
		}
		if err := this.index.Insert(item.GetId(), item.GetValue(), metadata, vertex.Level()); err != nil {
			errors[item.GetId()] = err
		}
	}
	this.notificator.Notify(notificationId, errors)
	return nil
}

func (this *partition) batchDeleteValue(notificationId uuid.UUID, items []*pb.BatchItem) error {
	errors := make(partitionBatchResult)
	for _, item := range items {
		if err := this.index.Remove(item.GetId()); err != nil {
			errors[item.GetId()] = err
		}
	}
	this.notificator.Notify(notificationId, errors)
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
	case pb.PartitionChangeType_PartitionChangeInsertValue:
		return this.insertValue(notificationId, change.GetId(), change.GetValue(), change.GetMetadata(), int(change.GetLevel()))
	case pb.PartitionChangeType_PartitionChangeUpdateValue:
		return this.updateValue(notificationId, change.GetId(), change.GetValue(), change.GetMetadata())
	case pb.PartitionChangeType_PartitionChangeDeleteValue:
		return this.deleteValue(notificationId, change.GetId())
	case pb.PartitionChangeType_PartitionChangeBatchInsertValue:
		return this.batchInsertValue(notificationId, change.GetBatchItems())
	case pb.PartitionChangeType_PartitionChangeBatchUpdateValue:
		return this.batchUpdateValue(notificationId, change.GetBatchItems())
	case pb.PartitionChangeType_PartitionChangeBatchDeleteValue:
		return this.batchDeleteValue(notificationId, change.GetBatchItems())
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

func (this *partition) isOnNode(nodeId uint64) bool {
	for _, id := range this.nodeIds() {
		if id == nodeId {
			return true
		}
	}
	return false
}

func (this *partition) randomNodeId() uint64 {
	nodeIds := this.nodeIds()
	return nodeIds[rand.Intn(len(nodeIds))]
}