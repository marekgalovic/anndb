package storage

import (
	"errors";
	"context";
	"sync";

	pb "github.com/marekgalovic/anndb/pkg/protobuf";
	"github.com/marekgalovic/anndb/pkg/index";
	"github.com/marekgalovic/anndb/pkg/math";
	"github.com/marekgalovic/anndb/pkg/storage/raft";
	
	"github.com/satori/go.uuid";
	badger "github.com/dgraph-io/badger/v2";
)

var (
	DimensionMissmatchErr error = errors.New("Value dimension does not match dataset dimension")
)

type Dataset struct {
	meta *pb.Dataset

	partitions []*partition
	partitionsMu *sync.RWMutex
}

func newDataset(meta *pb.Dataset, raftWalDB *badger.DB, raftTransport *raft.RaftTransport) (*Dataset, error) {
	d := &Dataset {
		meta: meta,
		partitions: make([]*partition, meta.GetPartitionCount()),
	}

	for i := 0; i < int(meta.GetPartitionCount()); i++ {
		pid, err := uuid.FromBytes(meta.Partitions[i].GetId())
		if err != nil {
			return nil, err
		}
		d.partitions[i] = newPartition(pid, meta.Partitions[i].GetNodeIds(), d, raftWalDB, raftTransport)
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
	return nil, nil
}

func (this *Dataset) getPartitionForId(id uint64) *partition {
	return this.partitions[id % uint64(this.Meta().GetPartitionCount())]
}

func (this *Dataset) checkDimension(value *math.Vector) error {
	if uint32(len(*value)) != this.Meta().GetDimension() {
		return DimensionMissmatchErr
	}
	return nil
}