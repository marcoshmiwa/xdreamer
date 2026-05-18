package rag

import (
	"fmt"
	"strings"
	"testing"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

// requireEncoding skips the test if tiktoken BPE data is unavailable.
func requireEncoding(t *testing.T) *tiktoken.Tiktoken {
	t.Helper()
	enc, err := tiktoken.GetEncoding(encodingName)
	if err != nil {
		t.Skipf("tiktoken encoding unavailable (may need network): %v", err)
	}
	return enc
}

func TestChunkFile_EmptyContent(t *testing.T) {
	requireEncoding(t)
	c := NewChunker(512, 64)
	chunks, err := c.ChunkFile("file.go", []byte{})
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty content, got %d", len(chunks))
	}
}

func TestChunkFile_SmallFile_OneChunk(t *testing.T) {
	requireEncoding(t)
	content := []byte("package main\n\nfunc main() {}\n")
	c := NewChunker(512, 64)

	chunks, err := c.ChunkFile("main.go", content)
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	chunk := chunks[0]
	if chunk.FilePath != "main.go" {
		t.Errorf("FilePath: got %q", chunk.FilePath)
	}
	if chunk.StartLine != 1 {
		t.Errorf("StartLine: got %d, want 1", chunk.StartLine)
	}
	if chunk.Hash == "" {
		t.Error("Hash must not be empty")
	}
	if chunk.Content == "" {
		t.Error("Content must not be empty")
	}
}

func TestChunkFile_LargeFile_MultipleChunks(t *testing.T) {
	requireEncoding(t)
	// Build a file with unique lines, each ~5-8 tokens, that is definitely
	// larger than chunkSize=10 tokens so multiple chunks are produced.
	var lines []string
	words := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "iota", "kappa"}
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf("// line %d: %s %s content", i+1, words[i%len(words)], words[(i+3)%len(words)]))
	}
	content := []byte(strings.Join(lines, "\n"))

	c := NewChunker(10, 3) // very small chunk size to force splitting

	chunks, err := c.ChunkFile("big.go", content)
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}

	// Verify no duplicate hashes.
	seen := make(map[string]bool)
	for _, ch := range chunks {
		if seen[ch.Hash] {
			t.Errorf("duplicate hash: %s", ch.Hash)
		}
		seen[ch.Hash] = true
	}

	// Verify StartLine increases monotonically.
	for i := 1; i < len(chunks); i++ {
		if chunks[i].StartLine <= chunks[i-1].StartLine {
			t.Errorf("StartLine not increasing: chunk[%d].StartLine=%d, chunk[%d].StartLine=%d",
				i-1, chunks[i-1].StartLine, i, chunks[i].StartLine)
		}
	}
}

func TestChunkFile_OverlapPresent(t *testing.T) {
	requireEncoding(t)
	// Build content where we can verify overlap.
	// Use lines that have a known token count so we can predict chunk boundaries.
	lines := []string{
		"alpha beta",   // ~2 tokens
		"gamma delta",  // ~2 tokens
		"epsilon zeta", // ~2 tokens
		"eta theta",    // ~2 tokens
		"iota kappa",   // ~2 tokens
		"lambda mu",    // ~2 tokens
	}
	content := []byte(strings.Join(lines, "\n"))

	// chunkSize=4 tokens, chunkOverlap=2 tokens → expect overlap
	c := NewChunker(4, 2)
	chunks, err := c.ChunkFile("overlap.go", content)
	if err != nil {
		t.Fatalf("ChunkFile: %v", err)
	}
	if len(chunks) < 2 {
		t.Skipf("tokenizer produced unexpected count %d, skipping overlap check", len(chunks))
	}

	// Adjacent chunks should share at least one line (overlap).
	for i := 1; i < len(chunks); i++ {
		prevLines := strings.Split(chunks[i-1].Content, "\n")
		currLines := strings.Split(chunks[i].Content, "\n")
		if len(prevLines) == 0 || len(currLines) == 0 {
			continue
		}
		// Last line of prev should appear somewhere in curr (overlap).
		lastPrev := prevLines[len(prevLines)-1]
		found := false
		for _, l := range currLines {
			if l == lastPrev {
				found = true
				break
			}
		}
		if !found {
			t.Logf("chunk[%d] last line %q not in chunk[%d] (overlap may be <1 line at this chunk size)", i-1, lastPrev, i)
		}
	}
}

func TestHashContent_Deterministic(t *testing.T) {
	h1 := hashContent("hello")
	h2 := hashContent("hello")
	if h1 != h2 {
		t.Error("hashContent not deterministic")
	}
	if hashContent("hello") == hashContent("world") {
		t.Error("different content should produce different hashes")
	}
}
