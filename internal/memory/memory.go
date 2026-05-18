package memory

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/xdreamer/xdreamer/internal/config"
)

// Memory holds per-project agent notes loaded at session start and
// optionally persisted at session end. When disabled every method is a no-op.
type Memory struct {
	filePath string
	content  string
	enabled  bool
}

// Load reads project memory from the file named in cfg.
// Returns a disabled, no-op Memory when cfg.Enabled is false.
// A missing memory file is not an error; the session starts with empty memory.
func Load(cfg config.MemoryConfig) (*Memory, error) {
	if !cfg.Enabled {
		return &Memory{enabled: false}, nil
	}
	content, err := os.ReadFile(cfg.File)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load memory: %w", err)
	}
	return &Memory{
		filePath: cfg.File,
		content:  string(content),
		enabled:  true,
	}, nil
}

// Content returns the raw memory text, or an empty string when disabled.
func (m *Memory) Content() string {
	if !m.enabled {
		return ""
	}
	return m.content
}

// Append adds a markdown section to memory in-memory.
// Does not write to disk; call Save to persist.
// No-op when disabled.
func (m *Memory) Append(section, text string) {
	if !m.enabled {
		return
	}
	m.content += "\n## " + section + "\n" + text + "\n"
}

// Save writes the current memory content to disk.
// No-op when disabled; always returns nil in that case.
func (m *Memory) Save() error {
	if !m.enabled {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(m.filePath), 0o755); err != nil {
		return fmt.Errorf("save memory: mkdir: %w", err)
	}
	if err := os.WriteFile(m.filePath, []byte(m.content), 0o644); err != nil {
		return fmt.Errorf("save memory: write: %w", err)
	}
	return nil
}
