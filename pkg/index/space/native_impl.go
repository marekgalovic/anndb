package space

import (
	"github.com/marekgalovic/anndb/pkg/math";
)

type nativeSpaceImpl struct {}

func (nativeSpaceImpl) EuclideanDistance(a, b math.Vector) float32 {
    var distance float32
    for i := 0; i < len(a); i++ {
        distance += math.Square(a[i] - b[i])
    }
    
    return math.Sqrt(distance)
}

func (nativeSpaceImpl) ManhattanDistance(a, b math.Vector) float32 {
    var distance float32
    for i := 0; i < len(a); i++ {
        distance += math.Abs(a[i] - b[i])
    }

    return distance
}

func (nativeSpaceImpl) CosineDistance(a, b math.Vector) float32 {
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