using xDreamer.Agent;
using Xunit;

namespace xDreamer.Agent.Tests;

[Trait("Category", "Unit")]
public class TokenEstimatorTests
{
    private const int ContextLimitTokens = 100;

    [Fact]
    public void Estimate_AtExactContextLimit_DoesNotExceedLimit()
    {
        string text = new string('a', ContextLimitTokens * 4);

        int estimated = TokenEstimator.Estimate(text);

        Assert.Equal(ContextLimitTokens, estimated);
        Assert.False(estimated > ContextLimitTokens);
    }

    [Fact]
    public void Estimate_OneTokenOverContextLimit_ExceedsLimit()
    {
        string text = new string('a', (ContextLimitTokens + 1) * 4);

        int estimated = TokenEstimator.Estimate(text);

        Assert.Equal(ContextLimitTokens + 1, estimated);
        Assert.True(estimated > ContextLimitTokens);
    }

    [Fact]
    public void Estimate_EmptyString_ReturnsZero()
    {
        Assert.Equal(0, TokenEstimator.Estimate(string.Empty));
    }
}
