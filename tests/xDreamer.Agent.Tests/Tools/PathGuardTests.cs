using xDreamer.Agent.Tools;
using Xunit;

namespace xDreamer.Agent.Tests.Tools;

[Trait("Category", "Unit")]
public class PathGuardTests
{
    [Fact]
    public void EnsureWithinCwd_PathInsideCwd_DoesNotThrow()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(tempDir.Path, "subdir", "file.txt");

        var exception = Record.Exception(() => PathGuard.EnsureWithinCwd(path, tempDir.Path));

        Assert.Null(exception);
    }

    [Fact]
    public void EnsureWithinCwd_PathOutsideCwd_Throws()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        using var otherDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(otherDir.Path, "file.txt");

        Assert.Throws<PathOutsideCwdException>(() => PathGuard.EnsureWithinCwd(path, tempDir.Path));
    }

    [Fact]
    public void EnsureWithinCwd_PathWithParentDirectoryTraversal_Throws()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(tempDir.Path, "..", "escaped.txt");

        Assert.Throws<PathOutsideCwdException>(() => PathGuard.EnsureWithinCwd(path, tempDir.Path));
    }
}
