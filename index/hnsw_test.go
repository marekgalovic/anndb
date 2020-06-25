package index

import (
    "testing";
	"context";
	"sort";

    "github.com/marekgalovic/anndb/math";
    "github.com/marekgalovic/anndb/index/space";

    "github.com/stretchr/testify/assert";

    "github.com/satori/go.uuid";
)

func TestHnswSearchResultVsBruteForce(t *testing.T) {
	space := space.NewEuclidean()
    index := NewHnsw(64, space)

    N := 100
    ids := make([]uuid.UUID, N)
    vectors := make([][]float32, N)
    for i, _ := range vectors {
    	ids[i] = uuid.NewV4()
    	vectors[i] = math.RandomUniformVector(64)
    	index.Insert(ids[i], vectors[i], nil, index.RandomLevel())
    }

    query := math.RandomUniformVector(64)

    result, err := index.Search(context.Background(), query, 10)
    assert.Nil(t, err)
    if err != nil {
    	return
    }

    bruteForceResult := make(SearchResult, N)
    for i, vec := range vectors {
    	bruteForceResult[i] = SearchResultItem {
    		Id: ids[i],
    		Score: space.Distance(vec, query),
    	}
    }
    sort.Sort(bruteForceResult)
    
    for i := 0; i < len(result); i++ {
    	assert.True(t, uuid.Equal(result[i].Id, bruteForceResult[i].Id))
    }
}