package main

import (
	"math/rand";
	"context";
	"time";
	"sync/atomic";
	"runtime";

	"github.com/marekgalovic/anndb/pkg/index";
	"github.com/marekgalovic/anndb/pkg/index/space";
	"github.com/marekgalovic/anndb/pkg/math";

	log "github.com/sirupsen/logrus";
)

const dim int = 512

var inserts uint64 = 0
var deletes uint64 = 0

func worker(ctx context.Context, index index.Index, tasks <- chan int64) {
	var err error
	for {
		select {
		case id := <- tasks:
			if id >= 0 {
				err = index.Insert(uint64(id), math.RandomUniformVector(dim))
				if err == nil {
					atomic.AddUint64(&inserts, 1)
				}
			} else {
				err = index.Remove(uint64(-id))
				if err == nil {
					atomic.AddUint64(&deletes, 1)
				}
			}
		case <- ctx.Done():
			break
		}
	}
}

func main() {
	for numThreads := 16; numThreads <= runtime.NumCPU(); numThreads++ {
		ctx, cancel := context.WithCancel(context.Background())

		idx := index.NewHnsw(uint(dim), space.NewEuclidean());
		queue := make(chan int64)
		for i := 0; i < numThreads; i++ {
			go worker(ctx, idx, queue)
		}

		startAt := time.Now()
		n := 100000
		for i := 0; i < n; i++ {
			if (i > 100) && (math.RandomUniform() <= 0.2) {
				queue <- -rand.Int63n(int64(i-100))
			} else {
				queue <- int64(i)
			}
		}

		time.Sleep(1 * time.Second)
		cancel()

		log.Infof("[%d, %.1f],", numThreads, float64(n) / float64(time.Since(startAt).Seconds()))
	}
}	