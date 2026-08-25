using Agent;
using Xunit;

namespace Agent.Tests;

[Trait("Category", "Unit")]
public class AgentLoopTests
{
    [Fact]
    public void Constructor_UsesInjectedReadLineWriteLineDelegates_NeverTouchesConsole()
    {
        Func<Task<string?>> readLine = () => throw new InvalidOperationException("ReadLine should not be invoked by the constructor");
        Action<string> writeLine = _ => throw new InvalidOperationException("WriteLine should not be invoked by the constructor");

        var exception = Record.Exception(() => new AgentLoop(readLine, writeLine));

        Assert.Null(exception);
    }
}
