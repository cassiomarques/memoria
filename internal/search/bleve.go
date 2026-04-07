package search

import (
	"errors"
	"os"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/cassiomarques/memoria/internal/note"
)

// SearchIndex wraps a Bleve index for full-text search of notes.
type SearchIndex struct {
	index bleve.Index
}

// SearchResult represents a single search hit.
type SearchResult struct {
	Path      string
	Score     float64
	Fragments map[string][]string // field -> highlighted fragments
}

// noteDocument is the internal document type indexed by Bleve.
type noteDocument struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Tags    string `json:"tags"` // space-separated tags for searching
	Folder  string `json:"folder"`
}

// buildIndexMapping creates the custom index mapping used by all index types.
func buildIndexMapping() mapping.IndexMapping {
	textAnalyzed := func() *mapping.FieldMapping {
		fm := mapping.NewTextFieldMapping()
		fm.Analyzer = "en"
		fm.Store = true
		fm.Index = true
		fm.IncludeTermVectors = true
		fm.IncludeInAll = true
		return fm
	}

	pathField := mapping.NewTextFieldMapping()
	pathField.Store = true
	pathField.Index = false
	pathField.IncludeInAll = false

	noteMapping := bleve.NewDocumentMapping()
	noteMapping.AddFieldMappingsAt("content", textAnalyzed())
	noteMapping.AddFieldMappingsAt("title", textAnalyzed())
	noteMapping.AddFieldMappingsAt("tags", textAnalyzed())
	noteMapping.AddFieldMappingsAt("folder", textAnalyzed())
	noteMapping.AddFieldMappingsAt("path", pathField)

	im := bleve.NewIndexMapping()
	im.DefaultMapping = noteMapping
	im.DefaultAnalyzer = "en"
	return im
}

// NewSearchIndex creates or opens a Bleve index at the given filesystem path.
func NewSearchIndex(path string) (*SearchIndex, error) {
	idx, err := bleve.Open(path)
	if err != nil {
		if errors.Is(err, bleve.ErrorIndexPathDoesNotExist) || os.IsNotExist(err) {
			idx, err = bleve.New(path, buildIndexMapping())
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return &SearchIndex{index: idx}, nil
}

// NewMemorySearchIndex creates an in-memory Bleve index for testing.
func NewMemorySearchIndex() (*SearchIndex, error) {
	idx, err := bleve.NewMemOnly(buildIndexMapping())
	if err != nil {
		return nil, err
	}
	return &SearchIndex{index: idx}, nil
}

// Close closes the underlying Bleve index.
func (s *SearchIndex) Close() error {
	return s.index.Close()
}

// Index adds or updates a note in the search index, using note.Path as the document ID.
func (s *SearchIndex) Index(n *note.Note) error {
	doc := noteDocument{
		Path:    n.Path,
		Title:   n.Title,
		Content: n.Content,
		Tags:    strings.Join(n.Tags, " "),
		Folder:  n.Folder,
	}
	return s.index.Index(n.Path, doc)
}

// Remove deletes a note from the search index by its path.
func (s *SearchIndex) Remove(path string) error {
	return s.index.Delete(path)
}

// Search performs a query-string search and returns results sorted by score descending.
func (s *SearchIndex) Search(queryStr string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	q := bleve.NewQueryStringQuery(queryStr)
	req := bleve.NewSearchRequest(q)
	req.Size = limit
	req.Highlight = bleve.NewHighlight()
	req.Fields = []string{"path"}

	res, err := s.index.Search(req)
	if err != nil {
		return nil, err
	}

	return hitsToResults(res), nil
}

// SearchFuzzy performs a typo-tolerant search using MatchQuery with fuzziness on each term.
// SearchFuzzy performs a full-text search using parsed query tokens.
// Multiple words are AND'd: a note must match all tokens.
// Quoted phrases are matched as exact match phrases.
// Tag tokens (#tag) are ignored here (handled by the caller).
func (s *SearchIndex) SearchFuzzy(queryStr string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	tokens := note.TextTokens(note.ParseQuery(queryStr))
	if len(tokens) == 0 {
		return []SearchResult{}, nil
	}

	conjuncts := make([]query.Query, 0, len(tokens))
	for _, tok := range tokens {
		if tok.Exact {
			pq := bleve.NewMatchPhraseQuery(tok.Text)
			conjuncts = append(conjuncts, pq)
		} else {
			mq := bleve.NewMatchQuery(tok.Text)
			mq.SetFuzziness(2)
			conjuncts = append(conjuncts, mq)
		}
	}

	q := query.NewConjunctionQuery(conjuncts)
	req := bleve.NewSearchRequest(q)
	req.Size = limit
	req.Highlight = bleve.NewHighlight()
	req.Fields = []string{"path"}

	res, err := s.index.Search(req)
	if err != nil {
		return nil, err
	}

	return hitsToResults(res), nil
}

// Reindex deletes all existing documents and re-indexes the provided notes using a batch.
func (s *SearchIndex) Reindex(notes []*note.Note) error {
	// Delete all existing documents by iterating the index.
	// Use a search-all query to find every document ID.
	countBefore, err := s.index.DocCount()
	if err != nil {
		return err
	}

	if countBefore > 0 {
		q := bleve.NewMatchAllQuery()
		req := bleve.NewSearchRequest(q)
		req.Size = int(countBefore)
		res, err := s.index.Search(req)
		if err != nil {
			return err
		}
		batch := s.index.NewBatch()
		for _, hit := range res.Hits {
			batch.Delete(hit.ID)
		}
		if err := s.index.Batch(batch); err != nil {
			return err
		}
	}

	// Now index all provided notes in a batch.
	batch := s.index.NewBatch()
	for _, n := range notes {
		doc := noteDocument{
			Path:    n.Path,
			Title:   n.Title,
			Content: n.Content,
			Tags:    strings.Join(n.Tags, " "),
			Folder:  n.Folder,
		}
		if err := batch.Index(n.Path, doc); err != nil {
			return err
		}
	}
	return s.index.Batch(batch)
}

// Count returns the number of documents in the index.
func (s *SearchIndex) Count() (uint64, error) {
	return s.index.DocCount()
}

func hitsToResults(res *bleve.SearchResult) []SearchResult {
	results := make([]SearchResult, 0, len(res.Hits))
	for _, hit := range res.Hits {
		path := hit.ID
		fragments := make(map[string][]string, len(hit.Fragments))
		for field, frags := range hit.Fragments {
			fragments[field] = frags
		}
		results = append(results, SearchResult{
			Path:      path,
			Score:     hit.Score,
			Fragments: fragments,
		})
	}
	return results
}
