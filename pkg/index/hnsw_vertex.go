package index

import (
    "sync";
    
    "github.com/marekgalovic/anndb/pkg/math";
)

type hnswEdgeSet map[*hnswVertex]float32

type hnswVertex struct {
    id uint64
    vector math.Vector
    level int
    edges []hnswEdgeSet
    edgeMutexes []*sync.RWMutex
}

func newHnswVertex(id uint64, vector math.Vector, level int) *hnswVertex {
    vertex := &hnswVertex {
        id: id,
        vector: vector,
        level: level,
    }
    vertex.setLevel(level)

    return vertex
}

func (this *hnswVertex) setLevel(level int) {
    this.edges = make([]hnswEdgeSet, level + 1)
    this.edgeMutexes = make([]*sync.RWMutex, level + 1)

    for i := 0; i <= level; i++ {
        this.edges[i] = make(hnswEdgeSet)
        this.edgeMutexes[i] = &sync.RWMutex{}
    }
}

func (this *hnswVertex) edgesCount(level int) int {
    defer this.edgeMutexes[level].RUnlock()
    this.edgeMutexes[level].RLock()

    return len(this.edges[level])
}

func (this *hnswVertex) addEdge(level int, edge *hnswVertex, distance float32) {
    defer this.edgeMutexes[level].Unlock()
    this.edgeMutexes[level].Lock()

    this.edges[level][edge] = distance
}

func (this *hnswVertex) removeEdge(level int, edge *hnswVertex) {
    defer this.edgeMutexes[level].Unlock()
    this.edgeMutexes[level].Lock()

    delete(this.edges[level], edge);
}

func (this *hnswVertex) getEdges(level int) hnswEdgeSet {
    defer this.edgeMutexes[level].RUnlock()
    this.edgeMutexes[level].RLock()

    return this.edges[level]
}

func (this *hnswVertex) setEdges(level int, edges hnswEdgeSet) {
    defer this.edgeMutexes[level].Unlock()
    this.edgeMutexes[level].Lock()

    this.edges[level] = edges
}