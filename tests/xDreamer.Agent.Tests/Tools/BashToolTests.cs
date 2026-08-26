using xDreamer.Agent.Tools;
using Xunit;

namespace xDreamer.Agent.Tests.Tools;

[Trait("Category", "Unit")]
public class BashToolTests
{
    [Fact]
    public void Execute_InvalidCommand_ReturnsSpawnError()
    {
        // An empty command can never be spawned — distinct from a command that runs but exits nonzero,
        // which is normal command failure (exit_code), not a spawn_error.
        var (output, error) = BashTool.Execute(new BashTool.Input("   ", null, null));

        Assert.Null(output);
        Assert.Equal("spawn_error", error!.Code);
    }

    [Fact]
    public void Execute_CommandExceedsTimeoutMs_ReturnsSuccessTrueWithOutputTimedOutTrue()
    {
        string sleepCommand = OperatingSystem.IsWindows() ? "ping -n 6 127.0.0.1 >nul" : "sleep 5";

        var (output, error) = BashTool.Execute(new BashTool.Input(sleepCommand, null, TimeoutMs: 200));

        Assert.Null(error);
        Assert.NotNull(output);
        Assert.True(output!.TimedOut);
    }

    [Fact]
    public void Execute_SuccessfulCommand_ReturnsStdoutAndZeroExitCode()
    {
        string echoCommand = OperatingSystem.IsWindows() ? "echo hello" : "echo hello";

        var (output, error) = BashTool.Execute(new BashTool.Input(echoCommand, null, null));

        Assert.Null(error);
        Assert.Equal(0, output!.ExitCode);
        Assert.Contains("hello", output.Stdout);
        Assert.False(output.TimedOut);
    }

    [Fact]
    public void Execute_FailingCommand_ReturnsNonZeroExitCode_NotSpawnError()
    {
        string failingCommand = OperatingSystem.IsWindows() ? "exit 3" : "exit 3";

        var (output, error) = BashTool.Execute(new BashTool.Input(failingCommand, null, null));

        Assert.Null(error);
        Assert.Equal(3, output!.ExitCode);
    }

    [Fact]
    public void Execute_WithCwd_RunsInSpecifiedDirectory()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string printCwdCommand = OperatingSystem.IsWindows() ? "cd" : "pwd";

        var (output, error) = BashTool.Execute(new BashTool.Input(printCwdCommand, tempDir.Path, null));

        Assert.Null(error);
        Assert.Contains(Path.GetFileName(tempDir.Path.TrimEnd(Path.DirectorySeparatorChar)), output!.Stdout);
    }

    [Fact]
    public void Execute_CwdDoesNotExist_ReturnsSpawnError()
    {
        string nonexistentCwd = Path.Combine(Path.GetTempPath(), "agent-tests-does-not-exist-" + Guid.NewGuid());

        var (output, error) = BashTool.Execute(new BashTool.Input("echo hi", nonexistentCwd, null));

        Assert.Null(output);
        Assert.Equal("spawn_error", error!.Code);
    }
}
