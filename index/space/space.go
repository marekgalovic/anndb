package space

import (
    "github.com/marekgalovic/anndb/math";

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

type Euclidean struct { space }

type Manhattan struct { space }

type Cosine struct { space }

func NewEuclidean() Space {
    return &Euclidean{newSpace()}
}

func (this *Euclidean) Distance(a, b math.Vector) float32 {
    return this.impl.EuclideanDistance(a, b)
}

func NewManhattan() Space {
    return &Manhattan{newSpace()}
}

func (this *Manhattan) Distance(a, b math.Vector) float32 {
    return this.impl.ManhattanDistance(a, b)
}

func NewCosine() Space {
    return &Cosine{newSpace()}
}

func (this *Cosine) Distance(a, b math.Vector) float32 {
    return this.impl.CosineDistance(a, b)
}

