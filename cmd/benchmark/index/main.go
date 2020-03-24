package main

import (
	"math/rand";
	"context";
	"time";
	"sync/atomic";
	"runtime";

	"github.com/marekgalovic/anndb/pkg/index";
	"github.com/marekgalovic/anndb/pkg/math";

	log "github.com/sirupsen/logrus";
)

var inserts uint64 = 0
var deletes uint64 = 0

func worker(ctx context.Context, index index.Index, tasks <- chan int64) {
	var err error
	for {
		select {
		case id := <- tasks:
			if id >= 0 {
				err = index.Insert(uint64(id), math.RandomUniformVector(32))
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
	ctx, cancel := context.WithCancel(context.Background())

	idx := index.NewHnsw(32, index.NewEuclideanSpace());
	queue := make(chan int64)
	for i := 0; i < runtime.NumCPU(); i++ {
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

	time.Sleep(2 * time.Second)

	expectedSize := atomic.LoadUint64(&inserts) - atomic.LoadUint64(&deletes)
	log.Infof("Expected size: %d", expectedSize)
	log.Infof("Size: %d", idx.Len())
	log.Infof("OPs/s: %.2f", float64(n) / float64(time.Since(startAt).Seconds()))
	cancel()
}	