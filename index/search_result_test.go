package index

import (
	"math/rand"
	"sort"
	"testing"

	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

func TestSearchResultSort(t *testing.T) {
	sr := make(SearchResult, 1000)
	for i := 0; i < len(sr); i++ {
		sr[i].Id = uuid.NewV4()
		sr[i].Score = rand.Float32()
	}
	sort.Sort(sr)

	for i := 1; i < len(sr); i++ {
		assert.LessOrEqual(t, sr[i-1].Score, sr[i].Score)
	}
}
