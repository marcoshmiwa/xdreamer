package config

import "errors"

var (
	ErrMissingBaseURL    = errors.New("model.base_url is required")
	ErrMissingModel      = errors.New("model.model is required")
	ErrMissingEmbedModel = errors.New("model.embed_model is required")
	ErrInvalidMaxSteps   = errors.New("agent.max_steps must be greater than 0")
	ErrInvalidChunkSize  = errors.New("rag.chunk_size must be greater than 0")
)

// Validate checks that cfg has all required fields and sane values.
// Returns the first validation error found.
func Validate(cfg *Config) error {
	if cfg.Model.BaseURL == "" {
		return ErrMissingBaseURL
	}
	if cfg.Model.Model == "" {
		return ErrMissingModel
	}
	if cfg.Model.EmbedModel == "" {
		return ErrMissingEmbedModel
	}
	if cfg.Agent.MaxSteps <= 0 {
		return ErrInvalidMaxSteps
	}
	if cfg.RAG.Enabled && cfg.RAG.ChunkSize <= 0 {
		return ErrInvalidChunkSize
	}
	return nil
}
