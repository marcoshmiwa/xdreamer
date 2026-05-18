package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	chromem "github.com/philippgille/chromem-go"
)

const collectionName = "codebase"

// EmbedFunc is a function that returns a vector embedding for a text string.
type EmbedFunc func(ctx context.Context, text string) ([]float32, error)

// Store wraps a chromem-go persistent vector database.
type Store struct {
	db         *chromem.DB
	collection *chromem.Collection
	indexDir   string
}

// OpenStore opens or creates a persistent vector store at indexDir.
func OpenStore(ctx context.Context, indexDir string) (*Store, error) {
	if err := os.MkdirAll(indexDir, 0o755); err != nil {
		return nil, fmt.Errorf("open store: mkdir: %w", err)
	}
	dbPath := filepath.Join(indexDir, "vectors.db")
	db, err := chromem.NewPersistentDB(dbPath, false)
	if err != nil {
		return nil, fmt.Errorf("open store: open db: %w", err)
	}
	// nil embedding func: all embeddings are provided explicitly.
	col, err := db.GetOrCreateCollection(collectionName, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("open store: get collection: %w", err)
	}
	return &Store{db: db, collection: col, indexDir: indexDir}, nil
}

// Upsert embeds each chunk and adds it to the store.
// Chunks whose hash already exists in the store are skipped.
func (s *Store) Upsert(ctx context.Context, chunks []Chunk, embed EmbedFunc) error {
	for _, chunk := range chunks {
		embedding, err := embed(ctx, chunk.Content)
		if err != nil {
			return fmt.Errorf("upsert: embed: %w", err)
		}
		doc := chromem.Document{
			ID:        chunk.Hash,
			Embedding: embedding,
			Content:   chunk.Content,
			Metadata: map[string]string{
				"file":    chunk.FilePath,
				"line":    strconv.Itoa(chunk.StartLine),
				"content": chunk.Content,
			},
		}
		if err := s.collection.AddDocument(ctx, doc); err != nil {
			if isAlreadyExistsErr(err) {
				continue
			}
			return fmt.Errorf("upsert: add document: %w", err)
		}
	}
	return nil
}

// Query embeds the query string and returns the topK most similar chunks.
func (s *Store) Query(ctx context.Context, query string, topK int, embed EmbedFunc) ([]Chunk, error) {
	embedding, err := embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query: embed: %w", err)
	}
	nResults := topK
	if count := s.collection.Count(); count < nResults {
		nResults = count
	}
	if nResults == 0 {
		return nil, nil
	}
	results, err := s.collection.QueryEmbedding(ctx, embedding, nResults, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return chunksFromResults(results), nil
}

// Close is a no-op; chromem-go persists data on every write.
func (s *Store) Close() error { return nil }

func chunksFromResults(results []chromem.Result) []Chunk {
	chunks := make([]Chunk, 0, len(results))
	for _, r := range results {
		line, _ := strconv.Atoi(r.Metadata["line"])
		chunks = append(chunks, Chunk{
			FilePath:  r.Metadata["file"],
			StartLine: line,
			Content:   r.Metadata["content"],
			Hash:      r.ID,
		})
	}
	return chunks
}

func isAlreadyExistsErr(err error) bool {
	return strings.Contains(err.Error(), "already exists")
}
