using Agent.Tools;
using Xunit;

namespace Agent.Tests.Tools;

[Trait("Category", "Unit")]
public class ReadFileToolTests
{
    [Fact]
    public void Execute_PathDoesNotExist_ReturnsNotFound()
    {
        using var tempDir = new TempDirectory();
        string path = Path.Combine(tempDir.Path, "missing.txt");

        var (output, error) = ReadFileTool.Execute(new ReadFileTool.Input(path, null, null));

        Assert.Null(output);
        Assert.Equal("not_found", error!.Code);
    }

    [Fact]
    public void Execute_PathIsDirectory_ReturnsIsDirectory()
    {
        using var tempDir = new TempDirectory();

        var (output, error) = ReadFileTool.Execute(new ReadFileTool.Input(tempDir.Path, null, null));

        Assert.Null(output);
        Assert.Equal("is_directory", error!.Code);
    }

    [Fact]
    public void Execute_UnreadablePath_ReturnsReadError()
    {
        using var tempDir = new TempDirectory();
        string path = Path.Combine(tempDir.Path, "locked.txt");
        File.WriteAllText(path, "content");
        using var lockStream = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.None);

        var (output, error) = ReadFileTool.Execute(new ReadFileTool.Input(path, null, null));

        Assert.Null(output);
        Assert.Equal("read_error", error!.Code);
    }

    [Fact]
    public void Execute_PathOutsideCwd_DoesNotReturnPathOutsideCwdError()
    {
        using var tempDir = new TempDirectory();
        string path = Path.Combine(tempDir.Path, "elsewhere.txt");
        File.WriteAllText(path, "line one\nline two");

        var (output, error) = ReadFileTool.Execute(new ReadFileTool.Input(path, null, null));

        Assert.Null(error);
        Assert.NotNull(output);
        Assert.Equal("line one\nline two", output!.Content);
    }

    [Fact]
    public void Execute_ExistingFile_ReturnsContentAndNotTruncated()
    {
        using var tempDir = new TempDirectory();
        string path = Path.Combine(tempDir.Path, "file.txt");
        File.WriteAllText(path, "a\nb\nc");

        var (output, error) = ReadFileTool.Execute(new ReadFileTool.Input(path, null, null));

        Assert.Null(error);
        Assert.Equal("a\nb\nc", output!.Content);
        Assert.False(output.Truncated);
    }

    [Fact]
    public void Execute_LimitLessThanLineCount_ReturnsTruncatedTrue()
    {
        using var tempDir = new TempDirectory();
        string path = Path.Combine(tempDir.Path, "file.txt");
        File.WriteAllText(path, "a\nb\nc\nd");

        var (output, error) = ReadFileTool.Execute(new ReadFileTool.Input(path, Offset: 0, Limit: 2));

        Assert.Null(error);
        Assert.Equal("a\nb", output!.Content);
        Assert.True(output.Truncated);
    }

    [Fact]
    public void Execute_NegativeOffset_ClampsToZero()
    {
        using var tempDir = new TempDirectory();
        string path = Path.Combine(tempDir.Path, "file.txt");
        File.WriteAllText(path, "a\nb\nc");

        var (output, error) = ReadFileTool.Execute(new ReadFileTool.Input(path, Offset: -5, Limit: null));

        Assert.Null(error);
        Assert.Equal("a\nb\nc", output!.Content);
    }

    /// <summary>Real Directory.CreateTempSubdirectory() per test, cleaned up in Dispose — never mocked
    /// (TECH-SPEC §4 Mocking Boundaries). Shared internally across the Tools test files.</summary>
    internal sealed class TempDirectory : IDisposable
    {
        private readonly DirectoryInfo _directory = Directory.CreateTempSubdirectory("agent-tests-");

        public string Path => _directory.FullName;

        public void Dispose()
        {
            try
            {
                _directory.Delete(recursive: true);
            }
            catch (IOException)
            {
            }
        }
    }
}
