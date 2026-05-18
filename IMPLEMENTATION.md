# xdreamer — Implementation Plan

**Reference spec:** `SPEC.md`  
**Target language:** Go 1.22+  
**Date:** 2026-05-17

This document is a complete, step-by-step implementation guide for an AI coding agent.
Tasks are ordered strictly by dependency. Do not begin a phase until all phases it depends on are complete.
Every task states exactly which files to create or modify and what the acceptance criteria are.

---

## Coding Conventions (read before writing any code)

### Clean Code Rules

1. **One responsibility per function.** If a function needs a comment to explain what it does, split it.
2. **No function exceeds 40 lines.** Extract helpers with meaningful names instead.
3. **No magic values.** Every constant is a named `const` or `var`. No bare string literals or raw numbers in logic.
4. **Error messages are lowercase and contextual.** Always: `fmt.Errorf("load config: %w", err)`. Never: `fmt.Errorf("Error loading config: %v", err)`.
5. **Sentinel errors are declared at package level.** Example: `var ErrToolNotFound = errors.New("tool not found")`.
6. **No comments on obvious code.** Only comment when the *why* is non-obvious. A well-named function needs no docstring.
7. **No dead code.** If it is not used, delete it.
8. **Use `t.TempDir()`** in all tests that touch the filesystem.
9. **Table-driven tests** for any function with more than two input/output cases.
10. **`context.Context` is the first parameter** of every function that does I/O.

### SOLID Application

| Principle | Concrete rule in this codebase |
|---|---|
| **S** — Single Responsibility | Each package owns one concern. `config` only loads/validates config. `model` only talks to LM Studio. `tools` only executes actions. `rag` only retrieves context. `agent` only orchestrates. |
| **O** — Open/Closed | Add new tools by implementing `tools.Tool` and registering in the CLI wiring. Add new providers by implementing `model.Provider`. Do not add `if provider == "X"` branches in `agent`. |
| **L** — Liskov Substitution | Any `model.Provider` must return valid `Response` structs in all success cases. Any `tools.Tool` must return a non-empty string on success and a non-nil `error` on failure — never both or neither. Test mocks must obey the same contract as real implementations. |
| **I** — Interface Segregation | Do not add methods to an interface just because one implementation needs it. If only one caller uses a method, it does not belong on the interface. |
| **D** — Dependency Inversion | `agent.Runner` is constructed with interface types only. Concrete types (`*lmstudio.Provider`, `*chromem.Store`) are instantiated only in `cmd/xdreamer/main.go`. |

### Package Dependency Rules

```
cmd/xdreamer  →  agent, config, model, tools, rag, memory, worktree
agent         →  model, tools, rag, memory, worktree, config
tools/codegen →  model
rag           →  model, config
memory        →  config
worktree      →  (stdlib only)
model         →  (stdlib + go-openai only)
config        →  (stdlib + toml only)
```

No circular imports. No package imports `agent`. No package imports `cmd`.

---

## Phase 1 — Project Scaffold ✅

### Task 1.1 — Initialize Go module ✅

**Files to create:** `go.mod`, `cmd/xdreamer/main.go`

**Steps:**
- [x] Run `go mod init github.com/xdreamer/xdreamer`
- [x] Set minimum Go version to `go 1.22` — installed Go 1.26.3, module targets 1.26.3
- [x] Create `cmd/xdreamer/main.go`:

```go
package main

import "fmt"

func main() {
    fmt.Println("xdreamer v0.1.0")
}
```

- [x] Run `go build ./...` — must succeed

**Acceptance:** `go build ./...` exits 0. ✅

---

### Task 1.2 — Create package stubs ✅

**Files to create** (`package <name>` stub in each):
- `internal/config/config.go`
- `internal/model/provider.go`
- `internal/tools/tool.go`
- `internal/rag/engine.go`
- `internal/memory/memory.go`
- `internal/worktree/worktree.go`
- `internal/agent/runner.go`

Each file contains only `package <dirname>` as its first line. This establishes the import graph without implementing anything yet.

**Acceptance:** `go build ./...` exits 0. ✅

---

### Task 1.3 — Add third-party dependencies ✅

**Steps:**
- [x] `go get github.com/BurntSushi/toml@latest` — added v1.6.0
- [x] `go get github.com/spf13/cobra@latest` — added v1.10.2
- [x] `go get github.com/sashabaranov/go-openai@latest` — added v1.41.2
- [x] `go get github.com/philippgille/chromem-go@latest` — added v0.7.0
- [x] `go get github.com/pkoukk/tiktoken-go@latest` — added v0.1.8
- [x] `go mod tidy`
- [x] `go build ./...` — must succeed

**Note:** `go mod tidy` removes unused dependencies from `go.mod` because the Phase 1 stubs have no imports. All packages are downloaded to the module cache and will be re-added to `go.mod` automatically as Phase 2+ code imports them.

**Acceptance:** All five dependencies are cached in the module cache. `go build ./...` exits 0. ✅

---

## Phase 2 — Configuration ✅

### Task 2.1 — Define config structs and defaults ✅

**File:** `internal/config/config.go`

**Steps:**
- [x] Replace the stub with the following types:

```go
package config

type Config struct {
    Model  ModelConfig  `toml:"model"`
    Agent  AgentConfig  `toml:"agent"`
    RAG    RAGConfig    `toml:"rag"`
    Memory MemoryConfig `toml:"memory"`
    Output OutputConfig `toml:"output"`
}

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

type AgentConfig struct {
    Auto         bool   `toml:"auto"`
    MaxSteps     int    `toml:"max_steps"`
    WorktreeBase string `toml:"worktree_base"`
}

type RAGConfig struct {
    Enabled      bool     `toml:"enabled"`
    ChunkSize    int      `toml:"chunk_size"`
    ChunkOverlap int      `toml:"chunk_overlap"`
    TopK         int      `toml:"top_k"`
    IndexDir     string   `toml:"index_dir"`
    Ignore       []string `toml:"ignore"`
}

type MemoryConfig struct {
    Enabled bool   `toml:"enabled"`
    File    string `toml:"file"`
}

type OutputConfig struct {
    LogDir string `toml:"log_dir"`
}
```

- [x] Implement `Defaults() Config` that returns values matching SPEC.md §14.1 exactly:
  - `Model.Provider = "lmstudio"`, `Model.BaseURL = "http://localhost:1234/v1"`, `Model.Model = "qwen2.5-coder-7b"`, `Model.EmbedModel = "nomic-embed-text-v1.5"`, `Model.ContextWindow = 8192`, `Model.Temperature = 0.2`, `Model.MaxRetries = 3`, `Model.RetryDelayMS = 1000`
  - `Agent.MaxSteps = 50`, `Agent.WorktreeBase = ".xdreamer/worktrees"`
  - `RAG.Enabled = true`, `RAG.ChunkSize = 512`, `RAG.ChunkOverlap = 64`, `RAG.TopK = 10`, `RAG.IndexDir = ".xdreamer/index"`, `RAG.Ignore = ["vendor/", "node_modules/", ".git/"]`
  - `Memory.File = ".xdreamer/MEMORY.md"`
  - `Output.LogDir = "."`

- [x] Write `internal/config/config_test.go` verifying every default value

**Acceptance:** `go test ./internal/config/...` passes. ✅

---

### Task 2.2 — Implement config loader ✅

**File:** `internal/config/loader.go`

**Steps:**
- [x] Implement:

```go
// Load builds a Config by merging defaults, global config, project config,
// and environment variable overrides in that order.
func Load(projectDir string) (*Config, error)
```

- [x] Layer order:
  1. Start from `Defaults()`
  2. Merge `~/.xdreamer/config.toml` — missing file is **not** an error, return the default as-is
  3. Merge `<projectDir>/.xdreamer.toml` — missing file is **not** an error
  4. Apply env var overrides per SPEC.md §14.2 mapping

- [x] Implement a private `mergeEnvVars(cfg *Config)` function that applies each env var:
  - Read with `os.Getenv`; skip if empty
  - Parse `XDREAMER_AUTO`: accept `"true"` or `"1"` (case-insensitive) → `true`; anything else → `false`
  - Parse `XDREAMER_MAX_STEPS` with `strconv.Atoi`; on parse error, return `fmt.Errorf("XDREAMER_MAX_STEPS: %w", err)`

- [x] Write `internal/config/loader_test.go`:
  - Test defaults returned when no files exist and no env vars set
  - Test project `.xdreamer.toml` overrides a global field
  - Test `XDREAMER_MODEL_BASE_URL` env var overrides file config
  - Test `XDREAMER_AUTO=true` sets `Agent.Auto = true`
  - Test invalid `XDREAMER_MAX_STEPS` returns an error

**Acceptance:** `go test ./internal/config/...` passes. ✅

---

### Task 2.3 — Implement config validator ✅

**File:** `internal/config/validator.go`

**Steps:**
- [x] Implement:

```go
var (
    ErrMissingBaseURL   = errors.New("model.base_url is required")
    ErrMissingModel     = errors.New("model.model is required")
    ErrMissingEmbedModel = errors.New("model.embed_model is required")
    ErrInvalidMaxSteps  = errors.New("agent.max_steps must be greater than 0")
    ErrInvalidChunkSize = errors.New("rag.chunk_size must be greater than 0")
)

// Validate checks that a loaded Config has all required fields and sane values.
func Validate(cfg *Config) error
```

- [x] Validate rules:
  - `Model.BaseURL` must not be empty → `ErrMissingBaseURL`
  - `Model.Model` must not be empty → `ErrMissingModel`
  - `Model.EmbedModel` must not be empty → `ErrMissingEmbedModel`
  - `Agent.MaxSteps` must be > 0 → `ErrInvalidMaxSteps`
  - `RAG.ChunkSize` must be > 0 if `RAG.Enabled` → `ErrInvalidChunkSize`
  - Return the first error found (not a combined error)

- [x] Write `internal/config/validator_test.go`:
  - Test that `Defaults()` passes `Validate()`
  - Test each invalid field returns its sentinel error

**Acceptance:** `go test ./internal/config/...` passes. ✅

---

## Phase 3 — Model Provider ✅

### Task 3.1 — Define Provider interface and types ✅

**File:** `internal/model/provider.go`

**Steps:**
- [x] Replace the stub with:

```go
package model

import "context"

type Role string

const (
    RoleSystem    Role = "system"
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleTool      Role = "tool"
)

type Message struct {
    Role       Role
    Content    string
    ToolCallID string    // set when Role == RoleTool
    ToolCalls  []ToolCall
}

type ToolCall struct {
    ID        string
    Name      string
    Arguments string    // raw JSON string
}

type ToolDefinition struct {
    Name        string
    Description string
    Parameters  map[string]any  // JSON Schema "properties" object
}

type Response struct {
    Content   string
    ToolCalls []ToolCall
    Done      bool    // true when model signals DONE
}

// Provider is the single abstraction the agent uses to talk to a language model.
// Embed is used by the RAG engine independently of Chat.
type Provider interface {
    Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (Response, error)
    Embed(ctx context.Context, text string) ([]float32, error)
}
```

- [x] No concrete types in this file — only the interface and shared types.

**Acceptance:** `go build ./internal/model/...` exits 0. ✅

---

### Task 3.2 — Implement LM Studio provider ✅

**File:** `internal/model/lmstudio.go`

**Steps:**
- [x] Define:

```go
type LMStudio struct {
    client      *openai.Client
    model       string
    embedModel  string
    temperature float32
    maxRetries  int
    retryDelay  time.Duration
}

func NewLMStudio(baseURL, model, embedModel string, temperature float32, maxRetries int, retryDelayMS int) *LMStudio
```

- [x] `NewLMStudio`: configure `openai.ClientConfig` with `BaseURL` and an empty API key (LM Studio does not require one); wrap in `openai.NewClientWithConfig`.

- [x] `Chat()` implementation:
  1. Convert `[]model.Message` → `[]openai.ChatCompletionMessage` (map each Role and ToolCalls field)
  2. Convert `[]model.ToolDefinition` → `[]openai.Tool` with `Type: openai.ToolTypeFunction`
  3. Call `client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{...})`
  4. On HTTP 429 or 5xx: wait `retryDelay`, retry up to `maxRetries` times; use a private `shouldRetry(err error) bool` helper
  5. Convert `openai.ChatCompletionMessage` back to `model.Response`
  6. Set `Response.Done = true` if the response content (trimmed) ends with the line `"DONE"`

- [x] `Embed()` implementation:
  1. Call `client.CreateEmbeddings(ctx, openai.EmbeddingRequest{Model: embedModel, Input: []string{text}})`
  2. On retry-eligible errors, retry up to `maxRetries` times
  3. Return `response.Data[0].Embedding` as `[]float32`

- [x] Write `internal/model/lmstudio_test.go` using `httptest.NewServer`:
  - Test Chat success with tool calls in response
  - Test Chat with DONE in response sets `Response.Done = true`
  - Test Chat retries on 429 and succeeds on second attempt
  - Test Chat returns error after exhausting retries
  - Test Embed success
  - Test Embed error propagation

**Acceptance:** `go test ./internal/model/...` passes. ✅

---

## Phase 4 — Tool Registry ✅

### Task 4.1 — Define Tool interface and Registry ✅

**File:** `internal/tools/tool.go`

**Steps:**
- [x] Replace the stub with:

```go
package tools

import (
    "context"
    "encoding/json"

    "github.com/xdreamer/xdreamer/internal/model"
)

var ErrToolNotFound = errors.New("tool not found")

// Tool is the interface every built-in and future tool must satisfy.
type Tool interface {
    Name()        string
    Description() string
    Parameters()  map[string]any
    Execute(ctx context.Context, params json.RawMessage) (string, error)
    Destructive() bool
}

// Registry holds all registered tools and provides lookup and conversion.
type Registry struct {
    tools map[string]Tool
}

func NewRegistry() *Registry {
    return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool)
func (r *Registry) Get(name string) (Tool, bool)
func (r *Registry) All() []Tool

// Definitions converts all registered tools to model.ToolDefinition for the LLM.
func (r *Registry) Definitions() []model.ToolDefinition
```

- [x] `Definitions()` must map each `Tool` to `model.ToolDefinition{Name, Description, Parameters}`.
- [x] Write `internal/tools/tool_test.go`:
  - Register a mock tool, verify `Get` finds it, verify `Definitions()` output matches

**Acceptance:** `go test ./internal/tools/...` passes. ✅

---

### Task 4.2 — Implement file tools ✅

**File:** `internal/tools/files.go`

- [x] `read_file` — safe; `write_file` — destructive; `delete_file` — destructive
- [x] `NewReadFileTool()`, `NewWriteFileTool()`, `NewDeleteFileTool()` exported
- [x] `files_test.go` with subtests using `t.TempDir()`

**Acceptance:** `go test ./internal/tools/...` passes. ✅

---

### Task 4.3 — Implement directory tool ✅

**File:** `internal/tools/dir.go`

- [x] `list_dir` — safe; flat (`os.ReadDir`) and recursive (`filepath.WalkDir`); skips `.git` and `.xdreamer`
- [x] `dir_test.go` verifying flat/recursive listing and `.git` exclusion

**Acceptance:** `go test ./internal/tools/...` passes. ✅

---

### Task 4.4 — Implement search tool ✅

**File:** `internal/tools/search.go`

- [x] `search_code` — safe; `const maxSearchMatches = 200`; `globMatches()` handles `**/` prefix
- [x] `search_test.go` with table-driven `globMatches` tests and integration tests

**Acceptance:** `go test ./internal/tools/...` passes. ✅

---

### Task 4.5 — Implement shell tool ✅

**File:** `internal/tools/shell.go`

- [x] `run_shell` — destructive; interpreter constants; non-zero exit prefixed with `[exit N]`
- [x] `shell_test.go`: success, non-zero exit, context cancellation

**Acceptance:** `go test ./internal/tools/...` passes. ✅

---

### Task 4.6 — Implement git tools ✅

**File:** `internal/tools/git.go`

- [x] `runGit` shared helper; `git_status`, `git_diff`, `git_log`, `git_commit`
- [x] `git_test.go` using `initGitRepo` helper with temp dir

**Acceptance:** `go test ./internal/tools/...` passes. ✅

---

### Task 4.7 — Implement web tools ✅

**File:** `internal/tools/web.go`

- [x] `web_fetch` — HTML stripping, `defaultMaxBytes`; `web_search` — DDG API, `ddgBaseURL` var for test injection
- [x] `web_test.go` with `httptest.NewServer` mocks

**Acceptance:** `go test ./internal/tools/...` passes. ✅

---

### Task 4.8 — Implement generate_code tool ✅

**File:** `internal/tools/codegen.go`

- [x] `codeGenTool` with injected `model.Provider`; single non-tool Chat call
- [x] `codegen_test.go` with `mockProvider` and `capturingMockProvider`

**Acceptance:** `go test ./internal/tools/...` passes. ✅

---

## Phase 5 — Git Worktree ✅

### Task 5.1 — Implement worktree manager ✅

**File:** `internal/worktree/worktree.go`

**Steps:**
- [x] Define:

```go
package worktree

import "errors"

var ErrNotGitRepo = errors.New("directory is not a git repository")

type Worktree struct {
    Branch string
    Path   string
    Root   string
}

// Slug converts a task string into a safe git branch name segment.
func Slug(task string) string

// Create makes a new git worktree for the given task slug under <root>/<base>/<slug>.
func Create(root, base, slug string) (*Worktree, error)

// Remove deletes the worktree and its branch.
func (w *Worktree) Remove() error
```

- [x] `Slug()`: lowercase; `slugIllegal` regex replaces non-[a-z0-9-]; `slugMultiDash` collapses dashes; trim; truncate to 40 with trailing-dash trim
- [x] `Create()`: `verifyGitRepo` → `addWorktree` (tries `-b`, falls back to attach on existing branch)
- [x] `Remove()`: `git worktree remove --force` + `git branch -D`; shared `gitRun` helper
- [x] `worktree_test.go`: 11-case Slug table test, trailing-dash truncation edge case, Create/Remove roundtrip, existing-branch attach, ErrNotGitRepo

**Acceptance:** `go test ./internal/worktree/...` passes. ✅ (15/15)

---

## Phase 6 — RAG Engine

### Task 6.1 — Implement text chunker

**File:** `internal/rag/chunker.go`

**Steps:**
- [ ] Define:

```go
package rag

import "crypto/sha256"
import "fmt"

type Chunk struct {
    FilePath  string
    StartLine int
    Content   string
    Hash      string  // hex(sha256(Content))
}

type Chunker struct {
    chunkSize    int
    chunkOverlap int
}

func NewChunker(chunkSize, chunkOverlap int) *Chunker
func (c *Chunker) ChunkFile(filePath string, content []byte) ([]Chunk, error)
```

- [ ] `ChunkFile()`:
  1. Split content by newline into lines
  2. Use `tiktoken-go` with encoding `cl100k_base` to count tokens per line
  3. Accumulate lines into a chunk until the token count reaches `chunkSize`
  4. At each chunk boundary, carry over the last `chunkOverlap` tokens worth of lines into the next chunk
  5. Compute `Hash = fmt.Sprintf("%x", sha256.Sum256([]byte(chunk.Content)))`
  6. Return empty slice (not error) for empty files

- [ ] Declare `const encodingName = "cl100k_base"` — no bare string in logic

- [ ] Write `internal/rag/chunker_test.go`:
  - Empty file → zero chunks
  - File smaller than one chunk → one chunk
  - File larger than one chunk → multiple chunks; verify overlap content

**Acceptance:** `go test ./internal/rag/...` passes.

---

### Task 6.2 — Implement vector store

**File:** `internal/rag/store.go`

**Steps:**
- [ ] Define:

```go
type EmbedFunc func(ctx context.Context, text string) ([]float32, error)

type Store struct {
    db         *chromem.DB
    collection *chromem.Collection
    indexDir   string
}

func OpenStore(ctx context.Context, indexDir string) (*Store, error)
func (s *Store) Upsert(ctx context.Context, chunks []Chunk, embed EmbedFunc) error
func (s *Store) Query(ctx context.Context, query string, topK int, embed EmbedFunc) ([]Chunk, error)
func (s *Store) Close() error
```

- [ ] `OpenStore()`: create `indexDir` with `os.MkdirAll`; open `chromem.NewPersistentDB(filepath.Join(indexDir, "vectors.db"), false)`; get-or-create a collection named `"codebase"`

- [ ] `Upsert()`: for each chunk, call `embed(ctx, chunk.Content)`; add to collection with document ID = `chunk.Hash` and metadata `{"file": chunk.FilePath, "line": strconv.Itoa(chunk.StartLine), "content": chunk.Content}`; skip if ID already exists

- [ ] `Query()`: embed query string, call `collection.Query(ctx, queryEmbedding, topK, nil, nil)`; reconstruct `[]Chunk` from result metadata

- [ ] Write `internal/rag/store_test.go` using `t.TempDir()`:
  - Use a deterministic mock `EmbedFunc` that returns a fixed vector per text
  - Upsert 5 chunks, query, verify results are returned
  - Verify duplicate hashes are not stored twice

**Acceptance:** `go test ./internal/rag/...` passes.

---

### Task 6.3 — Implement index manifest

**File:** `internal/rag/manifest.go`

**Steps:**
- [ ] Define:

```go
type manifest struct {
    Files map[string]string `json:"files"` // path → sha256 hash
}

func loadManifest(indexDir string) (*manifest, error)
func (m *manifest) save(indexDir string) error
func (m *manifest) isChanged(filePath, hash string) bool
func (m *manifest) mark(filePath, hash string)
```

- [ ] `loadManifest()`: read `<indexDir>/manifest.json`; if not found, return empty manifest (not error)
- [ ] `save()`: marshal manifest to JSON and write to `<indexDir>/manifest.json`
- [ ] `isChanged()`: return true if path is not in `Files` or hash differs
- [ ] `mark()`: update `Files[filePath] = hash`

- [ ] Write `internal/rag/manifest_test.go` using `t.TempDir()`

**Acceptance:** `go test ./internal/rag/...` passes.

---

### Task 6.4 — Implement RAG engine orchestrator

**File:** `internal/rag/engine.go`

**Steps:**
- [ ] Replace the stub with:

```go
type Engine struct {
    store    *Store
    chunker  *Chunker
    embedFn  EmbedFunc
    cfg      config.RAGConfig
    manifest *manifest
}

func NewEngine(ctx context.Context, cfg config.RAGConfig, embedFn EmbedFunc) (*Engine, error)
func (e *Engine) Index(ctx context.Context, projectRoot string) error
func (e *Engine) Retrieve(ctx context.Context, query string) ([]Chunk, error)
func (e *Engine) Close() error
```

- [ ] `NewEngine()`: call `OpenStore`, `loadManifest`, create `NewChunker`

- [ ] `Index()`:
  1. `filepath.WalkDir(projectRoot, ...)` to visit all files
  2. Skip directories matching any pattern in `cfg.Ignore` (prefix match or `filepath.Match`)
  3. Skip binary files: read first 512 bytes; if any byte is 0x00, skip
  4. Read file; compute SHA-256 hash; skip if `manifest.isChanged` returns false
  5. Call `chunker.ChunkFile`; call `store.Upsert` with chunks and `embedFn`
  6. Call `manifest.mark(filePath, hash)` on success
  7. After all files: call `manifest.save(cfg.IndexDir)`

- [ ] `Retrieve()`: call `store.Query(ctx, query, cfg.TopK, embedFn)` and return results

- [ ] Write `internal/rag/engine_test.go` with a temp project containing 3 Go files

**Acceptance:** `go test ./internal/rag/...` passes.

---

## Phase 7 — Memory System

### Task 7.1 — Implement memory

**File:** `internal/memory/memory.go`

**Steps:**
- [ ] Replace the stub with:

```go
package memory

type Memory struct {
    filePath string   // empty when disabled
    content  string
    enabled  bool
}

// Load reads project memory from the configured file.
// Returns a no-op Memory if cfg.Enabled is false.
func Load(cfg config.MemoryConfig) (*Memory, error)

// Content returns the raw memory content, or empty string if disabled.
func (m *Memory) Content() string

// Append adds a new section to memory in-memory. Does not write to disk.
func (m *Memory) Append(section, text string)

// Save writes updated memory to disk. No-op if disabled.
func (m *Memory) Save() error
```

- [ ] `Load()`: if `!cfg.Enabled`, return `&Memory{enabled: false}` — all methods on this are no-ops
- [ ] If enabled: read `cfg.File` with `os.ReadFile`; if file not found, start with empty string; store content
- [ ] `Append()`: if disabled, return; append `"\n## " + section + "\n" + text + "\n"` to `m.content`
- [ ] `Save()`: if disabled, return nil; `os.MkdirAll(filepath.Dir(m.filePath), 0755)` then `os.WriteFile`

- [ ] Write `internal/memory/memory_test.go`:
  - Disabled memory — all methods are no-ops, no file access
  - Enabled memory — Load + Append + Save roundtrip reads the same content back
  - Missing file with enabled memory — starts empty without error

**Acceptance:** `go test ./internal/memory/...` passes.

---

## Phase 8 — Agent

### Task 8.1 — Implement Transcript

**File:** `internal/agent/transcript.go`

**Steps:**
- [ ] Define:

```go
package agent

type ToolResult struct {
    ToolName string
    Params   string
    Output   string
    IsError  bool
    Skipped  bool  // true when dry-run or user denied
}

type Step struct {
    Number      int
    UserMessage string
    LLMResponse string
    ToolResults []ToolResult
}

type Transcript struct {
    Task  string
    Steps []Step
}

func (t *Transcript) AddStep(s Step)
func (t *Transcript) WriteTo(w io.Writer) error
func (t *Transcript) SaveToFile(path string) error
```

- [ ] `WriteTo()`: write a human-readable log. Each step:
  ```
  === Step N ===
  [User] <UserMessage>
  [LLM]  <LLMResponse>
  [Tool: <ToolName>] params: <Params>
  [Result] <Output>
  ```

- [ ] Write `internal/agent/transcript_test.go`

**Acceptance:** `go test ./internal/agent/...` passes.

---

### Task 8.2 — Implement system prompt renderer

**File:** `internal/agent/prompt.go`

**Steps:**
- [ ] Define:

```go
type promptData struct {
    WorktreePath  string
    MemoryContent string
}

const systemPromptTemplate = `You are xdreamer, an autonomous coding agent. Work step by step using the tools provided.
After each tool result, select the next action. When the task is complete, write a short
summary of what you did and end your message with the single word DONE on its own line.

Rules:
- Call exactly one tool per turn.
- Never guess file contents — always use read_file before modifying a file.
- Always use write_file to persist changes — never output code in prose.
- Run shell commands in the worktree directory: {{.WorktreePath}}
- Project memory:
{{.MemoryContent}}`

func renderSystemPrompt(worktreePath, memoryContent string) (string, error)
```

- [ ] Use `text/template` to render the template — do not use `strings.Replace`
- [ ] Write a test verifying the template renders the worktree path and memory content correctly

**Acceptance:** `go test ./internal/agent/...` passes.

---

### Task 8.3 — Implement Confirmer interface

**File:** `internal/agent/confirm.go`

**Steps:**
- [ ] Define:

```go
// Confirmer decides whether a destructive action may proceed.
type Confirmer interface {
    Confirm(ctx context.Context, toolName, description string) (bool, error)
}

// TerminalConfirmer asks the user via stdin.
type TerminalConfirmer struct {
    In  io.Reader
    Out io.Writer
}

// AutoConfirmer always approves. Used with --auto flag.
type AutoConfirmer struct{}
```

- [ ] `TerminalConfirmer.Confirm()`:
  1. Print to `Out`: `"[destructive] <toolName>: <description>\nProceed? (y/N): "`
  2. Read one line from `In` with `bufio.NewScanner`
  3. Return true if input (trimmed, lowercased) is `"y"` or `"yes"`

- [ ] `AutoConfirmer.Confirm()`: always return `true, nil`

- [ ] Write `internal/agent/confirm_test.go`:
  - `AutoConfirmer` always returns true
  - `TerminalConfirmer` with mock reader returning `"y"` returns true
  - `TerminalConfirmer` with mock reader returning `"n"` returns false
  - `TerminalConfirmer` with mock reader returning `"YES"` returns true (case-insensitive)

**Acceptance:** `go test ./internal/agent/...` passes.

---

### Task 8.4 — Implement Runner (step loop)

**File:** `internal/agent/runner.go`

This is the most critical file. Implement with care. Every dependency is an interface.

**Steps:**
- [ ] Define:

```go
type RunnerConfig struct {
    MaxSteps   int
    DryRun     bool
    WorktreePath string
}

type Runner struct {
    cfg        RunnerConfig
    provider   model.Provider
    registry   *tools.Registry
    rag        *rag.Engine
    memory     *memory.Memory
    confirmer  Confirmer
    transcript *Transcript
}

func NewRunner(
    cfg RunnerConfig,
    provider model.Provider,
    registry *tools.Registry,
    ragEngine *rag.Engine,
    mem *memory.Memory,
    confirmer Confirmer,
) *Runner

func (r *Runner) Run(ctx context.Context, task string) (*Transcript, error)
```

- [ ] `Run()` — implement exactly this sequence:

```
1.  Render system prompt via renderSystemPrompt(cfg.WorktreePath, memory.Content())
2.  history := []model.Message{{Role: RoleSystem, Content: systemPrompt}, {Role: RoleUser, Content: task}}
3.  stepCount := 0
4.  LOOP:
    a.  stepCount++; if stepCount > cfg.MaxSteps → return nil, ErrMaxStepsExceeded
    b.  ragChunks := r.rag.Retrieve(ctx, lastUserMessage(history))
    c.  contextMsg := buildRAGMessage(ragChunks)  // "Relevant context:\n<chunks>"
    d.  promptHistory := append(history, contextMsg)  // do NOT mutate history
    e.  response, err := r.provider.Chat(ctx, promptHistory, r.registry.Definitions())
    f.  if err → return nil, fmt.Errorf("step %d: chat: %w", stepCount, err)
    g.  append response as model.Message{Role: RoleAssistant, Content: response.Content, ToolCalls: response.ToolCalls} to history
    h.  if response.Done → break loop
    i.  if len(response.ToolCalls) == 0 → append {Role: RoleUser, Content: "continue"} to history; loop
    j.  step := Step{Number: stepCount, LLMResponse: response.Content}
    k.  for each toolCall in response.ToolCalls:
        i.   tool, ok := r.registry.Get(toolCall.Name); if !ok → record ToolResult{IsError: true}, append error to history, continue
        ii.  if cfg.DryRun → record ToolResult{Skipped: true}, append "[dry-run] skipped" to history, continue
        iii. if tool.Destructive() → allowed, err = r.confirmer.Confirm(ctx, toolCall.Name, toolCall.Arguments)
             if !allowed → record ToolResult{Skipped: true}, append "[denied] user denied" to history, continue
        iv.  output, execErr := tool.Execute(ctx, json.RawMessage(toolCall.Arguments))
        v.   toolResult := ToolResult{ToolName: toolCall.Name, Params: toolCall.Arguments, Output: output, IsError: execErr != nil}
        vi.  append model.Message{Role: RoleTool, ToolCallID: toolCall.ID, Content: output} to history
        vii. append toolResult to step.ToolResults
    l.  r.transcript.AddStep(step)
5.  return r.transcript, nil
```

- [ ] Declare sentinel errors:
  ```go
  var ErrMaxStepsExceeded = errors.New("max steps exceeded")
  ```

- [ ] Private helper `lastUserMessage(history []model.Message) string`: return content of the last message with `Role == RoleUser`

- [ ] Private helper `buildRAGMessage(chunks []rag.Chunk) model.Message`: format as `"Relevant context:\n\n<file>:<line>\n<content>\n---\n..."`, Role: RoleSystem

- [ ] Write `internal/agent/runner_test.go` using mock implementations of all interfaces:
  - Test 2-step run: first response has a tool call, second response has Done=true → transcript has 2 steps
  - Test max steps exceeded returns `ErrMaxStepsExceeded`
  - Test dry-run: tool call is skipped, result shows Skipped=true
  - Test destructive tool denied by confirmer: ToolResult.Skipped=true
  - Test unknown tool name: ToolResult.IsError=true
  - Test RAG context is injected into prompt

**Acceptance:** `go test ./internal/agent/...` passes.

---

### Task 8.5 — Implement output writer

**File:** `internal/agent/output.go`

**Steps:**
- [ ] Define:

```go
// WritePatch runs git diff HEAD in worktreePath and saves the output as a patch file.
func WritePatch(ctx context.Context, worktreePath, outputDir, slug string) (patchPath string, err error)

// GenerateSummary asks the model to summarize the run and writes SUMMARY.md to worktreePath.
func GenerateSummary(ctx context.Context, provider model.Provider, transcript *Transcript, worktreePath string) error
```

- [ ] `WritePatch()`:
  1. Run `git -C <worktreePath> diff HEAD` via `exec.CommandContext`
  2. Write output to `filepath.Join(outputDir, "xdreamer-"+slug+".patch")`
  3. Return the patch file path

- [ ] `GenerateSummary()`:
  1. Build a single user message: `"Summarize what you did during this coding session in markdown format. Be concise."`
  2. Append the transcript as context: join all step LLMResponse values
  3. Call `provider.Chat(ctx, messages, nil)` with no tools
  4. Write `response.Content` to `filepath.Join(worktreePath, "SUMMARY.md")`

- [ ] Write tests:
  - `WritePatch` with a temp git repo that has an unstaged change
  - `GenerateSummary` with a mock provider, verify SUMMARY.md is written with model content

**Acceptance:** `go test ./internal/agent/...` passes.

---

## Phase 9 — CLI Layer

### Task 9.1 — Implement root command

**Files:**
- `cmd/xdreamer/main.go` (rewrite)
- `cmd/xdreamer/cmd/root.go` (new)

**Steps:**
- [ ] Replace `main.go` with:

```go
package main

import "github.com/xdreamer/xdreamer/cmd/xdreamer/cmd"

func main() {
    cmd.Execute()
}
```

- [ ] In `cmd/root.go`, define the cobra root command:
  - Use: `"xdreamer"`
  - Short: `"An autonomous coding agent powered by LM Studio"`
  - Persistent flags matching SPEC.md §6.2 exactly:
    - `--model string`
    - `--dir string` (default `"."`)
    - `--auto bool`
    - `--no-memory bool`
    - `--worktree string`
    - `--log string`
    - `--dry-run bool`
  - `PersistentPreRunE`: call `config.Load(dir)`, then `config.Validate(cfg)`, apply flag overrides, store `*config.Config` in cobra command context via `cmd.SetContext(context.WithValue(cmd.Context(), configKey, cfg))`

- [ ] Define an unexported `contextKey` type and `configKey` constant to avoid collisions in context

- [ ] Export `Execute() ` which calls `rootCmd.Execute()`

**Acceptance:** `xdreamer --help` lists all flags.

---

### Task 9.2 — Implement shared wiring helper

**File:** `cmd/xdreamer/cmd/wire.go`

All three modes (one-shot, file, REPL) share the same dependency wiring. Extract it:

```go
type session struct {
    cfg       *config.Config
    provider  model.Provider
    registry  *tools.Registry
    ragEngine *rag.Engine
    memory    *memory.Memory
}

func newSession(ctx context.Context, cfg *config.Config) (*session, error)
func (s *session) close() error
```

- [ ] `newSession()`:
  1. Create `model.NewLMStudio(cfg.Model.BaseURL, ...)` — SRP: wiring only, no logic here
  2. If `cfg.RAG.Enabled`: create `rag.NewEngine(ctx, cfg.RAG, provider.Embed)` and call `engine.Index(ctx, cfg.Dir)`
  3. Create `memory.Load(cfg.Memory)`
  4. Create `tools.NewRegistry()`, register all built-in tools in order:
     - `NewReadFileTool()`, `NewWriteFileTool()`, `NewDeleteFileTool()`
     - `NewListDirTool()`, `NewSearchCodeTool()`
     - `NewRunShellTool()`
     - `NewGitStatusTool()`, `NewGitDiffTool()`, `NewGitLogTool()`, `NewGitCommitTool()`
     - `NewWebFetchTool()`, `NewWebSearchTool()`
     - `NewCodeGenTool(provider)`
  5. Return `&session{...}`

- [ ] `close()`: call `ragEngine.Close()` if non-nil

**Acceptance:** `go build ./cmd/...` exits 0.

---

### Task 9.3 — Implement shared task runner helper

**File:** `cmd/xdreamer/cmd/wire.go` (add to existing file)

All three modes call the same sequence after getting a task string:

```go
func runTask(ctx context.Context, s *session, cfg *config.Config, task, worktreeName string, dryRun, auto bool) error
```

- [ ] `runTask()`:
  1. Compute slug: `worktree.Slug(task)` (or use `worktreeName` if overridden)
  2. Create worktree: `worktree.Create(cfg.Dir, cfg.Agent.WorktreeBase, slug)`
  3. Choose confirmer: `AutoConfirmer{}` if `auto`, else `TerminalConfirmer{In: os.Stdin, Out: os.Stderr}`
  4. Create runner: `agent.NewRunner(agent.RunnerConfig{MaxSteps: cfg.Agent.MaxSteps, DryRun: dryRun, WorktreePath: wt.Path}, s.provider, s.registry, s.ragEngine, s.memory, confirmer)`
  5. Call `runner.Run(ctx, task)` → get transcript
  6. Call `agent.GenerateSummary(ctx, s.provider, transcript, wt.Path)`
  7. Call `gitCommitTool` inline or via shell: `git -C <wt.Path> add -A && git commit -m "xdreamer: <task>"`
  8. Call `agent.WritePatch(ctx, wt.Path, cfg.Output.LogDir, slug)` — print patch path to stderr
  9. Call `transcript.SaveToFile(filepath.Join(cfg.Output.LogDir, "xdreamer-"+slug+".log"))`
  10. Print diff to stdout: run `git -C <wt.Path> diff HEAD` and print
  11. Print: `"Branch: xdreamer/<slug>"`
  12. Prompt merge UX: `"[M]erge / [K]eep branch / [D]iscard? "` — read stdin
      - `M`: run `git -C <cfg.Dir> merge <wt.Branch>`
      - `K`: print `"Branch kept: <wt.Branch>"`; do not delete worktree
      - `D`: call `wt.Remove()`

**Acceptance:** `go build ./cmd/...` exits 0.

---

### Task 9.4 — Implement one-shot mode

**File:** `cmd/xdreamer/cmd/run.go`

**Steps:**
- [ ] Register a `RunE` on the root command that fires when exactly one positional argument is given:
  1. Extract `*config.Config` from context
  2. Call `newSession(ctx, cfg)` → defer `session.close()`
  3. Call `runTask(ctx, session, cfg, args[0], flagWorktree, flagDryRun, flagAuto)`

**Acceptance:** `xdreamer --auto "list all Go files"` runs against a valid git repo without panic.

---

### Task 9.5 — Implement task-file mode

**File:** `cmd/xdreamer/cmd/file.go`

**Steps:**
- [ ] Add `-f` / `--file` persistent flag to root command
- [ ] In root `PersistentPreRunE` (or in `RunE`): if `--file` is set, read the file with `os.ReadFile`, use its content as the task string
- [ ] Pass task string to `runTask()` — identical flow to one-shot

**Acceptance:** `xdreamer -f task.md` reads the file and runs the agent.

---

### Task 9.6 — Implement REPL mode

**File:** `cmd/xdreamer/cmd/repl.go`

**Steps:**
- [ ] When the root command is invoked with zero arguments and no `--file` flag:
  1. Print banner: `"xdreamer — type your task, or /exit to quit"`
  2. Create `newSession(ctx, cfg)` once — shared across all REPL tasks
  3. Start `bufio.NewScanner(os.Stdin)` loop:
     - Read line; trim whitespace
     - If empty: loop
     - If starts with `/`: handle as REPL command (see below)
     - Otherwise: call `runTask(ctx, session, cfg, line, "", cfg.Flags.DryRun, cfg.Flags.Auto)`
     - After each task: print `"\n--- ready ---\n"`
  4. On EOF or `/exit`: call `session.close()`, print `"bye"`, return

- [ ] REPL command handler (private `handleReplCommand(cmd string, session *session, cfg *config.Config)`):
  - `/status`: print active worktree branches via `git worktree list`
  - `/memory`: print `session.memory.Content()` (or `"memory disabled"`)
  - `/undo`: run `git -C <latest worktree path> reset --soft HEAD~1`; print result
  - `/exit`: return a sentinel `errExit` to break the loop

**Acceptance:** Running `xdreamer` with no args shows the REPL prompt and responds to `/exit`.

---

## Phase 10 — End-to-End Test

### Task 10.1 — Smoke test

**File:** `test/e2e/smoke_test.go`

**Steps:**
- [ ] Build the binary in `TestMain`: `go build -o testbin/xdreamer ./cmd/xdreamer`

- [ ] `TestOneShotMode`:
  1. Create a temp git repo with a single file `main.go` containing a known bug (e.g., wrong return value)
  2. Start a mock LM Studio server with `httptest.NewServer` that responds in order:
     - Chat call 1: returns tool call `{"tool": "read_file", "params": {"path": "main.go"}}`
     - Chat call 2: returns tool call `{"tool": "write_file", "params": {"path": "main.go", "content": "<fixed content>"}}`
     - Chat call 3: returns `Response{Content: "Fixed the bug. DONE", Done: true}`
     - Embed calls: return a fixed `[]float32{0.1, 0.2, ...}`
  3. Set env vars: `XDREAMER_MODEL_BASE_URL=<mock server URL>`, `XDREAMER_MODEL=test`, `XDREAMER_EMBED_MODEL=test`
  4. Run: `./testbin/xdreamer --auto --dir <tempRepo> "fix the bug"` via `exec.Command`
  5. Assert:
     - Exit code is 0
     - `main.go` in the worktree contains the fixed content
     - `SUMMARY.md` exists in the worktree
     - A `.patch` file exists in the log dir
     - A `.log` file exists in the log dir

- [ ] `TestDryRun`:
  - Same setup but add `--dry-run` flag
  - Assert: no files modified in the worktree, `.patch` file is empty or absent

**Acceptance:** `go test ./test/e2e/...` passes.

---

## Phase 11 — Example Config and Docs

### Task 11.1 — Write example config

**File:** `.xdreamer.toml`

- [ ] Write the full config from SPEC.md §14.1 with a comment on every field explaining its purpose and accepted values

**Acceptance:** File exists and is valid TOML (`go run github.com/BurntSushi/toml/cmd/tomlv .xdreamer.toml` exits 0).

---

### Task 11.2 — Verify directory layout

**Steps:**
- [ ] Open `SPEC.md` §15 and compare it to the actual directory structure
- [ ] If any files in the implementation do not appear in the layout, add them
- [ ] If any files in the layout were not created, note them as missing

**Acceptance:** `SPEC.md §15` matches reality.

---

### Task 11.3 — Final build and test

**Steps:**
- [ ] `go build ./...` — must exit 0
- [ ] `go test ./...` — all tests must pass
- [ ] `go vet ./...` — must exit 0
- [ ] `go build -o xdreamer ./cmd/xdreamer` — binary must exist

**Acceptance:** All four commands exit 0.

---

## Dependency Graph

```
Phase 1 (scaffold)
  └─► Phase 2 (config)
        ├─► Phase 3 (model provider)
        │     ├─► Phase 4 (tools)     ← codegen needs model.Provider
        │     └─► Phase 6 (RAG)       ← engine.go needs EmbedFunc from model.Provider
        ├─► Phase 5 (worktree)         ← depends only on stdlib
        └─► Phase 7 (memory)           ← depends only on config
Phase 8 (agent) ────► depends on 2, 3, 4, 5, 6, 7
Phase 9 (CLI)   ────► depends on all above
Phase 10 (e2e)  ────► depends on Phase 9
Phase 11 (docs) ────► last
```

Phases 5 and 7 can be done in parallel with Phases 3 and 4.

---

## SPEC Coverage Matrix

Verify every SPEC section is covered by at least one implementation task:

| SPEC Section | Covered by Task(s) |
|---|---|
| §5 — CLI Modes & Flags | 9.1, 9.4, 9.5, 9.6 |
| §6.1 — Task Lifecycle | 8.4, 9.3 |
| §6.2 — Autonomy Model | 8.3, 8.4 |
| §6.3 — Step Planning (linear, max_steps) | 8.4 |
| §6.4 — System Prompt Template | 8.2 |
| §7 — Tool Registry & Interface | 4.1 |
| §7.2 — All 13 built-in tools | 4.2–4.8 |
| §8.1 — model.Provider interface | 3.1 |
| §8.2 — LM Studio client | 3.2 |
| §8.3 — Tool calling + JSON fallback | 3.2 |
| §8.4 — Retry policy (max_retries, retry_delay_ms) | 3.2 |
| §9 — RAG Engine | 6.1–6.4 |
| §10 — Memory System | 7.1 |
| §11 — Git Worktree Isolation | 5.1 |
| §12 — Output Artifacts (log, summary, patch) | 8.5, 9.3 |
| §13 — Configuration layers & env vars | 2.1, 2.2 |
| §13 — Config validation | 2.3 |
| §6.3 — REPL /commands | 9.6 |
| §6.2 — --dry-run flag | 8.4, 9.3 |
| §11 — Merge UX (M/K/D) | 9.3 |
| §16 — Resolved: retry policy | 3.2 |
| §16 — Resolved: chromem-go store | 6.2 |
| §16 — Resolved: generate_code as tool | 4.8 |

---

## Completion Checklist

- [x] Phase 1 — Project scaffold
- [x] Phase 2 — Configuration (structs + loader + validator)
- [x] Phase 3 — Model provider interface + LM Studio implementation
- [x] Phase 4 — Tool registry + all 13 built-in tools
- [x] Phase 5 — Git worktree manager
- [ ] Phase 6 — RAG engine (chunker + manifest + store + orchestrator)
- [ ] Phase 7 — Memory system
- [ ] Phase 8 — Agent (transcript + prompt + confirmer + runner + output)
- [ ] Phase 9 — CLI (root + wire + run + file + repl)
- [ ] Phase 10 — End-to-end smoke test
- [ ] Phase 11 — Example config + spec layout verified
- [ ] `go build ./...` exits 0
- [ ] `go test ./...` passes
- [ ] `go vet ./...` exits 0
