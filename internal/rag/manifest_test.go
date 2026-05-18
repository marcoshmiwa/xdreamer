package rag

import (
	"path/filepath"
	"testing"
)

func TestManifest_NewIsEmpty(t *testing.T) {
	m := newManifest()
	if len(m.Files) != 0 {
		t.Errorf("new manifest should be empty, got %d entries", len(m.Files))
	}
}

func TestManifest_IsChanged_NewPath(t *testing.T) {
	m := newManifest()
	if !m.isChanged("file.go", "abc123") {
		t.Error("isChanged should return true for untracked path")
	}
}

func TestManifest_MarkAndIsChanged(t *testing.T) {
	m := newManifest()
	m.mark("file.go", "abc123")

	if m.isChanged("file.go", "abc123") {
		t.Error("isChanged should return false when hash matches")
	}
	if !m.isChanged("file.go", "different") {
		t.Error("isChanged should return true when hash differs")
	}
}

func TestManifest_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	m := newManifest()
	m.mark("a.go", "hash1")
	m.mark("b.go", "hash2")

	if err := m.save(dir); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if loaded.isChanged("a.go", "hash1") {
		t.Error("a.go should not be changed after reload")
	}
	if loaded.isChanged("b.go", "hash2") {
		t.Error("b.go should not be changed after reload")
	}
	if !loaded.isChanged("c.go", "hash3") {
		t.Error("c.go should be changed (not in manifest)")
	}
}

func TestLoadManifest_MissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	m, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("loadManifest on missing file: %v", err)
	}
	if len(m.Files) != 0 {
		t.Errorf("expected empty manifest, got %d entries", len(m.Files))
	}
}

func TestManifest_SaveCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	m := newManifest()
	if err := m.save(dir); err != nil {
		t.Fatalf("save to new nested dir: %v", err)
	}
}
