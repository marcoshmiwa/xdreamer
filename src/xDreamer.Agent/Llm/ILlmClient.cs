namespace Agent.Llm;

/// <summary>Port for the LLM backend (§2 Ports &amp; Adapter). Exactly one member — no speculative
/// streaming/embedding methods until FUNC-SPEC actually scopes them in (ISP).</summary>
public interface ILlmClient
{
    Task<ChatResponse> CompleteAsync(ChatRequest request);
}
