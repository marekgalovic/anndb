package math

import (
    "testing";

    "github.com/stretchr/testify/assert";
)

func TestAbs(t *testing.T) {
    assert.Equal(t, float32(0), Abs(0))
    assert.Equal(t, float32(1), Abs(1))
    assert.Equal(t, float32(1), Abs(-1))
}

func TestPow(t *testing.T) {
    assert.Equal(t, float32(3), Pow(3, 1))
    assert.Equal(t, float32(9), Pow(-3, 2))
    assert.Equal(t, float32(27), Pow(3, 3))
}

func TestSquare(t *testing.T) {
    assert.Equal(t, float32(4), Square(2))
    assert.Equal(t, float32(4), Square(-2))
}

func TestSqrt(t *testing.T) {
    assert.Equal(t, float32(2), Sqrt(4))
    assert.Equal(t, float32(4), Sqrt(16))
}

func TestLog(t *testing.T) {
    assert.Equal(t, float32(0), Log(1))
}

func TestTrunc(t *testing.T) {
    assert.Equal(t, 2, Trunc(2.0))
    assert.Equal(t, 2, Trunc(2.5))
    assert.Equal(t, 2, Trunc(2.9))
    assert.Equal(t, 3, Trunc(3.1))
}

func TestFloor(t *testing.T) {
    assert.Equal(t, 2, Floor(2.1))
    assert.Equal(t, 2, Floor(2.9))
    assert.Equal(t, 3, Floor(3.0))
}

func TestMin(t *testing.T) {
    assert.Equal(t, float32(0), Min(0, 12, 4, 18))
    assert.Equal(t, float32(-5), Min(0, 12, -5, 18))
}

func TestMax(t *testing.T) {
    assert.Equal(t, float32(18), Max(0, 12, 4, 18))
    assert.Equal(t, float32(-2), Max(-2, -4, -10, -15))   
}

func TestEquidistanPlane(t *testing.T) {
    ehp := EquidistantPlane(
        Vector{1, 2, 3},
        Vector{0, 1, 2},
    )

    assert.Equal(t, Vector{-1, -1, -1}, ehp[:3])
    assert.Equal(t, float32(-4.5), ehp[3])
}