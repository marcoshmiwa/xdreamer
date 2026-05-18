package rag

import (
	"context"
	"testing"
)

// fixedEmbedFunc returns the same unit vector for any input.
// This lets all documents appear equally similar to any query.
func fixedEmbedFunc(_ context.Context, _ string) ([]float32, error) {
	return []float32{1, 0, 0, 0}, nil
}

func makeChunks(n int) []Chunk {
	chunks := make([]Chunk, n)
	for i := 0; i < n; i++ {
		content := "chunk content " + string(rune('A'+i))
		chunks[i] = Chunk{
			FilePath:  "file.go",
			StartLine: i + 1,
			Content:   content,
			Hash:      hashContent(content),
		}
	}
	return chunks
}

func TestOpenStore_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(context.Background(), dir+"/sub/index")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()
}

func TestStore_UpsertAndQuery(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(context.Background(), dir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	chunks := makeChunks(5)
	if err := s.Upsert(context.Background(), chunks, fixedEmbedFunc); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	results, err := s.Query(context.Background(), "any query", 3, fixedEmbedFunc)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected results after Upsert, got none")
	}
	if len(results) > 3 {
		t.Errorf("got %d results, want at most 3", len(results))
	}
}

func TestStore_QueryEmpty_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(context.Background(), dir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	results, err := s.Query(context.Background(), "query", 5, fixedEmbedFunc)
	if err != nil {
		t.Fatalf("Query on empty store: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty store, got %d", len(results))
	}
}

func TestStore_UpsertDuplicate_DoesNotError(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(context.Background(), dir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	chunks := makeChunks(2)
	if err := s.Upsert(context.Background(), chunks, fixedEmbedFunc); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	// Upsert the same chunks again — must not error.
	if err := s.Upsert(context.Background(), chunks, fixedEmbedFunc); err != nil {
		t.Fatalf("second Upsert (duplicate): %v", err)
	}
}

func TestStore_ChunkMetadataPreserved(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(context.Background(), dir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	original := Chunk{
		FilePath:  "internal/foo/bar.go",
		StartLine: 42,
		Content:   "unique content xyz",
		Hash:      hashContent("unique content xyz"),
	}
	if err := s.Upsert(context.Background(), []Chunk{original}, fixedEmbedFunc); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	results, err := s.Query(context.Background(), "xyz", 1, fixedEmbedFunc)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	got := results[0]
	if got.FilePath != original.FilePath {
		t.Errorf("FilePath: got %q, want %q", got.FilePath, original.FilePath)
	}
	if got.StartLine != original.StartLine {
		t.Errorf("StartLine: got %d, want %d", got.StartLine, original.StartLine)
	}
}
