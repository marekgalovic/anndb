package space

import (
	"github.com/marekgalovic/anndb/math";
	"github.com/marekgalovic/anndb/simd/sse";
)

type sseSpaceImpl struct {}

func (sseSpaceImpl) EuclideanDistance(a, b math.Vector) float32 {
    return sse.EuclideanDistance(a, b)
}

func (sseSpaceImpl) ManhattanDistance(a, b math.Vector) float32 {
    return sse.ManhattanDistance(a, b)
}

func (sseSpaceImpl) CosineDistance(a, b math.Vector) float32 {
    return sse.CosineDistance(a, b)
}