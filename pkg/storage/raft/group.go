package raft

import (
	"context";
	"time";
	"io";
	"errors"

	pb "github.com/marekgalovic/anndb/pkg/protobuf";
	"github.com/marekgalovic/anndb/pkg/storage/wal";

	"github.com/satori/go.uuid";
	"google.golang.org/grpc";
	etcdRaft "github.com/coreos/etcd/raft";
	"github.com/coreos/etcd/raft/raftpb";
	log "github.com/sirupsen/logrus";
)

var (
	ProcessFnAlreadyRegisteredErr error = errors.New("ProcessFn already registered")
)

type ProcessFn func([]byte) error

type RaftGroup struct {
	id uuid.UUID
	transport *RaftTransport
	ctx context.Context
	ctxCancel context.CancelFunc
	processFn ProcessFn

	raft etcdRaft.Node
	wal wal.WAL
}

func NewRaftGroup(id uuid.UUID, nodeIds []uint64, storage wal.WAL, transport *RaftTransport) (*RaftGroup, error) {
	logger := log.WithFields(log.Fields{
    	"raft_node_id": transport.nodeId,
    	"raft_group_id": id.String(),
    })

	ctx, ctxCancel := context.WithCancel(context.Background())
	raftConfig := &etcdRaft.Config {
	    ID: transport.nodeId,
	    ElectionTick: 10,
	    HeartbeatTick: 1,
	    Storage: storage,
	    MaxSizePerMsg: 4096,
	    MaxInflightMsgs: 256,
	    Logger: logger,
	}

	var raftNode etcdRaft.Node
	if len(nodeIds) > 0 {
		peers := make([]etcdRaft.Peer, len(nodeIds))
		for i, nodeId := range nodeIds {
			peers[i] = etcdRaft.Peer{ID: nodeId}
		}
		raftNode = etcdRaft.StartNode(raftConfig, peers)
	} else {
		raftNode = etcdRaft.RestartNode(raftConfig)
	}

	g := &RaftGroup {
		id: id,
		transport: transport,
		ctx: ctx,
		ctxCancel: ctxCancel,
		processFn: nil,
		raft: raftNode,
		wal: storage,
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
		log.Error(err)
	}
}

func (this *RaftGroup) RegisterProcessFn(fn ProcessFn) error {
	if this.processFn != nil {
		return ProcessFnAlreadyRegisteredErr
	}
	this.processFn = fn
	return nil
}

func (this *RaftGroup) Propose(ctx context.Context, data []byte) error {
	return this.raft.Propose(ctx, data)
}

func (this *RaftGroup) Join(nodes []string) error {
	var conn *grpc.ClientConn
	var err error
	for i, addr := range nodes {
		conn, err = grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			if i == len(nodes)-1 {
				return err
			}
			continue
		}
		break
	}
	defer conn.Close()

	nodesStream, err := pb.NewRaftTransportClient(conn).ProposeJoin(this.ctx, &pb.RaftJoinMessage {
		NodeId: this.transport.NodeId(),
		GroupId: this.id.Bytes(),
		Address: this.transport.Address(),
	})
	if err != nil {
		return err
	}

	for {
		node, err := nodesStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		this.transport.addNodeAddress(node.GetId(), node.GetAddress())
	}
	return nil
}

func (this *RaftGroup) run() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <- ticker.C:
			this.raft.Tick()
		case rd := <- this.raft.Ready():
			if err := this.wal.Save(rd.HardState, rd.Entries, rd.Snapshot); err != nil {
				log.Fatal(err)
			}
			this.transport.Send(this.ctx, this, rd.Messages);
			if !etcdRaft.IsEmptySnap(rd.Snapshot) {
				this.processSnapshot(rd.Snapshot)
			}
			for _, entry := range rd.CommittedEntries {
				if entry.Type == raftpb.EntryConfChange {
					this.processConfChange(entry)
				} else if entry.Type == raftpb.EntryNormal {
					if len(entry.Data) == 0 {
						continue
					}
					if err := this.processFn(entry.Data); err != nil {
						log.Fatal(err)
					}
				}
			}
			this.raft.Advance()
		case <- this.ctx.Done():
			return
		}
	}
}

func (this *RaftGroup) receive(messages []raftpb.Message) error {
	for _, msg := range messages {
		if err := this.raft.Step(this.ctx, msg); err != nil {
			log.Error(err)
		}
	}
	return nil
}

func (this *RaftGroup) proposeJoin(nodeId uint64, address string) error {
	var cc raftpb.ConfChange
	cc.Type = raftpb.ConfChangeAddNode
	cc.NodeID = nodeId
	cc.Context = []byte(address)

	return this.raft.ProposeConfChange(this.ctx, cc)
}

func (this *RaftGroup) reportUnreachable(nodeId uint64) {
	this.raft.ReportUnreachable(nodeId)
}

func (this *RaftGroup) reportSnapshot(nodeId uint64, status etcdRaft.SnapshotStatus) {
	this.raft.ReportSnapshot(nodeId, status)
}

func (this *RaftGroup) processSnapshot(snapshot raftpb.Snapshot) error {
	return nil
}

func (this *RaftGroup) processConfChange(entry raftpb.Entry) error {
	var cc raftpb.ConfChange
	if err := cc.Unmarshal(entry.Data); err != nil {
		return err
	}

	switch cc.Type {
	case raftpb.ConfChangeAddNode:
		this.transport.addNodeAddress(cc.NodeID, string(cc.Context))
	case raftpb.ConfChangeRemoveNode:
		this.transport.removeNodeAddress(cc.NodeID)
	}

	this.raft.ApplyConfChange(cc)

	return nil
}