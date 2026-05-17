package config

import "testing"

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"Model.Provider", cfg.Model.Provider, "lmstudio"},
		{"Model.BaseURL", cfg.Model.BaseURL, "http://localhost:1234/v1"},
		{"Model.Model", cfg.Model.Model, "qwen2.5-coder-7b"},
		{"Model.EmbedModel", cfg.Model.EmbedModel, "nomic-embed-text-v1.5"},
		{"Model.ContextWindow", cfg.Model.ContextWindow, 8192},
		{"Model.Temperature", cfg.Model.Temperature, float32(0.2)},
		{"Model.MaxRetries", cfg.Model.MaxRetries, 3},
		{"Model.RetryDelayMS", cfg.Model.RetryDelayMS, 1000},
		{"Agent.Auto", cfg.Agent.Auto, false},
		{"Agent.MaxSteps", cfg.Agent.MaxSteps, 50},
		{"Agent.WorktreeBase", cfg.Agent.WorktreeBase, ".xdreamer/worktrees"},
		{"RAG.Enabled", cfg.RAG.Enabled, true},
		{"RAG.ChunkSize", cfg.RAG.ChunkSize, 512},
		{"RAG.ChunkOverlap", cfg.RAG.ChunkOverlap, 64},
		{"RAG.TopK", cfg.RAG.TopK, 10},
		{"RAG.IndexDir", cfg.RAG.IndexDir, ".xdreamer/index"},
		{"Memory.Enabled", cfg.Memory.Enabled, false},
		{"Memory.File", cfg.Memory.File, ".xdreamer/MEMORY.md"},
		{"Output.LogDir", cfg.Output.LogDir, "."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestDefaultsRAGIgnore(t *testing.T) {
	cfg := Defaults()
	want := []string{"vendor/", "node_modules/", ".git/"}
	if len(cfg.RAG.Ignore) != len(want) {
		t.Fatalf("RAG.Ignore: got %v, want %v", cfg.RAG.Ignore, want)
	}
	for i, v := range want {
		if cfg.RAG.Ignore[i] != v {
			t.Errorf("RAG.Ignore[%d]: got %q, want %q", i, cfg.RAG.Ignore[i], v)
		}
	}
}
