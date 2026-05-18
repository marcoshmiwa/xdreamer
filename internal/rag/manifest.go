package rag

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const manifestFileName = "manifest.json"

// manifest tracks which files have been indexed and their content hashes.
// It enables incremental re-indexing: only changed files are re-embedded.
type manifest struct {
	Files map[string]string `json:"files"` // filepath → sha256 hash
}

func newManifest() *manifest {
	return &manifest{Files: make(map[string]string)}
}

// loadManifest reads the manifest from indexDir. A missing file is not an error.
func loadManifest(indexDir string) (*manifest, error) {
	path := filepath.Join(indexDir, manifestFileName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return newManifest(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("load manifest: %w", err)
	}
	m := newManifest()
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("load manifest: decode: %w", err)
	}
	return m, nil
}

// save writes the manifest to indexDir/manifest.json.
func (m *manifest) save(indexDir string) error {
	if err := os.MkdirAll(indexDir, 0o755); err != nil {
		return fmt.Errorf("save manifest: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("save manifest: encode: %w", err)
	}
	path := filepath.Join(indexDir, manifestFileName)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("save manifest: write: %w", err)
	}
	return nil
}

// isChanged reports whether filePath is new or its recorded hash differs from hash.
func (m *manifest) isChanged(filePath, hash string) bool {
	recorded, exists := m.Files[filePath]
	return !exists || recorded != hash
}

// mark records filePath as indexed with the given hash.
func (m *manifest) mark(filePath, hash string) {
	m.Files[filePath] = hash
}
