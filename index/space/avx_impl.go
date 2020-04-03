package space

import (
	"github.com/marekgalovic/anndb/math";
	"github.com/marekgalovic/anndb/simd/avx";
)

type avxSpaceImpl struct {}

func (avxSpaceImpl) EuclideanDistance(a, b math.Vector) float32 {
    return avx.EuclideanDistance(a, b)
}

func (avxSpaceImpl) ManhattanDistance(a, b math.Vector) float32 {
    return avx.ManhattanDistance(a, b)
}

func (avxSpaceImpl) CosineDistance(a, b math.Vector) float32 {
    return avx.CosineDistance(a, b)
}