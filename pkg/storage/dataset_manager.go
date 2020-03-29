package storage

import (
	pb "github.com/marekgalovic/anndb/pkg/protobuf";
	"github.com/marekgalovic/anndb/pkg/storage/raft";

	"github.com/satori/go.uuid";
	"github.com/golang/protobuf/proto";
	log "github.com/sirupsen/logrus";
)

type DatasetManager struct {
	zeroGroup *raft.RaftGroup
	datasets map[uuid.UUID]*Dataset
}

func NewDatasetManager(zeroGroup *raft.RaftGroup) (*DatasetManager, error) {
	dm := &DatasetManager {
		zeroGroup: zeroGroup,
		datasets: make(map[uuid.UUID]*Dataset),
	}

	if err := zeroGroup.RegisterProcessFn(dm.process); err != nil {
		return nil, err
	}

	return dm, nil
}

func (this *DatasetManager) Get(id uuid.UUID) (*Dataset, error) {
	return nil, nil
}

func (this *DatasetManager) Create(dataset *pb.Dataset) (*Dataset, error) {
	proposal := &pb.DatasetsChange {
		Type: pb.DatasetsChangeType_Create,
		Dataset: dataset,
	}
	proposalData, err := proto.Marshal(proposal)
	if err != nil {
		return nil, err
	}
	if err := this.zeroGroup.Propose(proposalData); err != nil {
		return nil, err
	}
	return nil, nil
}

func (this *DatasetManager) Delete(id uuid.UUID) error {
	return nil
}

func (this *DatasetManager) process(data []byte) error {
	var change pb.DatasetsChange
	if err := proto.Unmarshal(data, &change); err != nil {
		return err
	}

	log.Info(change.Type)
	log.Info(change.Dataset)
	return nil
}

