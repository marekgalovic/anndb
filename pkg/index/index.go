package index

import (
	// "io";
	"context";

	"github.com/marekgalovic/anndb/pkg/math";
)

type Index interface {
	Len() int
	Insert(uint64, math.Vector) error
	Get(uint64) (math.Vector, error)
	Remove(uint64) error
	Search(context.Context, math.Vector) (SearchResult, error)
}

// type Index interface {
//     Build(context.Context)
//     Search(context.Context, int, math.Vector) SearchResult
//     ByteSize() int
//     Len() int
//     Size() int
//     Reset()
//     Items() map[int64]math.Vector
//     Add(int64, math.Vector) error
//     Get(int64) math.Vector
//     Load(io.Reader) error
//     Save(io.Writer) error
//     ToProto() *pb.Index
// }