using System.Text.Json;
using System.Text.Json.Serialization;

namespace Agent.Messages;

/// <summary>First message only (orchestrator to agent); starts the loop.</summary>
public sealed record TaskMessage(
    [property: JsonPropertyName("task_id")] string TaskId,
    [property: JsonPropertyName("instructions")] string Instructions,
    [property: JsonPropertyName("cwd")] string Cwd,
    [property: JsonPropertyName("config")] TaskConfig? Config)
{
    [JsonPropertyName("type")]
    public string Type { get; init; } = "task";
}

public sealed record TaskConfig(
    [property: JsonPropertyName("llm")] LlmConfig? Llm,
    [property: JsonPropertyName("max_turns")] int? MaxTurns,
    [property: JsonPropertyName("context_limit_tokens")] int? ContextLimitTokens);

public sealed record LlmConfig(
    [property: JsonPropertyName("base_url")] string? BaseUrl,
    [property: JsonPropertyName("model")] string? Model,
    [property: JsonPropertyName("temperature")] double? Temperature);

/// <summary>Agent to orchestrator; announces an ungated (read_file) call. Informational, non-blocking.</summary>
public sealed record ToolCallMessage(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("tool")] string Tool,
    [property: JsonPropertyName("input")] JsonElement Input)
{
    [JsonPropertyName("type")]
    public string Type { get; init; } = "tool_call";
}

/// <summary>Agent to orchestrator; announces a gated (write_file/edit_file/bash) call. Blocks execution.</summary>
public sealed record PermissionRequestMessage(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("tool")] string Tool,
    [property: JsonPropertyName("input")] JsonElement Input)
{
    [JsonPropertyName("type")]
    public string Type { get; init; } = "permission_request";
}

/// <summary>Orchestrator to agent; answers a permission_request, correlated by Id.</summary>
public sealed record PermissionResponseMessage(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("decision")] string Decision,
    [property: JsonPropertyName("reason")] string? Reason)
{
    [JsonPropertyName("type")]
    public string Type { get; init; } = "permission_response";
}

/// <summary>Agent to orchestrator; outcome of any tool call, gated or not.</summary>
public sealed record ToolResultMessage(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("tool")] string Tool,
    [property: JsonPropertyName("success")] bool Success,
    [property: JsonPropertyName("output")] JsonElement? Output,
    [property: JsonPropertyName("error")] ToolError? Error)
{
    [JsonPropertyName("type")]
    public string Type { get; init; } = "tool_result";
}

public sealed record ToolError(
    [property: JsonPropertyName("code")] string Code,
    [property: JsonPropertyName("message")] string Message);

/// <summary>Agent to orchestrator; terminal message — success or structured failure.</summary>
public sealed record TaskCompleteMessage(
    [property: JsonPropertyName("task_id")] string? TaskId,
    [property: JsonPropertyName("result")] string Result,
    [property: JsonPropertyName("summary")] string? Summary,
    [property: JsonPropertyName("error")] TaskCompleteError? Error)
{
    [JsonPropertyName("type")]
    public string Type { get; init; } = "task_complete";
}

public sealed record TaskCompleteError(
    [property: JsonPropertyName("code")] string Code,
    [property: JsonPropertyName("message")] string Message);
