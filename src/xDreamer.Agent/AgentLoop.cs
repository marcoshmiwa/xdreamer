using System.Text;
using System.Text.Json;
using Agent.Llm;
using Agent.Messages;
using Agent.Tools;

namespace Agent;

/// <summary>Loop controller: turns, tool dispatch, permission-gate correlation. Depends only on ILlmClient
/// and the stdio delegates — never on Console or HttpClient directly (DIP). Open to new ILlmClient
/// implementations, closed to modification (OCP) — the tool dispatch switch (ToolDispatch) is the sole
/// documented OCP exception, since FUNC-SPEC fixes the tool set at exactly four primitives.</summary>
public sealed class AgentLoop
{
    private static readonly IReadOnlyList<ToolDefinition> ToolDefinitions = BuildToolDefinitions();

    private readonly ILlmClient _llmClient;
    private readonly Func<Task<string?>> _readLine;
    private readonly Action<string> _writeLine;

    public AgentLoop(ILlmClient llmClient, Func<Task<string?>> readLine, Action<string> writeLine)
    {
        _llmClient = llmClient ?? throw new ArgumentNullException(nameof(llmClient));
        _readLine = readLine ?? throw new ArgumentNullException(nameof(readLine));
        _writeLine = writeLine ?? throw new ArgumentNullException(nameof(writeLine));
    }

    /// <summary>Runs the full task lifecycle: AwaitingTask → CallingLLM → DispatchingTools →
    /// (ExecutingUngated | AwaitingPermission) → Complete/Failed. Returns the process exit code
    /// (0 on success, non-zero on any failure path).</summary>
    public async Task<int> RunAsync()
    {
        // AwaitingTask
        string? firstLine = await _readLine().ConfigureAwait(false);
        if (!TryParseValidTask(firstLine, out TaskMessage? task))
        {
            EmitTaskComplete(taskId: null, result: "failure", errorCode: "malformed_message", errorMessage: "First message was not a valid task message.");
            return 1;
        }

        var messages = new List<ChatMessage> { new("user", task!.Instructions) };
        int maxTurns = task.Config!.MaxTurns!.Value;
        int contextLimitTokens = task.Config.ContextLimitTokens!.Value;

        int turn = 0;
        while (true)
        {
            // CallingLLM
            turn++;
            if (turn > maxTurns)
            {
                EmitTaskComplete(task.TaskId, "failure", errorCode: "max_turns_exceeded", errorMessage: $"Exceeded max_turns ({maxTurns}).");
                return 1;
            }

            int estimatedTokens = TokenEstimator.Estimate(EstimateRequestText(messages));
            if (estimatedTokens > contextLimitTokens)
            {
                EmitTaskComplete(task.TaskId, "failure", errorCode: "context_limit_exceeded", errorMessage: $"Estimated {estimatedTokens} tokens exceeds context_limit_tokens ({contextLimitTokens}).");
                return 1;
            }

            ChatResponse response;
            try
            {
                response = await _llmClient.CompleteAsync(new ChatRequest(messages, ToolDefinitions)).ConfigureAwait(false);
            }
            catch (LlmUnreachableException ex)
            {
                EmitTaskComplete(task.TaskId, "failure", errorCode: "llm_unreachable", errorMessage: ex.Message);
                return 1;
            }

            if (response.ToolCalls.Count == 0)
            {
                // Complete
                EmitTaskComplete(task.TaskId, "success", summary: response.Content);
                return 0;
            }

            // DispatchingTools
            messages.Add(new ChatMessage("assistant", response.Content, ToolCalls: response.ToolCalls));

            foreach (RequestedToolCall call in response.ToolCalls)
            {
                (JsonElement? Output, ToolError? Error) result;

                if (!ToolDispatch.IsGated(call.ToolName))
                {
                    // ExecutingUngated: announce (non-blocking) and execute immediately — never waits on the orchestrator.
                    _writeLine(Serialize(new ToolCallMessage(call.Id, call.ToolName, call.Arguments), JsonContext.Default.ToolCallMessage));
                    result = ToolDispatch.Execute(call.ToolName, call.Arguments, task.Cwd);
                }
                else
                {
                    // AwaitingPermission: blocks until a permission_response whose id matches this call arrives.
                    _writeLine(Serialize(new PermissionRequestMessage(call.Id, call.ToolName, call.Arguments), JsonContext.Default.PermissionRequestMessage));
                    PermissionResponseMessage? decision = await AwaitMatchingPermissionResponseAsync(call.Id).ConfigureAwait(false);

                    if (decision is null)
                    {
                        // Orchestrator disconnected while a permission decision was pending.
                        EmitTaskComplete(task.TaskId, "failure", errorCode: "internal_error", errorMessage: "stdin closed while awaiting permission_response.");
                        return 1;
                    }

                    result = decision.Decision == "allow"
                        ? ToolDispatch.Execute(call.ToolName, call.Arguments, task.Cwd)
                        : (null, new ToolError("permission_denied", decision.Reason ?? "Denied by orchestrator."));
                }

                bool success = result.Error is null;
                _writeLine(Serialize(new ToolResultMessage(call.Id, call.ToolName, success, result.Output, result.Error), JsonContext.Default.ToolResultMessage));
                messages.Add(ToolResultToChatMessage(call.Id, result.Output, result.Error));
            }

            // DispatchingTools --> CallingLLM (all calls this turn resolved)
        }
    }

    /// <summary>Reads permission_response messages until one whose id matches expectedId arrives (Validation
    /// Criterion #5) — a response with an unmatched id is silently discarded, not applied to any pending call.
    /// Returns null if stdin closes before a match arrives.</summary>
    private async Task<PermissionResponseMessage?> AwaitMatchingPermissionResponseAsync(string expectedId)
    {
        while (true)
        {
            string? line = await _readLine().ConfigureAwait(false);
            if (line is null)
            {
                return null;
            }

            PermissionResponseMessage? response = TryDeserialize(line, JsonContext.Default.PermissionResponseMessage);
            if (response is not null && response.Type == "permission_response" && response.Id == expectedId)
            {
                return response;
            }
        }
    }

    private void EmitTaskComplete(string? taskId, string result, string? summary = null, string? errorCode = null, string? errorMessage = null)
    {
        TaskCompleteError? error = errorCode is null ? null : new TaskCompleteError(errorCode, errorMessage ?? errorCode);
        _writeLine(Serialize(new TaskCompleteMessage(taskId, result, summary, error), JsonContext.Default.TaskCompleteMessage));
    }

    private static bool TryParseValidTask(string? line, out TaskMessage? task)
    {
        task = null;
        if (line is null)
        {
            return false;
        }

        TaskMessage? parsed = TryDeserialize(line, JsonContext.Default.TaskMessage);
        if (parsed is null || parsed.Type != "task")
        {
            return false;
        }

        if (string.IsNullOrWhiteSpace(parsed.TaskId) || string.IsNullOrWhiteSpace(parsed.Instructions) || string.IsNullOrWhiteSpace(parsed.Cwd))
        {
            return false;
        }

        TaskConfig? config = parsed.Config;
        if (config?.Llm is null
            || string.IsNullOrWhiteSpace(config.Llm.BaseUrl)
            || string.IsNullOrWhiteSpace(config.Llm.Model)
            || config.MaxTurns is null or <= 0
            || config.ContextLimitTokens is null or <= 0)
        {
            return false;
        }

        task = parsed;
        return true;
    }

    private static ChatMessage ToolResultToChatMessage(string toolCallId, JsonElement? output, ToolError? error)
    {
        string content = error is null
            ? output!.Value.GetRawText()
            : $"Error ({error.Code}): {error.Message}";
        return new ChatMessage("tool", content, ToolCallId: toolCallId);
    }

    private static string EstimateRequestText(IReadOnlyList<ChatMessage> messages)
    {
        var builder = new StringBuilder();
        foreach (ChatMessage message in messages)
        {
            builder.Append(message.Role).Append(':').Append(message.Content).Append('\n');
        }

        return builder.ToString();
    }

    private static string Serialize<T>(T value, System.Text.Json.Serialization.Metadata.JsonTypeInfo<T> typeInfo)
        => JsonSerializer.Serialize(value, typeInfo);

    private static T? TryDeserialize<T>(string json, System.Text.Json.Serialization.Metadata.JsonTypeInfo<T> typeInfo)
    {
        try
        {
            return JsonSerializer.Deserialize(json, typeInfo);
        }
        catch (JsonException)
        {
            return default;
        }
    }

    private static IReadOnlyList<ToolDefinition> BuildToolDefinitions() =>
    [
        new ToolDefinition(
            "read_file",
            "Read the contents of a file, optionally starting at a line offset with a line limit.",
            ParseSchema("""{"type":"object","properties":{"path":{"type":"string"},"offset":{"type":"integer"},"limit":{"type":"integer"}},"required":["path"]}""")),
        new ToolDefinition(
            "write_file",
            "Write content to a file within the working directory, creating or overwriting it. Requires permission.",
            ParseSchema("""{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}""")),
        new ToolDefinition(
            "edit_file",
            "Replace an exact substring within a file in the working directory. Requires permission.",
            ParseSchema("""{"type":"object","properties":{"path":{"type":"string"},"old_string":{"type":"string"},"new_string":{"type":"string"},"replace_all":{"type":"boolean"}},"required":["path","old_string","new_string"]}""")),
        new ToolDefinition(
            "bash",
            "Run a shell command. Requires permission.",
            ParseSchema("""{"type":"object","properties":{"command":{"type":"string"},"cwd":{"type":"string"},"timeout_ms":{"type":"integer"}},"required":["command"]}""")),
    ];

    private static JsonElement ParseSchema(string rawJsonSchema) => JsonDocument.Parse(rawJsonSchema).RootElement.Clone();
}
