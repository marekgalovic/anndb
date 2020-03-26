package space

import (
    "testing";

    "github.com/stretchr/testify/assert";
)

func TestEuclideanDistance(t *testing.T) {
    distance := EuclideanDistance(
        Vector{1, 2, 3},
        Vector{1, 2, 3},
    )
    assert.Equal(t, float32(0), distance)

    distance = EuclideanDistance(
        Vector{1, 2, 2},
        Vector{0, 0, 0},
    )
    assert.Equal(t, float32(3), distance)
}

func TestManhattanDistance(t *testing.T) {
    distance := ManhattanDistance(
        Vector{1, 2, 3},
        Vector{1, 2, 3},
    )
    assert.Equal(t, float32(0), distance)

    distance = ManhattanDistance(
        Vector{1, 2, 3},
        Vector{0, 0, 0},
    )
    assert.Equal(t, float32(6), distance)
}

func TestCosineDistance(t *testing.T) {
    distance := CosineDistance(
        Vector{1, 1},
        Vector{1, 1},
    )
    assert.True(t, Abs(1 - distance) <= 1e-5)

    distance = CosineDistance(
        Vector{0, 1},
        Vector{1, 0},
    )
    assert.True(t, Abs(0 - distance) <= 1e-5)
}