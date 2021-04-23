package services

import (
	"context"
	"fmt"

	pb "github.com/marekgalovic/anndb/protobuf"
	"github.com/marekgalovic/anndb/storage"

	uuid "github.com/satori/go.uuid"
)

type dataManagerServer struct {
	datasetManager *storage.DatasetManager
}

func NewDataManagerServer(datasetManager *storage.DatasetManager) *dataManagerServer {
	return &dataManagerServer{
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

	id, err := uuid.FromBytes(req.GetId())
	if err != nil {
		return nil, err
	}
	if err := dataset.Insert(ctx, id, req.GetValue(), req.GetMetadata()); err != nil {
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

	id, err := uuid.FromBytes(req.GetId())
	if err != nil {
		return nil, err
	}
	if err := dataset.Update(ctx, id, req.GetValue(), req.GetMetadata()); err != nil {
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

	id, err := uuid.FromBytes(req.GetId())
	if err != nil {
		return nil, err
	}
	if err := dataset.Remove(ctx, id); err != nil {
		return nil, err
	}

	return &pb.EmptyMessage{}, nil
}

func (this *dataManagerServer) BatchInsert(ctx context.Context, req *pb.BatchRequest) (*pb.BatchResponse, error) {
	datasetId, err := uuid.FromBytes(req.GetDatasetId())
	if err != nil {
		return nil, err
	}
	dataset, err := this.datasetManager.Get(datasetId)
	if err != nil {
		return nil, err
	}

	errors, err := dataset.BatchInsert(ctx, req.GetItems())
	if err != nil {
		return nil, err
	}

	return &pb.BatchResponse{Errors: this.errorsMapToBatchResponse(errors)}, nil
}

func (this *dataManagerServer) PartitionBatchInsert(ctx context.Context, req *pb.PartitionBatchRequest) (*pb.BatchResponse, error) {
	datasetId, err := uuid.FromBytes(req.GetDatasetId())
	if err != nil {
		return nil, err
	}
	partitionId, err := uuid.FromBytes(req.GetPartitionId())
	if err != nil {
		return nil, err
	}
	dataset, err := this.datasetManager.Get(datasetId)
	if err != nil {
		return nil, err
	}

	errors, err := dataset.PartitionBatchInsert(ctx, partitionId, req.GetItems())
	if err != nil {
		return nil, err
	}

	return &pb.BatchResponse{Errors: this.errorsMapToBatchResponse(errors)}, nil
}

func (this *dataManagerServer) BatchUpdate(ctx context.Context, req *pb.BatchRequest) (*pb.BatchResponse, error) {
	datasetId, err := uuid.FromBytes(req.GetDatasetId())
	if err != nil {
		return nil, err
	}
	dataset, err := this.datasetManager.Get(datasetId)
	if err != nil {
		return nil, err
	}

	errors, err := dataset.BatchUpdate(ctx, req.GetItems())
	if err != nil {
		return nil, err
	}

	return &pb.BatchResponse{Errors: this.errorsMapToBatchResponse(errors)}, nil
}

func (this *dataManagerServer) PartitionBatchUpdate(ctx context.Context, req *pb.PartitionBatchRequest) (*pb.BatchResponse, error) {
	datasetId, err := uuid.FromBytes(req.GetDatasetId())
	if err != nil {
		return nil, err
	}
	partitionId, err := uuid.FromBytes(req.GetPartitionId())
	if err != nil {
		return nil, err
	}
	dataset, err := this.datasetManager.Get(datasetId)
	if err != nil {
		return nil, err
	}

	errors, err := dataset.PartitionBatchUpdate(ctx, partitionId, req.GetItems())
	if err != nil {
		return nil, err
	}

	return &pb.BatchResponse{Errors: this.errorsMapToBatchResponse(errors)}, nil
}

func (this *dataManagerServer) BatchRemove(ctx context.Context, req *pb.BatchRequest) (*pb.BatchResponse, error) {
	datasetId, err := uuid.FromBytes(req.GetDatasetId())
	if err != nil {
		return nil, err
	}
	dataset, err := this.datasetManager.Get(datasetId)
	if err != nil {
		return nil, err
	}

	errors, err := dataset.BatchRemove(ctx, req.GetItems())
	if err != nil {
		return nil, err
	}

	return &pb.BatchResponse{Errors: this.errorsMapToBatchResponse(errors)}, nil
}

func (this *dataManagerServer) PartitionBatchRemove(ctx context.Context, req *pb.PartitionBatchRequest) (*pb.BatchResponse, error) {
	datasetId, err := uuid.FromBytes(req.GetDatasetId())
	if err != nil {
		return nil, err
	}
	partitionId, err := uuid.FromBytes(req.GetPartitionId())
	if err != nil {
		return nil, err
	}
	dataset, err := this.datasetManager.Get(datasetId)
	if err != nil {
		return nil, err
	}

	errors, err := dataset.PartitionBatchRemove(ctx, partitionId, req.GetItems())
	if err != nil {
		return nil, err
	}

	return &pb.BatchResponse{Errors: this.errorsMapToBatchResponse(errors)}, nil
}

func (this *dataManagerServer) PartitionInfo(ctx context.Context, req *pb.PartitionInfoRequest) (*pb.PartitionInfoResponse, error) {
	datasetId, err := uuid.FromBytes(req.GetDatasetId())
	if err != nil {
		return nil, err
	}
	partitionId, err := uuid.FromBytes(req.GetPartitionId())
	if err != nil {
		return nil, err
	}
	dataset, err := this.datasetManager.Get(datasetId)
	if err != nil {
		return nil, err
	}

	len, bs, err := dataset.PartitionInfo(ctx, partitionId)
	if err != nil {
		return nil, err
	}

	return &pb.PartitionInfoResponse{Len: len, BytesSize: bs}, nil
}

func (this *dataManagerServer) errorsMapToBatchResponse(m map[uuid.UUID]error) map[string]string {
	result := make(map[string]string)
	for id, err := range m {
		result[id.String()] = fmt.Sprintf("%s", err)
	}
	return result
}
