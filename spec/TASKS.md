# Implementation Tasks

## Task 1: Project scaffolding & solution structure
- **Status:** Done
- **Source:** TECH-SPEC §1 (Runtime & Language, Distribution), §5 (System Topology & File Structure)
- **Subtasks:**
  - [x] 1.1 Create `global.json` pinning the SDK to the `10.0.1xx` feature band, .NET 10 LTS `>= 10.0.11` — TECH-SPEC §1
  - [x] 1.2 Create `Agent.sln` referencing `src/Agent/Agent.csproj` and `tests/Agent.Tests/Agent.Tests.csproj` — TECH-SPEC §5
  - [x] 1.3 Create `src/Agent/Agent.csproj` targeting `net10.0`, configured for self-contained trimmed Native AOT single-file publish — TECH-SPEC §1
  - [x] 1.4 Create `tests/Agent.Tests/Agent.Tests.csproj` referencing xUnit v3 4.0.0 and `coverlet.collector` (test-only) — TECH-SPEC §1, §4
  - [x] 1.5 Create directory shape `src/Agent/{Messages,Llm,Tools,Transport}/`, `tests/Agent.Tests/{Messages,Llm,Tools}/`, `scripts/` — TECH-SPEC §5
- **Tests (Definition of Done):**
  - *(No dedicated automated test in TECH-SPEC §4's Testing Strategy covers scaffolding — DoD is build-tooling verification, not an xUnit test.)*
  - `dotnet build Agent.sln` succeeds with `global.json` pinning the SDK to the `10.0.1xx` feature band — TECH-SPEC §1
  - `dotnet publish src/Agent/Agent.csproj -c Release` succeeds, producing a self-contained, trimmed, Native AOT single-file executable — TECH-SPEC §1
  - `dotnet test` resolves xUnit v3 4.0.0 and `coverlet.collector` in `Agent.Tests.csproj` — TECH-SPEC §1, §4

## Task 2: Wire protocol message types & JSON serialization
- **Status:** Done
- **Source:** FUNC-SPEC §2 (Wire Protocol, Message Schemas), TECH-SPEC §5 (Messages/)
- **Subtasks:**
  - [x] 2.1 Define `task`, `tool_call`, `permission_request`, `permission_response`, `tool_result`, `task_complete` record types in `WireMessages.cs` — FUNC-SPEC §2
  - [x] 2.2 Define source-generated `JsonSerializerContext` in `JsonContext.cs` for all six message types (AOT-safe, no reflection) — TECH-SPEC §1, §5
  - [x] 2.3 Unit test NDJSON message round-trip (de)serialization for all six message types — FUNC-SPEC §2
- **Tests (Definition of Done):**
  - *(Flagged: TECH-SPEC §5's tree draws no dedicated test file for `WireMessages.cs`/`JsonContext.cs` — only `NdjsonStdioTests.cs` appears under `Messages/`. `WireMessagesTests.cs` is proposed here per §5's own note that every `src/Agent/**` file has a direct test counterpart.)*
  - `WireMessagesTests.TaskMessage_RoundTripsThroughJsonContext` — FUNC-SPEC §2
  - `WireMessagesTests.ToolCallMessage_RoundTripsThroughJsonContext` — FUNC-SPEC §2
  - `WireMessagesTests.PermissionRequestMessage_RoundTripsThroughJsonContext` — FUNC-SPEC §2
  - `WireMessagesTests.PermissionResponseMessage_RoundTripsThroughJsonContext` — FUNC-SPEC §2
  - `WireMessagesTests.ToolResultMessage_RoundTripsThroughJsonContext` — FUNC-SPEC §2
  - `WireMessagesTests.TaskCompleteMessage_RoundTripsThroughJsonContext` — FUNC-SPEC §2

## Task 3: NDJSON stdio transport
- **Status:** Done
- **Source:** FUNC-SPEC §2 (Wire Protocol), §3 (Validation Criterion #1); TECH-SPEC §3 (DIP), §5 (Transport/NdjsonStdio.cs)
- **Subtasks:**
  - [x] 3.1 Implement `NdjsonStdio.cs` — line framing/reassembly over stdin/stdout, one JSON object per line — FUNC-SPEC §2
  - [x] 3.2 `AgentLoop` receives stdio via `Func<Task<string?>> ReadLine`/`Action<string> WriteLine` constructor params, never calling `Console.*` directly — TECH-SPEC §3 (DIP)
  - [x] 3.3 Unit test (`NdjsonStdioTests.cs`): NDJSON lines buffered/reassembled correctly across partial stdin reads — FUNC-SPEC §3 Validation Criterion #1, TECH-SPEC §4 row #1
- **Tests (Definition of Done):**
  - `NdjsonStdioTests.ReadLine_ReassemblesMessageSplitAcrossMultiplePartialReads` — Validation Criterion #1, TECH-SPEC §4 row #1
  - `NdjsonStdioTests.ReadLine_ReturnsExactlyOneJsonObjectPerLine` — Validation Criterion #1
  - `AgentLoopTests.Constructor_UsesInjectedReadLineWriteLineDelegates_NeverTouchesConsole` — TECH-SPEC §3 (DIP)

## Task 4: LLM client port & adapter
- **Status:** Done
- **Source:** FUNC-SPEC §2 (LLM Backend Interface); TECH-SPEC §2 (Ports & Adapter), §3 (LSP, ISP), §5 (Llm/)
- **Subtasks:**
  - [x] 4.1 Define `ILlmClient` port — single method `Task<ChatResponse> CompleteAsync(ChatRequest request)` — TECH-SPEC §2, §3 (ISP)
  - [x] 4.2 Define `ChatRequest`/`ChatResponse` plain types and `LlmUnreachableException` in `ChatModels.cs` — TECH-SPEC §2, §3 (LSP)
  - [x] 4.3 Implement `LmStudioChatClient : ILlmClient` — non-streaming POST `{base_url}/chat/completions`, OpenAI tool-calling schema, `base_url`/`model` sourced from `task.config.llm` — FUNC-SPEC §2
  - [x] 4.4 `LmStudioChatClient` maps connection failures to `LlmUnreachableException`, never a raw `HttpRequestException` — TECH-SPEC §3 (LSP)
- **Tests (Definition of Done):**
  - `LmStudioChatClientTests.CompleteAsync_SendsOpenAiCompatibleRequestBody` — Validation Criterion #10, TECH-SPEC §4 row #10
  - `LmStudioChatClientTests.CompleteAsync_ParsesToolCallsFromResponse` — Validation Criterion #10
  - `LmStudioChatClientTests.CompleteAsync_ServerUnreachable_ThrowsLlmUnreachableException` — Validation Criterion #9, TECH-SPEC §3 (LSP)
  - `LmStudioChatClientTests.CompleteAsync_UsesBaseUrlAndModelFromConfig_NoHardcodedEndpoint` — FUNC-SPEC §2

## Task 5: LLM backend test infrastructure & integration tests
- **Status:** Done
- **Source:** TECH-SPEC §4 (Mocking Boundaries, Coverage Boundaries), §5 (tests/Agent.Tests/Llm/)
- **Subtasks:**
  - [x] 5.1 Build `MockLmStudioServer.cs` — `HttpListener`-based in-process mock implementing `POST /v1/chat/completions`, `IClassFixture<T>`, disposed after use — TECH-SPEC §4
  - [x] 5.2 Integration test: LM Studio unreachable → immediate failure, zero retries — FUNC-SPEC §3 Validation Criterion #9, TECH-SPEC §4 row #9
  - [x] 5.3 Integration test: mock server request-shape + `tool_calls` handling — FUNC-SPEC §3 Validation Criterion #10, TECH-SPEC §4 row #10
  - [x] 5.4 Integration test (`[Theory]`): `base_url` swap to a second mock-server instance, zero code change, same test body — FUNC-SPEC §3 Validation Criterion #12, TECH-SPEC §4 row #12
- **Tests (Definition of Done):**
  - `LmStudioChatClientTests.AgentLoop_LlmUnreachable_EmitsTaskCompleteFailureLlmUnreachable_ZeroRetries` — Validation Criterion #9, TECH-SPEC §4 row #9
  - `LmStudioChatClientTests.CompleteAsync_SendsOpenAiCompatibleRequestBody` — Validation Criterion #10, TECH-SPEC §4 row #10 (proves the `MockLmStudioServer` fixture)
  - `LmStudioChatClientTests.CompleteAsync_AgainstEitherMockServerInstance_ProducesSameResult` (`[Theory]` over two `IClassFixture<MockLmStudioServer>` instances) — Validation Criterion #12, TECH-SPEC §4 row #12

## Task 6: Token estimator
- **Status:** Done
- **Source:** FUNC-SPEC §3 (Failure Handling — context_limit_exceeded); TECH-SPEC §4 (Token Estimator Testability), §5 (TokenEstimator.cs)
- **Subtasks:**
  - [x] 6.1 Implement `TokenEstimator.Estimate(string) -> int` as a standalone pure function — TECH-SPEC §4
  - [x] 6.2 Unit test at exact-limit and one-over-limit boundaries — FUNC-SPEC §3 Validation Criterion #8, TECH-SPEC §4 row #8
- **Tests (Definition of Done):**
  - `TokenEstimatorTests.Estimate_AtExactContextLimit_DoesNotExceedLimit` — Validation Criterion #8, TECH-SPEC §4 row #8
  - `TokenEstimatorTests.Estimate_OneTokenOverContextLimit_ExceedsLimit` — Validation Criterion #8

## Task 7: read_file tool
- **Status:** Done
- **Source:** FUNC-SPEC §2 (Tool I/O Contracts); TECH-SPEC §3 (SRP example), §5 (Tools/ReadFileTool.cs), §4 (Tool-Level Error Code Coverage)
- **Subtasks:**
  - [x] 7.1 Implement `ReadFileTool` — input `path, offset?, limit?`; output `content, truncated`; calls only `File.ReadAllText`/`File.Exists`, never constructs a `tool_result` message — FUNC-SPEC §2, TECH-SPEC §3 (SRP)
  - [x] 7.2 Confirm `read_file` is ungated and unrestricted by `PathGuard` (no `path_outside_cwd` in its contract) — FUNC-SPEC §2, TECH-SPEC §5
  - [x] 7.3 Unit test error codes `not_found`, `is_directory`, `read_error` against a real temp dir — FUNC-SPEC §2, TECH-SPEC §4
- **Tests (Definition of Done):**
  - `ReadFileToolTests.Execute_PathDoesNotExist_ReturnsNotFound` — FUNC-SPEC §2, TECH-SPEC §4 Tool-Level Error Code Coverage
  - `ReadFileToolTests.Execute_PathIsDirectory_ReturnsIsDirectory` — same
  - `ReadFileToolTests.Execute_UnreadablePath_ReturnsReadError` — same
  - `AgentLoopTests.DispatchReadFile_DoesNotAwaitPermissionResponse` — Validation Criterion #3
  - *(Supplementary, not tied to a numbered criterion)* `ReadFileToolTests.Execute_PathOutsideCwd_DoesNotReturnPathOutsideCwdError` — regression guard for the intentionally-unrestricted contract: FUNC-SPEC §2 Tool I/O Contracts omits `path_outside_cwd` for `read_file`; TECH-SPEC §5 note

## Task 8: PathGuard shared containment check
- **Status:** Done
- **Source:** TECH-SPEC §6 audit finding #2, §5 (Tools/PathGuard.cs)
- **Subtasks:**
  - [x] 8.1 Implement `PathGuard.EnsureWithinCwd(path, cwd)` shared helper, used only by `WriteFileTool`/`EditFileTool` — TECH-SPEC §6 finding #2, §5
  - [x] 8.2 Unit test (`PathGuardTests.cs`) — containment cases: inside cwd, outside cwd, `..` traversal edge cases — TECH-SPEC §5, §4
- **Tests (Definition of Done):**
  - `PathGuardTests.EnsureWithinCwd_PathInsideCwd_DoesNotThrow` — TECH-SPEC §6 finding #2, §5
  - `PathGuardTests.EnsureWithinCwd_PathOutsideCwd_Throws` — same
  - `PathGuardTests.EnsureWithinCwd_PathWithParentDirectoryTraversal_Throws` — same

## Task 9: write_file tool
- **Status:** Not Started
- **Source:** FUNC-SPEC §2 (Tool I/O Contracts — write_file), §3 (Validation Criterion #4); TECH-SPEC §6 finding #2, §5 (Tools/WriteFileTool.cs)
- **Subtasks:**
  - [ ] 9.1 Implement `WriteFileTool` — input `path, content`; output `bytes_written, created`; calls `PathGuard.EnsureWithinCwd` before writing — FUNC-SPEC §2, TECH-SPEC §6 finding #2
  - [ ] 9.2 Unit test error codes `write_error`, `path_outside_cwd` against a real temp dir — FUNC-SPEC §2, TECH-SPEC §4
  - [ ] 9.3 Unit test: gated tool produces zero side effects before matching `permission_response` arrives — FUNC-SPEC §3 Validation Criterion #4, TECH-SPEC §4 row #4
- **Tests (Definition of Done):**
  - `WriteFileToolTests.Execute_WriteFails_ReturnsWriteError` — FUNC-SPEC §2, TECH-SPEC §4
  - `WriteFileToolTests.Execute_PathOutsideCwd_ReturnsPathOutsideCwd` — same, exercises `PathGuard` (Task 8)
  - `AgentLoopIntegrationTests.WriteFile_NoFileWrittenBeforePermissionResponseReceived` — Validation Criterion #4, TECH-SPEC §4 row #4 (authored under Task 15's `AgentLoopIntegrationTests.cs`)

## Task 10: edit_file tool
- **Status:** Not Started
- **Source:** FUNC-SPEC §2 (Tool I/O Contracts — edit_file); TECH-SPEC §6 finding #8, §5 (Tools/EditFileTool.cs)
- **Subtasks:**
  - [ ] 10.1 Implement `EditFileTool` — input `path, old_string, new_string, replace_all?` (default `false` per TECH-SPEC §6 finding #8); output `replacements_made`; calls `PathGuard.EnsureWithinCwd` — FUNC-SPEC §2
  - [ ] 10.2 Unit test error codes `old_string_not_found`, `old_string_not_unique`, `write_error`, `path_outside_cwd` against a real temp dir — FUNC-SPEC §2, TECH-SPEC §4
  - [ ] 10.3 Unit test: `replace_all` omitted defaults to `false` (first/only match); ambiguous single-replace attempt triggers `old_string_not_unique` — TECH-SPEC §6 finding #8
- **Tests (Definition of Done):**
  - `EditFileToolTests.Execute_OldStringNotFound_ReturnsOldStringNotFound` — FUNC-SPEC §2, TECH-SPEC §4
  - `EditFileToolTests.Execute_OldStringNotUnique_ReturnsOldStringNotUnique` — same
  - `EditFileToolTests.Execute_WriteFails_ReturnsWriteError` — same
  - `EditFileToolTests.Execute_PathOutsideCwd_ReturnsPathOutsideCwd` — same, exercises `PathGuard` (Task 8)
  - `EditFileToolTests.Execute_ReplaceAllOmitted_DefaultsToFalse_ReplacesFirstOccurrenceOnly` — TECH-SPEC §6 finding #8

## Task 11: bash tool
- **Status:** Not Started
- **Source:** FUNC-SPEC §2 (Tool I/O Contracts — bash); TECH-SPEC §6 finding #1, §3 (DIP scoping), §5 (Tools/BashTool.cs)
- **Subtasks:**
  - [ ] 11.1 Implement `BashTool` — input `command, cwd?, timeout_ms?`; output `stdout, stderr, exit_code, timed_out`; calls `System.Diagnostics.Process` directly (no interface abstraction) — FUNC-SPEC §2, TECH-SPEC §3 (DIP scoping)
  - [ ] 11.2 On timeout, resolve as `success:true, output.timed_out:true` — never `error.code:"timeout"` — TECH-SPEC §6 finding #1
  - [ ] 11.3 Unit test error code `spawn_error` on invalid command — TECH-SPEC §4
  - [ ] 11.4 Unit test timeout behavior: `success:true, output:{timed_out:true}` — TECH-SPEC §6 finding #1, §4
- **Tests (Definition of Done):**
  - `BashToolTests.Execute_InvalidCommand_ReturnsSpawnError` — TECH-SPEC §4 Tool-Level Error Code Coverage
  - `BashToolTests.Execute_CommandExceedsTimeoutMs_ReturnsSuccessTrueWithOutputTimedOutTrue` — TECH-SPEC §6 finding #1, §4

## Task 12: Tool dispatch & gating lookup
- **Status:** Not Started
- **Source:** FUNC-SPEC §3 (Permission Gate); TECH-SPEC §2 (Rejected: Strategy/Command pattern), §5 (Tools/ToolDispatch.cs)
- **Subtasks:**
  - [ ] 12.1 Implement `ToolDispatch` — switch expression / static dictionary mapping tool name → handler function (no `ITool` interface, per TECH-SPEC §2's documented rejection) — TECH-SPEC §2
  - [ ] 12.2 Implement gated-tool static lookup: `read_file` ungated; `write_file`/`edit_file`/`bash` gated — FUNC-SPEC §3 (Permission Gate)
- **Tests (Definition of Done):**
  - *(Flagged: TECH-SPEC §5's tree draws no dedicated test file for `ToolDispatch.cs` under `Tools/`. `ToolDispatchTests.cs` is proposed on the same basis as Task 2's flag; the gating behavior is also exercised end-to-end by Validation Criterion #11.)*
  - `ToolDispatchTests.GatedTools_AreExactlyWriteFileEditFileBash` — FUNC-SPEC §3 (Permission Gate)
  - `ToolDispatchTests.UngatedTools_AreExactlyReadFile` — same

## Task 13: AgentLoop core orchestration & state machine
- **Status:** Not Started
- **Source:** FUNC-SPEC §3 (Core Behaviors, State Diagram, Failure Handling); TECH-SPEC §3 (SRP, OCP, DIP), §5 (AgentLoop.cs)
- **Subtasks:**
  - [ ] 13.1 Implement `AgentLoop` constructor taking `ILlmClient` + stdio delegates; state machine matches FUNC-SPEC's Mermaid diagram: AwaitingTask → CallingLLM → DispatchingTools → (ExecutingUngated | AwaitingPermission) → Complete/Failed — FUNC-SPEC §3, TECH-SPEC §3 (DIP)
  - [ ] 13.2 Malformed/non-`task` first message → `task_complete failure/malformed_message`, `task_id: null`, non-zero exit — FUNC-SPEC §3 Failure Handling, Validation Criterion #2
  - [ ] 13.3 A structurally-valid `task` message with missing/invalid `config` fields folds into the same `malformed_message` path — TECH-SPEC §6 finding #3
  - [ ] 13.4 Permission correlation: distinct `id` per call within a turn; a `permission_response` with an unmatched `id` is not applied to any pending call — FUNC-SPEC §3, Validation Criterion #5
  - [ ] 13.5 Denial handling: `deny` → `tool_result{success:false, error.code:permission_denied}` fed back to the LLM, loop continues — FUNC-SPEC §3 Denial Behavior, Validation Criterion #6
  - [ ] 13.6 `max_turns` exceeded → `task_complete failure/max_turns_exceeded`, no further LLM calls made — FUNC-SPEC §3 Failure Handling, Validation Criterion #7
  - [ ] 13.7 Pre-call token check: estimated tokens for the next request exceed `context_limit_tokens` → fail fast `context_limit_exceeded` before the LM Studio call is sent — FUNC-SPEC §3 Failure Handling, Validation Criterion #8
  - [ ] 13.8 LM Studio connection error → immediate `task_complete failure/llm_unreachable`, zero retries — FUNC-SPEC §3 Failure Handling, Validation Criterion #9
  - [ ] 13.9 `read_file` calls execute immediately without waiting on the orchestrator (never block) — FUNC-SPEC §3, Validation Criterion #3
  - [ ] 13.10 OCP compliance: `AgentLoop` open to new `ILlmClient` implementations, closed to modification; the tool dispatch switch remains the sole documented OCP exception — TECH-SPEC §3 (OCP)
- **Tests (Definition of Done)** — verified by the aggregate of:
  - `AgentLoopTests.Run_FirstMessageNotTaskType_EmitsTaskCompleteFailureMalformedMessage` and `_ExitsNonZero` — Validation Criterion #2
  - `AgentLoopTests.Run_TaskMessageMissingRequiredConfigField_EmitsTaskCompleteFailureMalformedMessage` — TECH-SPEC §6 finding #3
  - `AgentLoopTests.DispatchMultipleToolCallsInOneTurn_EachGetsDistinctId` — Validation Criterion #5
  - `AgentLoopTests.PermissionResponse_WithUnmatchedId_IsNotAppliedToAnyPendingCall` — Validation Criterion #5
  - `AgentLoopTests.PermissionResponse_Deny_EmitsToolResultPermissionDenied` — Validation Criterion #6
  - `AgentLoopTests.PermissionResponse_Deny_LoopContinuesToNextTurn` — Validation Criterion #6
  - `AgentLoopTests.Run_MaxTurnsExceeded_EmitsTaskCompleteFailureMaxTurnsExceeded` — Validation Criterion #7
  - `AgentLoopTests.Run_MaxTurnsExceeded_MakesNoFurtherLlmCalls` — Validation Criterion #7
  - `AgentLoopTests.Run_EstimatedTokensExceedContextLimit_EmitsTaskCompleteFailureContextLimitExceeded_BeforeLlmCallIsSent` — Validation Criterion #8
  - `LmStudioChatClientTests.AgentLoop_LlmUnreachable_EmitsTaskCompleteFailureLlmUnreachable_ZeroRetries` — Validation Criterion #9
  - `AgentLoopTests.DispatchReadFile_DoesNotAwaitPermissionResponse` — Validation Criterion #3
  - `AgentLoopIntegrationTests.FullTaskLifecycle_AllFourTools_PermissionGate_EndsInTaskComplete` — Validation Criterion #11 (proves the overall state machine and OCP-compliant wiring together)

## Task 14: AgentLoop unit tests
- **Status:** Not Started
- **Source:** TECH-SPEC §4 (Coverage Boundaries table)
- **Subtasks:**
  - [ ] 14.1 `AgentLoopTests.cs` covering Validation Criteria #2, #3, #5, #6, #7 against a fake transport — TECH-SPEC §4 rows #2, #3, #5, #6, #7
- **Tests (Definition of Done):**
  - `AgentLoopTests.cs` exists, tagged `[Trait("Category","Unit")]`, and contains and passes every `AgentLoopTests.*` method listed under Task 13 — TECH-SPEC §4 rows #2, #3, #5, #6, #7

## Task 15: Full end-to-end fake-orchestrator integration test
- **Status:** Not Started
- **Source:** FUNC-SPEC §3 (Validation Criterion #11); TECH-SPEC §3 (DIP example), §4 row #11, §5 (AgentLoopIntegrationTests.cs)
- **Subtasks:**
  - [ ] 15.1 `AgentLoopIntegrationTests.cs`: scripted fake orchestrator drives a full `task → (tool_call|permission_request) → permission_response → tool_result → task_complete` sequence against the mock LLM and a real temp directory, exercising all four tools plus the permission gate — FUNC-SPEC §3 Validation Criterion #11
  - [ ] 15.2 Construct `new AgentLoop(mockLlmClient, fakeReadLine, fakeWriteLine)`, assert on captured output with zero real stdio involved — TECH-SPEC §3 (DIP example)
- **Tests (Definition of Done):**
  - `AgentLoopIntegrationTests.FullTaskLifecycle_AllFourTools_PermissionGate_EndsInTaskComplete` — Validation Criterion #11, TECH-SPEC §4 row #11
  - `AgentLoopIntegrationTests.FullTaskLifecycle_AssertsOnCapturedOutput_NoRealStdioInvolved` — TECH-SPEC §3 (DIP example)

## Task 16: Program.cs composition root
- **Status:** Not Started
- **Source:** FUNC-SPEC §2 (Task Delivery); TECH-SPEC §2 (Wiring), §5 (Program.cs)
- **Subtasks:**
  - [ ] 16.1 `Program.cs` spawns with no task-specific argv, opens stdio and waits for the first `task` line; constructs `new LmStudioChatClient(config.llm)` and wires real `Console.In`/`Console.OpenStandardOutput()` stdio into `AgentLoop` via constructor injection, no DI container — FUNC-SPEC §2 Task Delivery, TECH-SPEC §2 (Wiring)
- **Tests (Definition of Done):**
  - No automated unit test required — `Program.cs` is explicitly excluded from the 90% coverage target (TECH-SPEC §4 Coverage Target)
  - Verified instead by Task 18's manual smoke test (`scripts/smoke-lmstudio.ps1`), which invokes the published executable — i.e., `Program.cs` — end-to-end against a real LM Studio instance — TECH-SPEC §4 Manual Smoke Test

## Task 17: CI & governance
- **Status:** Not Started
- **Source:** TECH-SPEC §1 (Governance, CVE Audit), §4 (CI Wiring)
- **Subtasks:**
  - [ ] 17.1 CI runs `dotnet list package --vulnerable --include-transitive` on every build as the automated zero-tolerance CVE gate — TECH-SPEC §1
  - [ ] 17.2 CI runs `dotnet test` (full unit + integration suite) on every push/PR — TECH-SPEC §4
  - [ ] 17.3 Add `coverlet.collector` to the test project; collect cobertura-format coverage via `dotnet test --collect:"XPlat Code Coverage"` — TECH-SPEC §4
  - [ ] 17.4 CI step enforces the 90% line-coverage target on domain logic (`AgentLoop`, tool handlers, `ILlmClient`/`LmStudioChatClient`, NDJSON transport, `TokenEstimator`; excludes `Program.cs`) via a `reportgenerator`-parsed cobertura gate, build-failing below threshold — TECH-SPEC §4
  - [ ] 17.5 Re-run `dotnet list package --vulnerable --include-transitive` and re-check .NET/xUnit advisories at implementation time before publishing, since §1's CVE audit is a dated point-in-time snapshot (2026-08-23) — TECH-SPEC §1
- **Tests (Definition of Done)** — operational/pipeline verification, not xUnit tests:
  - CI run shows `dotnet list package --vulnerable --include-transitive` passing with zero unpatched critical/high CVEs — TECH-SPEC §1
  - CI run shows `dotnet test` executing the full unit + integration suite with zero failures — TECH-SPEC §4
  - CI run produces a cobertura-format coverage report via `coverlet.collector` — TECH-SPEC §4
  - CI run fails the build when a deliberate dry-run test removal pushes domain-logic coverage below 90% (verified once, then reverted) — TECH-SPEC §4
  - `dotnet list package --vulnerable --include-transitive` re-run at implementation time returns zero unpatched critical/high CVEs, and .NET/xUnit versions are re-checked against current advisories — TECH-SPEC §1

## Task 18: Native AOT publish & manual smoke test
- **Status:** Not Started
- **Source:** TECH-SPEC §1 (Distribution), §4 (Manual (non-CI) Smoke Test)
- **Subtasks:**
  - [ ] 18.1 Configure `dotnet publish` as a self-contained, trimmed, Native AOT single-file executable targeting `net10.0`; document the framework-dependent self-contained fallback if AOT trimming ever conflicts with the JSON source-gen setup — TECH-SPEC §1
  - [ ] 18.2 Create `scripts/smoke-lmstudio.ps1` — manual, non-CI script exercising the Validation Criterion #11 flow against a real LM Studio instance, invoking the `dotnet publish`-produced Native AOT single-file executable directly (not `dotnet run`) — TECH-SPEC §4
- **Tests (Definition of Done):**
  - `dotnet publish` completes with no trimming/reflection warnings or errors, producing a working single-file executable — TECH-SPEC §1
  - Manual run of `scripts/smoke-lmstudio.ps1` against a real LM Studio instance completes a full task successfully, invoking the published binary directly (not `dotnet run`) — TECH-SPEC §4 Manual Smoke Test
  - Not covered by any automated test — explicitly documented as an accepted gap (TECH-SPEC §6 finding #6)

## Task Guardian Audit & Readiness Sign-off

**Scope**: full holistic cross-check of all 18 tasks in this file against `FUNC-SPEC.md` (Objective, Inputs/Outputs, Core Behaviors §1–§3, all 12 Validation Criteria) and `TECH-SPEC.md` (§1–§6, including the full System Topology tree and all 8 audit findings), hunting for coverage gaps, orphaned tasks, invented scope, missing Definitions of Done, unresolved diff-markup, and silent conflicts with `In Progress`/`Done` work.

**Coverage Verification**
- All 12 FUNC-SPEC §3 Validation Criteria trace to at least one task and one named test (Criteria #1–#12 confirmed across Tasks 3, 4, 5, 6, 7, 9, 13, 14, 15).
- Every file in TECH-SPEC §5's topology tree has an owning task, including the two files Step 10 added beyond the tree (`WireMessagesTests.cs` under Task 2, `ToolDispatchTests.cs` under Task 12 — both flagged inline as extrapolations from §5's general test-counterpart note).
- All 8 findings in TECH-SPEC §6's audit table produced a corresponding task action (Tasks 8, 10.1/10.3, 11.2/11.4, 13.3, 17.4, 17.5, 18.2).
- No task reintroduces a pattern TECH-SPEC §2 explicitly rejected (Tasks 12.1 and 16.1 cite the rejections directly as guardrails); no task reaches into FUNC-SPEC's Out-of-Scope list.
- All 18 tasks are `Not Started` and all 18 carry a Step-10 Tests (Definition of Done) subsection — none missing.
- No `[NEW]`/`~~struck-through~~` markup remains in `FUNC-SPEC.md`, `TECH-SPEC.md`, or this file.
- No `In Progress`/`Done` tasks exist yet, so no duplicate-vs-follow-up conflict is possible at this time.

**Findings** (none blocking)

| # | Severity | Finding | Disposition |
|---|---|---|---|
| 1 | Low | TECH-SPEC §4 requires every test file be tagged `[Trait("Category","Unit"\|"Integration")]`, but only Task 14's DoD gates this explicitly (on `AgentLoopTests.cs`). Tasks 5, 7–11, 15 also produce test files without an equivalent explicit trait-tagging line. | Accepted as a convention-level gap, not a coverage gap — easily caught in code review. No task edit made (read-only per this skill's mutation protocol). |
| 2 | Informational | TECH-SPEC §5's prose mandates an `*IntegrationTests.cs` filename suffix for integration tests, but its own tree names `LmStudioChatClientTests.cs` (no suffix) as the home for Criteria #9/10/12's integration tests, while `AgentLoopIntegrationTests.cs` does carry the suffix — a latent self-inconsistency in the already-`READY` TECH-SPEC. | Not a TASKS.md defect — Tasks 4/5 correctly mirror the approved tree as-is. Flagged for a possible future TECH-SPEC cleanup pass, outside this skill's scope. |
| 3 | Informational | TECH-SPEC §6's Cross-Document Note recommends a follow-up edit to FUNC-SPEC §2/§3 to explicitly document the config-validation and bash-timeout resolutions. Still open (carried over from Step 9's sign-off). | Outside TASKS.md's and this skill's mutation scope. No implementation task warranted — it's a documentation-hygiene item, not a functional gap, since TECH-SPEC §6 already resolved the ambiguity in prose. |

**STATUS: READY**
Every requirement traces to a task, every task traces to a requirement, every task has a Definition of Done, and no unresolved coverage gap, orphaned task, or conflict remains. Cleared for Step 12 (Implementation Executor).

*Audited 2026-08-25.*
