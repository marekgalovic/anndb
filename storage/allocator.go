package storage

import (
	"context";
	"math/rand";
	"sync";
	"errors";

	"github.com/marekgalovic/anndb/cluster";
	"github.com/marekgalovic/anndb/math";

	"github.com/satori/go.uuid";
	log "github.com/sirupsen/logrus";
)

var (
	partitionNotFoundErr error = errors.New("Partition not found")
)

type allocator struct {
	ctx context.Context
	cancelCtx context.CancelFunc
	clusterConn *cluster.Conn

	updatesC chan interface{}
	partitions map[uuid.UUID]*partition
	partitionsMu *sync.RWMutex
}

type watchPartitionUpdate struct {
	partition *partition
}

type unwatchPartitionUpdate struct {
	partition *partition
}

func NewAllocator(clusterConn *cluster.Conn) *allocator {
	ctx, cancelCtx := context.WithCancel(context.Background())

	a := &allocator {
		ctx: ctx,
		cancelCtx: cancelCtx,
		clusterConn: clusterConn,

		updatesC: make(chan interface{}),
		partitions: make(map[uuid.UUID]*partition),
		partitionsMu: &sync.RWMutex{},
	}

	go a.run()

	return a
}

func (this *allocator) Stop() {
	this.cancelCtx()
	close(this.updatesC)
}

func (this *allocator) watch(partition *partition) {
	this.partitionsMu.Lock()
	defer this.partitionsMu.Unlock()

	if _, exists := this.partitions[partition.id]; !exists {
		this.partitions[partition.id] = partition
		this.updatesC <- &watchPartitionUpdate{partition}
	}
}

func (this *allocator) unwatch(id uuid.UUID) {
	this.partitionsMu.Lock()
	defer this.partitionsMu.Unlock()

	if partition, exists := this.partitions[id]; exists {
		delete(this.partitions, id)
		this.updatesC <- &unwatchPartitionUpdate{partition}
	}
}

func (this *allocator) getPartition(id uuid.UUID) (*partition, error) {
	this.partitionsMu.RLock()
	defer this.partitionsMu.RUnlock()

	if partition, exists := this.partitions[id]; exists {
		return partition, nil
	}
	return nil, partitionNotFoundErr
}

func (this *allocator) getPartitionsNodeIds(partitionCount uint, replicationFactor uint) [][]uint64 {
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

func (this *allocator) run() {
	nodeChanges := this.clusterConn.NodeChangesNotifications()

	for {
		select {
		case change := <- nodeChanges:
			if change == nil {
				continue
			}
			switch change.Type {
			case cluster.NodesChangeAddNode:
				log.Infof("Allocator added node: %16x", change.NodeId)
			case cluster.NodesChangeRemoveNode:
				log.Infof("Allocator removed node: %16x", change.NodeId)
			}
		case update := <- this.updatesC:
			if update == nil {
				continue
			}
			switch update.(type) {
			case *watchPartitionUpdate:
				partition := update.(*watchPartitionUpdate).partition
				if this.isPartitionAssignedToNode(partition) {
					partition.loadRaft()
				}
			case *unwatchPartitionUpdate:
				partition := update.(*unwatchPartitionUpdate).partition
				if this.isPartitionAssignedToNode(partition) {
					partition.unloadRaft()
				}
			}
		case <- this.ctx.Done():
			return
		}
	}
}

func (this *allocator) isPartitionAssignedToNode(partition *partition) bool {
	for _, nodeId := range partition.nodeIds {
		if this.clusterConn.Id() == nodeId {
			return true
		}
	}
	return false
}