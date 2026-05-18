package rag

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/xdreamer/xdreamer/internal/config"
)

const binaryProbeBytes = 512

// Engine orchestrates codebase indexing and chunk retrieval.
type Engine struct {
	store    *Store
	chunker  *Chunker
	embedFn  EmbedFunc
	cfg      config.RAGConfig
	manifest *manifest
}

// NewEngine creates an Engine, opening the persistent vector store.
func NewEngine(ctx context.Context, cfg config.RAGConfig, embedFn EmbedFunc) (*Engine, error) {
	store, err := OpenStore(ctx, cfg.IndexDir)
	if err != nil {
		return nil, fmt.Errorf("new engine: %w", err)
	}
	mf, err := loadManifest(cfg.IndexDir)
	if err != nil {
		return nil, fmt.Errorf("new engine: %w", err)
	}
	return &Engine{
		store:    store,
		chunker:  NewChunker(cfg.ChunkSize, cfg.ChunkOverlap),
		embedFn:  embedFn,
		cfg:      cfg,
		manifest: mf,
	}, nil
}

// Index walks projectRoot and embeds any new or changed text files.
func (e *Engine) Index(ctx context.Context, projectRoot string) error {
	err := filepath.WalkDir(projectRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldIgnoreDir(d.Name(), e.cfg.Ignore) {
				return filepath.SkipDir
			}
			return nil
		}
		return e.indexFile(ctx, path)
	})
	if err != nil {
		return fmt.Errorf("index: walk: %w", err)
	}
	if err := e.manifest.save(e.cfg.IndexDir); err != nil {
		return fmt.Errorf("index: save manifest: %w", err)
	}
	return nil
}

// Retrieve returns the topK chunks most relevant to query.
func (e *Engine) Retrieve(ctx context.Context, query string) ([]Chunk, error) {
	chunks, err := e.store.Query(ctx, query, e.cfg.TopK, e.embedFn)
	if err != nil {
		return nil, fmt.Errorf("retrieve: %w", err)
	}
	return chunks, nil
}

// Close releases any resources held by the engine.
func (e *Engine) Close() error {
	return e.store.Close()
}

func (e *Engine) indexFile(ctx context.Context, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if isBinary(content) {
		return nil
	}
	hash := fileHash(content)
	if !e.manifest.isChanged(path, hash) {
		return nil
	}
	chunks, err := e.chunker.ChunkFile(path, content)
	if err != nil {
		return fmt.Errorf("chunk %s: %w", path, err)
	}
	if len(chunks) == 0 {
		return nil
	}
	if err := e.store.Upsert(ctx, chunks, e.embedFn); err != nil {
		return fmt.Errorf("upsert %s: %w", path, err)
	}
	e.manifest.mark(path, hash)
	return nil
}

// shouldIgnoreDir reports whether a directory name matches any ignore pattern.
func shouldIgnoreDir(name string, patterns []string) bool {
	for _, pattern := range patterns {
		// Strip trailing slash for comparison.
		p := strings.TrimSuffix(pattern, "/")
		if name == p {
			return true
		}
		if matched, _ := filepath.Match(p, name); matched {
			return true
		}
	}
	return false
}

// isBinary returns true if the first 512 bytes contain a null byte.
func isBinary(content []byte) bool {
	probe := content
	if len(probe) > binaryProbeBytes {
		probe = probe[:binaryProbeBytes]
	}
	return bytes.ContainsRune(probe, 0)
}

func fileHash(content []byte) string {
	sum := sha256.Sum256(content)
	return fmt.Sprintf("%x", sum)
}
