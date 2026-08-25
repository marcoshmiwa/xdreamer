using System.Text.Json;
using Agent;
using Agent.Llm;
using Agent.Messages;
using Xunit;

namespace Agent.Tests;

[Trait("Category", "Unit")]
public class AgentLoopTests
{
    [Fact]
    public void Constructor_UsesInjectedReadLineWriteLineDelegates_NeverTouchesConsole()
    {
        ILlmClient llmClient = new ThrowingLlmClient();
        Func<Task<string?>> readLine = () => throw new InvalidOperationException("ReadLine should not be invoked by the constructor");
        Action<string> writeLine = _ => throw new InvalidOperationException("WriteLine should not be invoked by the constructor");

        var exception = Record.Exception(() => new AgentLoop(llmClient, readLine, writeLine));

        Assert.Null(exception);
    }

    [Fact]
    public async Task Run_FirstMessageNotTaskType_EmitsTaskCompleteFailureMalformedMessage()
    {
        var stdio = new ScriptedStdio("""{"type":"permission_response","id":"x","decision":"allow"}""");
        var loop = new AgentLoop(new ThrowingLlmClient(), stdio.ReadLine, stdio.WriteLine);

        await loop.RunAsync();

        TaskCompleteMessage complete = stdio.LastAs(JsonContext.Default.TaskCompleteMessage);
        Assert.Equal("failure", complete.Result);
        Assert.Equal("malformed_message", complete.Error!.Code);
        Assert.Null(complete.TaskId);
    }

    [Fact]
    public async Task Run_FirstMessageNotTaskType_ExitsNonZero()
    {
        var stdio = new ScriptedStdio("""{"type":"permission_response","id":"x","decision":"allow"}""");
        var loop = new AgentLoop(new ThrowingLlmClient(), stdio.ReadLine, stdio.WriteLine);

        int exitCode = await loop.RunAsync();

        Assert.NotEqual(0, exitCode);
    }

    [Fact]
    public async Task Run_TaskMessageMissingRequiredConfigField_EmitsTaskCompleteFailureMalformedMessage()
    {
        // Missing max_turns and context_limit_tokens.
        var stdio = new ScriptedStdio(
            """{"type":"task","task_id":"t1","instructions":"do stuff","cwd":"C:\\repo","config":{"llm":{"base_url":"http://localhost:1234/v1","model":"m"}}}""");
        var loop = new AgentLoop(new ThrowingLlmClient(), stdio.ReadLine, stdio.WriteLine);

        await loop.RunAsync();

        TaskCompleteMessage complete = stdio.LastAs(JsonContext.Default.TaskCompleteMessage);
        Assert.Equal("failure", complete.Result);
        Assert.Equal("malformed_message", complete.Error!.Code);
        Assert.Null(complete.TaskId);
    }

    [Fact]
    public async Task DispatchMultipleToolCallsInOneTurn_EachGetsDistinctId()
    {
        var stdio = new ScriptedStdio(TaskLine());
        var llm = new ScriptedLlmClient(
            ToolCallResponse(("call_1", "read_file", """{"path":"a.txt"}"""), ("call_2", "read_file", """{"path":"b.txt"}""")),
            FinalResponse("done"));
        var loop = new AgentLoop(llm, stdio.ReadLine, stdio.WriteLine);

        await loop.RunAsync();

        List<ToolCallMessage> toolCalls = stdio.AllAs(JsonContext.Default.ToolCallMessage, "tool_call");
        Assert.Equal(2, toolCalls.Count);
        Assert.Equal(["call_1", "call_2"], toolCalls.Select(m => m.Id));
    }

    [Fact]
    public async Task PermissionResponse_WithUnmatchedId_IsNotAppliedToAnyPendingCall()
    {
        var stdio = new ScriptedStdio(
            TaskLine(),
            """{"type":"permission_response","id":"wrong_id","decision":"allow"}""",
            """{"type":"permission_response","id":"call_A","decision":"deny"}""");
        var llm = new ScriptedLlmClient(
            ToolCallResponse(("call_A", "bash", """{"command":"echo hi"}""")),
            FinalResponse("done"));
        var loop = new AgentLoop(llm, stdio.ReadLine, stdio.WriteLine);

        await loop.RunAsync();

        // If the unmatched "wrong_id" response had incorrectly been applied, the call would have been
        // allowed rather than denied.
        ToolResultMessage result = stdio.AllAs(JsonContext.Default.ToolResultMessage, "tool_result").Single();
        Assert.False(result.Success);
        Assert.Equal("permission_denied", result.Error!.Code);
    }

    [Fact]
    public async Task PermissionResponse_Deny_EmitsToolResultPermissionDenied()
    {
        var stdio = new ScriptedStdio(TaskLine(), """{"type":"permission_response","id":"call_A","decision":"deny"}""");
        var llm = new ScriptedLlmClient(
            ToolCallResponse(("call_A", "bash", """{"command":"echo hi"}""")),
            FinalResponse("done"));
        var loop = new AgentLoop(llm, stdio.ReadLine, stdio.WriteLine);

        await loop.RunAsync();

        ToolResultMessage result = stdio.AllAs(JsonContext.Default.ToolResultMessage, "tool_result").Single();
        Assert.False(result.Success);
        Assert.Equal("permission_denied", result.Error!.Code);
    }

    [Fact]
    public async Task PermissionResponse_Deny_LoopContinuesToNextTurn()
    {
        var stdio = new ScriptedStdio(TaskLine(), """{"type":"permission_response","id":"call_A","decision":"deny"}""");
        var llm = new ScriptedLlmClient(
            ToolCallResponse(("call_A", "bash", """{"command":"echo hi"}""")),
            FinalResponse("done"));
        var loop = new AgentLoop(llm, stdio.ReadLine, stdio.WriteLine);

        int exitCode = await loop.RunAsync();

        Assert.Equal(0, exitCode);
        Assert.Equal(2, llm.Requests.Count);
        TaskCompleteMessage complete = stdio.LastAs(JsonContext.Default.TaskCompleteMessage);
        Assert.Equal("success", complete.Result);
    }

    [Fact]
    public async Task Run_MaxTurnsExceeded_EmitsTaskCompleteFailureMaxTurnsExceeded()
    {
        var stdio = new ScriptedStdio(TaskLine(maxTurns: 1));
        var llm = new ScriptedLlmClient(
            ToolCallResponse(("call_1", "read_file", """{"path":"a.txt"}""")),
            FinalResponse("unreachable, should not be requested"));
        var loop = new AgentLoop(llm, stdio.ReadLine, stdio.WriteLine);

        await loop.RunAsync();

        TaskCompleteMessage complete = stdio.LastAs(JsonContext.Default.TaskCompleteMessage);
        Assert.Equal("failure", complete.Result);
        Assert.Equal("max_turns_exceeded", complete.Error!.Code);
    }

    [Fact]
    public async Task Run_MaxTurnsExceeded_MakesNoFurtherLlmCalls()
    {
        var stdio = new ScriptedStdio(TaskLine(maxTurns: 1));
        var llm = new ScriptedLlmClient(
            ToolCallResponse(("call_1", "read_file", """{"path":"a.txt"}""")),
            FinalResponse("unreachable, should not be requested"));
        var loop = new AgentLoop(llm, stdio.ReadLine, stdio.WriteLine);

        await loop.RunAsync();

        Assert.Single(llm.Requests);
    }

    [Fact]
    public async Task Run_EstimatedTokensExceedContextLimit_EmitsTaskCompleteFailureContextLimitExceeded_BeforeLlmCallIsSent()
    {
        var stdio = new ScriptedStdio(TaskLine(contextLimitTokens: 1));
        var llm = new ScriptedLlmClient(FinalResponse("should never be requested"));
        var loop = new AgentLoop(llm, stdio.ReadLine, stdio.WriteLine);

        await loop.RunAsync();

        TaskCompleteMessage complete = stdio.LastAs(JsonContext.Default.TaskCompleteMessage);
        Assert.Equal("failure", complete.Result);
        Assert.Equal("context_limit_exceeded", complete.Error!.Code);
        Assert.Empty(llm.Requests);
    }

    [Fact]
    public async Task DispatchReadFile_DoesNotAwaitPermissionResponse()
    {
        // Only the task line is queued — if read_file (ungated) incorrectly awaited a permission_response,
        // the next ReadLine call would throw (nothing left to read), failing this test.
        var stdio = new ScriptedStdio(throwIfExhausted: true, TaskLine());
        var llm = new ScriptedLlmClient(
            ToolCallResponse(("call_1", "read_file", """{"path":"a.txt"}""")),
            FinalResponse("done"));
        var loop = new AgentLoop(llm, stdio.ReadLine, stdio.WriteLine);

        int exitCode = await loop.RunAsync();

        Assert.Equal(0, exitCode);
        Assert.Single(stdio.AllAs(JsonContext.Default.ToolCallMessage, "tool_call"));
    }

    private static string TaskLine(int maxTurns = 10, int contextLimitTokens = 8192)
    {
        var message = new TaskMessage(
            TaskId: "task-1",
            Instructions: "do the thing",
            Cwd: Path.GetTempPath(),
            Config: new TaskConfig(
                Llm: new LlmConfig("http://localhost:1234/v1", "test-model", null),
                MaxTurns: maxTurns,
                ContextLimitTokens: contextLimitTokens));
        return JsonSerializer.Serialize(message, JsonContext.Default.TaskMessage);
    }

    private static ChatResponse ToolCallResponse(params (string Id, string Tool, string ArgumentsJson)[] calls)
        => new(
            Content: null,
            ToolCalls: calls.Select(c => new RequestedToolCall(c.Id, c.Tool, JsonDocument.Parse(c.ArgumentsJson).RootElement.Clone())).ToList());

    private static ChatResponse FinalResponse(string summary) => new(summary, ToolCalls: []);

    private sealed class ThrowingLlmClient : ILlmClient
    {
        public Task<ChatResponse> CompleteAsync(ChatRequest request)
            => throw new InvalidOperationException("CompleteAsync should not have been called.");
    }

    private sealed class ScriptedLlmClient(params ChatResponse[] responses) : ILlmClient
    {
        private readonly Queue<ChatResponse> _responses = new(responses);

        public List<ChatRequest> Requests { get; } = [];

        public Task<ChatResponse> CompleteAsync(ChatRequest request)
        {
            Requests.Add(request);
            if (_responses.Count == 0)
            {
                throw new InvalidOperationException("ScriptedLlmClient ran out of scripted responses.");
            }

            return Task.FromResult(_responses.Dequeue());
        }
    }

    /// <summary>In-memory fake stdio: a scripted queue of incoming lines and a capture list of outgoing
    /// lines. Defined directly in this test file per TECH-SPEC §5 (no separate TestHelpers/ folder).</summary>
    private sealed class ScriptedStdio
    {
        private readonly Queue<string?> _incoming;
        private readonly bool _throwIfExhausted;

        public ScriptedStdio(params string?[] incomingLines) : this(throwIfExhausted: false, incomingLines)
        {
        }

        public ScriptedStdio(bool throwIfExhausted, params string?[] incomingLines)
        {
            _incoming = new Queue<string?>(incomingLines);
            _throwIfExhausted = throwIfExhausted;
        }

        public List<string> Written { get; } = [];

        public Task<string?> ReadLine()
        {
            if (_incoming.Count == 0)
            {
                if (_throwIfExhausted)
                {
                    throw new InvalidOperationException("ReadLine called with no more scripted lines — the loop blocked when it should not have.");
                }

                return Task.FromResult<string?>(null);
            }

            return Task.FromResult(_incoming.Dequeue());
        }

        public void WriteLine(string line) => Written.Add(line);

        public T LastAs<T>(System.Text.Json.Serialization.Metadata.JsonTypeInfo<T> typeInfo)
            => JsonSerializer.Deserialize(Written[^1], typeInfo)!;

        public List<T> AllAs<T>(System.Text.Json.Serialization.Metadata.JsonTypeInfo<T> typeInfo, string type)
            => Written
                .Select(line => JsonDocument.Parse(line))
                .Where(doc => doc.RootElement.GetProperty("type").GetString() == type)
                .Select(doc => JsonSerializer.Deserialize(doc.RootElement.GetRawText(), typeInfo)!)
                .ToList();
    }
}
