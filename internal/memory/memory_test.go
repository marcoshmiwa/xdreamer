package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xdreamer/xdreamer/internal/config"
)

func disabledCfg() config.MemoryConfig {
	return config.MemoryConfig{Enabled: false, File: "/should/never/be/accessed"}
}

func enabledCfg(t *testing.T) config.MemoryConfig {
	t.Helper()
	return config.MemoryConfig{
		Enabled: true,
		File:    filepath.Join(t.TempDir(), ".xdreamer", "MEMORY.md"),
	}
}

// --- disabled memory ---

func TestDisabled_LoadReturnsNoOp(t *testing.T) {
	m, err := Load(disabledCfg())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m == nil {
		t.Fatal("Load returned nil")
	}
}

func TestDisabled_ContentReturnsEmpty(t *testing.T) {
	m, _ := Load(disabledCfg())
	if got := m.Content(); got != "" {
		t.Errorf("Content: got %q, want empty string", got)
	}
}

func TestDisabled_AppendIsNoOp(t *testing.T) {
	m, _ := Load(disabledCfg())
	m.Append("Section", "some text")
	if got := m.Content(); got != "" {
		t.Errorf("Content after Append on disabled memory: got %q", got)
	}
}

func TestDisabled_SaveReturnsNil(t *testing.T) {
	m, _ := Load(disabledCfg())
	if err := m.Save(); err != nil {
		t.Errorf("Save on disabled memory: %v", err)
	}
}

func TestDisabled_SaveDoesNotCreateFile(t *testing.T) {
	dir := t.TempDir()
	m := &Memory{enabled: false, filePath: filepath.Join(dir, "MEMORY.md")}
	_ = m.Save()
	if _, err := os.Stat(filepath.Join(dir, "MEMORY.md")); !os.IsNotExist(err) {
		t.Error("Save on disabled memory must not create a file")
	}
}

// --- enabled memory ---

func TestEnabled_MissingFileStartsEmpty(t *testing.T) {
	cfg := enabledCfg(t) // file path points to non-existent file
	m, err := Load(cfg)
	if err != nil {
		t.Fatalf("Load with missing file: %v", err)
	}
	if m.Content() != "" {
		t.Errorf("Content: got %q, want empty string", m.Content())
	}
}

func TestEnabled_LoadExistingFile(t *testing.T) {
	cfg := enabledCfg(t)
	// Pre-create the file with known content.
	if err := os.MkdirAll(filepath.Dir(cfg.File), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := "# xdreamer Memory\n\nprevious notes"
	if err := os.WriteFile(cfg.File, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := Load(cfg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Content() != existing {
		t.Errorf("Content: got %q, want %q", m.Content(), existing)
	}
}

func TestEnabled_AppendAddsSection(t *testing.T) {
	cfg := enabledCfg(t)
	m, _ := Load(cfg)

	m.Append("Decisions", "Used chromem-go for vector store.")

	content := m.Content()
	if !strings.Contains(content, "## Decisions") {
		t.Errorf("section header missing: %q", content)
	}
	if !strings.Contains(content, "Used chromem-go for vector store.") {
		t.Errorf("section text missing: %q", content)
	}
}

func TestEnabled_AppendMultipleSections(t *testing.T) {
	cfg := enabledCfg(t)
	m, _ := Load(cfg)

	m.Append("Architecture", "Entry point is cmd/xdreamer/main.go")
	m.Append("Conventions", "All errors wrapped with fmt.Errorf")

	content := m.Content()
	if !strings.Contains(content, "## Architecture") {
		t.Errorf("Architecture section missing")
	}
	if !strings.Contains(content, "## Conventions") {
		t.Errorf("Conventions section missing")
	}
	// Architecture must appear before Conventions.
	if strings.Index(content, "## Architecture") > strings.Index(content, "## Conventions") {
		t.Error("sections out of order")
	}
}

func TestEnabled_SaveRoundtrip(t *testing.T) {
	cfg := enabledCfg(t)
	m, _ := Load(cfg)
	m.Append("Notes", "This is a test note.")

	if err := m.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Load again and verify content survived the roundtrip.
	m2, err := Load(cfg)
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if !strings.Contains(m2.Content(), "This is a test note.") {
		t.Errorf("note missing after reload: %q", m2.Content())
	}
	if !strings.Contains(m2.Content(), "## Notes") {
		t.Errorf("section header missing after reload: %q", m2.Content())
	}
}

func TestEnabled_SaveCreatesParentDirs(t *testing.T) {
	cfg := config.MemoryConfig{
		Enabled: true,
		File:    filepath.Join(t.TempDir(), "a", "b", "c", "MEMORY.md"),
	}
	m, _ := Load(cfg)
	m.Append("Test", "content")

	if err := m.Save(); err != nil {
		t.Fatalf("Save to nested path: %v", err)
	}
	if _, err := os.Stat(cfg.File); err != nil {
		t.Errorf("file not created: %v", err)
	}
}
