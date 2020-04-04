package math

import (
    goMath "math";
)

var (
    parallelThreshold = 100000
    numRoutines = 4
)

func SetParallelThreshold(threshold int) { parallelThreshold = threshold }

func SetNumRoutines(n int) { numRoutines = n }

const MaxFloat = float32(goMath.MaxFloat32)
const MaxIntVal = int((^uint(0)) >> 1) 
const MinIntVal = -MaxIntVal - 1

func Abs(x float32) float32 {
    return float32(goMath.Abs(float64(x)))
}

func Pow(x, power float32) float32 {
    // Slow
    return float32(goMath.Pow(float64(x), float64(power)))
}

func Square(x float32) float32 {
    return x * x
}

func Sqrt(x float32) float32 {
    return float32(goMath.Sqrt(float64(x)))
}

func Log(x float32) float32 {
    return float32(goMath.Log(float64(x)))
}

func Trunc(x float32) int {
    return int(goMath.Trunc(float64(x)))
}

func Floor(x float32) int {
    return int(goMath.Floor(float64(x)))
}

func Min(values ...float32) float32 {
    min := MaxFloat
    for _, value := range values {
        if value < min {
            min = value
        }
    }
    return min
}

func MinInt(values ...int) int {
    min := MaxIntVal
    for _, value := range values {
        if value < min {
            min = value
        }
    }
    return min
}

func Max(values ...float32) float32 {
    max := -MaxFloat
    for _, value := range values {
        if value > max {
            max = value
        }
    }
    return max
}

func MaxInt(values ...int) int {
    max := -MaxIntVal
    for _, value := range values {
        if value > max {
            max = value
        }
    }
    return max
}