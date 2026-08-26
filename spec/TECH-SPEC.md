# Technical Specification

## 1. Technology Stack

**Project Classification**
- **Open-Source** — standard OSS hygiene, no internal tech-radar/compliance process.
- **CVE Policy: zero-tolerance** — no unpatched critical/high CVEs in any pinned version.

**Runtime & Language**
- **.NET 10 (LTS)**, pinned `>= 10.0.11`, supported through Nov 2028.
- **C# 14**.
- `global.json` pins the SDK to the `10.0.1xx` feature band so builds cannot silently drift to an unpatched SDK.

**CVE Audit (verified 2026-08-23)**
- .NET 10 is the current LTS. Latest patch **10.0.11** (released 2026-08-11, Patch Tuesday) fixes CVE-2026-62900 (.NET Information Disclosure) plus ~9 other CVEs (RCE/EoP/DoS/security-feature-bypass) disclosed the same day — 10.0.11 is the *patched* version, not one with known-open issues. Pinning `>= 10.0.11` satisfies zero-tolerance.
- Earlier 2026 .NET advisories (CVE-2026-47302, -50648, -26130, -45591, -45491, -33116) are all in components this CLI never touches (`EncryptedXml`, SignalR, Blazor Server, `TarFile.ExtractToDirectory`) — outside our exposure surface regardless of patch status.
- **xUnit v3 4.0.0** (released 2026-08-14) — current stable, no known CVEs. Older 2.x-era transitive CVEs (CVE-2018-8292, CVE-2019-0820) do not apply at this version.
- This audit is a point-in-time snapshot (2026-08-23). Re-run `dotnet list package --vulnerable --include-transitive` and re-check .NET/xUnit advisories at implementation time before publishing — zero-tolerance requires currency at build time, not spec time (§6 audit).

**Production Dependencies — zero third-party NuGet packages.** Everything the agent loop needs is in the BCL, minimizing the exact surface a zero-tolerance CVE policy has to track:
- `System.Net.Http.HttpClient` — LM Studio (OpenAI-compatible) calls
- `System.Text.Json` with a source-generated `JsonSerializerContext` — NDJSON parsing/emission (source-gen avoids reflection, keeps Native AOT publishing clean)
- `System.Diagnostics.Process` — the `bash` tool
- `Console.In` / `Console.OpenStandardOutput()` — the stdio transport
- `System.IO.File` / `Path` — `read_file` / `write_file` / `edit_file`

**Test Dependencies**
- **xUnit v3 4.0.0** — the only third-party package in the solution, referenced by the test project only (never ships).
- **`System.Net.HttpListener`** (BCL, cross-platform since .NET Core) — hosts the mock LM Studio HTTP server for Validation Criteria #10–12, avoiding an ASP.NET Core framework reference just to stub one endpoint.

**Distribution**
- `dotnet publish` as a **self-contained, trimmed, Native AOT single-file executable** targeting `net10.0`.
- Rationale: the orchestrator spawns the CLI per-task, so process cold-start latency matters — AOT starts near-instantly with no JIT warmup — and a single binary needs no separate .NET runtime install on the host.
- Fallback if AOT trimming ever conflicts with the JSON source-gen setup: framework-dependent self-contained publish, same TFM.

**Governance**
- CI runs `dotnet list package --vulnerable --include-transitive` on every build as the automated zero-tolerance CVE gate.

## 2. Design Patterns

**Philosophy**: minimalist by default (YAGNI/KISS) — patterns are proposed only where they solve a concrete, spec-mandated problem. One pattern earns its place; everything else is plain procedural code.

**Ports & Adapter (single port) — LLM backend**
- **Port**: `ILlmClient`, a single-method interface — `Task<ChatResponse> CompleteAsync(ChatRequest request)`.
- **Adapter**: `LmStudioChatClient`, the sole implementation — wraps `HttpClient`, POSTs to `{base_url}/chat/completions` using the OpenAI tool-calling schema, maps request/response to/from the port's plain types.
- **Wiring**: no DI container — the loop controller receives an `ILlmClient` via constructor injection; `Program.cs` constructs `new LmStudioChatClient(config.llm)` and passes it in directly.
- **Justification**: FUNC-SPEC §1 requires the backend be "a config value, not something hardcoded into the loop" and calls this "a seam for future providers." Validation Criterion #12 (pointing `config.llm.base_url` at a second mock server with zero code change) is only provable if the loop depends on the port, never on `HttpClient` or LM-Studio-specific shapes directly.

**Rejected (with reasons)**

| Candidate | Why rejected |
|---|---|
| Strategy/Command pattern for the 4 tools (`ITool` interface + dispatch table) | FUNC-SPEC §1 fixes the tool set at "exactly four tool primitives" — not open-ended. Dispatch is a switch expression / static dictionary mapping tool name → plain handler function, with gating (`read_file` ungated; `write_file`/`edit_file`/`bash` gated) as a small static lookup. Handler functions are unit-testable directly without interface indirection. |
| Formal GoF State pattern (class-per-state) for the loop controller | The FUNC-SPEC §3 state diagram is linear/sequential per tool call — no concurrent fan-out across pending permission requests. A plain `while`/`foreach` async loop traces directly to the diagram's transitions; a class hierarchy would add indirection without solving a concrete problem. |
| Repository/DAO for file tools | `read_file`/`write_file`/`edit_file` are stateless, direct wrappers over `System.IO.File` — no query/persistence logic to abstract. |
| Factory for message (de)serialization | Already solved by the source-generated `JsonSerializerContext` (§1) — a factory would just wrap what source-gen already provides. |
| DI container for `ILlmClient` wiring | One interface, one implementation, one call site — constructor injection by hand is sufficient; a container adds a dependency and reflection surface this project doesn't need (conflicts with Native AOT/trimming goals in §1). |

## 3. SOLID Constraints

**S — Single Responsibility**
- **Rule**: each class has exactly one reason to change — message framing, loop orchestration, LLM transport, or one tool's OS operation — never combined.
- **Example**: `ReadFileTool.Execute(ReadFileInput)` calls only `File.ReadAllText`/`File.Exists` and never constructs a `tool_result` JSON message — that's `AgentLoop`'s job alone.

**O — Open/Closed**
- **Rule**: `AgentLoop` must be open to new LLM backends, closed to modification — extend by adding an `ILlmClient` implementation, never by editing `AgentLoop`. The 4-tool dispatch switch is a deliberate, documented exception (§2): the tool set is spec-fixed, not extensible, so OCP does not apply there.
- **Example**: a future cloud backend means writing `ClaudeChatClient : ILlmClient` plus a config change — zero lines change in `AgentLoop.cs`.

**L — Liskov Substitution**
- **Rule**: every `ILlmClient` implementation, current and future, must map backend-specific failures onto the same shared types (e.g., `LlmUnreachableException`, `ChatResponse`) so `AgentLoop`'s single error-handling path never branches on implementation type.
- **Example**: `LmStudioChatClient` must throw `LlmUnreachableException` on connection failure — never a raw `HttpRequestException` — so `AgentLoop`'s one `catch` stays correct for any future adapter too.

**I — Interface Segregation**
- **Rule**: `ILlmClient` exposes exactly one member, `CompleteAsync(ChatRequest) -> Task<ChatResponse>` — no speculative streaming/embedding methods until FUNC-SPEC actually scopes them in.
- **Example**: a future streaming capability gets its own `IStreamingLlmClient`, not an unused method bolted onto `ILlmClient`.

**D — Dependency Inversion**
- **Rule**: `AgentLoop` receives `ILlmClient` and a minimal stdio transport (`Func<Task<string?>> ReadLine`, `Action<string> WriteLine`) via constructor parameters — never calling `Console.*` or `new HttpClient()` directly. Scoped narrowly: this does *not* extend to the tool handlers — `read_file`/`write_file`/`edit_file`/`bash` call `System.IO.File`/`System.Diagnostics.Process` directly, since FUNC-SPEC's validation criteria test them against a real temp directory, not mocks.
- **Example**: the fake-orchestrator integration test (Validation Criterion #11) constructs `new AgentLoop(mockLlmClient, fakeReadLine, fakeWriteLine)` and asserts on captured output with zero real stdio involved.

## 4. Testing Strategy

**Test Project Structure**: single xUnit v3 4.0.0 project `xDreamer.Agent.Tests` (per §1). Unit and integration tests are collocated but tagged `[Trait("Category", "Unit"|"Integration")]` so CI can report/filter each independently. No separate e2e project.

**Mocking Boundaries** (consistent with the DIP scoping fixed in §3):
- **LLM backend**: `System.Net.HttpListener`-based in-process mock server implementing `POST /v1/chat/completions` (per §1), one instance per test class via `IClassFixture<T>`, disposed after. No real LM Studio process in any automated test.
- **stdio transport**: `AgentLoop` is constructed directly with in-memory `Func<Task<string?>>`/`Action<string>` delegates — tests never touch real `Console.In`/`Console.Out`.
- **Filesystem**: real `Directory.CreateTempSubdirectory()` per test, cleaned up in `IDisposable.Dispose()` — never mocked.
- **Process execution (`bash` tool)**: real `System.Diagnostics.Process`, invoking only deterministic, cross-platform-safe commands — never mocked.
- **Assertions**: xUnit v3's built-in `Assert` only — no FluentAssertions/Shouldly, consistent with the zero-extra-dependency stance in §1.

**Coverage Boundaries — mapped to FUNC-SPEC §3 Validation Criteria**

| # | Validation Criterion | Test Type |
|---|---|---|
| 1 | NDJSON reassembly across partial stdin reads | Unit — transport/reader class only |
| 2 | Malformed first message → immediate failure | Unit — AgentLoop, fake transport |
| 3 | `read_file` never blocks | Unit — dispatch logic, fake transport |
| 4 | Gated tools: zero side effects before `permission_response` | Unit — fake transport + real temp dir, assert file untouched pre-response |
| 5 | Distinct ids per call; unmatched id not applied | Unit — permission-correlation logic |
| 6 | Denial → `tool_result(permission_denied)`, loop continues | Unit — AgentLoop, fake transport |
| 7 | `max_turns` exceeded → `task_complete` failure | Unit — AgentLoop turn-counter |
| 8 | Token estimate over `context_limit_tokens` → fail fast | Unit — `TokenEstimator` pure function (boundary cases) + AgentLoop wiring |
| 9 | LM Studio unreachable → immediate failure, zero retries | Integration — HttpListener mock refusing/closing connection |
| 10 | Mock server request-shape + `tool_calls` handling | Integration — HttpListener mock, assert captured request body |
| 11 | Full fake-orchestrator sequence, all 4 tools + gate | Integration — the one true end-to-end test: mock LLM + real temp dir + real process exec, fake stdio |
| 12 | `base_url` swap to 2nd mock server, zero code change | Integration — `[Theory]` over two `IClassFixture` mock-server instances, same test body |

**Tool-Level Error Code Coverage** (gap identified in §6 audit — FUNC-SPEC §2's Tool I/O Contracts table defines these independently of the 12 numbered Validation Criteria above; each needs its own explicit test case)

| Tool | Error Code | Test Type |
|---|---|---|
| `read_file` | `not_found`, `is_directory`, `read_error` | Unit — real temp dir |
| `write_file` | `write_error`, `path_outside_cwd` | Unit — real temp dir; `path_outside_cwd` exercises `PathGuard` (§5) |
| `edit_file` | `old_string_not_found`, `old_string_not_unique`, `write_error`, `path_outside_cwd` | Unit — real temp dir; `path_outside_cwd` exercises `PathGuard` (§5) |
| `bash` | `spawn_error` | Unit — invalid command |
| `bash` | timeout (resolved, §6 audit) | Unit — `success:true, output:{timed_out:true}`; `error.code:"timeout"` is not exercised — treated as unreachable in favor of the `output.timed_out` field, consistent with how `exit_code` already carries command outcome without signaling tool failure |

**Token Estimator Testability**: `TokenEstimator.Estimate(string) -> int` is a standalone pure function, unit-tested at the exact-limit and one-over-limit boundaries. The estimation heuristic itself (e.g., chars/4) is an implementation detail decided during build, not fixed here — only its testability contract (pure, injectable, boundary-tested) is.

**Coverage Target**: minimum **90% line coverage** on domain logic (`AgentLoop`, tool handlers, `ILlmClient`/`LmStudioChatClient`, NDJSON transport, `TokenEstimator`) — excludes `Program.cs` wiring/composition root.

**CI Wiring**:
- `dotnet test` runs the full suite (unit + integration) on every push/PR — no separate integration-only stage; all I/O is either in-process (HttpListener, temp dir) or trivial local process spawns, so total runtime stays small.
- `coverlet.collector` (test-only NuGet package, never ships) added via `dotnet test --collect:"XPlat Code Coverage"` for cobertura-format coverage reports. Covered by the existing `dotnet list package --vulnerable --include-transitive` gate (§1).
- A CI step enforces the 90% coverage target as a build-failing gate — a coverage-report tool (e.g. `reportgenerator`) parses the cobertura XML from `coverlet.collector` and fails the build below threshold, matching the rigor of the automated CVE gate in §1 (gap identified in §6 audit).

**Manual (non-CI) Smoke Test**: a checked-in script/instructions (`scripts/smoke-lmstudio.*`) for a human to run locally against a real LM Studio instance before a release — exercises the same task flow as Criterion #11 but against the real backend. Not part of automated CI; catches drift between the OpenAI-compatible schema assumed here and LM Studio's actual behavior. The script invokes the `dotnet publish`-produced Native AOT single-file executable directly (not `dotnet run`) — trimming-related reflection failures only surface at that boundary, and no automated test exercises the published artifact (gap identified in §6 audit).

**Out of Scope for v1** (mirrors FUNC-SPEC's own Out-of-Scope list):
- No load/performance testing — single task per process, no concurrent-request surface to stress.
- No mutation testing / property-based testing framework (e.g., FsCheck) — the 12 validation criteria are concrete enough for example-based tests; a PBT library would be a new dependency without a concrete problem it solves.

## 5. System Topology & File Structure

**Project Split**: single executable project (`src/xDreamer.Agent/`) — no separate Core-library/Cli-host split. `dotnet publish` targets one AOT single-file output (§1); boundaries defined in §3 (e.g., `AgentLoop` depending on `ILlmClient` rather than `LmStudioChatClient`) are enforced by convention and code review, not a compiler-checked project reference, since the codebase is small (~12–15 files).

**Directory Shape**: component subfolders (`Llm/`, `Tools/`, `Transport/`, `Messages/`) under `src/xDreamer.Agent/`, matching the component boundaries already named in §2–§3.

**Test Layout**: `tests/xDreamer.Agent.Tests/` mirrors `src/xDreamer.Agent/` folder-for-folder. Integration tests sit beside their unit-test siblings in the same folder, distinguished only by `[Trait("Category","Integration")]` and an `*IntegrationTests.cs` filename suffix — no separate top-level `Integration/` folder.

**Tree Structure**:
```text
/
├── global.json                      # SDK feature-band pin (§1)
├── xDreamer.Agent.sln
├── src/
│   └── xDreamer.Agent/
│       ├── xDreamer.Agent.csproj    # net10.0, AOT/trimmed, single-file publish
│       ├── Program.cs               # composition root: wires LmStudioChatClient + stdio into AgentLoop
│       ├── AgentLoop.cs             # loop controller: turns, tool dispatch, permission-gate correlation
│       ├── TokenEstimator.cs        # pure function, §4
│       ├── Messages/
│       │   ├── WireMessages.cs      # task/tool_call/permission_request/permission_response/tool_result/task_complete records
│       │   └── JsonContext.cs       # source-generated JsonSerializerContext
│       ├── Llm/
│       │   ├── ILlmClient.cs
│       │   ├── LmStudioChatClient.cs
│       │   └── ChatModels.cs        # ChatRequest/ChatResponse, LlmUnreachableException
│       ├── Tools/
│       │   ├── ToolDispatch.cs      # switch/dictionary dispatch + gated-tool lookup (§2: no ITool interface)
│       │   ├── ReadFileTool.cs
│       │   ├── WriteFileTool.cs
│       │   ├── EditFileTool.cs
│       │   ├── BashTool.cs
│       │   └── PathGuard.cs         # shared path_outside_cwd containment check (§6 audit) — used by Write/EditFileTool only, not ReadFileTool
│       └── Transport/
│           └── NdjsonStdio.cs       # line framing/reassembly over stdin/stdout
├── tests/
│   └── xDreamer.Agent.Tests/
│       ├── xDreamer.Agent.Tests.csproj # xUnit v3, coverlet.collector (§4)
│       ├── AgentLoopTests.cs            # Unit — Validation Criteria #2,3,5,6,7
│       ├── AgentLoopIntegrationTests.cs # Integration — Validation Criteria #4,11
│       ├── TokenEstimatorTests.cs       # Unit — Validation Criterion #8
│       ├── Messages/
│       │   └── NdjsonStdioTests.cs      # Unit — Validation Criterion #1
│       ├── Llm/
│       │   ├── MockLmStudioServer.cs        # test-only HttpListener fixture, IClassFixture (no src/ equivalent)
│       │   └── LmStudioChatClientTests.cs   # Integration — Validation Criteria #9,10,12
│       └── Tools/
│           ├── ReadFileToolTests.cs
│           ├── WriteFileToolTests.cs
│           ├── EditFileToolTests.cs
│           ├── BashToolTests.cs
│           └── PathGuardTests.cs    # path_outside_cwd containment cases (§6 audit)
├── scripts/
│   └── smoke-lmstudio.ps1           # manual, non-CI e2e smoke test (§4)
├── FUNC-SPEC.md
└── TECH-SPEC.md
```

**Notes**:
- Every `src/xDreamer.Agent/**` file has a direct test counterpart at the same relative path under `tests/xDreamer.Agent.Tests/**`.
- `MockLmStudioServer.cs` is the one test-only fixture with no `src/` equivalent; it lives in `Llm/` since that's the component it fakes.
- In-memory fake stdio delegates (for `AgentLoop` construction in tests) are defined directly in `AgentLoopTests.cs`/`AgentLoopIntegrationTests.cs` — no separate `TestHelpers/` folder, given the small file count.
- `PathGuard.cs`/`PathGuardTests.cs` added per the Step 8 audit (§6): a shared containment check used only by `WriteFileTool`/`EditFileTool`. `read_file` remains intentionally unrestricted — it has no `path_outside_cwd` in its FUNC-SPEC error-code contract, so reads may go outside `cwd` while writes stay sandboxed.

## 6. Spec Audit & Readiness Sign-off

**Scope**: full holistic review of `FUNC-SPEC.md` (Objective, Inputs/Outputs, Core Behaviors §1–§3) cross-checked against `TECH-SPEC.md` §1–§5, hunting for contradictions, missing edge cases, unhandled failure states, security blind spots, and ambiguous requirements.

**Findings & Resolutions**

| # | Severity | Finding | Resolution |
|---|---|---|---|
| 1 | High | `bash` tool's contract listed both `output.timed_out` (implying `success:true`) and `error.code:"timeout"` (implying `success:false`) for the same condition, with no rule for which applies, and no Validation Criterion exercised it. | Resolved: `success:true` + `output.timed_out:true` is authoritative, consistent with how `exit_code` already carries command outcome without signaling tool failure. `error.code:"timeout"` is treated as unreachable. Documented in §4's new Tool-Level Error Code Coverage table. |
| 2 | High | `write_file`/`edit_file`'s `path_outside_cwd` error code had no architectural owner in §2/§3/§5 — no component defined where cwd-containment validation happens. | Resolved: a shared `PathGuard.EnsureWithinCwd(path, cwd)` helper, used only by `WriteFileTool`/`EditFileTool`. `read_file` is confirmed intentionally unrestricted (no `path_outside_cwd` in its contract). Added to §5's topology tree and §4's coverage table. |
| 3 | Medium | A structurally-valid `task` message with missing/invalid `config` fields (e.g. absent `max_turns`) had no defined failure path — FUNC-SPEC only covers a non-`task` first message. | Resolved: folded into the existing `malformed_message` path (`task_id: null`, non-zero exit) — no new error code introduced. **This is a clarification of FUNC-SPEC's own contract; FUNC-SPEC.md itself was not edited under this skill's mutation protocol — recommend a follow-up pass to state this explicitly in FUNC-SPEC §3's Failure Handling list.** |
| 4 | Medium | §4's Coverage Boundaries table mapped only the 12 numbered Validation Criteria; the 9 granular tool-level error codes from FUNC-SPEC §2's Tool I/O Contracts table were not individually required as test cases. | Resolved: added a Tool-Level Error Code Coverage table to §4 enumerating every error code per tool. |
| 5 | Medium | The 90% coverage target (§4) had no automated CI enforcement, unlike the automated CVE gate in §1 — inconsistent rigor. | Resolved: added a CI step to §4 that fails the build below the 90% threshold, parsing `coverlet.collector`'s cobertura output. |
| 6 | Low | No automated test exercises the Native AOT/trimmed *published* binary — all tests run under `dotnet test` (JIT), so trimming-related reflection failures would only surface at release time. | Resolved: extended §4's manual smoke test to invoke the published executable directly, not `dotnet run`. |
| 7 | Low | §1's CVE audit is a dated point-in-time snapshot (2026-08-23); zero-tolerance requires currency at build time. | Resolved: added a re-verify-at-Step-9 note to §1. |
| 8 | Low | `edit_file`'s `replace_all?` default when omitted is unstated in FUNC-SPEC. | Documented assumption only (no spec text changed): default is `false` (replace first/only match), per Validation Criteria's `old_string_not_unique` error existing specifically to catch ambiguous single-replace attempts. |

**Residual Accepted Risk** (not a gap — an explicit FUNC-SPEC decision, flagged for visibility): the `bash` tool executes arbitrary commands with no sandboxing or container isolation (FUNC-SPEC §1 Out of Scope: "trusted local execution assumed"). This CLI's threat model assumes a trusted orchestrator and trusted task input; it is not safe to expose to untrusted task sources without an additional isolation layer outside this spec's scope.

**Cross-Document Note**: findings #1 and #3 resolve ambiguities in FUNC-SPEC's own data contract. This skill's mutation protocol scopes writes to `TECH-SPEC.md` only — the resolutions are recorded here, but `FUNC-SPEC.md` was not modified. A follow-up edit to FUNC-SPEC §2/§3 to reflect these resolutions explicitly is recommended before Step 9 (implementation) begins.

**STATUS: READY**
All identified gaps have documented resolutions; no unresolved contradictions remain between `FUNC-SPEC.md` and `TECH-SPEC.md` §1–§5. Cleared for Step 9 (Implementation Executor), subject to the FUNC-SPEC follow-up edit noted above.

*Audited 2026-08-25.*
