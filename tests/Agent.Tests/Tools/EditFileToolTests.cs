using Agent.Tools;
using Xunit;

namespace Agent.Tests.Tools;

[Trait("Category", "Unit")]
public class EditFileToolTests
{
    [Fact]
    public void Execute_OldStringNotFound_ReturnsOldStringNotFound()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(tempDir.Path, "file.txt");
        File.WriteAllText(path, "hello world");

        var (output, error) = EditFileTool.Execute(new EditFileTool.Input(path, "not present", "x", null), tempDir.Path);

        Assert.Null(output);
        Assert.Equal("old_string_not_found", error!.Code);
    }

    [Fact]
    public void Execute_OldStringNotUnique_ReturnsOldStringNotUnique()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(tempDir.Path, "file.txt");
        File.WriteAllText(path, "foo bar foo");

        var (output, error) = EditFileTool.Execute(new EditFileTool.Input(path, "foo", "baz", null), tempDir.Path);

        Assert.Null(output);
        Assert.Equal("old_string_not_unique", error!.Code);
        Assert.Equal("foo bar foo", File.ReadAllText(path));
    }

    [Fact]
    public void Execute_WriteFails_ReturnsWriteError()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(tempDir.Path, "file.txt");
        File.WriteAllText(path, "the old value here");
        using var lockStream = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read);

        var (output, error) = EditFileTool.Execute(new EditFileTool.Input(path, "old value", "new value", null), tempDir.Path);

        Assert.Null(output);
        Assert.Equal("write_error", error!.Code);
    }

    [Fact]
    public void Execute_PathOutsideCwd_ReturnsPathOutsideCwd()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        using var otherDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(otherDir.Path, "file.txt");
        File.WriteAllText(path, "hello world");

        var (output, error) = EditFileTool.Execute(new EditFileTool.Input(path, "hello", "goodbye", null), tempDir.Path);

        Assert.Null(output);
        Assert.Equal("path_outside_cwd", error!.Code);
        Assert.Equal("hello world", File.ReadAllText(path));
    }

    [Fact]
    public void Execute_ReplaceAllOmitted_DefaultsToFalse_ReplacesFirstOccurrenceOnly()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(tempDir.Path, "file.txt");
        File.WriteAllText(path, "hello world");

        var (output, error) = EditFileTool.Execute(new EditFileTool.Input(path, "hello", "goodbye", null), tempDir.Path);

        Assert.Null(error);
        Assert.Equal(1, output!.ReplacementsMade);
        Assert.Equal("goodbye world", File.ReadAllText(path));
    }

    [Fact]
    public void Execute_ReplaceAllTrue_ReplacesEveryOccurrence()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(tempDir.Path, "file.txt");
        File.WriteAllText(path, "foo bar foo");

        var (output, error) = EditFileTool.Execute(new EditFileTool.Input(path, "foo", "baz", true), tempDir.Path);

        Assert.Null(error);
        Assert.Equal(2, output!.ReplacementsMade);
        Assert.Equal("baz bar baz", File.ReadAllText(path));
    }
}
