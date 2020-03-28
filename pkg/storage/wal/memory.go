package wal

import (
	etcdRaft "github.com/coreos/etcd/raft";
	"github.com/coreos/etcd/raft/raftpb";
)

type memoryWAL struct {
	*etcdRaft.MemoryStorage
}

func NewMemoryWAL() *memoryWAL {
	return &memoryWAL {etcdRaft.NewMemoryStorage()}
}

func (this *memoryWAL) Save(hs raftpb.HardState, es []raftpb.Entry, snap raftpb.Snapshot) error {
	if err := this.Append(es); err != nil {
		return err
	}
	if err := this.SetHardState(hs); err != nil {
		return err
	}
	if !etcdRaft.IsEmptySnap(snap) {
		if err := this.ApplySnapshot(snap); err != nil {
			return err
		}
	}
	return nil
}