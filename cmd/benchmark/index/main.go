package main

import (
	// "math/rand";
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/marekgalovic/anndb/index"
	"github.com/marekgalovic/anndb/index/space"
	"github.com/marekgalovic/anndb/math"
	"github.com/marekgalovic/anndb/utils"

	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

const dim int = 50

var inserts uint64 = 0
var deletes uint64 = 0

type workerTask struct {
	insert bool
	id     uuid.UUID
}

func worker(ctx context.Context, index *index.Hnsw, tasks <-chan *workerTask, wg *sync.WaitGroup) {
	defer wg.Done()

	var err error
	for {
		select {
		case t := <-tasks:
			if t.insert {
				err = index.Insert(t.id, math.RandomUniformVector(dim), nil, index.RandomLevel())
				if err == nil {
					atomic.AddUint64(&inserts, 1)
				}
			} else {
				err = index.Remove(t.id)
				if err == nil {
					atomic.AddUint64(&deletes, 1)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	numThreads := runtime.NumCPU()
	ctx, cancel := context.WithCancel(context.Background())

	idx := index.NewHnsw(uint(dim), space.NewEuclidean())
	queue := make(chan *workerTask)
	wg := &sync.WaitGroup{}
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go worker(ctx, idx, queue, wg)
	}

	insertIds := make(map[uuid.UUID]struct{})
	startAt := time.Now()
	n := 500200
	for i := 0; i < n; i++ {
		id := uuid.NewV4()
		insertIds[id] = struct{}{}
		queue <- &workerTask{true, id}
		// if (i > 100) && (math.RandomUniform() <= 0.2) {
		// 	var id uuid.UUID
		// 	for k := range insertIds {
		// 		id = k
		// 		break
		// 	}
		// 	delete(insertIds, id)
		// 	queue <- &workerTask{false, id}
		// } else {
		// 	id := uuid.NewV4()
		// 	insertIds[id] = struct{}{}
		// 	queue <- &workerTask{true, id}
		// }
	}

	cancel()
	wg.Wait()

	log.Infof("[%d, %.1f],", numThreads, float64(n)/float64(time.Since(startAt).Seconds()))
	<-utils.InterruptSignal()
}
