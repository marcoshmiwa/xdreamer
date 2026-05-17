package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultsWhenNoFilesExist(t *testing.T) {
	dir := t.TempDir()
	clearEnv(t)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := Defaults()
	if cfg.Model.BaseURL != want.Model.BaseURL {
		t.Errorf("BaseURL: got %q, want %q", cfg.Model.BaseURL, want.Model.BaseURL)
	}
	if cfg.Agent.MaxSteps != want.Agent.MaxSteps {
		t.Errorf("MaxSteps: got %d, want %d", cfg.Agent.MaxSteps, want.Agent.MaxSteps)
	}
}

func TestLoad_ProjectConfigOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	clearEnv(t)

	tomlContent := `[model]
base_url = "http://192.168.1.10:1234/v1"
`
	writeFile(t, filepath.Join(dir, ".xdreamer.toml"), tomlContent)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Model.BaseURL != "http://192.168.1.10:1234/v1" {
		t.Errorf("BaseURL: got %q, want %q", cfg.Model.BaseURL, "http://192.168.1.10:1234/v1")
	}
	// non-overridden field must remain default
	if cfg.Agent.MaxSteps != 50 {
		t.Errorf("MaxSteps: got %d, want 50", cfg.Agent.MaxSteps)
	}
}

func TestLoad_EnvVarBaseURLOverridesFile(t *testing.T) {
	dir := t.TempDir()
	clearEnv(t)
	t.Setenv("XDREAMER_MODEL_BASE_URL", "http://envhost:1234/v1")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Model.BaseURL != "http://envhost:1234/v1" {
		t.Errorf("BaseURL: got %q, want %q", cfg.Model.BaseURL, "http://envhost:1234/v1")
	}
}

func TestLoad_EnvVarAutoTrue(t *testing.T) {
	dir := t.TempDir()
	clearEnv(t)
	t.Setenv("XDREAMER_AUTO", "true")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.Agent.Auto {
		t.Error("Agent.Auto: got false, want true")
	}
}

func TestLoad_EnvVarAutoOne(t *testing.T) {
	dir := t.TempDir()
	clearEnv(t)
	t.Setenv("XDREAMER_AUTO", "1")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.Agent.Auto {
		t.Error("Agent.Auto: got false, want true")
	}
}

func TestLoad_EnvVarMaxStepsInvalid(t *testing.T) {
	dir := t.TempDir()
	clearEnv(t)
	t.Setenv("XDREAMER_MAX_STEPS", "notanumber")

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid XDREAMER_MAX_STEPS, got nil")
	}
}

func TestLoad_EnvVarMaxStepsValid(t *testing.T) {
	dir := t.TempDir()
	clearEnv(t)
	t.Setenv("XDREAMER_MAX_STEPS", "25")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Agent.MaxSteps != 25 {
		t.Errorf("MaxSteps: got %d, want 25", cfg.Agent.MaxSteps)
	}
}

// clearEnv unsets all XDREAMER_* env vars so tests are isolated.
func clearEnv(t *testing.T) {
	t.Helper()
	vars := []string{
		"XDREAMER_MODEL_BASE_URL",
		"XDREAMER_MODEL",
		"XDREAMER_EMBED_MODEL",
		"XDREAMER_AUTO",
		"XDREAMER_MAX_STEPS",
		"XDREAMER_LOG_DIR",
	}
	for _, v := range vars {
		t.Setenv(v, "")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
