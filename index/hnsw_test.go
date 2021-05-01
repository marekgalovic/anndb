package index

import (
	"context"
	goMath "math"
	"sort"
	"testing"

	"github.com/marekgalovic/anndb/index/space"
	"github.com/marekgalovic/anndb/math"
	"github.com/marekgalovic/anndb/utils"

	"github.com/stretchr/testify/assert"

	uuid "github.com/satori/go.uuid"
	// log "github.com/sirupsen/logrus"
)

var DIM uint = 64

func TestHnswSearchResultVsBruteForce(t *testing.T) {
	space := space.NewEuclidean()
	index := NewHnsw(DIM, space, HnswSearchAlgorithm(HnswSearchHeuristic))

	N := 1000
	ids := make([]uuid.UUID, N)
	vectors := make([][]float32, N)
	for i, _ := range vectors {
		ids[i] = uuid.NewV4()
		vectors[i] = math.RandomNormalVector(int(DIM), math.RandomUniform()*10, 1000)
		index.Insert(ids[i], vectors[i], nil, index.RandomLevel())
	}

	query := math.RandomUniformVector(int(DIM))

	result, err := index.Search(context.Background(), query, 100)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	bruteForceResult := make(SearchResult, N)
	for i, vec := range vectors {
		bruteForceResult[i] = SearchResultItem{
			Id:    ids[i],
			Score: space.Distance(vec, query),
		}
	}
	sort.Sort(bruteForceResult)

	for i := 0; i < 10; i++ {
		assert.True(t, uuid.Equal(result[i].Id, bruteForceResult[i].Id))
	}
}

func generateCompleteGraph(space space.Space, dim uint, n int, deleteMod int) []*hnswVertex {
	vertices := make([]*hnswVertex, n)

	for i := 0; i < len(vertices); i++ {
		vertices[i] = newHnswVertex(uuid.NewV4(), math.RandomUniformVector(int(dim)), nil, 0)
		if (deleteMod > 0) && (i%deleteMod == 0) {
			vertices[i].setDeleted()
		}
	}

	for i := 0; i < len(vertices); i++ {
		for j := i + 1; j < len(vertices); j++ {
			distance := space.Distance(vertices[i].Vector(), vertices[j].Vector())
			vertices[i].addEdge(0, vertices[j], distance)
			vertices[j].addEdge(0, vertices[i], distance)
		}
	}

	return vertices
}

func TestHnswGreedyClosestNeighbor(t *testing.T) {
	space := space.NewEuclidean()
	index := NewHnsw(DIM, space)

	vertices := generateCompleteGraph(space, DIM, 100, 0)
	query := math.RandomUniformVector(int(DIM))

	var closestNeighbor *hnswVertex
	var minDistance float32 = goMath.MaxFloat32
	for i := 0; i < len(vertices); i++ {
		distance := space.Distance(query, vertices[i].Vector())
		if distance < minDistance {
			minDistance = distance
			closestNeighbor = vertices[i]
		}
	}

	assert.NotNil(t, closestNeighbor)

	v, d := index.greedyClosestNeighbor(query, vertices[0], goMath.MaxFloat32, 0)
	assert.Equal(t, v.Id(), closestNeighbor.Id())
	assert.Equal(t, minDistance, d)
}

func TestHnswGreedyClosestNeighborWithDeletedVertices(t *testing.T) {
	space := space.NewEuclidean()
	index := NewHnsw(DIM, space)

	vertices := generateCompleteGraph(space, DIM, 100, 2)
	query := math.RandomUniformVector(int(DIM))

	var closestNeighbor *hnswVertex
	var minDistance float32 = goMath.MaxFloat32
	for i := 0; i < len(vertices); i++ {
		if vertices[i].isDeleted() {
			continue
		}
		distance := space.Distance(query, vertices[i].Vector())
		if distance < minDistance {
			minDistance = distance
			closestNeighbor = vertices[i]
		}
	}

	assert.NotNil(t, closestNeighbor)

	v, d := index.greedyClosestNeighbor(query, vertices[0], goMath.MaxFloat32, 0)
	assert.Equal(t, v.Id(), closestNeighbor.Id())
	assert.Equal(t, minDistance, d)
}

func TestHnswSearchLevel(t *testing.T) {
	space := space.NewEuclidean()
	index := NewHnsw(DIM, space)

	vertices := generateCompleteGraph(space, DIM, 100, 0)
	query := math.RandomUniformVector(int(DIM))

	// var maxDistance float32 = -1
	var entrypoint *hnswVertex = vertices[0]
	expected := utils.NewMaxPriorityQueue()
	for _, v := range vertices {
		distance := space.Distance(query, v.Vector())
		expected.Push(utils.NewPriorityQueueItem(distance, v))
		// if distance > maxDistance {
		//     maxDistance = distance
		//     entrypoint = v
		// }
	}

	for expected.Len() > 10 {
		expected.Pop()
	}

	result := index.searchLevel(query, entrypoint, 10, 0)
	assert.Equal(t, 10, result.Len())

	for result.Len() > 0 {
		resultItem := result.Pop()
		resultVertex := resultItem.Value().(*hnswVertex)
		expectedItem := expected.Pop()
		expectedVertex := expectedItem.Value().(*hnswVertex)

		assert.Equal(t, expectedVertex.Id(), resultVertex.Id())
	}
}

func TestHnswSearchLevelWithDeletedVertices(t *testing.T) {
	space := space.NewEuclidean()
	index := NewHnsw(DIM, space)

	vertices := generateCompleteGraph(space, DIM, 100, 2)
	query := math.RandomUniformVector(int(DIM))

	var entrypoint *hnswVertex = vertices[0]
	expected := utils.NewMaxPriorityQueue()
	for _, v := range vertices {
		if v.isDeleted() {
			continue
		}
		distance := space.Distance(query, v.Vector())
		expected.Push(utils.NewPriorityQueueItem(distance, v))
	}

	for expected.Len() > 10 {
		expected.Pop()
	}

	result := index.searchLevel(query, entrypoint, 10, 0)
	assert.Equal(t, 10, result.Len())

	for result.Len() > 0 {
		resultItem := result.Pop()
		resultVertex := resultItem.Value().(*hnswVertex)
		expectedItem := expected.Pop()
		expectedVertex := expectedItem.Value().(*hnswVertex)

		assert.Equal(t, expectedVertex.Id(), resultVertex.Id())
	}
}

func TestHnswSelectNeighbors(t *testing.T) {
	queue := utils.NewMaxPriorityQueue()
	for i := 0; i < 100; i++ {
		queue.Push(utils.NewPriorityQueueItem(float32(i), i))
	}

	index := NewHnsw(DIM, nil)
	neighbors := index.selectNeighbors(queue, 10)

	assert.Equal(t, 10, neighbors.Len())
	for i := 0; i < 10; i++ {
		v := neighbors.Pop().Value().(int)
		assert.Equal(t, 9-i, v)
	}
}

func TestHnswSelectNeighborsHeuristic(t *testing.T) {
	space := space.NewEuclidean()
	query := math.RandomUniformVector(int(DIM))
	vertices := generateCompleteGraph(space, DIM, 100, 0)

	neighbors := utils.NewMaxPriorityQueue()
	neighbors.Push(utils.NewPriorityQueueItem(space.Distance(query, vertices[0].Vector()), vertices[0]))

	index := NewHnsw(DIM, space)
	result := index.selectNeighborsHeuristic(query, neighbors, 10, 0, true, true)
	assert.Equal(t, 10, result.Len())

	expected := utils.NewMaxPriorityQueue()
	for _, v := range vertices {
		expected.Push(utils.NewPriorityQueueItem(space.Distance(query, v.Vector()), v))
	}
	for expected.Len() > result.Len() {
		expected.Pop()
	}

	for i := 0; i < result.Len(); i++ {
		expectedVertex := expected.Pop().Value().(*hnswVertex)
		resultVertex := result.Pop().Value().(*hnswVertex)

		assert.Equal(t, expectedVertex, resultVertex)
	}
}

func TestHnswPruneNeighbors(t *testing.T) {
	space := space.NewEuclidean()

	vertex := newHnswVertex(uuid.NewV4(), math.RandomUniformVector(int(DIM)), nil, 0)
	neighborsQueue := utils.NewMinPriorityQueue()
	for i := 0; i < 100; i++ {
		neighbor := newHnswVertex(uuid.NewV4(), math.RandomUniformVector(int(DIM)), nil, 0)
		distance := space.Distance(vertex.Vector(), neighbor.Vector())
		neighborsQueue.Push(utils.NewPriorityQueueItem(distance, neighbor))

		vertex.addEdge(0, neighbor, distance)
		neighbor.addEdge(0, vertex, distance)
	}

	assert.Equal(t, 100, vertex.edgesCount(0))

	index := NewHnsw(DIM, space)
	index.pruneNeighbors(vertex, 10, 0)

	assert.Equal(t, 10, vertex.edgesCount(0))

	edgeSet := vertex.getEdges(0)
	for i := 0; i < len(edgeSet); i++ {
		neighbor := neighborsQueue.Pop().Value().(*hnswVertex)
		assert.Contains(t, edgeSet, neighbor)
	}
}

func TestHnswPruneNeighborsWithDeletedVertices(t *testing.T) {
	space := space.NewEuclidean()

	vertex := newHnswVertex(uuid.NewV4(), math.RandomUniformVector(int(DIM)), nil, 0)
	neighborsQueue := utils.NewMinPriorityQueue()
	for i := 0; i < 100; i++ {
		neighbor := newHnswVertex(uuid.NewV4(), math.RandomUniformVector(int(DIM)), nil, 0)
		distance := space.Distance(vertex.Vector(), neighbor.Vector())

		vertex.addEdge(0, neighbor, distance)
		neighbor.addEdge(0, vertex, distance)

		if i%2 == 0 {
			neighbor.setDeleted()
		} else {
			neighborsQueue.Push(utils.NewPriorityQueueItem(distance, neighbor))
		}
	}

	assert.Equal(t, 100, vertex.edgesCount(0))

	index := NewHnsw(DIM, space)
	index.pruneNeighbors(vertex, 10, 0)

	assert.Equal(t, 10, vertex.edgesCount(0))

	edgeSet := vertex.getEdges(0)
	for i := 0; i < len(edgeSet); i++ {
		neighbor := neighborsQueue.Pop().Value().(*hnswVertex)
		assert.Contains(t, edgeSet, neighbor)
	}
}
