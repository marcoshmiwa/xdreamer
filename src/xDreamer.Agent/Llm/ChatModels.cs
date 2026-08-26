using System.Text.Json;

namespace xDreamer.Agent.Llm;

/// <summary>Backend-agnostic request to the LLM port. <see cref="ILlmClient"/> implementations translate
/// this to/from their own wire shape (§2 Ports &amp; Adapter) — never leaks a backend-specific schema.</summary>
public sealed record ChatRequest(
    IReadOnlyList<ChatMessage> Messages,
    IReadOnlyList<ToolDefinition> Tools);

public sealed record ChatMessage(
    string Role,
    string? Content,
    IReadOnlyList<RequestedToolCall>? ToolCalls = null,
    string? ToolCallId = null);

/// <summary>One of the four agent tools, described using the OpenAI function-calling schema shape
/// (name/description/JSON-schema parameters) so it maps 1:1 onto a backend function definition.</summary>
public sealed record ToolDefinition(
    string Name,
    string Description,
    JsonElement ParametersSchema);

public sealed record RequestedToolCall(
    string Id,
    string ToolName,
    JsonElement Arguments);

/// <summary>Backend-agnostic response from the LLM port. <see cref="ToolCalls"/> is empty when the
/// response is a final answer (<see cref="Content"/> is then the answer text).</summary>
public sealed record ChatResponse(
    string? Content,
    IReadOnlyList<RequestedToolCall> ToolCalls);

/// <summary>Every <see cref="ILlmClient"/> implementation must map connection failures onto this shared
/// type — never a backend-specific exception (e.g. a raw HttpRequestException) — so AgentLoop's one
/// catch clause stays correct for any future adapter (LSP).</summary>
public sealed class LlmUnreachableException : Exception
{
    public LlmUnreachableException(string message, Exception? innerException = null)
        : base(message, innerException)
    {
    }
}
