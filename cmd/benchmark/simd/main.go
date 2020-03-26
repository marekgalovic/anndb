package main

import (
	"time";
	"github.com/marekgalovic/anndb/pkg/simd/avx2";
	"github.com/marekgalovic/anndb/pkg/index";
	log "github.com/sirupsen/logrus";
)

func main() {
	// a := [8]float32{}
	// b := [8]float32{}
	// for i := 0; i < 8; i++ {
	// 	a[i] = float32(i)
	// 	b[i] = float32(i * 2)
	// }

	// log.Info(simd.SubtractAndSquare(a, b))
	n := 512
	a := make([]float32, n)
	b := make([]float32, n)
	for i := 0; i < n; i++ {
		a[i] = float32(i)
		b[i] = float32(i*2)
	}

	log.Info(avx2.EuclideanDistance(a, b))
	log.Info(index.EuclideanDistance(a, b))

	loops := 100000
	t := time.Now()
	for i := 0; i < loops; i++ {
		avx2.EuclideanDistance(a, b)
	}
	log.Infof("GO Vectorized: %.4fms", time.Since(t).Seconds() * 1000)
	t = time.Now()
	for i := 0; i < loops; i++ {
		index.EuclideanDistance(a, b)
	}
	log.Infof("GO Native: %.4fms", time.Since(t).Seconds() * 1000)
}