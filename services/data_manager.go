package services

import (
	"context";

	pb "github.com/marekgalovic/anndb/protobuf";
	"github.com/marekgalovic/anndb/storage";

	"github.com/satori/go.uuid";
)

type dataManagerServer struct {
	datasetManager *storage.DatasetManager
}

func NewDataManagerServer(datasetManager *storage.DatasetManager) *dataManagerServer {
	return &dataManagerServer {
		datasetManager: datasetManager,
	}
}

func (this *dataManagerServer) Insert(ctx context.Context, req *pb.InsertRequest) (*pb.EmptyMessage, error) {
	datasetId, err := uuid.FromBytes(req.GetDatasetId())
	if err != nil {
		return nil, err
	}
	dataset, err := this.datasetManager.Get(datasetId)
	if err != nil {
		return nil, err
	}

	if err := dataset.Insert(ctx, req.GetId(), req.GetValue()); err != nil {
		return nil, err
	}

	return &pb.EmptyMessage{}, nil
}

func (this *dataManagerServer) Update(ctx context.Context, req *pb.UpdateRequest) (*pb.EmptyMessage, error) {
	datasetId, err := uuid.FromBytes(req.GetDatasetId())
	if err != nil {
		return nil, err
	}
	dataset, err := this.datasetManager.Get(datasetId)
	if err != nil {
		return nil, err
	}

	if err := dataset.Update(ctx, req.GetId(), req.GetValue()); err != nil {
		return nil, err
	}

	return &pb.EmptyMessage{}, nil
}

func (this *dataManagerServer) Remove(ctx context.Context, req *pb.RemoveRequest) (*pb.EmptyMessage, error) {
	datasetId, err := uuid.FromBytes(req.GetDatasetId())
	if err != nil {
		return nil, err
	}
	dataset, err := this.datasetManager.Get(datasetId)
	if err != nil {
		return nil, err
	}

	if err := dataset.Remove(ctx, req.GetId()); err != nil {
		return nil, err
	}

	return &pb.EmptyMessage{}, nil
}