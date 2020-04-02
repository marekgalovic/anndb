package storage

import (
	"context";
	"time";
	"bytes";

	pb "github.com/marekgalovic/anndb/pkg/protobuf";
	"github.com/marekgalovic/anndb/pkg/math";
	"github.com/marekgalovic/anndb/pkg/index";
	"github.com/marekgalovic/anndb/pkg/index/space";
	"github.com/marekgalovic/anndb/pkg/storage/raft";
	"github.com/marekgalovic/anndb/pkg/storage/wal";
	"github.com/marekgalovic/anndb/pkg/utils";

	"github.com/satori/go.uuid";
	"github.com/golang/protobuf/proto";
	badger "github.com/dgraph-io/badger/v2";
	log "github.com/sirupsen/logrus";
)

type partition struct {
	id uuid.UUID
	servingNodeIds []uint64
	dataset *Dataset
	index *index.Hnsw

	raft *raft.RaftGroup
	wal wal.WAL

	insertNotifications *utils.Notificator
	deleteNotifications *utils.Notificator
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

func newPartition(id uuid.UUID, nodeIds []uint64, dataset *Dataset, raftWalDB *badger.DB, raftTransport *raft.RaftTransport) *partition {
	wal := wal.NewBadgerWAL(raftWalDB, id)
	raft, err := raft.NewRaftGroup(id, nodeIds, wal, raftTransport)
	if err != nil {
		log.Fatal(err)
	}

	p := &partition {
		id: id,
		servingNodeIds: nodeIds,
		dataset: dataset,
		index: newIndexFromDatasetProto(dataset.Meta()),
		raft: raft,
		wal: wal,
		insertNotifications: utils.NewNotificator(),
		deleteNotifications: utils.NewNotificator(),
	}

	if err := raft.RegisterProcessFn(p.process); err != nil {
		log.Fatal(err)
	}
	if err := raft.RegisterProcessSnapshotFn(p.processSnapshot); err != nil {
		log.Fatal(err)
	}
	if err := raft.RegisterSnapshotFn(p.snapshot); err != nil {
		log.Fatal(err)
	}

	return p
}

func (this *partition) close() {
	this.raft.Stop()
}

func (this *partition) deleteData() error {
	this.raft.Stop()
	return this.wal.DeleteGroup()
}

func (this *partition) nodeIds() []uint64 {
	return this.servingNodeIds
}

func (this *partition) insert(ctx context.Context, id uint64, value math.Vector) error {
	ctx, cancelCtx := context.WithTimeout(ctx, 1 * time.Second)
	defer cancelCtx()

	proposal := &pb.PartitionChange {
		Type: pb.PartitionChangeType_InsertValue,
		Id: id,
		Value: value,
		Level: int32(this.index.RandomLevel()),
	}
	proposalData, err := proto.Marshal(proposal)
	if err != nil {
		return err
	}

	notif := this.insertNotifications.Create(id)
	defer func() { this.insertNotifications.Remove(id) }()

	if err := this.raft.Propose(ctx, proposalData); err != nil {
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

func (this *partition) remove(ctx context.Context, id uint64) error {
	ctx, cancelCtx := context.WithTimeout(ctx, 1 * time.Second)
	defer cancelCtx()

	proposal := &pb.PartitionChange {
		Type: pb.PartitionChangeType_DeleteValue,
		Id: id,
	}
	proposalData, err := proto.Marshal(proposal)
	if err != nil {
		return err
	}

	notif := this.deleteNotifications.Create(id)
	defer func() { this.deleteNotifications.Remove(id) }()

	if err := this.raft.Propose(ctx, proposalData); err != nil {
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

func (this *partition) search(ctx context.Context, query []float32, k uint) (index.SearchResult, error) {
	return this.index.Search(ctx, query, k)
}

func (this *partition) insertValue(id uint64, value []float32, level int) error {
	err := this.index.Insert(id, value, level);
	this.insertNotifications.Notify(id, err)
	return nil
}

func (this *partition) deleteValue(id uint64) error {
	err := this.index.Remove(id);
	this.deleteNotifications.Notify(id, err)
	return nil
}

func (this *partition) process(data []byte) error {
	var change pb.PartitionChange
	if err := proto.Unmarshal(data, &change); err != nil {
		return err
	}

	switch change.Type {
	case pb.PartitionChangeType_InsertValue:
		return this.insertValue(change.GetId(), change.GetValue(), int(change.GetLevel()))
	case pb.PartitionChangeType_DeleteValue:
		return this.deleteValue(change.GetId())
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