package storage

import (
	"errors";
	"context";

	pb "github.com/marekgalovic/anndb/pkg/protobuf";
	"github.com/marekgalovic/anndb/pkg/index";
	"github.com/marekgalovic/anndb/pkg/math";
	
	"github.com/satori/go.uuid";
)

var (
	DimensionMissmatchErr error = errors.New("Value dimension does not match dataset dimension")
)

type Dataset struct {
	meta *pb.Dataset

	partitions []*partition
}

func newDataset(meta *pb.Dataset) (*Dataset, error) {
	d := &Dataset {
		meta: meta,
		partitions: make([]*partition, meta.GetPartitionCount()),
	}

	for i := 0; i < int(meta.GetPartitionCount()); i++ {
		pid, err := uuid.FromBytes(meta.Partitions[i].Id)
		if err != nil {
			return nil, err
		}
		d.partitions[i] = newPartition(pid, d)
	}

	return d, nil
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