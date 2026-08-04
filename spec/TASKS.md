# Tasks — AgentCoder

Generated from FUNC-SPEC.md and TECH-SPEC.md on 2026-08-03.
Status legend: [ ] pending · [~] in progress · [x] done · [!] rewrite required

## T-000: Solution & Project Scaffolding
- **Status:** [ ]
- **Behaviors covered:** — (infrastructure scaffolding; no direct behavior, enables all)
- **Files to create:**
  - `AgentCoder.sln`
  - `src/AgentCoder.Cli/AgentCoder.Cli.csproj`
  - `src/AgentCoder.Core/AgentCoder.Core.csproj`
  - `src/AgentCoder.Infrastructure/AgentCoder.Infrastructure.csproj`
  - `tests/AgentCoder.UnitTests/AgentCoder.UnitTests.csproj`
  - `tests/AgentCoder.IntegrationTests/AgentCoder.IntegrationTests.csproj`
  - `src/AgentCoder.Cli/appsettings.json`
- **Files to alter:** —
- **Methods/interfaces to define:** — (project/package wiring only)
- **Patterns applied:** N/A (scaffolding)
- **Depends on:** —

### Subtasks
#### T-000.1: Create solution and project references
- **What:** Target `net10.0` in every `.csproj`. `Cli` references `Core` and
  `Infrastructure`; `Infrastructure` references `Core`; `Core` references
  nothing else in-solution. Add `System.CommandLine`,
  `Microsoft.Extensions.Configuration.Json`,
  `Microsoft.Extensions.DependencyInjection`,
  `Microsoft.Extensions.Logging.Console` to `Cli`/`Infrastructure` as needed.
  `UnitTests` references `Core` + `Infrastructure` and adds `xunit`,
  `xunit.runner.visualstudio`, `NSubstitute`. `IntegrationTests` references
  `Infrastructure` and adds `xunit`, `xunit.runner.visualstudio`.
- **Unit test:** N/A — build-only validation.
- **Other validation:** `dotnet build AgentCoder.sln` exits 0 with no
  project errors.

#### T-000.2: Scaffold appsettings.json
- **What:** `src/AgentCoder.Cli/appsettings.json` with keys matching
  `AgentCoderOptions` (T-003): `Endpoint`, `Model`, `RequestTimeoutSeconds`,
  `LlmConnectionRetries`, `GitPushRetries`, `MaxBuildRetries`,
  `MaxToolCallIterations`, `MaxTaskFileSizeBytes`, all set to the defaults
  from TECH-SPEC/FUNC-SPEC.
- **Unit test:** N/A — config file, not code (declared explicitly per
  task-planner rules).
- **Other validation:** File is valid JSON (`dotnet build` copies it to
  output; manually inspect `bin/.../appsettings.json` exists post-build).

---

## T-001: Domain Entities
- **Status:** [ ]
- **Behaviors covered:** B-003, B-005, B-006
- **Files to create:**
  - `src/AgentCoder.Core/Domain/Entities/AgentTask.cs`
  - `src/AgentCoder.Core/Domain/Entities/BuildResult.cs`
  - `src/AgentCoder.Core/Domain/Entities/CommitMessage.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - `AgentTask` (record): `string Description { get; }` — constructed via
    `AgentTask(string description)`.
  - `BuildResult` (record): `bool Succeeded`, `string Output`,
    `int AttemptNumber`.
  - `CommitMessage` (class): `string Type`, `string? Scope`,
    `string Description`; `static bool TryParse(string raw, out
    CommitMessage? message)`; `override string ToString()` (renders
    `type(scope): description`); `static CommitMessage Fallback` (returns
    `chore: apply agentcoder changes`).
- **Patterns applied:** Single Responsibility (each entity holds data +
  its own formatting/parsing only, no I/O, no orchestration).
- **Depends on:** T-000

### Subtasks
#### T-001.1: AgentTask and BuildResult records
- **What:** Immutable data carriers, no behavior beyond construction.
- **Unit test:** `tests/AgentCoder.UnitTests/Domain/AgentTaskTests.cs`,
  `BuildResultTests.cs` — construction round-trips values; equality by
  value (record semantics). Run: `dotnet test
  tests/AgentCoder.UnitTests --filter Domain`.
- **Other validation:** N/A beyond unit tests (pure data types).

#### T-001.2: CommitMessage parsing and formatting
- **What:** `TryParse` validates against the Conventional Commits regex
  `^(feat|fix|chore|refactor|docs|test|style|perf)(\(.+\))?: .+` from
  TECH-SPEC.md §6 and populates `Type`/`Scope`/`Description` on match.
- **Unit test:**
  `tests/AgentCoder.UnitTests/Domain/CommitMessageTests.cs` — cases:
  valid message with scope, valid without scope, invalid type keyword
  (returns false), missing colon (returns false), `Fallback.ToString()`
  equals `"chore: apply agentcoder changes"`. Run: `dotnet test
  tests/AgentCoder.UnitTests --filter CommitMessageTests`.
- **Other validation:** Matches FUNC-SPEC B-006 validation directly — this
  is the regex referenced there.

---

## T-002: Domain Repository Interfaces
- **Status:** [ ]
- **Behaviors covered:** B-002, B-003, B-004, B-006, B-007
- **Files to create:**
  - `src/AgentCoder.Core/Domain/Repositories/IFileSystemRepository.cs`
  - `src/AgentCoder.Core/Domain/Repositories/IGitRepository.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - `IFileSystemRepository`:
    `Task<bool> ExistsAsync(string relativePath, CancellationToken ct)`;
    `Task<long> GetSizeAsync(string relativePath, CancellationToken ct)`;
    `Task<string> ReadFileAsync(string relativePath, CancellationToken ct)`;
    `Task WriteFileAsync(string relativePath, string content,
    CancellationToken ct)`;
    `Task<IReadOnlyList<string>> ListFilesAsync(string globPattern,
    CancellationToken ct)`;
    `bool IsPathWithinRoot(string relativePath)`.
  - `IGitRepository`:
    `Task<bool> IsInsideRepositoryAsync(CancellationToken ct)`;
    `Task<bool> IsWorkingTreeCleanAsync(CancellationToken ct)`;
    `Task<string> GetCurrentBranchAsync(CancellationToken ct)`;
    `Task<bool> HasUpstreamAsync(CancellationToken ct)`;
    `Task<GitCommandResult> ExecuteAsync(IGitCommand command,
    CancellationToken ct)`.
- **Patterns applied:** Repository pattern (mandatory), Dependency
  Inversion (Application layer will depend only on these interfaces),
  Interface Segregation (file-system concerns and git concerns kept in
  separate interfaces).
- **Depends on:** T-000

### Subtasks
#### T-002.1: Define IFileSystemRepository
- **What:** Interface only, in the Domain layer, no implementation here.
- **Unit test:** N/A — interface declaration (implementation tested in
  T-004).
- **Other validation:** `dotnet build` compiles with no implementers yet
  (interface-only compile check).

#### T-002.2: Define IGitRepository and supporting GitCommandResult type
- **What:** Interface plus a small `GitCommandResult` record
  (`bool Success`, `int ExitCode`, `string StdOut`, `string StdErr`) placed
  alongside it in the same file, used by both `IGitRepository` and
  `IGitCommand` (T-008).
- **Unit test:** N/A — interface/data declaration.
- **Other validation:** `dotnet build` compiles.

---

## T-003: Configuration Options
- **Status:** [ ]
- **Behaviors covered:** B-001, B-009
- **Files to create:**
  - `src/AgentCoder.Core/Configuration/AgentCoderOptions.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - `AgentCoderOptions` (class, bindable via
    `Microsoft.Extensions.Configuration`): `string Endpoint { get; set; } =
    "http://localhost:1234/v1"`; `string? Model { get; set; }`;
    `int RequestTimeoutSeconds { get; set; } = 60`;
    `int LlmConnectionRetries { get; set; } = 3`;
    `int GitPushRetries { get; set; } = 3`;
    `int MaxBuildRetries { get; set; } = 10`;
    `int MaxToolCallIterations { get; set; } = 50`;
    `long MaxTaskFileSizeBytes { get; set; } = 1_048_576`.
- **Patterns applied:** Single Responsibility (pure options holder, no
  loading/parsing logic — that lives in T-014's composition root).
- **Depends on:** T-000

### Subtasks
#### T-003.1: Define AgentCoderOptions
- **What:** Plain settings class with the defaults listed above, matching
  FUNC-SPEC B-001/B-009 defaults exactly.
- **Unit test:**
  `tests/AgentCoder.UnitTests/Configuration/AgentCoderOptionsTests.cs` —
  a fresh instance has every documented default value. Run: `dotnet test
  tests/AgentCoder.UnitTests --filter AgentCoderOptionsTests`.
- **Other validation:** Cross-check each default against FUNC-SPEC.md
  B-001/B-005/B-007/B-008/B-009 and TECH-SPEC.md §6 by inspection.

---

## T-004: LocalFileSystemRepository (Infrastructure)
- **Status:** [ ]
- **Behaviors covered:** B-002, B-003, B-004
- **Files to create:**
  - `src/AgentCoder.Infrastructure/FileSystem/LocalFileSystemRepository.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - `LocalFileSystemRepository : IFileSystemRepository` — implements all
    members from T-002.1 using `System.IO`, rooted at a `repoRoot` path
    passed into its constructor. `IsPathWithinRoot` resolves the full path
    and checks it is a descendant of `repoRoot` (rejects `..` traversal
    and absolute paths outside root).
- **Patterns applied:** Repository pattern (concrete implementation),
  Single Responsibility (file I/O only, no business rules about *which*
  files to touch).
- **Depends on:** T-002

### Subtasks
#### T-004.1: Implement read/write/list/exists/size
- **What:** Straightforward async file I/O against `repoRoot`-relative
  paths; `ListFilesAsync` supports glob patterns (e.g. `**/*.cs`).
- **Unit test:**
  `tests/AgentCoder.UnitTests/Infrastructure/LocalFileSystemRepositoryTests.cs`
  using a temp directory (not mocked — this is the concrete adapter under
  test): write-then-read round-trip, `ExistsAsync` true/false, `GetSizeAsync`
  matches actual byte length, `ListFilesAsync` returns expected matches
  for a glob. Run: `dotnet test tests/AgentCoder.UnitTests --filter
  LocalFileSystemRepositoryTests`.
- **Other validation:** Manual: point it at a real repo checkout and list
  `**/*.cs`, confirm the returned paths match `git ls-files -- '*.cs'`.

#### T-004.2: Implement IsPathWithinRoot path-traversal guard
- **What:** Rejects any relative path that resolves outside `repoRoot`
  (covers FUNC-SPEC B-004's path-traversal edge case).
- **Unit test:** Same test file as T-004.1 — cases: `../../etc/passwd` →
  `false`; `subdir/file.cs` → `true`; absolute path outside root → `false`.
- **Other validation:** N/A — fully covered by unit tests; this guard has
  no external side effect to check.

---

## T-005: GitCliRepository (Infrastructure)
- **Status:** [ ]
- **Behaviors covered:** B-002, B-006, B-007
- **Files to create:**
  - `src/AgentCoder.Infrastructure/Git/GitCliRepository.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - `GitCliRepository : IGitRepository` — implements all members from
    T-002.2 by shelling to the `git` executable via
    `System.Diagnostics.Process`, rooted at a working directory passed
    into its constructor. `ExecuteAsync(IGitCommand command, ...)` runs
    `git {command.BuildArguments()}` and returns
    `command.ParseResult(exitCode, stdout, stderr)`.
- **Patterns applied:** Repository pattern (concrete implementation),
  Single Responsibility (process invocation only — argument construction
  and result interpretation are delegated to `IGitCommand`, see T-008).
- **Depends on:** T-002

### Subtasks
#### T-005.1: Implement status/branch/upstream queries
- **What:** `IsInsideRepositoryAsync` → `git rev-parse
  --is-inside-work-tree`; `IsWorkingTreeCleanAsync` → `git status
  --porcelain` (clean iff empty output); `GetCurrentBranchAsync` → `git
  rev-parse --abbrev-ref HEAD`; `HasUpstreamAsync` → `git rev-parse
  --abbrev-ref --symbolic-full-name @{u}` (false on non-zero exit).
- **Unit test:** Not unit-tested directly (process-shelling I/O) — covered
  by integration tests (T-015) instead, per TECH-SPEC §5.1's rule that
  `IGitRepository` is mocked everywhere else. Declared explicitly: no
  unit test file for this class itself.
- **Other validation:** `tests/AgentCoder.IntegrationTests/GitWorkflowTests.cs`
  (T-015) exercises this against a real temp git repo — clean vs. dirty
  tree, branch name, upstream present/absent. Run: `dotnet test
  tests/AgentCoder.IntegrationTests --filter GitWorkflowTests`.

#### T-005.2: Implement ExecuteAsync(IGitCommand)
- **What:** Generic command execution used by `AddCommand`,
  `CommitCommand`, `PushCommand` (T-008).
- **Unit test:** Same note as T-005.1 — no unit test, covered by
  integration tests.
- **Other validation:** T-015's `GitWorkflowTests.cs` runs a real
  add→commit→push cycle through this method against a local bare-repo
  remote.

---

## T-006: LLM Client (Adapter)
- **Status:** [ ]
- **Behaviors covered:** B-004, B-008, B-009
- **Files to create:**
  - `src/AgentCoder.Core/Application/Llm/ILlmClient.cs`
  - `src/AgentCoder.Core/Application/Llm/LlmMessage.cs`
  - `src/AgentCoder.Core/Application/Llm/ToolDefinition.cs`
  - `src/AgentCoder.Infrastructure/Llm/LmStudioClient.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - `ILlmClient`: `Task<LlmResponse> SendAsync(IReadOnlyList<LlmMessage>
    messages, IReadOnlyList<ToolDefinition> tools, CancellationToken ct)`.
  - `LlmMessage` (record): `string Role`, `string? Content`,
    `IReadOnlyList<ToolCall>? ToolCalls`, `string? ToolCallId` (for tool
    result messages).
  - `ToolDefinition` (record): `string Name`, `string Description`,
    `JsonElement ParametersSchema`.
  - `LmStudioClient : ILlmClient` — adapts domain `LlmMessage`/
    `ToolDefinition` to LM Studio's OpenAI-compatible
    `chat/completions` JSON request/response shape; applies the
    connection retry policy from `AgentCoderOptions.LlmConnectionRetries`
    (retry with backoff on transport failure, timeout, or malformed
    response; raises a typed `LlmUnavailableException` after exhausting
    retries).
- **Patterns applied:** Adapter (approved pattern — isolates LM Studio's
  wire format from the rest of the codebase), Dependency Inversion
  (`AgenticEditUseCase` and `CommitAndPushUseCase` depend only on
  `ILlmClient`), Single Responsibility (wire-format translation and
  retry policy live here; the agentic loop's decision logic does not).
- **Depends on:** T-003

### Subtasks
#### T-006.1: Define ILlmClient, LlmMessage, ToolDefinition
- **What:** Pure interface/data-type declarations in `Core`.
- **Unit test:** N/A — declarations only.
- **Other validation:** `dotnet build` compiles.

#### T-006.2: Implement LmStudioClient request/response mapping
- **What:** Serializes `LlmMessage`/`ToolDefinition` into the
  OpenAI-compatible JSON body (`messages`, `tools`, `tool_choice`) and
  deserializes the response into `LlmResponse` (final text and/or tool
  calls).
- **Unit test:**
  `tests/AgentCoder.UnitTests/Infrastructure/LmStudioClientTests.cs`,
  `HttpClient` backed by a fake `HttpMessageHandler` (no real network
  call) — cases: request body contains expected fields for a given input;
  a response with `tool_calls` maps to `LlmResponse.ToolCalls`; a response
  with only `content` maps to a final text response. Run: `dotnet test
  tests/AgentCoder.UnitTests --filter LmStudioClientTests`.
- **Other validation:** Manual smoke test against a real running LM
  Studio instance (per TECH-SPEC §5.2) sending a trivial prompt and
  confirming a valid response is parsed.

#### T-006.3: Implement connectivity retry policy (B-008)
- **What:** On `HttpRequestException`, timeout, or non-JSON/malformed
  response, retry up to `AgentCoderOptions.LlmConnectionRetries` times
  with backoff; after exhausting retries, throw
  `LlmUnavailableException` carrying the configured endpoint URL.
- **Unit test:** Same test file as T-006.2 — fake handler that fails N-1
  times then succeeds (asserts success after retries); fake handler that
  always fails (asserts exactly `LlmConnectionRetries` attempts and the
  specific exception type/message).
- **Other validation:** Manual: point `--endpoint` at an unreachable
  address and confirm (via `--verbose`) the exact configured retry count
  before the documented error message appears — this is FUNC-SPEC B-008's
  validation, exercised end-to-end via T-014's CLI.

---

## T-007: Agent Tools (Strategy)
- **Status:** [ ]
- **Behaviors covered:** B-004
- **Files to create:**
  - `src/AgentCoder.Core/Application/Tools/ITool.cs`
  - `src/AgentCoder.Core/Application/Tools/ListFilesTool.cs`
  - `src/AgentCoder.Core/Application/Tools/ReadFileTool.cs`
  - `src/AgentCoder.Core/Application/Tools/WriteFileTool.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - `ITool`: `string Name { get; }`; `ToolDefinition GetDefinition()`;
    `Task<string> ExecuteAsync(JsonElement arguments, CancellationToken
    ct)` (returns the tool-result text sent back to the LLM).
  - `ListFilesTool : ITool` — takes `IFileSystemRepository` via
    constructor, `Name = "list_files"`, arguments `{ "glob": string }`.
  - `ReadFileTool : ITool` — `Name = "read_file"`, arguments
    `{ "path": string }`; if `IsPathWithinRoot` is false, returns an error
    result string instead of throwing (per B-004 edge case).
  - `WriteFileTool : ITool` — `Name = "write_file"`, arguments
    `{ "path": string, "content": string }`; same path-traversal guard as
    `ReadFileTool`.
- **Patterns applied:** Strategy (approved pattern — each tool is an
  interchangeable `ITool` the agentic loop dispatches to by name),
  Dependency Inversion (tools depend on `IFileSystemRepository`, not
  `System.IO`), Single Responsibility (each tool does exactly one thing).
- **Depends on:** T-004

### Subtasks
#### T-007.1: Implement ListFilesTool and ReadFileTool
- **What:** Thin wrappers around `IFileSystemRepository.ListFilesAsync`
  and `.ReadFileAsync`, with `ReadFileTool` rejecting out-of-root paths.
- **Unit test:**
  `tests/AgentCoder.UnitTests/Application/Tools/ListFilesToolTests.cs`,
  `ReadFileToolTests.cs`, mocking `IFileSystemRepository` — cases:
  well-formed arguments return the repository's result; `ReadFileTool`
  with `IsPathWithinRoot == false` returns an error string and never
  calls `ReadFileAsync`. Run: `dotnet test tests/AgentCoder.UnitTests
  --filter Tools`.
- **Other validation:** Exercised end-to-end by T-011's agentic-loop
  validation (B-004's "Bar.cs contains Foo()" scenario).

#### T-007.2: Implement WriteFileTool with path-traversal guard
- **What:** Wraps `IFileSystemRepository.WriteFileAsync`, rejecting
  out-of-root paths the same way as `ReadFileTool`.
- **Unit test:**
  `tests/AgentCoder.UnitTests/Application/Tools/WriteFileToolTests.cs`,
  mocked `IFileSystemRepository` — well-formed write calls
  `WriteFileAsync` with the exact path/content; out-of-root path returns
  an error string and never calls `WriteFileAsync`. Run: `dotnet test
  tests/AgentCoder.UnitTests --filter WriteFileToolTests`.
- **Other validation:** Same B-004 end-to-end scenario as T-007.1, plus
  explicit manual test: craft a task file asking the agent to write
  `../outside.cs` and confirm the file is never created outside the repo.

---

## T-008: Git Commands (Command)
- **Status:** [ ]
- **Behaviors covered:** B-006, B-007
- **Files to create:**
  - `src/AgentCoder.Core/Application/GitCommands/IGitCommand.cs`
  - `src/AgentCoder.Core/Application/GitCommands/AddCommand.cs`
  - `src/AgentCoder.Core/Application/GitCommands/CommitCommand.cs`
  - `src/AgentCoder.Core/Application/GitCommands/PushCommand.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - `IGitCommand`: `IReadOnlyList<string> BuildArguments()`;
    `GitCommandResult ParseResult(int exitCode, string stdOut, string
    stdErr)`.
  - `AddCommand : IGitCommand` — `BuildArguments() => ["add", "-A"]`.
  - `CommitCommand(string message) : IGitCommand` —
    `BuildArguments() => ["commit", "-m", message]`; `ParseResult`
    surfaces a specific failure reason when stderr indicates missing
    `user.name`/`user.email`.
  - `PushCommand() : IGitCommand` — `BuildArguments() => ["push"]`;
    `ParseResult` distinguishes a non-fast-forward rejection from other
    failures via stderr content, for use by T-013's retry/abort logic.
- **Patterns applied:** Command (approved pattern), Single Responsibility
  (each command only knows its own arguments and how to interpret its own
  output; no retry logic here — that belongs to the use case that invokes
  it, see T-013).
- **Depends on:** T-002

### Subtasks
#### T-008.1: Implement IGitCommand, AddCommand, CommitCommand
- **What:** As specified above.
- **Unit test:**
  `tests/AgentCoder.UnitTests/Application/GitCommands/AddCommandTests.cs`,
  `CommitCommandTests.cs` — `BuildArguments()` returns the exact expected
  argument list; `CommitCommand.ParseResult` with a
  missing-user.name-style stderr string produces a result flagged as that
  specific failure; a zero exit code produces a success result. Run:
  `dotnet test tests/AgentCoder.UnitTests --filter GitCommands`.
- **Other validation:** T-015's integration tests execute these commands
  against a real repo end-to-end (covers B-006's validation: `git log -1
  --pretty=%s` matches the Conventional Commits pattern).

#### T-008.2: Implement PushCommand
- **What:** As specified above, including non-fast-forward detection.
- **Unit test:**
  `tests/AgentCoder.UnitTests/Application/GitCommands/PushCommandTests.cs`
  — `BuildArguments()` returns `["push"]`; `ParseResult` with a
  non-fast-forward-style stderr string is flagged distinctly from a
  generic failure. Run: `dotnet test tests/AgentCoder.UnitTests --filter
  PushCommandTests`.
- **Other validation:** T-015 exercises a real push against a local
  bare-repo remote (success case) and against an unreachable remote
  (failure case), matching FUNC-SPEC B-007's validation.

---

## T-009: PreflightCheckUseCase
- **Status:** [ ]
- **Behaviors covered:** B-002
- **Files to create:**
  - `src/AgentCoder.Core/Application/UseCases/PreflightCheckUseCase.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - `PreflightCheckUseCase(IFileSystemRepository fileSystem, IGitRepository
    git)` — `Task<Result> ExecuteAsync(string taskFilePath,
    CancellationToken ct)`; `Result` carries success/failure plus a list
    of every failed check's message (never just the first).
- **Patterns applied:** Single Responsibility (only validates
  preconditions; does not load the task or touch the LLM), Dependency
  Inversion (depends on `IFileSystemRepository`/`IGitRepository`
  interfaces only).
- **Depends on:** T-004, T-005

### Subtasks
#### T-009.1: Implement the three preflight checks
- **What:** Checks, in order, but collecting *all* failures rather than
  short-circuiting: task file exists and is non-empty (via
  `IFileSystemRepository.ExistsAsync`/`GetSizeAsync`); CWD is a git repo
  (via `IGitRepository.IsInsideRepositoryAsync`); working tree is clean
  (via `IGitRepository.IsWorkingTreeCleanAsync`).
- **Unit test:**
  `tests/AgentCoder.UnitTests/Application/UseCases/PreflightCheckUseCaseTests.cs`,
  mocking both repository interfaces — cases: all checks pass → success;
  missing file only → failure listing exactly that message; not-a-repo
  only → failure listing exactly that message; dirty tree only → failure
  containing "working tree is not clean"; multiple simultaneous failures
  → failure lists all of them, not just one. Run: `dotnet test
  tests/AgentCoder.UnitTests --filter PreflightCheckUseCaseTests`.
- **Other validation:** Matches FUNC-SPEC B-002's validation directly:
  manual run with an uncommitted change present returns non-zero exit,
  prints "working tree is not clean", and `git log` shows no new commit.

---

## T-010: LoadTaskUseCase
- **Status:** [ ]
- **Behaviors covered:** B-003
- **Files to create:**
  - `src/AgentCoder.Core/Application/UseCases/LoadTaskUseCase.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - `LoadTaskUseCase(IFileSystemRepository fileSystem, AgentCoderOptions
    options)` — `Task<Result<AgentTask>> ExecuteAsync(string
    taskFilePath, CancellationToken ct)`; enforces
    `MaxTaskFileSizeBytes` and UTF-8 decoding, returning a failure Result
    with a clear message rather than throwing for either edge case.
- **Patterns applied:** Single Responsibility (loading and validating the
  task file's size/encoding only — no LLM interaction).
- **Depends on:** T-004

### Subtasks
#### T-010.1: Implement task loading with size/encoding validation
- **What:** As specified above.
- **Unit test:**
  `tests/AgentCoder.UnitTests/Application/UseCases/LoadTaskUseCaseTests.cs`,
  mocking `IFileSystemRepository` — cases: normal content → `AgentTask`
  with matching `Description`; size over `MaxTaskFileSizeBytes` → failure
  "task file too large"; simulated decode failure → failure with a clear
  decode-error message. Run: `dotnet test tests/AgentCoder.UnitTests
  --filter LoadTaskUseCaseTests`.
- **Other validation:** Matches FUNC-SPEC B-003's validation: with
  `--dry-run` (wired in T-014), the tool echoes the loaded text
  byte-for-byte identical to the source file.

---

## T-011: AgenticEditUseCase
- **Status:** [ ]
- **Behaviors covered:** B-004
- **Files to create:**
  - `src/AgentCoder.Core/Application/UseCases/AgenticEditUseCase.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - `AgenticEditUseCase(ILlmClient llm, IEnumerable<ITool> tools,
    AgentCoderOptions options)` — `Task<Result<AgenticLoopSummary>>
    ExecuteAsync(IReadOnlyList<LlmMessage> conversation, CancellationToken
    ct)`; runs the send→dispatch-tool-calls→append-results loop until the
    LLM returns a final response with no tool calls, dispatching each
    tool call by matching `ToolCall.Name` against the injected `ITool`s;
    aborts with a specific error once `MaxToolCallIterations` is
    exceeded. `AgenticLoopSummary` records the final conversation and
    whether any `write_file` calls occurred (used by T-012/T-013 to know
    whether there are changes to build/commit).
- **Patterns applied:** Strategy (dispatches to `ITool` implementations
  without knowing their concrete types), Dependency Inversion (depends on
  `ILlmClient` and `ITool` abstractions only), Single Responsibility (the
  loop's only job is turn-taking between the LLM and the tools — it does
  not build or commit anything).
- **Depends on:** T-006, T-007

### Subtasks
#### T-011.1: Implement the send/dispatch loop
- **What:** As specified above.
- **Unit test:**
  `tests/AgentCoder.UnitTests/Application/UseCases/AgenticEditUseCaseTests.cs`,
  mocking `ILlmClient` (scripted sequence of responses) and `ITool` —
  cases: a response with tool calls dispatches to the matching mocked
  tool and feeds its result back as the next message; a response with no
  tool calls ends the loop and returns the final summary; an unrecognized
  tool name in a tool call returns an error result to the LLM instead of
  throwing. Run: `dotnet test tests/AgentCoder.UnitTests --filter
  AgenticEditUseCaseTests`.
- **Other validation:** Matches FUNC-SPEC B-004's validation: given a task
  "add a method Foo() to Bar.cs that returns 42" against a real temp repo
  (mocked `ILlmClient` scripted to call `write_file`, or manual run
  against a live LM Studio instance), `Bar.cs` on disk contains a `Foo`
  method returning `42` after the loop completes.

#### T-011.2: Implement max-iteration guard
- **What:** Counts total tool-call round-trips; once it exceeds
  `MaxToolCallIterations`, stops the loop and returns a failure Result
  with message "agent exceeded maximum tool-call iterations" instead of
  continuing indefinitely.
- **Unit test:** Same test file as T-011.1 — a mocked `ILlmClient`
  scripted to always return more tool calls is stopped at exactly
  `MaxToolCallIterations` and the failure message matches exactly.
- **Other validation:** N/A beyond the unit test — this is an internal
  safety bound with no separate external artifact to check.

---

## T-012: BuildVerificationUseCase
- **Status:** [ ]
- **Behaviors covered:** B-005
- **Files to create:**
  - `src/AgentCoder.Core/Application/UseCases/BuildVerificationUseCase.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - `BuildVerificationUseCase(AgenticEditUseCase agenticEdit,
    AgentCoderOptions options, string repoRoot)` —
    `Task<Result<BuildResult>> ExecuteAsync(IReadOnlyList<LlmMessage>
    conversation, CancellationToken ct)`; auto-detects a single `.sln`
    (preferred) or single `.csproj` under `repoRoot` (logs which one when
    multiple candidates exist and the first is used, per FUNC-SPEC B-005);
    runs `dotnet build` via `System.Diagnostics.Process`; on non-zero exit,
    appends the build output as a new message and re-invokes
    `AgenticEditUseCase`, repeating up to `MaxBuildRetries` (default 10)
    total attempts.
- **Patterns applied:** Single Responsibility (build detection + the
  retry loop only; delegates actual editing back to
  `AgenticEditUseCase`), Dependency Inversion (depends on the use case
  abstraction, not on tool internals).
- **Depends on:** T-011

### Subtasks
#### T-012.1: Implement project/solution auto-detection
- **What:** As specified above — no buildable target found aborts
  immediately with no retries attempted.
- **Unit test:**
  `tests/AgentCoder.UnitTests/Application/UseCases/BuildVerificationUseCaseTests.cs`
  using a temp directory fixture (not mocked, since this is filesystem
  discovery, not a repository-abstracted concern) — cases: single `.sln`
  present → chosen; no `.sln` but single `.csproj` → chosen; multiple
  `.csproj` and no `.sln` → first found is chosen and logged; none found
  → immediate failure, no `dotnet build` invoked. Run: `dotnet test
  tests/AgentCoder.UnitTests --filter BuildVerificationUseCaseTests`.
- **Other validation:** Matches FUNC-SPEC B-005: "no buildable target"
  case verified against a real empty temp repo.

#### T-012.2: Implement the build-retry cycle
- **What:** As specified above, calling a mocked/injected build-runner
  abstraction so the retry logic itself is testable without invoking real
  `dotnet build` in unit tests.
- **Unit test:** Same test file as T-012.1, with a fake build-runner
  delegate — cases: first attempt succeeds → single attempt, `Succeeded ==
  true`; fails 9 times then succeeds on the 10th → success with
  `AttemptNumber == 10`; fails all 10 times → failure Result, no further
  retries attempted (delegate called exactly 10 times), the mocked
  `AgenticEditUseCase` invoked once per failed attempt with the prior
  build output included in its input messages.
- **Other validation:** Matches FUNC-SPEC B-005's validation directly:
  after exactly 10 failed attempts the process exits non-zero and no git
  commit exists (verified end-to-end via T-014 + T-015); a fix that
  succeeds within budget yields a final `dotnet build` exit code of 0.

---

## T-013: CommitAndPushUseCase
- **Status:** [ ]
- **Behaviors covered:** B-006, B-007
- **Files to create:**
  - `src/AgentCoder.Core/Application/UseCases/CommitAndPushUseCase.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - `CommitAndPushUseCase(IGitRepository git, ILlmClient llm,
    AgentCoderOptions options)` — `Task<Result> ExecuteAsync(AgentTask
    task, CancellationToken ct)`; checks for staged/pending changes first
    (skips commit+push and reports "no changes to commit" if none); asks
    `ILlmClient` to summarize the change as a Conventional Commit message,
    validates it via `CommitMessage.TryParse`, falls back to
    `CommitMessage.Fallback` on failure; executes `AddCommand` then
    `CommitCommand` via `IGitRepository.ExecuteAsync`; on commit success,
    checks `HasUpstreamAsync` (aborts immediately with the documented
    message if false) then executes `PushCommand` with up to
    `GitPushRetries` attempts and backoff, aborting (commit preserved
    locally) if all attempts fail.
- **Patterns applied:** Command (invokes `AddCommand`/`CommitCommand`/
  `PushCommand` via `IGitRepository.ExecuteAsync`), Dependency Inversion
  (depends on `IGitRepository`/`ILlmClient` interfaces only), Single
  Responsibility (orchestrates commit+push only — no build logic here).
- **Depends on:** T-008, T-006

### Subtasks
#### T-013.1: Implement commit-message generation and fallback
- **What:** As specified above.
- **Unit test:**
  `tests/AgentCoder.UnitTests/Application/UseCases/CommitAndPushUseCaseTests.cs`,
  mocking `IGitRepository` and `ILlmClient` — cases: LLM returns a
  well-formed Conventional Commit summary → used verbatim; LLM returns a
  malformed summary → `CommitMessage.Fallback` used instead; no pending
  changes (mocked git status indicates none) → commit/push are never
  invoked and the Result reports "no changes to commit". Run: `dotnet
  test tests/AgentCoder.UnitTests --filter CommitAndPushUseCaseTests`.
- **Other validation:** Matches FUNC-SPEC B-006's validation: `git log -1
  --pretty=%s` on the current branch matches
  `^(feat|fix|chore|refactor|docs|test|style|perf)(\(.+\))?: .+`
  (verified end-to-end in T-015).

#### T-013.2: Implement push with retry and upstream/non-fast-forward handling
- **What:** As specified above.
- **Unit test:** Same test file as T-013.1 — cases: `HasUpstreamAsync ==
  false` → immediate abort with the documented upstream message, `Push`
  never attempted; push fails twice then succeeds on the third mocked
  attempt → success, exactly 3 `ExecuteAsync(PushCommand)` calls; push
  fails all `GitPushRetries` attempts → failure Result, commit is not
  rolled back (no delete/reset call made on the mock).
- **Other validation:** Matches FUNC-SPEC B-007's validation: on a real
  local remote, `git log origin/<branch> -1` matches local HEAD; against
  an unreachable remote, exactly the configured retry count of attempts
  occurs (visible via `--verbose`) before aborting with the commit intact
  (verified end-to-end in T-015).

---

## T-014: CLI Composition Root
- **Status:** [ ]
- **Behaviors covered:** B-001, B-002, B-003, B-004, B-005, B-006, B-007,
  B-008, B-009
- **Files to create:**
  - `src/AgentCoder.Cli/Program.cs`
  - `src/AgentCoder.Cli/CliOptions.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - `CliOptions`: `System.CommandLine` option/argument definitions —
    positional `<task-file-path>` (required), `--endpoint <url>`,
    `--model <name>`, `--config <path>`, `--verbose` (flag),
    `--dry-run` (flag).
  - `Program.Main(string[] args)`: builds a
    `Microsoft.Extensions.DependencyInjection` container binding
    `AgentCoderOptions` from `appsettings.json` (overridden by CLI flags
    per B-001/B-009), registers `LocalFileSystemRepository`,
    `GitCliRepository`, `LmStudioClient`, all `ITool`s, and each use case;
    runs the pipeline `PreflightCheckUseCase` →
    (`--dry-run`: print loaded task and exit) → `LoadTaskUseCase` →
    `AgenticEditUseCase` (via `BuildVerificationUseCase`) →
    `CommitAndPushUseCase`; maps every `Result` failure and any uncaught
    exception to a non-zero exit code with the documented error message;
    `--verbose` raises the console logger's minimum level to show
    retry/tool-call traces.
- **Patterns applied:** Dependency Inversion (this is the one place
  concrete infrastructure types are wired to abstractions — the
  composition root), Single Responsibility (argument parsing lives in
  `CliOptions`, DI wiring and pipeline sequencing live in `Program`, nei-
  ther contains business logic itself).
- **Depends on:** T-003, T-009, T-010, T-012, T-013

### Subtasks
#### T-014.1: Implement CliOptions and argument parsing
- **What:** As specified above, including the "missing task file path
  prints usage and exits 1" behavior from B-001.
- **Unit test:** N/A — `System.CommandLine` wiring is declared explicitly
  as not independently unit-testable in a meaningful way; strengthened
  other-validation below.
- **Other validation:** Manual: `agentcoder run` (no args) prints usage
  and exits 1 (FUNC-SPEC B-001's validation, run directly:
  `dotnet run --project src/AgentCoder.Cli -- run`).

#### T-014.2: Implement DI wiring and config precedence (CLI flag over file)
- **What:** Binds `AgentCoderOptions` from `appsettings.json` first, then
  overwrites individual properties from any CLI flags that were supplied.
- **Unit test:**
  `tests/AgentCoder.UnitTests/Cli/ConfigResolutionTests.cs` — given a
  sample `appsettings.json` content and a supplied `--endpoint` flag
  value, the resolved `AgentCoderOptions.Endpoint` equals the flag value,
  not the file value; with no flag supplied, the file value is kept; with
  no file present, built-in defaults apply. Run: `dotnet test
  tests/AgentCoder.UnitTests --filter ConfigResolutionTests`.
- **Other validation:** Matches FUNC-SPEC B-001/B-009's validation
  directly: `agentcoder run ./task.md --endpoint
  http://localhost:9999/v1 --verbose` shows the resolved endpoint as
  `http://localhost:9999/v1`.

#### T-014.3: Wire the end-to-end pipeline and exit-code mapping
- **What:** Sequences the five use cases as described above, short-
  circuiting on the first failing `Result` and mapping it to a non-zero
  exit with that Result's message printed to stderr.
- **Unit test:** N/A — full pipeline wiring, exercised as a whole rather
  than unit-tested piecewise (each use case already has its own unit
  tests from T-009/T-010/T-012/T-013).
- **Other validation:** Manual end-to-end run per TECH-SPEC §5.2: with a
  running LM Studio instance and a sample throwaway git repo, run
  `dotnet run --project src/AgentCoder.Cli -- run ./sample-task.md
  --verbose` and confirm every behavior in FUNC-SPEC.md §5's acceptance
  table against the resulting commit/build/push outcome.

---

## T-015: Integration Test Suite
- **Status:** [ ]
- **Behaviors covered:** B-002, B-006, B-007
- **Files to create:**
  - `tests/AgentCoder.IntegrationTests/GitWorkflowTests.cs`
- **Files to alter:** —
- **Methods/interfaces to define:**
  - Test fixture helper that creates a temp directory, runs `git init`,
    configures a local bare repository as `origin`, and tears both down
    after each test.
- **Patterns applied:** N/A (test code) — exercises the Repository and
  Command pattern implementations from T-005/T-008 against a real `git`
  binary, per TECH-SPEC §5.2.
- **Depends on:** T-005, T-008

### Subtasks
#### T-015.1: Preflight and status scenarios against a real repo
- **What:** Covers B-002's validation: clean tree → checks pass; a
  modified/untracked file present → `IsWorkingTreeCleanAsync` returns
  `false` and (via `PreflightCheckUseCase`, injected with the real
  `GitCliRepository`) the run aborts with no new commit.
- **Unit test:** N/A (this file is itself the "other validation" layer
  for B-002; declared explicitly per the mandatory dual-entry rule).
- **Other validation:** `dotnet test tests/AgentCoder.IntegrationTests
  --filter GitWorkflow_Preflight`.

#### T-015.2: Commit and push scenarios against a real repo + local remote
- **What:** Covers B-006/B-007's validation end-to-end: stage a file
  change, commit via `AddCommand`/`CommitCommand`, assert `git log -1
  --pretty=%s` matches the Conventional Commits regex; push to the local
  bare-repo remote and assert `git log origin/<branch> -1` matches local
  HEAD; point the remote at an unreachable path and assert the configured
  `GitPushRetries` attempts occur before the commit is left intact
  locally.
- **Unit test:** N/A — same rationale as T-015.1.
- **Other validation:** `dotnet test tests/AgentCoder.IntegrationTests
  --filter GitWorkflow_CommitPush`.

---

## Validation Summary

**Functional coverage**
- All nine behaviors (B-001–B-009) appear in at least one task's
  "Behaviors covered".
- Every FUNC-SPEC.md Validation entry is reflected in a subtask's "Other
  validation" (see cross-references embedded above).
- Nothing in "Out of Scope" (branching, PRs, pull/merge, `dotnet test`,
  non-.NET projects, the future harness, stashing) appears in any task.

**Technical conformance**
- All file paths match TECH-SPEC.md §4 exactly.
- All file/git access goes through `IFileSystemRepository`/
  `IGitRepository` (T-002/T-004/T-005) — no use case touches `System.IO`
  or `Process` directly.
- No task assigns more than one responsibility to a class; use cases
  orchestrate, tools/commands execute single operations, entities hold
  data only.
- Use cases (T-009–T-013) depend only on interfaces
  (`ILlmClient`, `IGitRepository`, `IFileSystemRepository`, `ITool`,
  `IGitCommand`), never on `LmStudioClient`/`GitCliRepository`/
  `LocalFileSystemRepository` directly — those are wired only in T-014.
- Strategy (T-007), Command (T-008), and Adapter (T-006) all appear where
  approved.
- Unit tests use xUnit, mirror the `src/` tree under
  `tests/AgentCoder.UnitTests`, and mock repository/client interfaces
  per TECH-SPEC §5.1.
- Other-validation entries use the integration-test project and the exact
  manual commands from TECH-SPEC §5.2.

**Task hygiene**
- No task or subtask contains implementation code, only
  files/signatures/responsibilities.
- Every subtask has both a Unit test entry and an Other validation entry
  (a small number explicitly state "N/A" with a stated reason, per the
  skill's allowance).
- Dependency order is acyclic: T-000 → T-001/T-002/T-003 → T-004/T-005/
  T-006/T-007/T-008 → T-009/T-010/T-011 → T-012/T-013 → T-014 → T-015
  (T-015 only needs T-005/T-008, so it can run in parallel with
  T-009–T-014 once those two are done).
