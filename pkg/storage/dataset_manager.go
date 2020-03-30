package storage

import (
	"errors";
	"context";
	"sync";

	pb "github.com/marekgalovic/anndb/pkg/protobuf";
	"github.com/marekgalovic/anndb/pkg/storage/raft";
	"github.com/marekgalovic/anndb/pkg/utils";

	"github.com/satori/go.uuid";
	"github.com/golang/protobuf/proto";
	log "github.com/sirupsen/logrus";
)

var (
	DatasetNotFoundErr error = errors.New("Dataset not found")
	DatasetAlreadyExistsErr error = errors.New("Dataset already exists")
)

type DatasetManager struct {
	zeroGroup *raft.RaftGroup
	datasets map[uuid.UUID]*Dataset
	datasetsMu *sync.RWMutex

	createdNotifications *utils.Notificator
	deletedNotifications *utils.Notificator
}

func NewDatasetManager(zeroGroup *raft.RaftGroup) (*DatasetManager, error) {
	dm := &DatasetManager {
		zeroGroup: zeroGroup,
		datasets: make(map[uuid.UUID]*Dataset),
		datasetsMu: &sync.RWMutex{},

		createdNotifications: utils.NewNotificator(),
		deletedNotifications: utils.NewNotificator(),
	}

	if err := zeroGroup.RegisterProcessFn(dm.process); err != nil {
		return nil, err
	}

	return dm, nil
}

func (this *DatasetManager) Get(id uuid.UUID) (*Dataset, error) {
	this.datasetsMu.RLock()
	defer this.datasetsMu.RUnlock()

	if dataset, exists := this.datasets[id]; exists {
		return dataset, nil
	}

	return nil, DatasetNotFoundErr
}

func (this *DatasetManager) Create(ctx context.Context, dataset *pb.Dataset) (*Dataset, error) {
	id := uuid.NewV4()
	dataset.Id = id.Bytes()
	dataset.Partitions = make([]*pb.Partition, dataset.GetPartitionCount())
	for i := 0; i < int(dataset.GetPartitionCount()); i++ {
		dataset.Partitions[i] = &pb.Partition {Id: uuid.NewV4().Bytes()}
	}

	proposal := &pb.DatasetsChange {
		Type: pb.DatasetsChangeType_CreateDataset,
		Dataset: dataset,
	}
	proposalData, err := proto.Marshal(proposal)
	if err != nil {
		return nil, err
	}

	notif := this.createdNotifications.Create(id)
	defer func() { this.createdNotifications.Remove(id) }()

	if err := this.zeroGroup.Propose(ctx, proposalData); err != nil {
		return nil, err
	}

	select {
	case err := <- notif:
		if err != nil {
			return nil, err.(error)
		}
		return this.Get(id)
	case <- ctx.Done():
		return nil, nil
	}
}

func (this *DatasetManager) Delete(ctx context.Context, id uuid.UUID) error {
	proposal := &pb.DatasetsChange {
		Type: pb.DatasetsChangeType_DeleteDataset,
		Dataset: &pb.Dataset {Id: id.Bytes()},
	}
	proposalData, err := proto.Marshal(proposal)
	if err != nil {
		return err
	}

	notif := this.deletedNotifications.Create(id)
	defer func() { this.deletedNotifications.Remove(id) }()

	if err := this.zeroGroup.Propose(ctx, proposalData); err != nil {
		return err
	}

	select {
	case err := <- notif:
		if err != nil {
			return err.(error)
		}
		return nil
	case <- ctx.Done():
		return nil
	}
}

func (this *DatasetManager) process(data []byte) error {
	var change pb.DatasetsChange
	if err := proto.Unmarshal(data, &change); err != nil {
		return err
	}

	switch change.Type {
	case pb.DatasetsChangeType_CreateDataset:
		return this.createDataset(change.Dataset)
	case pb.DatasetsChangeType_DeleteDataset:
		return this.deleteDataset(change.Dataset)
	}
	return nil
}

func (this *DatasetManager) createDataset(dataset *pb.Dataset) error {
	id, err := uuid.FromBytes(dataset.GetId())
	if err != nil {
		return err
	}

	this.datasetsMu.Lock()
	defer this.datasetsMu.Unlock()
	if _, exists := this.datasets[id]; exists {
		this.createdNotifications.Notify(id, DatasetAlreadyExistsErr)
		return nil
	}

	this.datasets[id], err = newDataset(dataset)
	if err != nil {
		this.createdNotifications.Notify(id, err)
		return nil
	}
	this.createdNotifications.Notify(id, nil)
	log.Infof("Created dataset: %s", id)
	return nil
}

func (this *DatasetManager) deleteDataset(dataset *pb.Dataset) error {
	id, err := uuid.FromBytes(dataset.GetId())
	if err != nil {
		return err
	}

	this.datasetsMu.Lock()
	defer this.datasetsMu.Unlock()
	if _, exists := this.datasets[id]; !exists {
		this.deletedNotifications.Notify(id, DatasetNotFoundErr)
		return nil
	}

	delete(this.datasets, id)
	this.deletedNotifications.Notify(id, nil)
	log.Infof("Deleted dataset: %s", id)
	return nil
}

