package space

import (
    "github.com/marekgalovic/anndb/pkg/math";

    "github.com/klauspost/cpuid";
)

type SpaceImpl interface {
    EuclideanDistance(math.Vector, math.Vector) float32
    ManhattanDistance(math.Vector, math.Vector) float32
    CosineDistance(math.Vector, math.Vector) float32
}

type Space interface {
    Distance(math.Vector, math.Vector) float32
}

type space struct {
    impl SpaceImpl
}

func newSpace() space {
    if cpuid.CPU.AVX() {
        return space {impl: avxSpaceImpl{}}
    }
    if cpuid.CPU.SSE() {
        return space {impl: sseSpaceImpl{}}
    }

    return space {impl: nativeSpaceImpl{}}
}

type euclideanSpace struct { space }

type manhattanSpace struct { space }

type cosineSpace struct { space }

func NewEuclidean() Space {
    return &euclideanSpace{newSpace()}
}

func (this *euclideanSpace) Distance(a, b math.Vector) float32 {
    return this.impl.EuclideanDistance(a, b)
}

func NewManhattan() Space {
    return &manhattanSpace{newSpace()}
}

func (this *manhattanSpace) Distance(a, b math.Vector) float32 {
    return this.impl.ManhattanDistance(a, b)
}

func NewCosine() Space {
    return &cosineSpace{newSpace()}
}

func (this *cosineSpace) Distance(a, b math.Vector) float32 {
    return this.impl.CosineDistance(a, b)
}

