package index

import (
    "github.com/marekgalovic/anndb/pkg/math";
    "github.com/marekgalovic/anndb/pkg/simd/avx2";
)

type Space interface {
    Distance(math.Vector, math.Vector) float32
    // ToProto() pb.SpaceType
}

type space struct {
    // spaceType pb.SpaceType
}

type euclideanSpace struct { space }

type manhattanSpace struct { space }

type cosineSpace struct { space }

// func (s *space) ToProto() pb.SpaceType {
//     return s.spaceType
// }

// func NewSpaceFromProto(spaceType pb.SpaceType) Space {
//     switch spaceType {
//     case pb.SpaceType_EUCLIDEAN:
//         return NewEuclideanSpace()
//     case pb.SpaceType_MANHATTAN:
//         return NewManhattanSpace()
//     case pb.SpaceType_COSINE:
//         return NewCosineSpace()
//     default:
//         panic("Invalid space type")
//     }
// }

func NewEuclideanSpace() Space {
    return &euclideanSpace{space{}}
}

func (s *euclideanSpace) Distance(a, b math.Vector) float32 {
    return avx2.EuclideanDistance(a, b)
    // return EuclideanDistance(a, b)
}

func EuclideanDistance(a, b math.Vector) float32 {
    var distance float32
    for i := 0; i < len(a); i++ {
        distance += math.Square(a[i] - b[i])
    }
    
    return math.Sqrt(distance)
}

func NewManhattanSpace() Space {
    return &manhattanSpace{space{}}
}

func (s *manhattanSpace) Distance(a, b math.Vector) float32 {
    return avx2.ManhattanDistance(a, b)
    // return ManhattanDistance(a, b)
}

func ManhattanDistance(a, b math.Vector) float32 {
    var distance float32
    for i := 0; i < len(a); i++ {
        distance += math.Abs(a[i] - b[i])
    }

    return distance
}

func NewCosineSpace() Space {
    return &cosineSpace{space{}}
}

func (s *cosineSpace) Distance(a, b math.Vector) float32 {
    return CosineDistance(a, b)
}

func CosineDistance(a, b math.Vector) float32 {
    var dot float32
    var aNorm float32
    var bNorm float32
    for i := 0; i < len(a); i++ {
        dot += a[i] * b[i]
        aNorm += math.Square(a[i])
        bNorm += math.Square(b[i])
    }

    return dot / (math.Sqrt(aNorm) * math.Sqrt(bNorm))
}