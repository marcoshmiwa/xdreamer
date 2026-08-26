using xDreamer.Agent.Tools;
using Xunit;

namespace xDreamer.Agent.Tests.Tools;

[Trait("Category", "Unit")]
public class WriteFileToolTests
{
    [Fact]
    public void Execute_WriteFails_ReturnsWriteError()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(tempDir.Path, "missing-subdir", "file.txt");

        var (output, error) = WriteFileTool.Execute(new WriteFileTool.Input(path, "content"), tempDir.Path);

        Assert.Null(output);
        Assert.Equal("write_error", error!.Code);
    }

    [Fact]
    public void Execute_PathOutsideCwd_ReturnsPathOutsideCwd()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        using var otherDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(otherDir.Path, "file.txt");

        var (output, error) = WriteFileTool.Execute(new WriteFileTool.Input(path, "content"), tempDir.Path);

        Assert.Null(output);
        Assert.Equal("path_outside_cwd", error!.Code);
        Assert.False(File.Exists(path));
    }

    [Fact]
    public void Execute_NewFile_WritesContentAndReturnsCreatedTrue()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(tempDir.Path, "new.txt");

        var (output, error) = WriteFileTool.Execute(new WriteFileTool.Input(path, "hello"), tempDir.Path);

        Assert.Null(error);
        Assert.True(output!.Created);
        Assert.Equal(5, output.BytesWritten);
        Assert.Equal("hello", File.ReadAllText(path));
    }

    [Fact]
    public void Execute_ExistingFile_OverwritesAndReturnsCreatedFalse()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(tempDir.Path, "existing.txt");
        File.WriteAllText(path, "old");

        var (output, error) = WriteFileTool.Execute(new WriteFileTool.Input(path, "new"), tempDir.Path);

        Assert.Null(error);
        Assert.False(output!.Created);
        Assert.Equal("new", File.ReadAllText(path));
    }
}
