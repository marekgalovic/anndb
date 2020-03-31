package wal

import (
	etcdRaft "github.com/coreos/etcd/raft";
	"github.com/coreos/etcd/raft/raftpb";
)

type WAL interface {
	etcdRaft.Storage
	Save(raftpb.HardState, []raftpb.Entry, raftpb.Snapshot) error
	DeleteGroup() error
}