package main

import (
	"time";
	"github.com/marekgalovic/anndb/simd/avx";
	"github.com/marekgalovic/anndb/simd/sse";
	log "github.com/sirupsen/logrus";
)

func main() {
	n := 512
	a := make([]float32, n)
	b := make([]float32, n)
	for i := 0; i < n; i++ {
		a[i] = float32(i)
		b[i] = float32(i*2)
	}

	log.Info(avx.EuclideanDistance(a, b))
	// log.Info(index.EuclideanDistance(a, b))

	loops := 1000000
	t := time.Now()
	for i := 0; i < loops; i++ {
		avx.EuclideanDistance(a, b)
	}
	log.Infof("GO Vectorized - AVX: %.4fms", time.Since(t).Seconds() * 1000)

	t = time.Now()
	for i := 0; i < loops; i++ {
		sse.EuclideanDistance(a, b)
	}
	log.Infof("GO Vectorized - SSE: %.4fms", time.Since(t).Seconds() * 1000)
	// t = time.Now()
	// for i := 0; i < loops; i++ {
	// 	index.EuclideanDistance(a, b)
	// }
	// log.Infof("GO Native: %.4fms", time.Since(t).Seconds() * 1000)
}