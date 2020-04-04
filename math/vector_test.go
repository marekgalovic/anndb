package math

import (
    "bytes";
    "testing";

    "github.com/stretchr/testify/assert";
)

func TestVectorSaveAndLoad(t *testing.T) {
    vec := make(Vector, 32)
    for i := 0; i < 32; i++ {
        vec[i] = float32(i * 2) + float32(i) / 2.0
    }

    var buf bytes.Buffer
    err := vec.Save(&buf);
    assert.Nil(t, err)

    otherVec := make(Vector, 32)
    err = otherVec.Load(&buf)
    assert.Nil(t, err)

    assert.Equal(t, vec, otherVec)
}

func TestZerosVector(t *testing.T) {
    vec := ZerosVector(16)

    assert.Equal(t, 16, len(vec))
    for _, e := range vec {
        assert.Equal(t, float32(0), e)
    }
}

func TestOnesVector(t *testing.T) {
    vec := OnesVector(16)

    assert.Equal(t, 16, len(vec))
    for _, e := range vec {
        assert.Equal(t, float32(1), e)
    }   
}

func TestDot(t *testing.T) {
    vecA := Vector{1, 2, 3}
    vecB := Vector{4, 5, 6}

    assert.Equal(t, float32(32), Dot(vecA, vecB))
}

func TestLength(t *testing.T) {
    vec := Vector{1, 1, 1, 1}

    assert.Equal(t, float32(2), Length(vec))
}

func TestVectorAdd(t *testing.T) {
    vecA := Vector{1, 2, 3, 4}
    vecB := Vector{5, 6, 7, 8}

    assert.Equal(t, Vector{6, 8, 10, 12}, VectorAdd(vecA, vecB))
}

func TestVectorSubtract(t *testing.T) {
    vecA := Vector{1, 2, 3, 4}
    vecB := Vector{5, 6, 7, 8}

    assert.Equal(t, Vector{-4, -4, -4, -4}, VectorSubtract(vecA, vecB))
}

func TestVectorMultiply(t *testing.T) {
    vecA := Vector{1, 2, 3, 4}
    vecB := Vector{5, 6, 7, 8}

    assert.Equal(t, Vector{5, 12, 21, 32}, VectorMultiply(vecA, vecB))
}

func TestVectorDivide(t *testing.T) {
    vecA := Vector{1, 2, 3, 4}
    vecB := Vector{5, 6, 7, 8}

    assert.Equal(t, Vector{1.0 / 5, 2.0 / 6, 3.0 / 7, 4.0 / 8}, VectorDivide(vecA, vecB))
}

func TestVectorScalarAdd(t *testing.T) {
    vec := Vector{1, 2, 3, 4}

    assert.Equal(t, Vector{11, 12, 13, 14}, VectorScalarAdd(vec, 10))
}

func TestVectorScalarSubtract(t *testing.T) {
    vec := Vector{1, 2, 3, 4}

    assert.Equal(t, Vector{-9, -8, -7, -6}, VectorScalarSubtract(vec, 10))
}

func TestVectorScalarMultiply(t *testing.T) {
    vec := Vector{1, 2, 3, 4}

    assert.Equal(t, Vector{10, 20, 30, 40}, VectorScalarMultiply(vec, 10))
}

func TestVectorScalarDivide(t *testing.T) {
    vec := Vector{1, 2, 3, 4}

    assert.Equal(t, Vector{0.1, 0.2, 0.3, 0.4}, VectorScalarDivide(vec, 10))
}