package services

import (
	"context"

	pb "github.com/marekgalovic/anndb/protobuf"
	"github.com/marekgalovic/anndb/storage"
	uuid "github.com/satori/go.uuid"
)

type datasetManagerServer struct {
	manager *storage.DatasetManager
}

func NewDatasetManagerServer(manager *storage.DatasetManager) *datasetManagerServer {
	return &datasetManagerServer{
		manager: manager,
	}
}

func (this *datasetManagerServer) List(req *pb.ListDatasetsRequest, stream pb.DatasetManager_ListServer) error {
	datasets, err := this.manager.List(stream.Context(), req.GetWithSize())
	if err != nil {
		return err
	}

	for _, dataset := range datasets {
		if err := stream.Send(dataset); err != nil {
			return err
		}
	}

	return nil
}

func (this *datasetManagerServer) Get(ctx context.Context, req *pb.GetDatasetRequest) (*pb.Dataset, error) {
	id, err := uuid.FromBytes(req.GetDatasetId())
	if err != nil {
		return nil, err
	}

	dataset, err := this.manager.Get(id)
	if err != nil {
		return nil, err
	}
	meta := dataset.Meta()

	if req.GetWithSize() {
		meta.Size, err = dataset.Len(ctx)
		if err != nil {
			return nil, err
		}
	}

	return meta, nil
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

func (this *datasetManagerServer) GetDatasetSize(ctx context.Context, req *pb.GetDatasetRequest) (*pb.DatasetSize, error) {
	id, err := uuid.FromBytes(req.GetDatasetId())
	if err != nil {
		return nil, err
	}

	dataset, err := this.manager.Get(id)
	if err != nil {
		return nil, err
	}

	len, bs, err := dataset.SizeInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.DatasetSize{
		Len:       len,
		BytesSize: bs,
	}, nil
}
