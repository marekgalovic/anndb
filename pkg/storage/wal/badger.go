package wal

import (
	"errors";
	"math";
	"encoding/binary";
	"bytes";

	"github.com/satori/go.uuid";
	etcdRaft "github.com/coreos/etcd/raft";
	"github.com/coreos/etcd/raft/raftpb";
	badger "github.com/dgraph-io/badger/v2";
)


var badgerRaftIdKey []byte = []byte("raftid")

var (
	entryNotFoundErr error = errors.New("Entry not found")
)

type badgerWAL struct {
	db *badger.DB
	nodeId uint64
	groupId uuid.UUID
}

func GetBadgerRaftId(db *badger.DB) (uint64, error) {
	var id uint64
	err := db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(badgerRaftIdKey)
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			id = binary.BigEndian.Uint64(val)
			return nil
		})
	})
	return id, err
}

func SetBadgerRaftId(db *badger.DB, id uint64) error {
	return db.Update(func(txn *badger.Txn) error {
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], id)
		return txn.Set(badgerRaftIdKey, b[:])
	})
} 

func NewBadgerWAL(db *badger.DB, nodeId uint64, groupId uuid.UUID) *badgerWAL {
	return &badgerWAL{
		db: db,
		nodeId: nodeId,
		groupId: groupId,
	}
}

func (this *badgerWAL) InitialState() (raftpb.HardState, raftpb.ConfState, error) {
	hardState, err := this.HardState()
	if err != nil {
		return raftpb.HardState{}, raftpb.ConfState{}, err
	}

	snapshot, err := this.Snapshot()
	if err != nil {
		return raftpb.HardState{}, raftpb.ConfState{}, err	
	}

	return hardState, snapshot.Metadata.ConfState, nil
}

func (this *badgerWAL) Entries(lo, hi, maxSize uint64) ([]raftpb.Entry, error) {
	firstIndex, err := this.FirstIndex()
	if err != nil {
		return nil, err
	}
	if lo < firstIndex {
		return nil, etcdRaft.ErrCompacted
	}

	lastIndex, err := this.LastIndex()
	if err != nil {
		return nil, err
	}
	if hi > lastIndex + 1 {
		return nil, etcdRaft.ErrUnavailable
	}

	return this.getEntries(lo, hi, maxSize)
}

func (this *badgerWAL) Term(idx uint64) (uint64, error) {
	firstIndex, err := this.FirstIndex()
	if err != nil {
		return 0, err
	}
	if idx < firstIndex - 1 {
		return 0, etcdRaft.ErrCompacted
	}

	var entry raftpb.Entry
	_, err = this.seekEntry(&entry, idx, false)
	if err == entryNotFoundErr {
		return 0, etcdRaft.ErrUnavailable
	} else if err != nil {
		return 0, err
	}

	if idx < entry.Index {
		return 0, etcdRaft.ErrCompacted
	}
	return entry.Term, nil
}

func (this *badgerWAL) LastIndex() (uint64, error) {
	return this.seekEntry(nil, math.MaxUint64, true)
}

func (this *badgerWAL) FirstIndex() (uint64, error) {
	index, err := this.seekEntry(nil, 0, false)
	if err != nil {
		return 0, err
	}
	return index + 1, nil
}

func (this *badgerWAL) Snapshot() (raftpb.Snapshot, error) {
	var snapshot raftpb.Snapshot
	err := this.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(this.snapshotKey())
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return snapshot.Unmarshal(val)
		})
	})
	if err == badger.ErrKeyNotFound {
		return snapshot, nil
	}
	return snapshot, err
}

func (this *badgerWAL) HardState() (raftpb.HardState, error) {
	var hardState raftpb.HardState
	err := this.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(this.hardStateKey())
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return hardState.Unmarshal(val)
		})
	})
	if err == badger.ErrKeyNotFound {
		return hardState, nil
	}
	return hardState, err
}

func (this *badgerWAL) Save(hardState raftpb.HardState, entries []raftpb.Entry, snapshot raftpb.Snapshot) error {
	batch := this.db.NewWriteBatch()
	defer batch.Cancel()

	if err := this.writeEntries(batch, entries); err != nil {
		return err
	}
	if err := this.writeHardState(batch, hardState); err != nil {
		return err
	}
	if err := this.writeSnapshot(batch, snapshot); err != nil {
		return err
	}

	return batch.Flush()
}

func (this *badgerWAL) entryKey(idx uint64) []byte {
	b := make([]byte, 32)
	binary.BigEndian.PutUint64(b[0:8], this.nodeId)
	copy(b[8:24], this.groupId.Bytes())
	binary.BigEndian.PutUint64(b[24:32], idx)
	return b
}

func (this *badgerWAL) entryPrefix() []byte {
	b := make([]byte, 24)
	binary.BigEndian.PutUint64(b[0:8], this.nodeId)
	copy(b[8:24], this.groupId.Bytes())
	return b
}

func (this *badgerWAL) hardStateKey() []byte {
	b := make([]byte, 26)
	binary.BigEndian.PutUint64(b[0:8], this.nodeId)
	copy(b[8:10], []byte("hs"))
	copy(b[10:26], this.groupId.Bytes())
	return b
}

func (this *badgerWAL) snapshotKey() []byte {
	b := make([]byte, 26)
	binary.BigEndian.PutUint64(b[0:8], this.nodeId)
	copy(b[8:10], []byte("ss"))
	copy(b[10:26], this.groupId.Bytes())
	return b
}

func (this *badgerWAL) parseIndex(key []byte) uint64 {
	return binary.BigEndian.Uint64(key[24:32])
}

func (this *badgerWAL) seekEntry(entry *raftpb.Entry, seekTo uint64, reverse bool) (uint64, error) {
	var index uint64
	err := this.db.View(func (txn *badger.Txn) error {
		iterOpt := badger.DefaultIteratorOptions
		iterOpt.PrefetchValues = false
		iterOpt.Prefix = this.entryPrefix()
		iterOpt.Reverse = reverse

		iterator := txn.NewIterator(iterOpt)
		defer iterator.Close()

		iterator.Seek(this.entryKey(seekTo))
		if !iterator.Valid() {
			return entryNotFoundErr
		}

		item := iterator.Item()
		index = this.parseIndex(item.Key())
		if entry == nil {
			return nil
		}
		return item.Value(func(val []byte) error {
			return entry.Unmarshal(val)
		})
	})
	return index, err
}

func (this *badgerWAL) getEntries(lo, hi, maxSize uint64) ([]raftpb.Entry, error) {
	var entries []raftpb.Entry
	err := this.db.View(func(txn *badger.Txn) error {
		iterOpt := badger.DefaultIteratorOptions
		iterOpt.PrefetchValues = false
		iterOpt.Prefix = this.entryPrefix()

		iterator := txn.NewIterator(iterOpt)
		defer iterator.Close()

		startKey := this.entryKey(lo)
		endKey := this.entryKey(hi)

		for iterator.Seek(startKey); iterator.Valid(); iterator.Next() {
			item := iterator.Item()
			var entry raftpb.Entry
			err := item.Value(func(val []byte) error {
				return entry.Unmarshal(val)
			})
			if err != nil {
				return err
			}

			if bytes.Compare(item.Key(), endKey) >= 0 {
				break
			}
			entries = append(entries, entry)
		}
		return nil

	})

	return entries, err
}

func (this *badgerWAL) writeEntries(batch *badger.WriteBatch, entries []raftpb.Entry) error {
	if len(entries) == 0 {
		return nil
	}

	firstIndex, err := this.FirstIndex()
	if err != nil {
		return err
	}
	firstEntryIndex := entries[0].Index
	if firstEntryIndex + uint64(len(entries)) - 1 < firstIndex {
		return nil
	}
	if firstIndex > firstEntryIndex {
		entries = entries[(firstIndex - firstEntryIndex):]
	}

	lastIndex, err := this.LastIndex()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		entryData, err := entry.Marshal()
		if err != nil {
			return err
		}
		err = batch.Set(this.entryKey(entry.Index), entryData)
		if err != nil {
			return err
		}
	}

	lastEntryIndex := entries[len(entries)-1].Index
	if lastIndex > lastEntryIndex {
		return this.deleteEntriesFromIndex(batch, lastEntryIndex + 1)
	}
	return nil
}

func (this *badgerWAL) writeHardState(batch *badger.WriteBatch, hardState raftpb.HardState) error {
	if etcdRaft.IsEmptyHardState(hardState) {
		return nil
	}

	hardStateData, err := hardState.Marshal()
	if err != nil {
		return err
	}

	return batch.Set(this.hardStateKey(), hardStateData)
}

func (this *badgerWAL) writeSnapshot(batch *badger.WriteBatch, snapshot raftpb.Snapshot) error {
	if etcdRaft.IsEmptySnap(snapshot) {
		return nil
	}

	snapshotData, err := snapshot.Marshal()
	if err != nil {
		return err
	}

	err = batch.Set(this.snapshotKey(), snapshotData)
	if err != nil {
		return err
	}

	entry := raftpb.Entry{Term: snapshot.Metadata.Term, Index: snapshot.Metadata.Index}
	entryData, err := entry.Marshal()
	if err != nil {
		return err
	}

	err = batch.Set(this.entryKey(entry.Index), entryData)
	if err != nil {
		return err
	}

	return nil
}

func (this *badgerWAL) deleteEntriesFromIndex(batch *badger.WriteBatch, fromIdx uint64) error {
	var keys [][]byte
	err := this.db.View(func (txn *badger.Txn) error {
		startKey := this.entryKey(fromIdx)

		iterOpt := badger.DefaultIteratorOptions
		iterOpt.PrefetchValues = false
		iterOpt.Prefix = this.entryPrefix()

		iterator := txn.NewIterator(iterOpt)
		defer iterator.Close()

		for iterator.Seek(startKey); iterator.Valid(); iterator.Next() {
			keys = append(keys, iterator.Item().Key())
		}
		return nil
	})
	if err != nil {
		return err
	}

	return this.deleteKeys(batch, keys)
}

func (this *badgerWAL) deleteKeys(batch *badger.WriteBatch, keys [][]byte) error {
	if len(keys) == 0 {
		return nil
	}

	for _, key := range keys {
		if err := batch.Delete(key); err != nil {
			return err
		}
	}

	return nil
}