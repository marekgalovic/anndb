package wal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"sync"

	etcdRaft "github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	badger "github.com/dgraph-io/badger/v2"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

var (
	badgerRaftIdKey    []byte = []byte("raftid")
	cacheSnapshotKey   string = "snapshot"
	cacheFirstIndexKey string = "firstindex"
	cacheLastIndexKey  string = "lastindex"
)

var (
	entryNotFoundErr  error = errors.New("Entry not found")
	EmptyConfStateErr error = errors.New("Empty ConfState")
)

type badgerWAL struct {
	groupId uuid.UUID
	db      *badger.DB
	cache   *sync.Map
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

func NewBadgerWAL(db *badger.DB, groupId uuid.UUID) *badgerWAL {
	wal := &badgerWAL{
		groupId: groupId,
		db:      db,
		cache:   new(sync.Map),
	}

	_, err := wal.FirstIndex()
	if err == entryNotFoundErr {
		// Empty WAL. Populate with a dummy entry at term 0.
		wal.reset(make([]raftpb.Entry, 1))
	} else if err != nil {
		log.Fatal(err)
	}

	return wal
}

func (this *badgerWAL) InitialState() (raftpb.HardState, raftpb.ConfState, error) {
	hardState, err := this.HardState()
	if err != nil {
		return raftpb.HardState{}, raftpb.ConfState{}, err
	}

	snapshot, err := this.Snapshot()
	if err != nil {
		return hardState, raftpb.ConfState{}, err
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
	if hi > lastIndex+1 {
		return nil, etcdRaft.ErrUnavailable
	}

	return this.getEntries(lo, hi, maxSize)
}

func (this *badgerWAL) Term(idx uint64) (uint64, error) {
	firstIndex, err := this.FirstIndex()
	if err != nil {
		return 0, err
	}
	if idx < firstIndex-1 {
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
	if v, exists := this.cache.Load(cacheLastIndexKey); exists {
		if idx, ok := v.(uint64); ok {
			return idx, nil
		}
	}
	return this.seekEntry(nil, math.MaxUint64, true)
}

func (this *badgerWAL) FirstIndex() (uint64, error) {
	// Try cache
	if v, exists := this.cache.Load(cacheSnapshotKey); exists {
		if snapshot, ok := v.(*raftpb.Snapshot); ok && !etcdRaft.IsEmptySnap(*snapshot) {
			return snapshot.Metadata.Index + 1, nil
		}
	}
	if v, exists := this.cache.Load(cacheFirstIndexKey); exists {
		if idx, ok := v.(uint64); ok {
			return idx, nil
		}
	}

	index, err := this.seekEntry(nil, 0, false)
	if err != nil {
		return 0, err
	}
	this.cache.Store(cacheFirstIndexKey, index+1)
	return index + 1, nil
}

func (this *badgerWAL) Snapshot() (raftpb.Snapshot, error) {
	if v, exists := this.cache.Load(cacheSnapshotKey); exists {
		if snapshot, ok := v.(*raftpb.Snapshot); ok && !etcdRaft.IsEmptySnap(*snapshot) {
			return *snapshot, nil
		}
	}
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
	if !etcdRaft.IsEmptySnap(snapshot) {
		if err := this.writeSnapshot(batch, snapshot); err != nil {
			return err
		}
		// Delete the log
		if err := this.deleteEntriesFromIndex(batch, 0); err != nil {
			return err
		}
	}

	return batch.Flush()
}

func (this *badgerWAL) CreateSnapshot(idx uint64, confState *raftpb.ConfState, data []byte) (raftpb.Snapshot, error) {
	var snapshot raftpb.Snapshot
	if confState == nil {
		return snapshot, EmptyConfStateErr
	}
	firstIndex, err := this.FirstIndex()
	if err != nil {
		return snapshot, err
	}
	if idx < firstIndex {
		return snapshot, etcdRaft.ErrSnapOutOfDate
	}

	var entry raftpb.Entry
	if _, err := this.seekEntry(&entry, idx, false); err != nil {
		return snapshot, err
	}
	if idx != entry.Index {
		return snapshot, entryNotFoundErr
	}

	snapshot.Metadata.Index = entry.Index
	snapshot.Metadata.Term = entry.Term
	if confState != nil {
		snapshot.Metadata.ConfState = *confState
	}
	snapshot.Data = data

	batch := this.db.NewWriteBatch()
	defer batch.Cancel()

	if err := this.writeSnapshot(batch, snapshot); err != nil {
		return snapshot, err
	}
	if err := this.deleteEntriesUntilIndex(batch, snapshot.Metadata.Index); err != nil {
		return snapshot, err
	}

	return snapshot, batch.Flush()
}

func (this *badgerWAL) DeleteGroup() error {
	return this.reset(nil)
}

func (this *badgerWAL) entryPrefix() []byte {
	return this.groupId.Bytes()
}

func (this *badgerWAL) entryKey(idx uint64) []byte {
	b := make([]byte, 24)
	copy(b[0:16], this.entryPrefix())
	binary.BigEndian.PutUint64(b[16:24], idx)
	return b
}

func (this *badgerWAL) hardStateKey() []byte {
	b := make([]byte, 18)
	copy(b[0:2], []byte("hs"))
	copy(b[2:18], this.groupId.Bytes())
	return b
}

func (this *badgerWAL) snapshotKey() []byte {
	b := make([]byte, 18)
	copy(b[0:2], []byte("ss"))
	copy(b[2:18], this.groupId.Bytes())
	return b
}

func (this *badgerWAL) parseIndex(key []byte) uint64 {
	return binary.BigEndian.Uint64(key[16:24])
}

func (this *badgerWAL) seekEntry(entry *raftpb.Entry, seekTo uint64, reverse bool) (uint64, error) {
	var index uint64
	err := this.db.View(func(txn *badger.Txn) error {
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
		if hi-lo == 1 {
			item, err := txn.Get(this.entryKey(lo))
			if err != nil {
				return err
			}
			return item.Value(func(val []byte) error {
				var entry raftpb.Entry
				if err := entry.Unmarshal(val); err != nil {
					return err
				}
				entries = append(entries, entry)
				return nil
			})
		}

		iterOpt := badger.DefaultIteratorOptions
		iterOpt.PrefetchValues = false
		iterOpt.Prefix = this.entryPrefix()

		iterator := txn.NewIterator(iterOpt)
		defer iterator.Close()

		startKey := this.entryKey(lo)
		endKey := this.entryKey(hi)

		var size uint64 = 0
		first := true
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
			size += uint64(entry.Size())
			if size > maxSize && !first {
				break
			}
			first = false
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
	if firstEntryIndex+uint64(len(entries))-1 < firstIndex {
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
	this.cache.Store(cacheLastIndexKey, lastEntryIndex)
	if lastIndex > lastEntryIndex {
		return this.deleteEntriesFromIndex(batch, lastEntryIndex+1)
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

	if v, exists := this.cache.Load(cacheLastIndexKey); exists {
		if v.(uint64) < entry.Index {
			this.cache.Store(cacheLastIndexKey, entry.Index)
		}
	}

	this.cache.Store(cacheSnapshotKey, &snapshot)

	return nil
}

func (this *badgerWAL) deleteEntriesFromIndex(batch *badger.WriteBatch, fromIdx uint64) error {
	var keys []string
	err := this.db.View(func(txn *badger.Txn) error {
		startKey := this.entryKey(fromIdx)

		iterOpt := badger.DefaultIteratorOptions
		iterOpt.PrefetchValues = false
		iterOpt.Prefix = this.entryPrefix()

		iterator := txn.NewIterator(iterOpt)
		defer iterator.Close()

		for iterator.Seek(startKey); iterator.Valid(); iterator.Next() {
			keys = append(keys, string(iterator.Item().Key()))
		}
		return nil
	})
	if err != nil {
		return err
	}

	return this.deleteKeys(batch, keys)
}

// Deletes entries in the range [0, untilIdx)
func (this *badgerWAL) deleteEntriesUntilIndex(batch *badger.WriteBatch, untilIdx uint64) error {
	var keys []string
	var index uint64
	err := this.db.View(func(txn *badger.Txn) error {
		iterOpt := badger.DefaultIteratorOptions
		iterOpt.PrefetchValues = false
		iterOpt.Prefix = this.entryPrefix()

		iterator := txn.NewIterator(iterOpt)
		defer iterator.Close()

		startKey := this.entryKey(0)
		first := true
		for iterator.Seek(startKey); iterator.Valid(); iterator.Next() {
			item := iterator.Item()
			index = this.parseIndex(item.Key())
			if first {
				first = false
				if untilIdx <= index {
					return etcdRaft.ErrCompacted
				}
			}
			if index >= untilIdx {
				break
			}
			keys = append(keys, string(item.Key()))
		}
		return nil
	})
	if err != nil {
		return err
	}

	if err := this.deleteKeys(batch, keys); err != nil {
		return err
	}

	if v, ok := this.cache.Load(cacheFirstIndexKey); ok {
		if v.(uint64) <= untilIdx {
			this.cache.Store(cacheFirstIndexKey, untilIdx+1)
		}
	}
	return nil
}

func (this *badgerWAL) deleteKeys(batch *badger.WriteBatch, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	for _, key := range keys {
		if err := batch.Delete([]byte(key)); err != nil {
			return err
		}
	}

	return nil
}

func (this *badgerWAL) reset(entries []raftpb.Entry) error {
	this.cache = new(sync.Map)

	batch := this.db.NewWriteBatch()
	defer batch.Cancel()

	if err := this.deleteEntriesFromIndex(batch, 0); err != nil {
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

	return batch.Flush()
}
