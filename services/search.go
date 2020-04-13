package services

import (
	pb "github.com/marekgalovic/anndb/protobuf";
	"github.com/marekgalovic/anndb/storage";

	"github.com/satori/go.uuid";
)

type searchServer struct {
	datasetManager *storage.DatasetManager
}

func NewSearchServer(datasetManager *storage.DatasetManager) *searchServer {
	return &searchServer {
		datasetManager: datasetManager,
	}
}

func (this *searchServer) Search(req *pb.SearchRequest, stream pb.Search_SearchServer) error {
	datasetId, err := uuid.FromBytes(req.GetDatasetId())
	if err != nil {
		return err
	}
	dataset, err := this.datasetManager.Get(datasetId)
	if err != nil {
		return err
	}

	items, err := dataset.Search(stream.Context(), req.GetQuery(), uint(req.GetK()))
	if err != nil {
		return err
	}

	for _, item := range items {
		err := stream.Send(&pb.SearchResultItem {
			Id: item.Id,
			Metadata: item.Metadata,
			Score: item.Score,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (this *searchServer) SearchPartitions(req *pb.SearchPartitionsRequest, stream pb.Search_SearchPartitionsServer) error {
	datasetId, err := uuid.FromBytes(req.GetDatasetId())
	if err != nil {
		return err
	}
	partitionIds := make([]uuid.UUID, len(req.GetPartitionIds()))
	for i, partitionIdBytes := range req.GetPartitionIds() {
		partitionIds[i], err = uuid.FromBytes(partitionIdBytes)
		if err != nil {
			return err
		}
	}

	dataset, err := this.datasetManager.Get(datasetId)
	if err != nil {
		return err
	}

	items, err := dataset.SearchPartitions(stream.Context(), partitionIds, req.GetQuery(), uint(req.GetK()))
	if err != nil {
		return err
	}

	for _, item := range items {
		err := stream.Send(&pb.SearchResultItem {
			Id: item.Id,
			Metadata: item.Metadata,
			Score: item.Score,
		})
		if err != nil {
			return err
		}
	}
	return nil
}