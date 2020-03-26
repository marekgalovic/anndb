package raft

import (
	"github.com/satori/go.uuid";
	etcdRaft "github.com/coreos/etcd/raft";
	"github.com/coreos/etcd/raft/raftpb";
)

type raftGroup struct {
	id uuid.UUID
	transport *raftTransport

	raft *etcdRaft.RawNode
}

func NewRaftGroup(id uuid.UUID, storage etcdRaft.Storage, transport *raftTransport) (*raftGroup, error) {
	raftConfig := &etcdRaft.Config {
	    ID: transport.nodeId,
	    ElectionTick: 10,
	    HeartbeatTick: 1,
	    Storage: storage,
	    MaxSizePerMsg: 4096,
	    MaxInflightMsgs: 256,
	}
	raft, err := etcdRaft.NewRawNode(raftConfig, nil)
	if err != nil {
		return nil, err
	}

	g := &raftGroup {
		id: id,
		transport: transport,
		raft: raft,
	}

	if err := transport.addGroup(g); err != nil {
		return nil, err
	}

	return g, nil
}

func (this *raftGroup) receive(messages []raftpb.Message) error {
	return nil
}

func (this *raftGroup) reportUnreachable(nodeId uint64) {
	this.raft.ReportUnreachable(nodeId)
}