package storage

import (
	"context"
	"errors"
	"math/rand"
	"sync"

	"github.com/marekgalovic/anndb/cluster"
	"github.com/marekgalovic/anndb/math"
	uuid "github.com/satori/go.uuid"

	log "github.com/sirupsen/logrus"
)

var (
	partitionNotFoundErr error = errors.New("Partition not found")
)

type Allocator struct {
	ctx         context.Context
	cancelCtx   context.CancelFunc
	clusterConn *cluster.Conn

	updatesC     chan interface{}
	partitions   map[uuid.UUID]*partition
	partitionsMu *sync.RWMutex
}

type watchPartitionUpdate struct {
	partition *partition
}

type unwatchPartitionUpdate struct {
	partition *partition
}

func NewAllocator(clusterConn *cluster.Conn) *Allocator {
	ctx, cancelCtx := context.WithCancel(context.Background())

	a := &Allocator{
		ctx:         ctx,
		cancelCtx:   cancelCtx,
		clusterConn: clusterConn,

		updatesC:     make(chan interface{}),
		partitions:   make(map[uuid.UUID]*partition),
		partitionsMu: &sync.RWMutex{},
	}

	go a.run()

	return a
}

func (this *Allocator) Stop() {
	this.cancelCtx()
	close(this.updatesC)
}

func (this *Allocator) watch(partition *partition) {
	this.partitionsMu.Lock()
	defer this.partitionsMu.Unlock()

	if _, exists := this.partitions[partition.id]; !exists {
		this.partitions[partition.id] = partition
		this.updatesC <- &watchPartitionUpdate{partition}
	}
}

func (this *Allocator) unwatch(id uuid.UUID) {
	this.partitionsMu.Lock()
	defer this.partitionsMu.Unlock()

	if partition, exists := this.partitions[id]; exists {
		delete(this.partitions, id)
		this.updatesC <- &unwatchPartitionUpdate{partition}
	}
}

func (this *Allocator) getPartition(id uuid.UUID) (*partition, error) {
	this.partitionsMu.RLock()
	defer this.partitionsMu.RUnlock()

	if partition, exists := this.partitions[id]; exists {
		return partition, nil
	}
	return nil, partitionNotFoundErr
}

func (this *Allocator) getPartitionsNodeIds(partitionCount uint, replicationFactor uint) [][]uint64 {
	nodeIds := this.clusterConn.NodeIds()
	partitionsNodeIds := make([][]uint64, partitionCount)
	for i := 0; i < int(partitionCount); i++ {
		rand.Shuffle(len(nodeIds), func(i, j int) {
			nodeIds[i], nodeIds[j] = nodeIds[j], nodeIds[i]
		})

		partitionsNodeIds[i] = nodeIds[:math.MinInt(len(nodeIds), int(replicationFactor))]
	}

	return partitionsNodeIds
}

func (this *Allocator) run() {
	nodeChanges := this.clusterConn.NodeChangesNotifications()

	for {
		select {
		case change := <-nodeChanges:
			if change == nil {
				continue
			}
			switch change.Type {
			case cluster.NodesChangeAddNode:
				this.addNodeToPartitions(change.NodeId)
			case cluster.NodesChangeRemoveNode:
				this.removeNodeFromPartitions(change.NodeId)
			}
		case update := <-this.updatesC:
			if update == nil {
				continue
			}
			switch update.(type) {
			case *watchPartitionUpdate:
				_partition := update.(*watchPartitionUpdate).partition
				if this.isPartitionAssignedToNode(_partition) {
					func(partition *partition) {
						defer func() {
							if r := recover(); r != nil {
								log.WithFields(log.Fields{"partition_id": partition.id, "dataset_id": partition.dataset.id}).Errorf("Partition loadRaft panicked: %v", r)
							}
						}()
						partition.loadRaft(partition.nodeIds())
					}(_partition)
				}
			case *unwatchPartitionUpdate:
				_partition := update.(*unwatchPartitionUpdate).partition
				if this.isPartitionAssignedToNode(_partition) {
					func(partition *partition) {
						defer func() {
							if r := recover(); r != nil {
								log.WithFields(log.Fields{"partition_id": partition.id, "dataset_id": partition.dataset.id}).Errorf("Partition unloadRaft panicked: %v", r)
							}
						}()
						partition.unloadRaft()
					}(_partition)
				}
			}
		case <-this.ctx.Done():
			return
		}
	}
}

func (this *Allocator) isPartitionAssignedToNode(partition *partition) bool {
	for _, nodeId := range partition.nodeIds() {
		if this.clusterConn.Id() == nodeId {
			return true
		}
	}
	return false
}

func (this *Allocator) canModifyPartition(partition *partition) bool {
	if len(partition.nodeIds()) > 0 {
		return partition.nodeIds()[0] == this.clusterConn.Id()
	}

	// Default leader for the whole cluster
	return this.clusterConn.Id() == this.clusterConn.NodeIds()[0]
}

func (this *Allocator) addNodeToPartitions(nodeId uint64) {
	this.partitionsMu.RLock()
	defer this.partitionsMu.RUnlock()

	for _, partition := range this.partitions {
		if this.canModifyPartition(partition) && partition.isUnderReplicated() {
			partition.proposeAddNode(this.ctx, nodeId)
		}
	}
}

func (this *Allocator) removeNodeFromPartitions(nodeId uint64) {
	this.partitionsMu.RLock()
	defer this.partitionsMu.RUnlock()

	for _, partition := range this.partitions {
		if this.canModifyPartition(partition) {
			partition.proposeRemoveNode(this.ctx, nodeId)
		}
	}
}
