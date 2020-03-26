package space

import (
	"github.com/marekgalovic/anndb/pkg/math";
	"github.com/marekgalovic/anndb/pkg/simd/avx2";
)

type avxSpaceImpl struct {}

func (avxSpaceImpl) EuclideanDistance(a, b math.Vector) float32 {
    return avx2.EuclideanDistance(a, b)
}

func (avxSpaceImpl) ManhattanDistance(a, b math.Vector) float32 {
    return avx2.ManhattanDistance(a, b)
}

func (avxSpaceImpl) CosineDistance(a, b math.Vector) float32 {
    return avx2.CosineDistance(a, b)
}