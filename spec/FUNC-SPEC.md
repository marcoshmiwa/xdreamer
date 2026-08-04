# Functional Specification — AgentCoder

## 1. Overview

AgentCoder is a command-line tool that automates the implementation of a
described coding task inside an existing local git repository, using a
locally-hosted LLM (served via LM Studio's OpenAI-compatible API) to read and
edit source files, verify the result builds, and commit + push the change.

**In scope (first moment / v1):**
- Reading a task description from a markdown/text file.
- An agentic loop where the LLM uses tools (list files, read file, write
  file) to inspect and modify the repository.
- Building the resulting .NET project/solution and retrying (feeding build
  errors back to the LLM) until it compiles or a retry budget is exhausted.
- Committing the change with a Conventional Commits message and pushing it
  to the current branch's upstream remote.

**Explicitly out of scope for this spec** (see Section 4): branch/PR
creation, pull/merge, running tests, non-.NET projects, and the "harness"
for retrieving best-practices/tooling information (a deferred second-phase
feature to be specified separately).

## 2. Users and Context

A single developer, working locally in a .NET git repository, who wants to
delegate a well-described, self-contained coding task to the agent instead
of writing the code by hand. The tool is run manually from a terminal, once
per task, inside the target repository (or pointed at one).

## 3. Expected Behaviors

### B-001: CLI Invocation & Configuration
- **Description:** The tool is invoked as `agentcoder run <task-file-path>`.
  On startup it loads LM Studio connection settings (base URL, model name,
  timeout, retry counts) from `appsettings.json`, with CLI flags
  (`--endpoint`, `--model`) able to override individual values.
- **Inputs:** Positional task file path (required); optional `--endpoint`,
  `--model`, `--config` flags.
- **Outputs:** Valid invocation proceeds to preflight validation (B-002).
  Missing task file argument prints usage text and exits with code 1.
- **Edge cases:** Missing `appsettings.json` → fall back to built-in
  defaults (`http://localhost:1234/v1`) and log a warning. Malformed config
  file → exit 1 with a parse error message. Both config file and CLI flag
  set the same value → CLI flag wins.
- **Validation:** `agentcoder run` with no arguments prints usage and exits
  1. `agentcoder run ./task.md --endpoint http://localhost:9999/v1` shows
  (via `--verbose`) the resolved endpoint as `http://localhost:9999/v1`,
  overriding any config file value.

### B-002: Preflight Validation
- **Description:** Before any LLM or git call, the agent verifies: (1) the
  task file exists, is readable, and non-empty; (2) the current working
  directory is inside a git repository; (3) `git status --porcelain` reports
  a clean working tree. Any failing check aborts the run before the LLM is
  contacted and before any git command runs.
- **Inputs:** Task file path, current working directory.
- **Outputs:** All checks pass → proceed to B-003. Any check fails → print
  every failed check (not just the first), exit non-zero, no side effects.
- **Edge cases:** Task file exists but is empty → error "task file is
  empty". CWD not a git repo → error "not a git repository". Dirty working
  tree (modified or untracked files) → error "working tree is not clean;
  commit or discard changes before running agentcoder".
- **Validation:** Running the tool with an uncommitted change present
  returns a non-zero exit code, prints a message containing "working tree
  is not clean", and `git log` shows no new commit afterward.

### B-003: Task Loading
- **Description:** The agent reads the full UTF-8 text content of the task
  file and uses it verbatim as the task description sent to the LLM.
- **Inputs:** Validated task file path.
- **Outputs:** In-memory task description string passed into the agentic
  loop (B-004).
- **Edge cases:** Non-UTF-8 file → clear decode-error message, non-zero
  exit. File exceeding a configurable max size (default 1 MB) → error "task
  file too large".
- **Validation:** With `--dry-run`, the tool prints the loaded text back to
  the console, byte-for-byte identical to the source file's content.

### B-004: Agentic Code-Editing Loop
- **Description:** The task description is sent to the LLM via LM Studio's
  chat-completions API with tool/function calling enabled, exposing three
  tools: `list_files` (glob-filtered file listing), `read_file` (returns
  file content by relative path), and `write_file` (creates or overwrites a
  file by relative path). The LLM issues tool calls in a loop, deciding
  autonomously what to inspect and change, until it returns a final
  response with no further tool calls.
- **Inputs:** Task description, repository root path.
- **Outputs:** Zero or more files written to disk within the repo. A full
  tool-call transcript is available via `--verbose`.
- **Edge cases:** A tool call requesting a path outside the repo root (path
  traversal) is rejected; an error result (not file content) is returned to
  the LLM and the attempt is logged. The loop exceeding a configurable
  max iteration count (default 50 tool calls) aborts the run with "agent
  exceeded maximum tool-call iterations". A loop that makes no file changes
  at all is treated as a valid no-op and proceeds to the build step.
- **Validation:** Given a task "add a method Foo() to Bar.cs that returns
  42", after the loop completes `Bar.cs` on disk contains a `Foo` method
  returning `42`.

### B-005: Build Verification with Retry
- **Description:** After the agentic loop (or after each fix retry), the
  agent auto-detects the buildable target — a single `.sln` in the repo
  root if present, otherwise a single `.csproj`; if multiple candidates
  exist it uses the first found and logs which one — and runs
  `dotnet build`. On failure, the build's error output is fed back into the
  same agentic loop as a new message (tools still available) asking the LLM
  to fix the errors; the edit-then-build cycle repeats up to 10 total
  attempts.
- **Inputs:** Repository root, `dotnet build` output.
- **Outputs:** Exit code 0 from `dotnet build` → proceed to B-006. Still
  failing after 10 attempts → abort, no commit, no push; print the last
  build error output.
- **Edge cases:** No `.sln`/`.csproj` found → abort immediately, no retries.
  `dotnet` not found on PATH → abort with "dotnet not found". Build
  succeeds with warnings only → treated as success.
- **Validation:** After exactly 10 failed attempts the process exits
  non-zero and no new git commit exists. When a fix succeeds within budget,
  the final `dotnet build` exit code is 0.

### B-006: Git Commit
- **Description:** Once the build succeeds and there are staged changes,
  the agent runs `git add -A` then `git commit` with a Conventional
  Commits-formatted message. The message is produced by a final LLM call
  summarizing the change; if the result doesn't match the Conventional
  Commits pattern, the agent falls back to
  `chore: apply agentcoder changes`.
- **Inputs:** Working tree changes, task description, LLM summary response.
- **Outputs:** A new commit on the current branch. If the build succeeded
  but no files were changed, commit and push are skipped and the tool
  reports "no changes to commit".
- **Edge cases:** LLM-generated message fails format validation → fallback
  message used. `git commit` fails for a git-level reason (e.g. no
  `user.name`/`user.email` configured) → abort with the raw git error, no
  push attempted.
- **Validation:** `git log -1 --pretty=%s` on the current branch matches
  `^(feat|fix|chore|refactor|docs|test|style|perf)(\(.+\))?: .+`.

### B-007: Git Push
- **Description:** Immediately after a successful commit, the agent runs
  `git push` against the current branch's configured upstream. On failure
  (auth, network, non-fast-forward rejection), it retries up to a
  configurable count (default 3) with backoff, then aborts — the commit
  remains local and is never rolled back.
- **Inputs:** Current branch, remote state.
- **Outputs:** Success → remote branch matches local HEAD. Failure after
  retries → non-zero exit with the last git error; commit stays local.
- **Edge cases:** No upstream configured for the current branch → abort
  immediately with "no upstream branch set; run 'git push -u origin
  <branch>' manually first" (no guessing of remote/branch). Non-fast-forward
  rejection → no automatic merge/rebase attempted; abort and report.
- **Validation:** On success, `git log origin/<branch> -1` matches the
  local HEAD commit hash. An unreachable remote produces exactly the
  configured retry count of attempts (visible via `--verbose`) before
  aborting with the commit still present locally.

### B-008: LLM Connectivity Handling
- **Description:** Every call to the LM Studio endpoint goes through a
  retry policy: on connection failure or timeout, retry up to a
  configurable count (default 3) with backoff before failing the run.
- **Inputs:** HTTP requests to the configured LM Studio base URL.
- **Outputs:** Persistent failure → non-zero exit with "could not reach LM
  Studio at <url>". If failure occurs before any commit, no git side
  effects exist.
- **Edge cases:** A reachable endpoint returning a malformed/non-JSON
  response is treated as a failure under the same retry policy. A timeout
  mid-loop, after some files were already written, aborts the run but
  leaves those file writes on disk (uncommitted) with an explicit warning
  that manual review is needed.
- **Validation:** Pointing `--endpoint` at an unreachable address results
  in exactly the configured number of connection attempts (visible in
  logs), followed by a non-zero exit and the specified error message.

### B-009: Config-Driven LM Studio Connection
- **Description:** LM Studio base URL, model name, request timeout, and
  retry counts live in `appsettings.json` (standard .NET configuration
  binding), individually overridable via CLI flags.
- **Inputs:** `appsettings.json`, CLI flags.
- **Outputs:** A resolved configuration object consumed by the LLM client.
- **Edge cases:** `appsettings.json` absent → built-in defaults apply
  (`http://localhost:1234/v1`; no default model — one must be supplied via
  config or `--model`, otherwise B-001's validation fails).
- **Validation:** Same as B-001's validation case — flag overrides config.

## 4. Out of Scope

- Branch creation, pull requests, merging, or rebasing.
- Automatic `git pull` or merge-conflict resolution.
- Running the project's test suite (`dotnet test`) — only `dotnet build` is
  performed.
- Any non-.NET project types.
- The future "harness" for retrieving best-practices and tooling
  information — a deferred second-phase feature, to be specified
  separately once this first version is implemented.
- Multi-repo or remote (non-local) repository operation.
- Interactive/conversational task refinement — task input is a single
  static file per run.
- Automatic stashing/restoring of pre-existing uncommitted changes (the
  tool refuses to run instead — see B-002).

## 5. Acceptance Criteria Summary

| ID | Behavior | Validation |
|----|----------|------------|
| B-001 | CLI invocation & configuration | Usage error on missing arg; flag overrides config, shown via `--verbose` |
| B-002 | Preflight validation | Dirty tree → non-zero exit, no commit created |
| B-003 | Task loading | `--dry-run` echoes file content byte-for-byte |
| B-004 | Agentic code-editing loop | Target file contains the requested change after the loop |
| B-005 | Build verification with retry | Success → exit 0 build; 10 failed attempts → abort, no commit |
| B-006 | Git commit | `git log -1 --pretty=%s` matches Conventional Commits pattern |
| B-007 | Git push | `origin/<branch>` HEAD matches local HEAD; retries visible in `--verbose` |
| B-008 | LLM connectivity handling | Unreachable endpoint → configured retry count, then clear error |
| B-009 | Config-driven LM Studio connection | CLI flag overrides `appsettings.json` value |
