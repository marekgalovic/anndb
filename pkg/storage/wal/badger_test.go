package wal

import (
	"os";
	"math";
	"reflect";
	"testing";
	"io/ioutil";

	"github.com/satori/go.uuid";
	etcdRaft "github.com/coreos/etcd/raft";
	pb "github.com/coreos/etcd/raft/raftpb";
	badger "github.com/dgraph-io/badger/v2";
)

func getTmpBadgerWal(entries []pb.Entry) (string, *badger.DB, *badgerWAL, error) {
	tmpDir, err := ioutil.TempDir("/tmp", "")
	if err != nil {
		return "", nil, nil, err
	}

	db, err := badger.Open(badger.LSMOnlyOptions(tmpDir))
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", nil, nil, err
	}

	wal := NewBadgerWAL(db, uuid.Nil)
	if err := wal.reset(entries); err != nil {
		os.RemoveAll(tmpDir)
		db.Close()
		return "", nil, nil, err
	}

	return tmpDir, db, wal, nil
}

func TestStorageTerm(t *testing.T) {
	ents := []pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}}
	tests := []struct {
		i uint64

		werr   error
		wterm  uint64
		wpanic bool
	}{
		{2, etcdRaft.ErrCompacted, 0, false},
		{3, nil, 3, false},
		{4, nil, 4, false},
		{5, nil, 5, false},
		{6, etcdRaft.ErrUnavailable, 0, false},
	}

	for i, tt := range tests {
		func() {
			tmpDir, db, wal, err := getTmpBadgerWal(ents)
			if err != nil {
				t.Error(err)
				return
			}
			defer os.RemoveAll(tmpDir)
			defer db.Close()

			defer func() {
				if r := recover(); r != nil {
					if !tt.wpanic {
						t.Errorf("%d: panic = %v, want %v", i, true, tt.wpanic)
					}
				}
			}()

			term, err := wal.Term(tt.i)
			if err != tt.werr {
				t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
			}
			if term != tt.wterm {
				t.Errorf("#%d: term = %d, want %d", i, term, tt.wterm)
			}
		}()
	}
}

func TestStorageEntries(t *testing.T) {
	ents := []pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 6}}
	tests := []struct {
		lo, hi, maxsize uint64

		werr     error
		wentries []pb.Entry
	}{
		{2, 6, math.MaxUint64, etcdRaft.ErrCompacted, nil},
		{3, 4, math.MaxUint64, etcdRaft.ErrCompacted, nil},
		{4, 5, math.MaxUint64, nil, []pb.Entry{{Index: 4, Term: 4}}},
		{4, 6, math.MaxUint64, nil, []pb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}}},
		{4, 7, math.MaxUint64, nil, []pb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 6}}},
		// even if maxsize is zero, the first entry should be returned
		{4, 7, 0, nil, []pb.Entry{{Index: 4, Term: 4}}},
		// limit to 2
		{4, 7, uint64(ents[1].Size() + ents[2].Size()), nil, []pb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}}},
		// limit to 2
		{4, 7, uint64(ents[1].Size() + ents[2].Size() + ents[3].Size()/2), nil, []pb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}}},
		{4, 7, uint64(ents[1].Size() + ents[2].Size() + ents[3].Size() - 1), nil, []pb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}}},
		// all
		{4, 7, uint64(ents[1].Size() + ents[2].Size() + ents[3].Size()), nil, []pb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 6}}},
	}

	for i, tt := range tests {
		func () {
			tmpDir, db, wal, err := getTmpBadgerWal(ents)
			if err != nil {
				t.Error(err)
				return
			}
			defer os.RemoveAll(tmpDir)
			defer db.Close()

			entries, err := wal.Entries(tt.lo, tt.hi, tt.maxsize)
			if err != tt.werr {
				t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
			}
			if !reflect.DeepEqual(entries, tt.wentries) {
				t.Errorf("#%d: entries = %v, want %v", i, entries, tt.wentries)
			}
		}()
	}
}

func TestStorageLastIndex(t *testing.T) {
	ents := []pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}}
	tmpDir, db, wal, err := getTmpBadgerWal(ents)
	if err != nil {
		t.Error(err)
		return
	}
	defer os.RemoveAll(tmpDir)
	defer db.Close()

	last, err := wal.LastIndex()
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if last != 5 {
		t.Errorf("last = %d, want %d", last, 5)
	}

	wb := db.NewWriteBatch()
	defer wb.Cancel()
	wal.writeEntries(wb, []pb.Entry{{Index: 6, Term: 5}})
	if err := wb.Flush(); err != nil {
		t.Error(err)
	}

	last, err = wal.LastIndex()
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if last != 6 {
		t.Errorf("last = %d, want %d", last, 6)
	}
}

func TestStorageFirstIndex(t *testing.T) {
	ents := []pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}}
	tmpDir, db, wal, err := getTmpBadgerWal(ents)
	if err != nil {
		t.Error(err)
		return
	}
	defer os.RemoveAll(tmpDir)
	defer db.Close()

	first, err := wal.FirstIndex()
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if first != 4 {
		t.Errorf("first = %d, want %d", first, 4)
	}

	wb := db.NewWriteBatch()
	defer wb.Cancel()
	wal.deleteEntriesUntilIndex(wb, 4)
	if err := wb.Flush(); err != nil {
		t.Error(err)
	}

	first, err = wal.FirstIndex()
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if first != 5 {
		t.Errorf("first = %d, want %d", first, 5)
	}
}

// func TestStorageCompact(t *testing.T) {
// 	ents := []pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}}
// 	tests := []struct {
// 		i uint64

// 		werr   error
// 		windex uint64
// 		wterm  uint64
// 		wlen   int
// 	}{
// 		{2, ErrCompacted, 3, 3, 3},
// 		{3, ErrCompacted, 3, 3, 3},
// 		{4, nil, 4, 4, 2},
// 		{5, nil, 5, 5, 1},
// 	}

// 	for i, tt := range tests {
// 		func () {
// 			tmpDir, db, wal, err := getTmpBadgerWal(ents)
// 			if err != nil {
// 				t.Error(err)
// 				return
// 			}
// 			defer os.RemoveAll(tmpDir)
// 			defer db.Close()

// 			err := wal.Compact(tt.i)
// 			if err != tt.werr {
// 				t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
// 			}
// 			if s.ents[0].Index != tt.windex {
// 				t.Errorf("#%d: index = %d, want %d", i, s.ents[0].Index, tt.windex)
// 			}
// 			if s.ents[0].Term != tt.wterm {
// 				t.Errorf("#%d: term = %d, want %d", i, s.ents[0].Term, tt.wterm)
// 			}
// 			if len(s.ents) != tt.wlen {
// 				t.Errorf("#%d: len = %d, want %d", i, len(s.ents), tt.wlen)
// 			}
// 		}()
// 	}
// }

func TestStorageCreateSnapshot(t *testing.T) {
	ents := []pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}}
	cs := &pb.ConfState{Learners: []uint64{1, 2, 3}}
	data := []byte("data")

	tests := []struct {
		i uint64

		werr  error
		wsnap pb.Snapshot
	}{
		{4, nil, pb.Snapshot{Data: data, Metadata: pb.SnapshotMetadata{Index: 4, Term: 4, ConfState: *cs}}},
		{5, nil, pb.Snapshot{Data: data, Metadata: pb.SnapshotMetadata{Index: 5, Term: 5, ConfState: *cs}}},
	}

	for i, tt := range tests {
		func() {
			tmpDir, db, wal, err := getTmpBadgerWal(ents)
			if err != nil {
				t.Error(err)
				return
			}
			defer os.RemoveAll(tmpDir)
			defer db.Close()

			snap, err := wal.CreateSnapshot(tt.i, cs, data)
			if err != tt.werr {
				t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
			}
			if !reflect.DeepEqual(snap, tt.wsnap) {
				t.Errorf("#%d: snap = %+v, want %+v", i, snap, tt.wsnap)
			}
		}()
	}
}

func TestStorageAppend(t *testing.T) {
	ents := []pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}}
	tests := []struct {
		entries []pb.Entry

		werr     error
		wentries []pb.Entry
	}{
		{
			[]pb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},
			nil,
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},
		{
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
			nil,
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},
		{
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 6}, {Index: 5, Term: 6}},
			nil,
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 6}, {Index: 5, Term: 6}},
		},
		{
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 5}},
			nil,
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 5}},
		},
		// truncate incoming entries, truncate the existing entries and append
		{
			[]pb.Entry{{Index: 2, Term: 3}, {Index: 3, Term: 3}, {Index: 4, Term: 5}},
			nil,
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 5}},
		},
		// truncate the existing entries and append
		{
			[]pb.Entry{{Index: 4, Term: 5}},
			nil,
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 5}},
		},
		// direct append
		{
			[]pb.Entry{{Index: 6, Term: 5}},
			nil,
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 5}},
		},
	}

	for i, tt := range tests {
		func () {
			tmpDir, db, wal, err := getTmpBadgerWal(ents)
			if err != nil {
				t.Error(err)
				return
			}
			defer os.RemoveAll(tmpDir)
			defer db.Close()

			wb := db.NewWriteBatch()
			defer wb.Cancel()

			err = wal.writeEntries(wb, tt.entries)
			if err != tt.werr {
				t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
			}

			if err := wb.Flush(); err != nil {
				t.Error(err)
			}

			lo, _ := wal.FirstIndex()
			hi, _ := wal.LastIndex()
			entries, err := wal.getEntries(lo - 1, hi + 1, math.MaxUint64)
			if err != nil {
				t.Error(err)
			}
			
			if !reflect.DeepEqual(entries, tt.wentries) {
				t.Errorf("#%d: entries = %v, want %v", i, entries, tt.wentries)
			}
		}()
	}
}

// // func TestStorageApplySnapshot(t *testing.T) {
// // 	cs := &pb.ConfState{Voters: []uint64{1, 2, 3}}
// // 	data := []byte("data")

// // 	tests := []pb.Snapshot{{Data: data, Metadata: pb.SnapshotMetadata{Index: 4, Term: 4, ConfState: *cs}},
// // 		{Data: data, Metadata: pb.SnapshotMetadata{Index: 3, Term: 3, ConfState: *cs}},
// // 	}

// // 	s := NewMemoryStorage()

// // 	//Apply Snapshot successful
// // 	i := 0
// // 	tt := tests[i]
// // 	err := s.ApplySnapshot(tt)
// // 	if err != nil {
// // 		t.Errorf("#%d: err = %v, want %v", i, err, nil)
// // 	}

// // 	//Apply Snapshot fails due to ErrSnapOutOfDate
// // 	i = 1
// // 	tt = tests[i]
// // 	err = s.ApplySnapshot(tt)
// // 	if err != ErrSnapOutOfDate {
// // 		t.Errorf("#%d: err = %v, want %v", i, err, ErrSnapOutOfDate)
// // 	}
// // }