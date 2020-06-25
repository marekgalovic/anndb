package index

import (
    "fmt";
    "testing";
	"errors";
    "bytes";
    "sync/atomic";
    // "math/rand";

    "github.com/marekgalovic/anndb/math";
    "github.com/marekgalovic/anndb/index/space";

    "github.com/stretchr/testify/assert";

    "github.com/satori/go.uuid";
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
            if len(vertex.metadata) != len(otherVertex.metadata) {
                return errors.New("Metadata size does not match")
            }
            for k, v := range vertex.metadata {
                if otherVertex.metadata[k] != v {
                    return errors.New("Metadata value missmatch")
                }
            }
            for l := vertex.level; l >= 0; l-- {
                neighborDistances := make(map[uuid.UUID]float32)
                otherNeighborDistances := make(map[uuid.UUID]float32)
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
    insertKeys := make(map[uuid.UUID]struct{})

    index := NewHnsw(uint(dim), space)
    delOffset := int(size / 10)
    for i := 0; i < size; i++ {
        if i > delOffset && (math.RandomUniform() <= 0.2) {
            var key uuid.UUID
            for k := range insertKeys {
                key = k
                break
            }
            delete(insertKeys, key)
            index.Remove(key)
        } else {
            id := uuid.NewV4()
            insertKeys[id] = struct{}{}
            index.Insert(id, math.RandomUniformVector(dim), Metadata{"foo": fmt.Sprintf("bar: %d", i), "id": id.String()}, index.RandomLevel())
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