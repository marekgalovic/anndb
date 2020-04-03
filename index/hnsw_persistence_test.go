package index

import (
    "testing";
	"errors";
    "bytes";
    "sync/atomic";
    "math/rand";

    "github.com/marekgalovic/anndb/math";
    "github.com/marekgalovic/anndb/index/space";

    "github.com/stretchr/testify/assert";
)

func hnswIsSame(a, b *Hnsw) error {
    if a.Len() != b.Len() {
    	return errors.New("Length missmatch")
    }

    for i, shard := range a.vertices {
        otherShard := b.vertices[i]
        if len(shard) != len(otherShard) {
        	return errors.New("Not same shard len")
        }
        for _, vertex := range shard {
            otherVertex, exists := otherShard[vertex.id];
            if !exists {
            	return errors.New("Other vertex does not exist")
            }
            if vertex.level != otherVertex.level {
            	return errors.New("Other vertex level does not match")
            }
            if len(vertex.vector) != len(otherVertex.vector) {
            	return errors.New("Other vertex vector size does not match")
            }
            for j := 0; j < len(vertex.vector); j++ {
            	if math.Abs(vertex.vector[j] - otherVertex.vector[j]) > 1e-4 {
            		return errors.New("Other vector value does not match")
            	}
            }
            for l := vertex.level; l >= 0; l-- {
                neighborDistances := make(map[uint64]float32)
                otherNeighborDistances := make(map[uint64]float32)
                for neighbor, distance := range vertex.edges[l] {
                	if atomic.LoadUint32(&neighbor.deleted) == 0 {
                		neighborDistances[neighbor.id] = distance
                	}
                }
                for neighbor, distance := range otherVertex.edges[l] {
                	if atomic.LoadUint32(&neighbor.deleted) == 0 {
                		otherNeighborDistances[neighbor.id] = distance
                	}
                }
                if len(neighborDistances) != len(otherNeighborDistances) {
                	return errors.New("Edges count not the same")
                }
                for id, distance := range neighborDistances {
                	otherDistance, exists := otherNeighborDistances[id]
                	if !exists {
                		return errors.New("Other neighbor does not exist")
                	}
                	if math.Abs(distance - otherDistance) > 1e-4 {
                		return errors.New("Distances do not match")
                	}
                }
            }
        }
    }
    return nil
}

func generateRandomIndex(dim, size int, space space.Space) *Hnsw {
    index := NewHnsw(uint(dim), space)
    delOffset := int(size / 10)
    for i := 0; i < size; i++ {
        if i > delOffset && (math.RandomUniform() <= 0.2) {
            index.Remove(uint64(rand.Intn(i-delOffset)))
        } else {
            index.Insert(uint64(i), math.RandomUniformVector(dim), index.RandomLevel())
        }
    }
    return index
}

func TestHnswSaveAndLoad(t *testing.T) {
    index := generateRandomIndex(128, 1000, space.NewEuclidean())

    var buf bytes.Buffer
    err := index.Save(&buf, true)
    assert.Nil(t, err)
    if err != nil {
        return
    }

    otherIndex := NewHnsw(128, space.NewEuclidean())
    err = otherIndex.Load(&buf, true)
    assert.Nil(t, err)
    if err != nil {
        return
    }

    assert.Nil(t, hnswIsSame(index, otherIndex))
}