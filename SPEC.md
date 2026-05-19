# xdreamer — Specification

**Version:** 0.2.0  
**Date:** 2026-05-17  
**Status:** Approved

---

## 1. Overview

**xdreamer** is a lightweight, autonomous coding agent written in Go. It is a simpler alternative to heavy AI coding agents, prioritizing clarity, local-first operation, and embeddability. It connects exclusively to locally-hosted language models via LM Studio.

The agent accepts a coding task, executes it step by step using a defined tool set, and produces code changes, logs, and a summary report — all isolated in a git worktree so the user's working branch is never touched.

---

## 2. Goals

- **Simple by design.** The codebase must remain small, readable, and easy to modify by anyone.
- **Local-model first.** Connects to LM Studio; no internet or cloud account required to run.
- **Safe by default.** Destructive actions (file writes, deletes, shell execution, git commits) require user confirmation unless explicitly bypassed.
- **Reproducible.** All work happens in a dedicated git worktree branch; the main branch is never modified.
- **Language-agnostic.** Works on any codebase regardless of programming language.
- **SOLID and clean.** Internals follow SOLID principles and clean code conventions so the project stays maintainable as it grows.

---

## 3. Non-Goals

- No web UI or browser dashboard.
- No multi-agent orchestration or parallel subtask execution.
- No cloud model providers (OpenAI, Anthropic, etc.) in v1.
- No plugin system for third-party tools in v1.
- No Windows PowerShell-native shell tool (shell commands use `cmd /C`; portability is the user's responsibility).

---

## 4. Design Principles

### 4.1 SOLID

| Principle | How it applies to xdreamer |
|---|---|
| **Single Responsibility** | Each package owns exactly one concern: `config` loads config, `model` talks to LM Studio, `tools` executes actions, `rag` retrieves context, `agent` orchestrates the loop. No package does two of these jobs. |
| **Open/Closed** | Core interfaces (`model.Provider`, `tools.Tool`, `agent.Confirmer`) are closed for modification but open for extension — new providers, tools, and confirmers can be added without touching existing code. |
| **Liskov Substitution** | Every implementation of an interface must be fully substitutable. `AutoConfirmer` and `TerminalConfirmer` both satisfy `Confirmer` identically. Mock implementations used in tests must behave the same way real ones do. |
| **Interface Segregation** | Interfaces are narrow. `model.Provider` has two methods. `tools.Tool` has five. `agent.Confirmer` has one. No interface forces an implementor to provide methods it does not need. |
| **Dependency Inversion** | High-level modules (`agent.Runner`) depend on abstractions (`model.Provider`, `tools.Tool`), not concrete types (`*LMStudio`). Concrete types are wired together only at the CLI entrypoint. |

### 4.2 Clean Code

- **Naming:** Names are self-explanatory. No abbreviations except idiomatic Go (`ctx`, `err`, `cfg`, `w`). Package names are singular nouns (`tool`, not `tools_pkg`).
- **Functions:** Each function does one thing and fits on a screen. Functions with more than four parameters take a struct.
- **Error handling:** All errors are wrapped with context: `fmt.Errorf("read file: %w", err)`. Errors are never swallowed silently. Sentinel errors are declared as package-level `var Err… = errors.New(…)`.
- **No magic values:** All constants are named. No bare string literals or raw numbers in logic code.
- **No comments on obvious code.** Comments explain *why*, not *what*. A well-named function needs no docstring.
- **Tests are first-class.** Every exported type has a `_test.go` file. Tests use `t.TempDir()` for file isolation. Table-driven tests are preferred.
- **No dead code.** If something is not used, it is deleted.

---

## 5. Architecture

```
┌──────────────────────────────────────────────────────┐
│                      CLI Layer                        │
│   xdreamer "task"  |  xdreamer -f task.md  |  REPL   │
└─────────────────────────┬────────────────────────────┘
                          │ Task string
                          ▼
┌─────────────────────────────────────────────────────┐
│                   agent.Runner                       │
│  - Reads task input                                  │
│  - Manages the step loop                             │
│  - Writes transcript + summary on exit               │
└───────────┬────────────────────────┬────────────────┘
            │ model.Provider calls   │ tools.Tool calls
            ▼                        ▼
┌──────────────────────┐  ┌───────────────────────────┐
│   model.Provider     │  │     tools.Registry         │
│  (LM Studio client)  │  │  file | shell | search    │
└──────────────────────┘  │  git  | web   | codegen   │
                          └───────────────────────────┘
            │ Retrieval
            ▼
┌─────────────────────────────────────────────────────┐
│                   rag.Engine                         │
│  - Embeds codebase chunks into local vector store    │
│  - Retrieves relevant chunks per step                │
└─────────────────────────────────────────────────────┘
```

**Data flow per step:**
1. RAG retrieves relevant chunks for the current task state.
2. `Runner` builds a prompt from: system prompt + memory + RAG chunks + message history.
3. `model.Provider` sends the prompt to LM Studio and returns a response (tool call or final answer).
4. If the response is a tool call: `Runner` checks destructiveness, confirms with user if needed, executes the tool, records the result, and loops.
5. If the response signals completion: `Runner` writes outputs and exits.

---

## 6. Interfaces

### 6.1 CLI Modes

| Mode | Syntax | Description |
|---|---|---|
| One-shot | `xdreamer "fix the login bug"` | Single task argument, runs to completion, then exits |
| Task file | `xdreamer -f task.md` | Reads task text from a file, runs to completion |
| REPL | `xdreamer` (no args) | Interactive session; each input line is a new task |

### 6.2 Global Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--model` | string | from config | LM Studio model identifier |
| `--dir` | string | `.` | Project root directory to operate on |
| `--auto` | bool | `false` | Skip confirmation for destructive actions |
| `--no-memory` | bool | `false` | Disable project memory for this run |
| `--worktree` | string | `xdreamer/<slug>` | Override the git branch/worktree name |
| `--log` | string | `./xdreamer-<slug>.log` | Override the transcript log output path |
| `--dry-run` | bool | `false` | Plan and log steps but do not execute any tool |

### 6.3 REPL Commands

When in REPL mode, lines starting with `/` are commands, not tasks:

| Command | Description |
|---|---|
| `/status` | Print current worktree branch and step count |
| `/memory` | Print the current project memory content |
| `/undo` | Discard the last commit in the current worktree |
| `/exit` | Exit the REPL cleanly |

---

## 7. Agent Behavior

### 7.1 Task Lifecycle

```
1.  Parse task input (arg / file / REPL line)
2.  Load config (TOML layers + env vars + flags)
3.  Load project memory (if enabled)
4.  Create git worktree at branch xdreamer/<slug>
5.  Index codebase via RAG engine (incremental on re-runs)
6.  Build system prompt from template (§7.4)
7.  LOOP (max steps enforced):
    a. Retrieve relevant chunks via RAG for current query
    b. Append RAG context to message history
    c. Call model.Provider.Chat() with history + tool definitions
    d. Append assistant response to history
    e. If response signals DONE → break loop
    f. If no tool calls → append "continue" user message, loop
    g. For each tool call:
       i.  Look up tool in registry; if unknown → record error, continue
       ii. If --dry-run → log action, skip execution, continue
       iii.If tool is destructive → call Confirmer.Confirm(); if denied → record denial, continue
       iv. Execute tool, capture output
       v.  Append tool result to message history
       vi. Record step in Transcript
8.  Commit all changes in worktree (staged automatically)
9.  Generate and write SUMMARY.md to worktree root
10. Write transcript log to <log-dir>/xdreamer-<slug>.log
11. Write patch file to <log-dir>/xdreamer-<slug>.patch
12. Print diff to stdout
13. Prompt user: [M]erge / [K]eep branch / [D]iscard
```

### 7.2 Autonomy Model

| Category | Examples | Default behavior |
|---|---|---|
| Safe | Read file, list dir, search, git status/log/diff, web fetch/search | Execute immediately, no prompt |
| Destructive | Write file, delete file, run shell, git commit | Prompt user; execute only on confirmation |

With `--auto`, all actions execute without confirmation.  
With `--dry-run`, no tool is ever executed regardless of `--auto`.

### 7.3 Step Planning

The agent uses a **linear, reactive planning model**:

- No upfront plan is shown to the user.
- The LLM selects one tool per turn based on the result of the previous tool.
- The result of each tool call is appended to the message history before the next call.
- The LLM signals task completion by ending its response with the word `DONE`.
- A hard step limit (`agent.max_steps`, default 50) prevents infinite loops.

### 7.4 System Prompt Template

```
You are xdreamer, an autonomous coding agent. Work step by step using the tools provided.
After each tool result, select the next action. When the task is complete, write a short
summary of what you did and end your message with the single word DONE on its own line.

Rules:
- Call exactly one tool per turn.
- Never guess file contents — always use read_file before modifying a file.
- Always use write_file to persist changes — never output code in prose.
- Run shell commands in the worktree directory: {{.WorktreePath}}
- Project memory:
{{.MemoryContent}}
```

---

## 8. Tool Registry

### 8.1 Tool Interface

```go
type Tool interface {
    Name()        string
    Description() string
    Parameters()  map[string]any   // JSON Schema "properties" object
    Execute(ctx context.Context, params json.RawMessage) (string, error)
    Destructive() bool
}
```

Tools are registered in a `Registry`. The `Runner` never references concrete tool types — it always goes through the registry.

### 8.2 Built-in Tools

| Tool | Destructive | Description |
|---|---|---|
| `read_file` | No | Read and return full file contents |
| `write_file` | **Yes** | Create or overwrite a file (creates parent dirs) |
| `delete_file` | **Yes** | Delete a file from the filesystem |
| `list_dir` | No | List directory entries (optionally recursive) |
| `search_code` | No | Regex search across files matching a glob pattern |
| `run_shell` | **Yes** | Execute a shell command in a given working directory |
| `git_status` | No | Run `git status --short` |
| `git_diff` | No | Run `git diff` or `git diff --staged` |
| `git_log` | No | Run `git log --oneline -N` |
| `git_commit` | **Yes** | Stage all changes and create a commit |
| `web_fetch` | No | GET a URL and return text content (strips HTML) |
| `web_search` | No | Search DuckDuckGo and return top results |
| `generate_code` | No | Ask the model for a code snippet (single sub-call, no tool loop) |

---

## 9. Model Provider

### 9.1 Interface

```go
type Provider interface {
    Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (Response, error)
    Embed(ctx context.Context, text string) ([]float32, error)
}
```

The interface is the only thing the `agent` package knows about. The concrete LM Studio implementation is wired in the CLI layer.

### 9.2 LM Studio Integration

xdreamer connects to LM Studio's local server, which exposes an OpenAI-compatible REST API.

**Default configuration:**

```toml
[model]
provider        = "lmstudio"
base_url        = "http://localhost:1234/v1"
model           = "qwen2.5-coder-7b"
embed_model     = "nomic-embed-text-v1.5"
context_window  = 8192
temperature     = 0.2
max_retries     = 3
retry_delay_ms  = 1000
```

**Changing the server address:**
- LM Studio: open the **Local Server** tab and set host/port.
- xdreamer config: set `base_url` to match, e.g., `http://192.168.1.10:1234/v1`.
- Or via env var: `XDREAMER_MODEL_BASE_URL=http://localhost:1234/v1`.

**Model identifiers:** LM Studio uses the loaded model's display name (e.g., `"qwen2.5-coder-7b"`). The exact string must match what LM Studio shows in its server tab. There is no fixed registry — set `model` in config to match what is loaded.

### 9.3 Tool Calling

xdreamer uses LM Studio's OpenAI-compatible function-calling API. For models that do not support function calling, the provider falls back to a JSON-in-prompt strategy: tool definitions are injected as a JSON block in the system message, and the model's response is parsed for a `{"tool": "...", "params": {...}}` JSON object.

### 9.4 Retry Policy

On HTTP 429 (rate limited) or 5xx errors, the provider retries up to `max_retries` times with `retry_delay_ms` milliseconds between attempts. Other errors are returned immediately.

---

## 10. RAG Engine

### 10.1 Purpose

Local models have limited context windows (typically 4k–32k tokens). The RAG engine indexes the project codebase and retrieves only the chunks most relevant to the current step, keeping each prompt within the model's limit.

### 10.2 Behavior

| Phase | Description |
|---|---|
| **Index** | On first run, all text files are chunked and embedded. Chunk boundaries are line-aligned; token count per chunk is bounded by `chunk_size`. |
| **Incremental** | On subsequent runs, only files whose SHA-256 hash has changed are re-embedded. The manifest is stored at `<index_dir>/manifest.json`. |
| **Retrieve** | Before each LLM call, the top-K most similar chunks are retrieved using cosine similarity against an embedding of the current query (last user message + task). |
| **Storage** | Embeddings are stored in a local persistent vector store (`chromem-go`) at `<index_dir>/vectors.db`. |

### 10.3 Configuration

```toml
[rag]
enabled       = true
chunk_size    = 512
chunk_overlap = 64
top_k         = 10
index_dir     = ".xdreamer/index"
ignore        = ["vendor/", "node_modules/", ".git/", "*.pb.go", "*.min.js"]
```

### 10.4 Design Decision — Vector Store

**chromem-go** is used as the vector store. Rationale: pure Go (no CGo, no external binary), in-process, supports persistence, MIT license.

---

## 11. Memory System

Memory is **disabled globally** and **opt-in per project**.

### 11.1 Modes

| Mode | Behavior |
|---|---|
| Disabled | Each run starts with no prior context. Nothing is read or written. |
| Enabled | `.xdreamer/MEMORY.md` is read at session start and injected into the system prompt. At session end, the agent appends a brief summary of key decisions made during the run. |

### 11.2 Memory File Format

```markdown
# xdreamer Project Memory

## Architecture decisions
- Auth middleware lives in internal/auth/

## Conventions
- Errors are wrapped: fmt.Errorf("context: %w", err)

## Known issues
- Tests in pkg/db/ require a live Postgres instance
```

### 11.3 Configuration

```toml
[memory]
enabled = false
file    = ".xdreamer/MEMORY.md"
```

---

## 12. Git Worktree Isolation

All agent work happens in a dedicated git worktree. The user's current branch is never modified.

| Property | Value |
|---|---|
| Branch name | `xdreamer/<task-slug>` |
| Worktree path | `<repo-root>/.xdreamer/worktrees/<task-slug>/` |
| Slug derivation | Lowercase task text; spaces and special chars replaced with `-`; truncated to 40 chars |
| On completion | User is prompted: **[M]erge** into current branch / **[K]eep** branch for later / **[D]iscard** branch and worktree |

---

## 13. Output Artifacts

| Artifact | Location | Description |
|---|---|---|
| Modified files | worktree branch | All code changes made by the agent |
| Transcript log | `<log_dir>/xdreamer-<slug>.log` | Full step-by-step record of all prompts, responses, and tool results |
| Summary report | `<worktree-path>/SUMMARY.md` | Human-readable markdown summary written by the model |
| Patch file | `<log_dir>/xdreamer-<slug>.patch` | Unified diff of all changes (output of `git diff HEAD`) |

---

## 14. Configuration

Configuration is layered; later layers override earlier ones:

| Priority | Source |
|---|---|
| 1 (lowest) | Compiled defaults |
| 2 | `~/.xdreamer/config.toml` (global user config) |
| 3 | `<project>/.xdreamer.toml` (per-project config) |
| 4 | `XDREAMER_*` environment variables |
| 5 (highest) | CLI flags |

### 14.1 Full Config Schema

```toml
[model]
provider        = "lmstudio"
base_url        = "http://localhost:1234/v1"
model           = "qwen2.5-coder-7b"
embed_model     = "nomic-embed-text-v1.5"
context_window  = 8192
temperature     = 0.2
max_retries     = 3
retry_delay_ms  = 1000

[agent]
auto            = false
max_steps       = 50
worktree_base   = ".xdreamer/worktrees"

[rag]
enabled         = true
chunk_size      = 512
chunk_overlap   = 64
top_k           = 10
index_dir       = ".xdreamer/index"
ignore          = ["vendor/", "node_modules/", ".git/"]

[memory]
enabled         = false
file            = ".xdreamer/MEMORY.md"

[output]
log_dir         = "."
```

### 14.2 Environment Variable Mapping

| Variable | Config field |
|---|---|
| `XDREAMER_MODEL_BASE_URL` | `model.base_url` |
| `XDREAMER_MODEL` | `model.model` |
| `XDREAMER_EMBED_MODEL` | `model.embed_model` |
| `XDREAMER_AUTO` | `agent.auto` (`"true"` / `"1"`) |
| `XDREAMER_MAX_STEPS` | `agent.max_steps` (integer) |
| `XDREAMER_LOG_DIR` | `output.log_dir` |

---

## 15. Directory Layout (source)

```
xdreamer/
├── cmd/
│   └── xdreamer/
│       ├── main.go              # entrypoint — calls cmd.Execute()
│       └── cmd/
│           ├── root.go          # cobra root command, persistent flags, dispatch
│           ├── wire.go          # session, newSession, runTask, merge UX
│           ├── run.go           # one-shot mode
│           ├── file.go          # task-file mode (-f flag)
│           └── repl.go          # REPL mode + /commands
├── internal/
│   ├── agent/
│   │   ├── confirm.go           # Confirmer interface + TerminalConfirmer + AutoConfirmer
│   │   ├── output.go            # WritePatch, GenerateSummary
│   │   ├── prompt.go            # system prompt template rendering
│   │   ├── runner.go            # step loop, Retriever interface, ErrMaxStepsExceeded
│   │   └── transcript.go        # Transcript, Step, ToolResult, WriteLog, SaveToFile
│   ├── config/
│   │   ├── config.go            # Config structs + Defaults() + Dir field
│   │   ├── loader.go            # TOML merge + env var override
│   │   └── validator.go         # Validate() — sentinel errors
│   ├── memory/
│   │   └── memory.go            # Load / Content / Append / Save
│   ├── model/
│   │   ├── provider.go          # Provider interface + Message/Response types
│   │   └── lmstudio.go          # LM Studio OpenAI-compatible client + retry
│   ├── rag/
│   │   ├── chunker.go           # text → []Chunk (tiktoken token counting)
│   │   ├── manifest.go          # incremental index manifest (path → hash)
│   │   ├── store.go             # chromem-go vector store + EmbedFunc
│   │   └── engine.go            # Index + Retrieve orchestration
│   ├── tools/
│   │   ├── tool.go              # Tool interface + Registry + Definitions()
│   │   ├── files.go             # read_file, write_file, delete_file
│   │   ├── dir.go               # list_dir (flat + recursive, skips .git)
│   │   ├── search.go            # search_code (regex + glob, **/ support)
│   │   ├── shell.go             # run_shell (cross-platform)
│   │   ├── git.go               # git_status, git_diff, git_log, git_commit
│   │   ├── web.go               # web_fetch, web_search (DuckDuckGo)
│   │   └── codegen.go           # generate_code (injected model.Provider)
│   └── worktree/
│       └── worktree.go          # Slug, Create, Remove, ErrNotGitRepo
├── test/
│   └── e2e/
│       └── smoke_test.go        # end-to-end test with mock LM Studio server
├── .xdreamer.toml               # example project config with comments
├── go.mod
├── go.sum
├── SPEC.md
└── IMPLEMENTATION.md
```

---

## 16. Resolved Design Decisions

These were previously open questions. All are now resolved:

| Question | Decision |
|---|---|
| Vector store library | **chromem-go** — pure Go, no CGo, persistent, MIT license |
| `generate_code` placement | **Tool** — keeps the agent loop uniform; the tool calls the model with a single non-tool prompt internally |
| Merge UX at end of session | **Interactive prompt in the same CLI session** — `[M]erge / [K]eep / [D]iscard` — no separate subcommand |
| REPL `/commands` | **Yes** — `/status`, `/memory`, `/undo`, `/exit` |
| Retry policy for LM Studio | **Exponential-ish: up to `max_retries` attempts, `retry_delay_ms` between each, only on 429 and 5xx** |
| LM Studio model ID | **User-configured** — set `model` to the exact name shown in LM Studio's server tab |

---

## 17. Out of Scope (v1)

- Multi-agent / parallel subtask execution
- Cloud model providers (OpenAI, Anthropic, etc.)
- Web UI or TUI
- Plugin system for third-party tools
- Windows PowerShell-native shell tool
