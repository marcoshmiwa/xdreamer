package config

import (
	"errors"
	"testing"
)

func TestValidate_DefaultsPass(t *testing.T) {
	cfg := Defaults()
	if err := Validate(&cfg); err != nil {
		t.Errorf("Defaults() failed Validate: %v", err)
	}
}

func TestValidate_SentinelErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr error
	}{
		{
			name:    "empty BaseURL",
			mutate:  func(c *Config) { c.Model.BaseURL = "" },
			wantErr: ErrMissingBaseURL,
		},
		{
			name:    "empty Model",
			mutate:  func(c *Config) { c.Model.Model = "" },
			wantErr: ErrMissingModel,
		},
		{
			name:    "empty EmbedModel",
			mutate:  func(c *Config) { c.Model.EmbedModel = "" },
			wantErr: ErrMissingEmbedModel,
		},
		{
			name:    "MaxSteps zero",
			mutate:  func(c *Config) { c.Agent.MaxSteps = 0 },
			wantErr: ErrInvalidMaxSteps,
		},
		{
			name:    "MaxSteps negative",
			mutate:  func(c *Config) { c.Agent.MaxSteps = -1 },
			wantErr: ErrInvalidMaxSteps,
		},
		{
			name: "ChunkSize zero when RAG enabled",
			mutate: func(c *Config) {
				c.RAG.Enabled = true
				c.RAG.ChunkSize = 0
			},
			wantErr: ErrInvalidChunkSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			tt.mutate(&cfg)
			err := Validate(&cfg)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_ChunkSizeIgnoredWhenRAGDisabled(t *testing.T) {
	cfg := Defaults()
	cfg.RAG.Enabled = false
	cfg.RAG.ChunkSize = 0

	if err := Validate(&cfg); err != nil {
		t.Errorf("expected no error when RAG disabled, got: %v", err)
	}
}
