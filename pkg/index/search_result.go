package index

type SearchResult []SearchResultItem

type SearchResultItem struct {
	Id uint64
	Score float32
}