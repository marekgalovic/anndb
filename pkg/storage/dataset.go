package storage

import (
	pb "github.com/marekgalovic/anndb/pkg/protobuf";
	
	"github.com/satori/go.uuid";
)

type Dataset struct {
	meta pb.Dataset

	partitions map[uuid.UUID]*partition
}

func newDataset(meta pb.Dataset) *Dataset {
	return &Dataset {
		meta: meta,
		partitions: make(map[uuid.UUID]*partition),
	}
}