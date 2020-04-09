package raft

import (
	"fmt";
	"context";
	"time";
	"errors"
	"sync/atomic";

	"github.com/marekgalovic/anndb/storage/wal";

	"github.com/satori/go.uuid";
	etcdRaft "github.com/coreos/etcd/raft";
	"github.com/coreos/etcd/raft/raftpb";
	log "github.com/sirupsen/logrus";
)

const snapshotOffset uint64 = 5000

var (
	ProcessFnAlreadyRegisteredErr error = errors.New("ProcessFn already registered")
	SnapshotFnAlreadyRegisteredErr error = errors.New("SnapshotFn already registered")
)

type ProcessFn func([]byte) error
type SnapshotFn func() ([]byte, error)

type RaftGroup struct {
	id uuid.UUID
	transport *RaftTransport
	ctx context.Context
	ctxCancel context.CancelFunc
	processFn ProcessFn
	processSnapshotFn ProcessFn
	snapshotFn SnapshotFn

	raft etcdRaft.Node
	raftConfState *raftpb.ConfState
	raftLeaderId uint64
	wal wal.WAL
	log *log.Entry
}

func startRaftNode(id uint64, nodeIds []uint64, storage wal.WAL, logger *log.Entry) (etcdRaft.Node, error) {
	raftConfig := &etcdRaft.Config {
	    ID: id,
	    ElectionTick: 10,
	    HeartbeatTick: 1,
	    Storage: storage,
	    MaxSizePerMsg: 4096,
	    MaxInflightMsgs: 256,
	    Logger: logger,
	}

	if len(nodeIds) > 0 {
		var peers []etcdRaft.Peer
		for _, nodeId := range nodeIds {
			peers = append(peers, etcdRaft.Peer{ID: nodeId})
		}
		return etcdRaft.StartNode(raftConfig, peers), nil
	} else {
		// Allow the group to join existing cluster
		return etcdRaft.RestartNode(raftConfig), nil
	}
}

func NewRaftGroup(id uuid.UUID, nodeIds []uint64, storage wal.WAL, transport *RaftTransport) (*RaftGroup, error) {
	logger := log.WithFields(log.Fields{
    	"node_id": fmt.Sprintf("%16x", transport.nodeId),
    	"group_id": id.String(),
    })

	ctx, ctxCancel := context.WithCancel(context.Background())
	raftNode, err := startRaftNode(transport.NodeId(), nodeIds, storage, logger)
	if err != nil {
		return nil, err
	}

	g := &RaftGroup {
		id: id,
		transport: transport,
		ctx: ctx,
		ctxCancel: ctxCancel,
		processFn: nil,
		processSnapshotFn: nil,
		snapshotFn: nil,
		raft: raftNode,
		wal: storage,
		log: logger,
	}

	if err := transport.addGroup(g); err != nil {
		return nil, err
	}

	go g.run()

	return g, nil
}

func (this *RaftGroup) Stop() {
	this.raft.Stop()
	this.ctxCancel()

	if err := this.transport.removeGroup(this.id); err != nil {
		this.log.Error(err)
	}
}

func (this *RaftGroup) RegisterProcessFn(fn ProcessFn) error {
	if this.processFn != nil {
		return ProcessFnAlreadyRegisteredErr
	}
	this.processFn = fn
	return nil
}

func (this *RaftGroup) RegisterProcessSnapshotFn(fn ProcessFn) error {
	if this.processSnapshotFn != nil {
		return ProcessFnAlreadyRegisteredErr
	}
	this.processSnapshotFn = fn
	return nil
}

func (this *RaftGroup) RegisterSnapshotFn(fn SnapshotFn) error {
	if this.snapshotFn != nil {
		return SnapshotFnAlreadyRegisteredErr
	}
	this.snapshotFn = fn
	return nil
}

func (this *RaftGroup) LeaderId() uint64 {
	return this.raftLeaderId
}

func (this *RaftGroup) Propose(ctx context.Context, data []byte) error {
	return this.raft.Propose(ctx, data)
}

func (this *RaftGroup) ProposeJoin(nodeId uint64, address string) error {
	var cc raftpb.ConfChange
	cc.Type = raftpb.ConfChangeAddNode
	cc.NodeID = nodeId
	cc.Context = []byte(address)

	return this.raft.ProposeConfChange(this.ctx, cc)
}

func (this *RaftGroup) ProposeLeave(nodeId uint64) error {
	var cc raftpb.ConfChange
	cc.Type = raftpb.ConfChangeRemoveNode
	cc.NodeID = nodeId

	return this.raft.ProposeConfChange(this.ctx, cc)
}

func (this *RaftGroup) run() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	snapshotTicker := time.NewTicker(10 * time.Second)
	defer snapshotTicker.Stop()

	var lastAppliedIdx uint64 = 0
	for {
		select {
		case <- snapshotTicker.C:
			if err := this.trySnapshot(lastAppliedIdx, snapshotOffset); err != nil {
				this.log.Errorf("Snapshot failed: %v", err)
				continue
			}
		case <- ticker.C:
			this.raft.Tick()
		case rd := <- this.raft.Ready():
			if rd.SoftState != nil {
				this.raftLeaderId = atomic.LoadUint64(&rd.SoftState.Lead)
			}
			if this.isLeader() {
				this.transport.Send(this.ctx, this, rd.Messages)
			}
			if err := this.wal.Save(rd.HardState, rd.Entries, rd.Snapshot); err != nil {
				this.log.Fatal(err)
			}
			if !etcdRaft.IsEmptySnap(rd.Snapshot) {
				if err := this.processSnapshotFn(rd.Snapshot.Data); err != nil {
					this.log.Fatal(err)
				}
				if rd.Snapshot.Metadata.Index > lastAppliedIdx {
					lastAppliedIdx = rd.Snapshot.Metadata.Index
				}
			}
			for _, entry := range rd.CommittedEntries {
				if entry.Type == raftpb.EntryConfChange {
					this.processConfChange(entry)
				} else if entry.Type == raftpb.EntryNormal {
					if len(entry.Data) > 0 {
						if err := this.processFn(entry.Data); err != nil {
							this.log.Fatal(err)
						}
					}
				}
				lastAppliedIdx = entry.Index
			}
			if !this.isLeader() {
				this.transport.Send(this.ctx, this, rd.Messages)
			}
			this.raft.Advance()
		case <- this.ctx.Done():
			this.log.Info("Stop Raft")
			return
		}
	}
}

func (this *RaftGroup) isLeader() bool {
	return this.LeaderId() == this.transport.NodeId()
}

func (this *RaftGroup) receive(message raftpb.Message) error {
	return this.raft.Step(this.ctx, message)
}

func (this *RaftGroup) reportUnreachable(nodeId uint64) {
	this.raft.ReportUnreachable(nodeId)
}

func (this *RaftGroup) reportSnapshot(nodeId uint64, status etcdRaft.SnapshotStatus) {
	this.raft.ReportSnapshot(nodeId, status)
}

func (this *RaftGroup) processConfChange(entry raftpb.Entry) error {
	var cc raftpb.ConfChange
	if err := cc.Unmarshal(entry.Data); err != nil {
		return err
	}

	if uuid.Equal(this.id, uuid.Nil) {
		// Only the zero group should update transport
		// This prevents partition node changes from adding/removing nodes
		switch cc.Type {
		case raftpb.ConfChangeAddNode:
			this.transport.addNodeAddress(cc.NodeID, string(cc.Context))
		case raftpb.ConfChangeRemoveNode:
			this.transport.removeNodeAddress(cc.NodeID)
		}
	}

	this.raftConfState = this.raft.ApplyConfChange(cc)
	return nil
}

func (this *RaftGroup) trySnapshot(lastCommittedIdx, skip uint64) error {
	if this.snapshotFn == nil {
		return nil
	}

	existing, err := this.wal.Snapshot()
	if err != nil {
		return err
	}
	if lastCommittedIdx <= existing.Metadata.Index + skip {
		// Not enough new log entries to create snapshot
		return nil
	}

	startAt := time.Now()
	snapshotData, err := this.snapshotFn()
	if err != nil {
		return err
	}

	_, err = this.wal.CreateSnapshot(lastCommittedIdx, this.raftConfState, snapshotData)
	if err == nil {
		this.log.Infof("Snapshot. Size: %d bytes. Took: %s", len(snapshotData), time.Since(startAt))
	}
	return err
}