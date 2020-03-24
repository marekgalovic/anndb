package index

import (
    "errors";
    "context";
    "sync";
    "sync/atomic";
    "unsafe";

    "github.com/marekgalovic/anndb/pkg/math";
    "github.com/marekgalovic/anndb/pkg/utils";

    // log "github.com/sirupsen/logrus";
)

const VERTICES_MAP_SHARD_COUNT int = 16

var (
    ItemNotFoundError error = errors.New("Item not found")
    ItemAlreadyExistsError error = errors.New("Item already exists")
    EmptyIndexError error = errors.New("Index is empty")
)

type hnsw struct {
    size uint
    space Space
	config hnswConfig

    len uint64
    vertices [VERTICES_MAP_SHARD_COUNT]map[uint64]*hnswVertex
    verticesMu [VERTICES_MAP_SHARD_COUNT]*sync.RWMutex

    entrypoint unsafe.Pointer
}

func NewHnsw(size uint, space Space, options ...HnswOption) *hnsw {
	index := &hnsw {
        size: size,
        space: space,
		config: newHnswConfig(options),

        len: 0,
        entrypoint: nil,
	}

    for i := 0; i < VERTICES_MAP_SHARD_COUNT; i++ {
        index.vertices[i] = make(map[uint64]*hnswVertex)
        index.verticesMu[i] = &sync.RWMutex{}
    }

    return index
}

func (this *hnsw) Len() int {
	return int(atomic.LoadUint64(&this.len));
}

func (this *hnsw) Insert(id uint64, value math.Vector) error {
    var vertex *hnswVertex
    if (*hnswVertex)(atomic.LoadPointer(&this.entrypoint)) == nil {
        vertex = newHnswVertex(id, value, 0)
        if err := this.storeVertex(vertex); err != nil {
            return err
        }
        if atomic.CompareAndSwapPointer(&this.entrypoint, nil, unsafe.Pointer(vertex)) {
            return nil
        } else {
            vertex.setLevel(this.randomLevel())   
        }
    } else {
        vertex = newHnswVertex(id, value, this.randomLevel())
        if err := this.storeVertex(vertex); err != nil {
            return err
        }
    }

    entrypoint := (*hnswVertex)(atomic.LoadPointer(&this.entrypoint))
    minDistance := this.space.Distance(vertex.vector, entrypoint.vector);
    for l := entrypoint.level; l > vertex.level; l-- {
        entrypoint, minDistance = this.greedyClosestNeighbor(vertex.vector, entrypoint, minDistance, l);
    }

    for l := math.MinInt(entrypoint.level, vertex.level); l >= 0; l-- {
        neighbors := this.searchLevel(vertex.vector, entrypoint, this.config.efConstruction, l)

        switch this.config.searchAlgorithm {
        case HnswSearchSimple:
            neighbors = this.selectNeighbors(neighbors, this.config.m)
        case HnswSearchHeuristic:
            neighbors = this.selectNeighborsHeuristic(vertex.vector, neighbors, this.config.m, l, true, true)
        }

        mMax := this.config.mMax
        if l == 0 {
            mMax = this.config.mMax0
        }

        for neighbors.Len() > 0 {
            item := neighbors.Pop()
            neighbor := item.Value().(*hnswVertex)
            entrypoint = neighbor

            vertex.addEdge(l, neighbor, item.Priority())
            neighbor.addEdge(l, vertex, item.Priority())

            if neighbor.edgesCount(l) > mMax {
                this.pruneNeighbors(neighbor, mMax, l)
            }
        }
    }

	return nil;
}

func (this *hnsw) Get(id uint64) (math.Vector, error) {
    m, mu := this.getVerticesShard(id)
    defer mu.RUnlock()
    mu.RLock()

    if vertex, exists := m[id]; exists {
        return vertex.vector, nil
    }
	return nil, ItemNotFoundError
}

func (this *hnsw) Remove(id uint64) error {
    vertex, err := this.removeVertex(id)
    if err != nil {
        return err
    }

    currEntrypoint := atomic.LoadPointer(&this.entrypoint)
    if (*hnswVertex)(currEntrypoint) == vertex {
        minDistance := math.MaxFloat
        var closestNeighbor *hnswVertex = nil

        for l := vertex.level; l >= 0; l-- {
            vertex.edgeMutexes[l].RLock()
            for neighbor, distance := range vertex.edges[l] {
                if distance < minDistance {
                    minDistance = distance
                    closestNeighbor = neighbor
                }
            }
            vertex.edgeMutexes[l].RUnlock()

            if closestNeighbor != nil {
                break
            }
        }
        atomic.CompareAndSwapPointer(&this.entrypoint, currEntrypoint, unsafe.Pointer(closestNeighbor))
    }

    for l := vertex.level; l >= 0; l-- {
        mMax := this.config.mMax
        if l == 0 {
            mMax = this.config.mMax0
        }

        vertex.edgeMutexes[l].RLock()
        neighbors := make([]*hnswVertex, len(vertex.edges[l]))
        i := 0
        for neighbor, _ := range vertex.edges[l] {
            neighbors[i] = neighbor
            i++
        }
        vertex.edgeMutexes[l].RUnlock()

        for _, neighbor := range neighbors {
            neighbor.removeEdge(l, vertex)
            this.pruneNeighbors(neighbor, mMax, l)
        }
    }

    return nil;
}

func (this *hnsw) Search(ctx context.Context, query math.Vector, k uint) (SearchResult, error) {
    entrypoint := (*hnswVertex)(atomic.LoadPointer(&this.entrypoint))
    if entrypoint == nil {
        return nil, EmptyIndexError
    }

    minDistance := this.space.Distance(query, entrypoint.vector)
    for l := entrypoint.level; l > 0; l-- {
        entrypoint, minDistance = this.greedyClosestNeighbor(query, entrypoint, minDistance, l)
    }

    ef := math.MaxInt(this.config.ef, int(k))
    neighbors := this.searchLevel(query, entrypoint, ef, 0)
    n := math.MinInt(int(k), neighbors.Len())

    result := make(SearchResult, n)
    for i := n-1; i >= 0; i-- {
        item := neighbors.Pop()
        result[i].Id = item.Value().(*hnswVertex).id
        result[i].Score = item.Priority()
    }
	
    return result, nil
}

func (this *hnsw) getVerticesShard(id uint64) (map[uint64]*hnswVertex, *sync.RWMutex) {
    return this.vertices[id % uint64(VERTICES_MAP_SHARD_COUNT)], this.verticesMu[id % uint64(VERTICES_MAP_SHARD_COUNT)]
}

func (this *hnsw) storeVertex(vertex *hnswVertex) error {
    m, mu := this.getVerticesShard(vertex.id)
    defer mu.Unlock()
    mu.Lock()

    if _, exists := m[vertex.id]; exists {
        return ItemAlreadyExistsError
    }

    m[vertex.id] = vertex
    atomic.AddUint64(&this.len, 1);
    return nil
}

func (this *hnsw) removeVertex(id uint64) (*hnswVertex, error) {
    m, mu := this.getVerticesShard(id)
    defer mu.Unlock()
    mu.Lock()

    if vertex, exists := m[id]; exists {
        delete(m, id);
        atomic.AddUint64(&this.len, ^uint64(0));
        atomic.StoreUint32(&vertex.deleted, 1)
        return vertex, nil
    }

    return nil, ItemNotFoundError
}

func (this *hnsw) randomLevel() int {
    return math.Floor(math.RandomExponential(this.config.levelMultiplier))
}

func (this *hnsw) greedyClosestNeighbor(query math.Vector, entrypoint *hnswVertex, minDistance float32, level int) (*hnswVertex, float32) {
    for {
        var closestNeighbor *hnswVertex

        entrypoint.edgeMutexes[level].RLock()
        for neighbor, _ := range entrypoint.edges[level] {
            if atomic.LoadUint32(&neighbor.deleted) == 1 {
                continue
            }
            if distance := this.space.Distance(query, neighbor.vector); distance < minDistance {
                minDistance = distance
                closestNeighbor = neighbor
            }
        }
        entrypoint.edgeMutexes[level].RUnlock()

        if closestNeighbor == nil {
            break
        }
        entrypoint = closestNeighbor
    }

    return entrypoint, minDistance
}

func (this *hnsw) searchLevel(query math.Vector, entrypoint *hnswVertex, ef, level int) utils.PriorityQueue {
    entrypointDistance := this.space.Distance(query, entrypoint.vector)

    pqItem := utils.NewPriorityQueueItem(entrypointDistance, entrypoint)
    candidateVertices := utils.NewMinPriorityQueue(pqItem)
    resultVertices := utils.NewMaxPriorityQueue(pqItem)

    visitedVertices := make(map[*hnswVertex]struct{}, ef * this.config.mMax0)
    visitedVertices[entrypoint] = struct{}{}

    for candidateVertices.Len() > 0 {
        candidateItem := candidateVertices.Pop()
        candidate := candidateItem.Value().(*hnswVertex)
        lowerBound := resultVertices.Peek().Priority()

        if candidateItem.Priority() > lowerBound {
            break   
        }

        candidate.edgeMutexes[level].RLock()
        for neighbor, _ := range candidate.edges[level] {
            if atomic.LoadUint32(&neighbor.deleted) == 1 {
                continue
            }
            if _, exists := visitedVertices[neighbor]; exists {
                continue
            }
            visitedVertices[neighbor] = struct{}{}

            distance := this.space.Distance(query, neighbor.vector)
            if (distance < lowerBound) || (resultVertices.Len() < ef) {
                pqItem := utils.NewPriorityQueueItem(distance, neighbor)
                candidateVertices.Push(pqItem)
                resultVertices.Push(pqItem)

                if resultVertices.Len() > ef {
                    resultVertices.Pop()
                }
            }
        }
        candidate.edgeMutexes[level].RUnlock()
    }

    // MaxPriorityQueue
    return resultVertices
}

func (this *hnsw) selectNeighbors(neighbors utils.PriorityQueue, k int) utils.PriorityQueue {
    for neighbors.Len() > k {
        neighbors.Pop()
    }

    return neighbors
}

func (this *hnsw) selectNeighborsHeuristic(query math.Vector, neighbors utils.PriorityQueue, k, level int, extendCandidates, keepPruned bool) utils.PriorityQueue {
    candidateVertices := neighbors.Reverse()  // MinPriorityQueue

    existingCandidatesSize := neighbors.Len()
    if extendCandidates {
        existingCandidatesSize += neighbors.Len() * this.config.mMax0
    }
    existingCandidates := make(map[*hnswVertex]struct{}, existingCandidatesSize)
    for _, neighbor := range neighbors.ToSlice() {
        existingCandidates[neighbor.Value().(*hnswVertex)] = struct{}{}
    }

    if extendCandidates {
        for neighbors.Len() > 0 {
            candidate := neighbors.Pop().Value().(*hnswVertex)

            candidate.edgeMutexes[level].RLock()
            for neighbor, _ := range candidate.edges[level] {
                if atomic.LoadUint32(&neighbor.deleted) == 1 {
                    continue
                }
                if _, exists := existingCandidates[neighbor]; exists {
                    continue
                }
                existingCandidates[neighbor] = struct{}{}

                distance := this.space.Distance(query, neighbor.vector)
                candidateVertices.Push(utils.NewPriorityQueueItem(distance, neighbor))
            }
            candidate.edgeMutexes[level].RUnlock()
        }
    }

    result := utils.NewMaxPriorityQueue()
    for (candidateVertices.Len() > 0) && (result.Len() < k) {
        result.Push(candidateVertices.Pop())
    }

    if keepPruned {
        for candidateVertices.Len() > 0 {
            result.Push(candidateVertices.Pop())

            if result.Len() >= k {
                break
            }
        }
    }

    return result
}

func (this *hnsw) pruneNeighbors(vertex *hnswVertex, k, level int) {
    neighborsQueue := utils.NewMaxPriorityQueue()

    vertex.edgeMutexes[level].RLock()
    for neighbor, distance := range vertex.edges[level] {
        if atomic.LoadUint32(&neighbor.deleted) == 1 {
            continue
        }
        neighborsQueue.Push(utils.NewPriorityQueueItem(distance, neighbor))
    }
    vertex.edgeMutexes[level].RUnlock()

    switch this.config.searchAlgorithm {
    case HnswSearchSimple:
        neighborsQueue = this.selectNeighbors(neighborsQueue, this.config.m)
    case HnswSearchHeuristic:
        neighborsQueue = this.selectNeighborsHeuristic(vertex.vector, neighborsQueue, this.config.m, level, true, true)
    }

    newNeighbors := make(hnswEdgeSet, neighborsQueue.Len())
    for _, item := range neighborsQueue.ToSlice() {
        newNeighbors[item.Value().(*hnswVertex)] = item.Priority()
    }

    vertex.setEdges(level, newNeighbors)
}