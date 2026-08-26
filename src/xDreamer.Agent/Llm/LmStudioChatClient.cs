using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using xDreamer.Agent.Messages;

namespace xDreamer.Agent.Llm;

/// <summary>Sole <see cref="ILlmClient"/> adapter: non-streaming POST {base_url}/chat/completions against
/// LM Studio's OpenAI-compatible endpoint, using the OpenAI tool-calling schema.</summary>
public sealed partial class LmStudioChatClient : ILlmClient
{
    private readonly HttpClient _httpClient;
    private readonly string _model;

    public LmStudioChatClient(LlmConfig config)
    {
        ArgumentNullException.ThrowIfNull(config);
        if (string.IsNullOrWhiteSpace(config.BaseUrl))
        {
            throw new ArgumentException("config.BaseUrl is required", nameof(config));
        }

        if (string.IsNullOrWhiteSpace(config.Model))
        {
            throw new ArgumentException("config.Model is required", nameof(config));
        }

        string baseUrl = config.BaseUrl.EndsWith('/') ? config.BaseUrl : config.BaseUrl + "/";
        _httpClient = new HttpClient { BaseAddress = new Uri(baseUrl) };
        _model = config.Model;
    }

    public async Task<ChatResponse> CompleteAsync(ChatRequest request)
    {
        ArgumentNullException.ThrowIfNull(request);

        var wireRequest = ToWireRequest(request);
        string requestJson = JsonSerializer.Serialize(wireRequest, LmStudioJsonContext.Default.OpenAiChatRequest);

        HttpResponseMessage response;
        try
        {
            using var content = new StringContent(requestJson, Encoding.UTF8);
            content.Headers.ContentType = new MediaTypeHeaderValue("application/json");
            response = await _httpClient.PostAsync("chat/completions", content).ConfigureAwait(false);
        }
        catch (HttpRequestException ex)
        {
            throw new LlmUnreachableException($"Could not reach LLM backend at {_httpClient.BaseAddress}", ex);
        }
        catch (TaskCanceledException ex)
        {
            throw new LlmUnreachableException($"LLM backend at {_httpClient.BaseAddress} did not respond", ex);
        }

        if (!response.IsSuccessStatusCode)
        {
            throw new LlmUnreachableException(
                $"LLM backend at {_httpClient.BaseAddress} returned HTTP {(int)response.StatusCode}");
        }

        string responseJson = await response.Content.ReadAsStringAsync().ConfigureAwait(false);
        var wireResponse = JsonSerializer.Deserialize(responseJson, LmStudioJsonContext.Default.OpenAiChatCompletionResponse)
            ?? throw new LlmUnreachableException($"LLM backend at {_httpClient.BaseAddress} returned an empty response body");

        return FromWireResponse(wireResponse);
    }

    private OpenAiChatRequest ToWireRequest(ChatRequest request)
    {
        var messages = request.Messages
            .Select(m => new OpenAiMessage(
                Role: m.Role,
                Content: m.Content,
                ToolCalls: m.ToolCalls is null or { Count: 0 }
                    ? null
                    : m.ToolCalls.Select(ToWireToolCall).ToList(),
                ToolCallId: m.ToolCallId))
            .ToList();

        var tools = request.Tools
            .Select(t => new OpenAiTool(
                Type: "function",
                Function: new OpenAiFunctionDef(t.Name, t.Description, t.ParametersSchema)))
            .ToList();

        return new OpenAiChatRequest(_model, messages, tools.Count == 0 ? null : tools);
    }

    private static OpenAiToolCall ToWireToolCall(RequestedToolCall call)
        => new(call.Id, "function", new OpenAiFunctionCall(call.ToolName, call.Arguments.GetRawText()));

    private static ChatResponse FromWireResponse(OpenAiChatCompletionResponse wireResponse)
    {
        OpenAiResponseMessage? message = wireResponse.Choices?.FirstOrDefault()?.Message;
        if (message is null)
        {
            return new ChatResponse(Content: null, ToolCalls: []);
        }

        var toolCalls = (message.ToolCalls ?? [])
            .Select(FromWireToolCall)
            .ToList();

        return new ChatResponse(message.Content, toolCalls);
    }

    private static RequestedToolCall FromWireToolCall(OpenAiToolCall call)
    {
        using JsonDocument document = JsonDocument.Parse(string.IsNullOrEmpty(call.Function.Arguments) ? "{}" : call.Function.Arguments);
        return new RequestedToolCall(call.Id, call.Function.Name, document.RootElement.Clone());
    }

    private sealed record OpenAiChatRequest(
        [property: JsonPropertyName("model")] string Model,
        [property: JsonPropertyName("messages")] List<OpenAiMessage> Messages,
        [property: JsonPropertyName("tools")] List<OpenAiTool>? Tools);

    private sealed record OpenAiMessage(
        [property: JsonPropertyName("role")] string Role,
        [property: JsonPropertyName("content")] string? Content,
        [property: JsonPropertyName("tool_calls")] List<OpenAiToolCall>? ToolCalls,
        [property: JsonPropertyName("tool_call_id")] string? ToolCallId);

    private sealed record OpenAiTool(
        [property: JsonPropertyName("type")] string Type,
        [property: JsonPropertyName("function")] OpenAiFunctionDef Function);

    private sealed record OpenAiFunctionDef(
        [property: JsonPropertyName("name")] string Name,
        [property: JsonPropertyName("description")] string Description,
        [property: JsonPropertyName("parameters")] JsonElement Parameters);

    private sealed record OpenAiToolCall(
        [property: JsonPropertyName("id")] string Id,
        [property: JsonPropertyName("type")] string Type,
        [property: JsonPropertyName("function")] OpenAiFunctionCall Function);

    private sealed record OpenAiFunctionCall(
        [property: JsonPropertyName("name")] string Name,
        [property: JsonPropertyName("arguments")] string Arguments);

    private sealed record OpenAiChatCompletionResponse(
        [property: JsonPropertyName("choices")] List<OpenAiChoice>? Choices);

    private sealed record OpenAiChoice(
        [property: JsonPropertyName("message")] OpenAiResponseMessage? Message);

    private sealed record OpenAiResponseMessage(
        [property: JsonPropertyName("role")] string Role,
        [property: JsonPropertyName("content")] string? Content,
        [property: JsonPropertyName("tool_calls")] List<OpenAiToolCall>? ToolCalls);

    [JsonSourceGenerationOptions(WriteIndented = false)]
    [JsonSerializable(typeof(OpenAiChatRequest))]
    [JsonSerializable(typeof(OpenAiChatCompletionResponse))]
    private partial class LmStudioJsonContext : JsonSerializerContext
    {
    }
}
