namespace xDreamer.Agent;

/// <summary>Pure, standalone token-count estimator used to fail fast before a request would exceed
/// context_limit_tokens (FUNC-SPEC §3 Failure Handling). Heuristic: ~4 characters per token.</summary>
public static class TokenEstimator
{
    private const int CharsPerToken = 4;

    public static int Estimate(string text)
    {
        ArgumentNullException.ThrowIfNull(text);
        if (text.Length == 0)
        {
            return 0;
        }

        return (text.Length + CharsPerToken - 1) / CharsPerToken;
    }
}
