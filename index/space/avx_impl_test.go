package space

import (
    "testing";

    "github.com/marekgalovic/anndb/math";

    "github.com/stretchr/testify/assert";
)

func TestAvxEuclideanDistance(t *testing.T) {
    var impl avxSpaceImpl
    distance := impl.EuclideanDistance(
        math.Vector{1, 2, 3, 4, 5, 6, 7, 8},
        math.Vector{1, 2, 3, 4, 5, 6, 7, 8},
    )
    assert.Equal(t, float32(0), distance)

    distance = impl.EuclideanDistance(
        math.Vector{1, 1, 1, 1, 1, 0, 0, 2},
        math.Vector{0, 0, 0, 0, 0, 0, 0, 0},
    )
    assert.Equal(t, float32(3), distance)
}

func TestAvxEuclideanDistanceUnaligned(t *testing.T) {
    var impl avxSpaceImpl
    distance := impl.EuclideanDistance(
        math.Vector{1, 2, 3},
        math.Vector{1, 2, 3},
    )
    assert.Equal(t, float32(0), distance)

    distance = impl.EuclideanDistance(
        math.Vector{1, 2, 2},
        math.Vector{0, 0, 0},
    )
    assert.Equal(t, float32(3), distance)
}

func TestAvxManhattanDistance(t *testing.T) {
    var impl avxSpaceImpl
    distance := impl.ManhattanDistance(
        math.Vector{1, 2, 3, 4, 5, 6, 7, 8},
        math.Vector{1, 2, 3, 4, 5, 6, 7, 8},
    )
    assert.Equal(t, float32(0), distance)

    distance = impl.ManhattanDistance(
        math.Vector{1, 2, 3, 4, 5, 6, 7, 8},
        math.Vector{0, 0, 0, 0, 0, 0, 0, 0},
    )
    assert.Equal(t, float32(36), distance)
}

func TestAvxManhattanDistanceUnaligned(t *testing.T) {
    var impl avxSpaceImpl
    distance := impl.ManhattanDistance(
        math.Vector{1, 2, 3},
        math.Vector{1, 2, 3},
    )
    assert.Equal(t, float32(0), distance)

    distance = impl.ManhattanDistance(
        math.Vector{1, 2, 3},
        math.Vector{0, 0, 0},
    )
    assert.Equal(t, float32(6), distance)
}

func TestAvxCosineDistance(t *testing.T) {
    var impl avxSpaceImpl
    distance := impl.CosineDistance(
        math.Vector{1, 1, 1, 1, 1, 1, 1, 1},
        math.Vector{1, 1, 1, 1, 1, 1, 1, 1},
    )
    assert.True(t, math.Abs(distance) <= 1e-5)

    distance = impl.CosineDistance(
        math.Vector{0, 1, 0, 0, 0, 0, 0, 0},
        math.Vector{1, 0, 0, 0, 0, 0, 0, 0},
    )

    assert.True(t, math.Abs(1 - distance) <= 1e-5)

    distance = impl.CosineDistance(
        math.Vector{-1, 0, 0, 0, 0, 0, 0, 0},
        math.Vector{1, 0, 0, 0, 0, 0, 0, 0},
    )
    assert.True(t, math.Abs(2 - distance) <= 1e-5)
}

func TestAvxCosineDistanceUnaligned(t *testing.T) {
    var impl avxSpaceImpl
    distance := impl.CosineDistance(
        math.Vector{1, 1},
        math.Vector{1, 1},
    )
    assert.True(t, math.Abs(distance) <= 1e-5)

    distance = impl.CosineDistance(
        math.Vector{0, 1},
        math.Vector{1, 0},
    )

    assert.True(t, math.Abs(1 - distance) <= 1e-5)

    distance = impl.CosineDistance(
        math.Vector{-1, 0},
        math.Vector{1, 0},
    )
    assert.True(t, math.Abs(2 - distance) <= 1e-5)
}