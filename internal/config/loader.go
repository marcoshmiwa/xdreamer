package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Load builds a Config by merging defaults, global config, project config,
// and environment variable overrides in that priority order.
func Load(projectDir string) (*Config, error) {
	cfg := Defaults()

	if err := mergeGlobalConfig(&cfg); err != nil {
		return nil, fmt.Errorf("load global config: %w", err)
	}

	if err := mergeProjectConfig(&cfg, projectDir); err != nil {
		return nil, fmt.Errorf("load project config: %w", err)
	}

	if err := mergeEnvVars(&cfg); err != nil {
		return nil, fmt.Errorf("apply env vars: %w", err)
	}

	return &cfg, nil
}

func mergeGlobalConfig(cfg *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	return mergeFile(cfg, filepath.Join(home, ".xdreamer", "config.toml"))
}

func mergeProjectConfig(cfg *Config, projectDir string) error {
	return mergeFile(cfg, filepath.Join(projectDir, ".xdreamer.toml"))
}

// mergeFile decodes a TOML file into cfg. A missing file is not an error.
func mergeFile(cfg *Config, path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func mergeEnvVars(cfg *Config) error {
	if v := os.Getenv("XDREAMER_MODEL_BASE_URL"); v != "" {
		cfg.Model.BaseURL = v
	}
	if v := os.Getenv("XDREAMER_MODEL"); v != "" {
		cfg.Model.Model = v
	}
	if v := os.Getenv("XDREAMER_EMBED_MODEL"); v != "" {
		cfg.Model.EmbedModel = v
	}
	if v := os.Getenv("XDREAMER_AUTO"); v != "" {
		cfg.Agent.Auto = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("XDREAMER_MAX_STEPS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("XDREAMER_MAX_STEPS: %w", err)
		}
		cfg.Agent.MaxSteps = n
	}
	if v := os.Getenv("XDREAMER_LOG_DIR"); v != "" {
		cfg.Output.LogDir = v
	}
	return nil
}
