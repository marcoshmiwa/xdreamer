package rag

import (
	"crypto/sha256"
	"fmt"
	"strings"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

const encodingName = "cl100k_base"

// Chunk is a slice of a source file with its location and a content hash.
type Chunk struct {
	FilePath  string
	StartLine int
	Content   string
	Hash      string // hex(sha256(Content))
}

// Chunker splits file content into overlapping token-bounded chunks.
type Chunker struct {
	chunkSize    int
	chunkOverlap int
}

// NewChunker creates a Chunker with the given token limits.
func NewChunker(chunkSize, chunkOverlap int) *Chunker {
	return &Chunker{chunkSize: chunkSize, chunkOverlap: chunkOverlap}
}

// ChunkFile splits content into chunks. Returns nil for empty content.
func (c *Chunker) ChunkFile(filePath string, content []byte) ([]Chunk, error) {
	if len(content) == 0 {
		return nil, nil
	}
	enc, err := tiktoken.GetEncoding(encodingName)
	if err != nil {
		return nil, fmt.Errorf("get encoding %s: %w", encodingName, err)
	}
	lines := strings.Split(string(content), "\n")
	return buildChunks(filePath, lines, enc, c.chunkSize, c.chunkOverlap), nil
}

func buildChunks(filePath string, lines []string, enc *tiktoken.Tiktoken, chunkSize, chunkOverlap int) []Chunk {
	// Precompute token count per line (including newline).
	toks := make([]int, len(lines))
	for i, line := range lines {
		toks[i] = len(enc.Encode(line+"\n", nil, nil))
	}

	var chunks []Chunk
	start := 0

	for start < len(lines) {
		end, total := expandChunk(start, toks, chunkSize)

		_ = total
		content := strings.Join(lines[start:end], "\n")
		chunks = append(chunks, Chunk{
			FilePath:  filePath,
			StartLine: start + 1,
			Content:   content,
			Hash:      hashContent(content),
		})

		if end >= len(lines) {
			break
		}

		start = nextChunkStart(start, end, toks, chunkOverlap)
	}

	return chunks
}

// expandChunk returns the end index and total tokens for a chunk starting at start.
func expandChunk(start int, toks []int, chunkSize int) (end, total int) {
	end = start
	for end < len(toks) {
		if total+toks[end] > chunkSize && end > start {
			break
		}
		total += toks[end]
		end++
	}
	return end, total
}

// nextChunkStart computes the start index of the next chunk by walking back
// from end until chunkOverlap tokens of overlap have been collected.
func nextChunkStart(start, end int, toks []int, chunkOverlap int) int {
	next := end
	overlapToks := 0
	for next > start+1 {
		overlapToks += toks[next-1]
		if overlapToks >= chunkOverlap {
			break
		}
		next--
	}
	return next
}

func hashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", sum)
}
