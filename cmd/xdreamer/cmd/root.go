package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/xdreamer/xdreamer/internal/config"
)

// contextKey is an unexported type so our context values never collide with
// keys set by other packages.
type contextKey int

const configKey contextKey = 0

// Package-level flag variables shared across run.go, file.go and repl.go.
var (
	flagDir      string
	flagModel    string
	flagAuto     bool
	flagNoMemory bool
	flagWorktree string
	flagLog      string
	flagDryRun   bool
	flagFile     string
)

var rootCmd = &cobra.Command{
	Use:   "xdreamer",
	Short: "An autonomous coding agent powered by LM Studio",
	// RunE is set in init() so all mode files can contribute.
}

func init() {
	f := rootCmd.PersistentFlags()
	f.StringVar(&flagDir, "dir", ".", "Project root directory")
	f.StringVar(&flagModel, "model", "", "LM Studio model identifier (overrides config)")
	f.BoolVar(&flagAuto, "auto", false, "Skip confirmation for destructive actions")
	f.BoolVar(&flagNoMemory, "no-memory", false, "Disable project memory for this run")
	f.StringVar(&flagWorktree, "worktree", "", "Override the git branch/worktree name")
	f.StringVar(&flagLog, "log", "", "Override transcript log output path")
	f.BoolVar(&flagDryRun, "dry-run", false, "Plan steps but do not execute any tool")
	f.StringVarP(&flagFile, "file", "f", "", "Read task from a file instead of the command line")

	rootCmd.PersistentPreRunE = loadAndStoreConfig
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		cfg := cfgFromContext(cmd.Context())
		switch {
		case len(args) == 1:
			return runOneShotMode(cmd.Context(), cfg, args[0])
		case flagFile != "":
			return runFileMode(cmd.Context(), cfg)
		default:
			return runREPL(cmd.Context(), cfg)
		}
	}
	rootCmd.SilenceUsage = true
}

// Execute is the CLI entrypoint called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// loadAndStoreConfig loads, validates and stores the config in the command context.
func loadAndStoreConfig(cmd *cobra.Command, _ []string) error {
	dir, err := filepath.Abs(flagDir)
	if err != nil {
		return fmt.Errorf("resolve dir: %w", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	applyFlagOverrides(cfg, dir)

	if err := config.Validate(cfg); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	cmd.SetContext(context.WithValue(cmd.Context(), configKey, cfg))
	return nil
}

// applyFlagOverrides pushes CLI flag values into cfg, taking precedence over
// values loaded from TOML files or environment variables.
func applyFlagOverrides(cfg *config.Config, dir string) {
	cfg.Dir = dir
	if flagModel != "" {
		cfg.Model.Model = flagModel
	}
	if flagAuto {
		cfg.Agent.Auto = true
	}
	if flagNoMemory {
		cfg.Memory.Enabled = false
	}
	if flagLog != "" {
		cfg.Output.LogDir = filepath.Dir(flagLog)
	}
}

// cfgFromContext retrieves the config stored by loadAndStoreConfig.
func cfgFromContext(ctx context.Context) *config.Config {
	return ctx.Value(configKey).(*config.Config)
}
