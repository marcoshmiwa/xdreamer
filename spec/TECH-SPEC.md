# Technical Specification — AgentCoder

## 1. Technology Stack

- **Language/Runtime:** C# on .NET 10 (LTS, released November 2025 —
  current LTS target; .NET 8/9 both reach end of support November 2026)
- **CLI framework:** `System.CommandLine` for argument/flag parsing
- **LLM connectivity:** Plain `HttpClient` + `System.Text.Json`, calling LM
  Studio's OpenAI-compatible chat-completions endpoint (tool/function
  calling enabled). No vendor SDK — LM Studio's API surface is the only
  target for this version.
- **Git operations:** Shelled out via `System.Diagnostics.Process` invoking
  the system `git` CLI directly (matches the requirement to "use the
  commands of git" — no git library dependency).
- **Configuration:** `Microsoft.Extensions.Configuration` binding
  `appsettings.json` (+ environment overrides) into a strongly-typed
  options object.
- **Logging:** `Microsoft.Extensions.Logging` with a console provider;
  `--verbose` raises the minimum log level.
- **Build target under test:** `dotnet build`, auto-detected `.sln`/`.csproj`
  in the target repository (this is the *target* repo the agent edits, a
  separate concern from AgentCoder's own build).

## 2. Code Principles (MANDATORY)

### 2.1 SOLID — obligatory in all code
All code produced for this project MUST follow the SOLID principles:

- **S — Single Responsibility Principle (OBLIGATORY, enforced strictly):**
  every class/module has exactly one reason to change. E.g. the class that
  calls the LLM API must not also parse git output; the class that runs
  `dotnet build` must not also decide retry counts. This principle takes
  priority: when reviewing or planning code, check it first.
- **O — Open/Closed Principle:** entities are open for extension, closed
  for modification. New agent tools or new git commands are added as new
  classes implementing existing interfaces, not by editing existing ones.
- **L — Liskov Substitution Principle:** any `ITool` or `IGitCommand`
  implementation must be usable anywhere its interface is expected, with no
  surprising behavior.
- **I — Interface Segregation Principle:** many small, focused interfaces
  (`ITool`, `IGitCommand`, `ILlmClient`, `IFileSystemRepository`,
  `IGitRepository`) instead of one large one. No client depends on methods
  it does not use.
- **D — Dependency Inversion Principle:** application/use-case code depends
  only on abstractions (`ILlmClient`, `IGitRepository`,
  `IFileSystemRepository`); the CLI composition root is the only place that
  wires concrete infrastructure implementations in.

### 2.2 Repository Pattern — obligatory
All external state access MUST go through repositories:
- `IFileSystemRepository` — reads, writes, and lists files under the
  target repo root. Implemented by `LocalFileSystemRepository`.
- `IGitRepository` — exposes git status/add/commit/push operations at the
  domain level (this is the literal git repository, doubling naturally as
  the Repository-pattern boundary for this project). Implemented by
  `GitCliRepository`, which shells out to the `git` process.
- Application/use-case code depends only on these interfaces, never
  directly on `System.IO` or `Process.Start("git", ...)`.

## 3. Additional Design Patterns

- **Strategy** — each agent tool (`ListFilesTool`, `ReadFileTool`,
  `WriteFileTool`) implements a common `ITool` interface, registered in a
  tool registry the agentic loop calls without knowing concrete types. New
  tools can be added later without changing the loop.
- **Command** — each git operation (`AddCommand`, `CommitCommand`,
  `PushCommand`) is encapsulated as an `IGitCommand` with its own
  execute/retry behavior, letting B-007's push-retry logic and B-005's
  rebuild-retry logic wrap individual commands uniformly.
- **Adapter** — `LmStudioClient` implements `ILlmClient` and adapts LM
  Studio's OpenAI-compatible request/response shape (chat messages, tool
  calls, tool results) to the domain's own message/tool-call types, so the
  rest of the codebase never depends on the wire format directly.

## 4. Architecture and Project Structure

```
src/
├── AgentCoder.Cli/
│   ├── Program.cs                        # composition root, wires DI
│   └── CliOptions.cs                     # System.CommandLine option defs
├── AgentCoder.Core/
│   ├── Domain/
│   │   ├── Entities/
│   │   │   ├── AgentTask.cs
│   │   │   ├── BuildResult.cs
│   │   │   └── CommitMessage.cs
│   │   └── Repositories/
│   │       ├── IFileSystemRepository.cs
│   │       └── IGitRepository.cs
│   ├── Application/
│   │   ├── UseCases/
│   │   │   ├── PreflightCheckUseCase.cs   # B-002
│   │   │   ├── LoadTaskUseCase.cs         # B-003
│   │   │   ├── AgenticEditUseCase.cs      # B-004
│   │   │   ├── BuildVerificationUseCase.cs# B-005
│   │   │   └── CommitAndPushUseCase.cs    # B-006, B-007
│   │   ├── Tools/                         # Strategy pattern
│   │   │   ├── ITool.cs
│   │   │   ├── ListFilesTool.cs
│   │   │   ├── ReadFileTool.cs
│   │   │   └── WriteFileTool.cs
│   │   ├── GitCommands/                   # Command pattern
│   │   │   ├── IGitCommand.cs
│   │   │   ├── AddCommand.cs
│   │   │   ├── CommitCommand.cs
│   │   │   └── PushCommand.cs
│   │   └── Llm/
│   │       └── ILlmClient.cs              # Adapter interface
│   └── Configuration/
│       └── AgentCoderOptions.cs           # B-009 bound options
├── AgentCoder.Infrastructure/
│   ├── FileSystem/
│   │   └── LocalFileSystemRepository.cs
│   ├── Git/
│   │   └── GitCliRepository.cs            # shells out to `git`
│   └── Llm/
│       └── LmStudioClient.cs              # Adapter implementation
tests/
├── AgentCoder.UnitTests/
│   ├── Application/
│   │   ├── PreflightCheckUseCaseTests.cs
│   │   ├── BuildVerificationUseCaseTests.cs
│   │   └── CommitAndPushUseCaseTests.cs
│   └── Domain/
└── AgentCoder.IntegrationTests/
    └── GitWorkflowTests.cs                # real temp git repo, no LLM
```

## 5. Testing Strategy

### 5.1 Unit tests
- **Framework:** xUnit (no team preference stated; xUnit chosen as the
  current de facto standard for .NET).
- **Location:** `tests/AgentCoder.UnitTests`, mirroring the `src/` folder
  structure, one test class per production class (`FooTests.cs` for
  `Foo.cs`).
- **Mocking:** `IGitRepository`, `IFileSystemRepository`, and `ILlmClient`
  are mocked at their interfaces (e.g. via `NSubstitute` or `Moq`) — no
  test ever shells out to real `git` or hits a real HTTP endpoint in a unit
  test.
- **Minimum expectations:** every use case (`PreflightCheckUseCase`,
  `BuildVerificationUseCase`, `CommitAndPushUseCase`, etc.) has tests
  covering both its success path and each documented edge case from
  FUNC-SPEC.md (e.g. dirty tree, build retry exhaustion, missing upstream).
- **Run command:** `dotnet test tests/AgentCoder.UnitTests`

### 5.2 Other tests
- **Integration tests:** `tests/AgentCoder.IntegrationTests` create a real
  temporary directory, run `git init` in it, and exercise
  `GitCliRepository` and the git-command classes against that real repo
  (add/commit/push against a local bare-repo remote) — validates B-002,
  B-006, B-007 without needing a live LM Studio instance.
  Run: `dotnet test tests/AgentCoder.IntegrationTests`
- **Manual end-to-end validation:** with a running local LM Studio
  instance and a sample throwaway git repo, run
  `dotnet run --project src/AgentCoder.Cli -- run ./sample-task.md
  --verbose` and confirm the behaviors in FUNC-SPEC.md Section 5 against
  the resulting commit and build output.

## 6. Conventions

- **Naming:** PascalCase for types/methods/public members, camelCase for
  locals and parameters, `I`-prefixed interfaces.
- **Async:** all I/O (file, process, HTTP) is `async`/`await` with
  `CancellationToken` threaded through from the CLI's cancellation source.
- **Error handling:** use cases return a `Result<T>`-style outcome
  (success/failure + message) rather than throwing for expected failure
  paths (dirty tree, build failure, push failure); unexpected exceptions
  are caught once at the composition root in `Program.cs`, logged, and
  mapped to a non-zero exit code.
- **Commit message format:** Conventional Commits, validated by regex
  `^(feat|fix|chore|refactor|docs|test|style|perf)(\(.+\))?: .+` (B-006).
- **Linting/formatting:** `dotnet format` run via
  `dotnet format --verify-no-changes` in CI; contributors run
  `dotnet format` before committing.
