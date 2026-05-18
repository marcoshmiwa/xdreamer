package config

// Config holds all runtime configuration for xdreamer.
type Config struct {
	// Dir is the project root directory. Set from the --dir CLI flag; never
	// written to or read from TOML config files.
	Dir string `toml:"-"`

	Model  ModelConfig  `toml:"model"`
	Agent  AgentConfig  `toml:"agent"`
	RAG    RAGConfig    `toml:"rag"`
	Memory MemoryConfig `toml:"memory"`
	Output OutputConfig `toml:"output"`
}

// ModelConfig configures the LM Studio connection and model selection.
type ModelConfig struct {
	Provider      string  `toml:"provider"`
	BaseURL       string  `toml:"base_url"`
	Model         string  `toml:"model"`
	EmbedModel    string  `toml:"embed_model"`
	ContextWindow int     `toml:"context_window"`
	Temperature   float32 `toml:"temperature"`
	MaxRetries    int     `toml:"max_retries"`
	RetryDelayMS  int     `toml:"retry_delay_ms"`
}

// AgentConfig controls agent runtime behaviour.
type AgentConfig struct {
	Auto         bool   `toml:"auto"`
	MaxSteps     int    `toml:"max_steps"`
	WorktreeBase string `toml:"worktree_base"`
}

// RAGConfig controls the retrieval-augmented generation engine.
type RAGConfig struct {
	Enabled      bool     `toml:"enabled"`
	ChunkSize    int      `toml:"chunk_size"`
	ChunkOverlap int      `toml:"chunk_overlap"`
	TopK         int      `toml:"top_k"`
	IndexDir     string   `toml:"index_dir"`
	Ignore       []string `toml:"ignore"`
}

// MemoryConfig controls per-project persistent memory.
type MemoryConfig struct {
	Enabled bool   `toml:"enabled"`
	File    string `toml:"file"`
}

// OutputConfig controls where output artifacts are written.
type OutputConfig struct {
	LogDir string `toml:"log_dir"`
}

// Defaults returns a Config populated with the values defined in SPEC.md §14.1.
func Defaults() Config {
	return Config{
		Model: ModelConfig{
			Provider:      "lmstudio",
			BaseURL:       "http://localhost:1234/v1",
			Model:         "qwen2.5-coder-7b",
			EmbedModel:    "nomic-embed-text-v1.5",
			ContextWindow: 8192,
			Temperature:   0.2,
			MaxRetries:    3,
			RetryDelayMS:  1000,
		},
		Agent: AgentConfig{
			Auto:         false,
			MaxSteps:     50,
			WorktreeBase: ".xdreamer/worktrees",
		},
		RAG: RAGConfig{
			Enabled:      true,
			ChunkSize:    512,
			ChunkOverlap: 64,
			TopK:         10,
			IndexDir:     ".xdreamer/index",
			Ignore:       []string{"vendor/", "node_modules/", ".git/"},
		},
		Memory: MemoryConfig{
			Enabled: false,
			File:    ".xdreamer/MEMORY.md",
		},
		Output: OutputConfig{
			LogDir: ".",
		},
	}
}
