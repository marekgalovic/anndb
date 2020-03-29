package services

import (
	"context";

	pb "github.com/marekgalovic/anndb/pkg/protobuf";
	"github.com/marekgalovic/anndb/pkg/storage";

	"github.com/satori/go.uuid";
)

type datasetManagerServer struct {
	manager *storage.DatasetManager
}

func NewDatasetManagerServer(manager *storage.DatasetManager) *datasetManagerServer {
	return &datasetManagerServer {
		manager: manager,
	}
}

func (this *datasetManagerServer) Get(ctx context.Context, req *pb.UUIDRequest) (*pb.Dataset, error) {
	id, err := uuid.FromBytes(req.GetId())
	if err != nil {
		return nil, err
	}

	dataset, err := this.manager.Get(id)
	if err != nil {
		return nil, err
	}

	return dataset.Meta(), nil
}

func (this *datasetManagerServer) Create(ctx context.Context, req *pb.Dataset) (*pb.Dataset, error) {
	dataset, err := this.manager.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	return dataset.Meta(), nil
}

func (this *datasetManagerServer) Delete(ctx context.Context, req *pb.UUIDRequest) (*pb.EmptyMessage, error) {
	id, err := uuid.FromBytes(req.GetId())
	if err != nil {
		return nil, err
	}

	if err := this.manager.Delete(ctx, id); err != nil {
		return nil, err
	}

	return &pb.EmptyMessage{}, nil
}