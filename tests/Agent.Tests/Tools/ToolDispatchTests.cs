using System.Text.Json;
using Agent.Tools;
using Xunit;

namespace Agent.Tests.Tools;

[Trait("Category", "Unit")]
public class ToolDispatchTests
{
    [Fact]
    public void Execute_ReadFile_RoundTripsInputAndOutputThroughJsonElement()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(tempDir.Path, "file.txt");
        File.WriteAllText(path, "hello");
        JsonElement input = JsonSerializer.SerializeToElement(new { path });

        var (output, error) = ToolDispatch.Execute("read_file", input, tempDir.Path);

        Assert.Null(error);
        Assert.Equal("hello", output!.Value.GetProperty("content").GetString());
    }

    [Fact]
    public void Execute_WriteFile_PathOutsideCwd_ReturnsPathOutsideCwdViaJsonElement()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        using var otherDir = new ReadFileToolTests.TempDirectory();
        string path = Path.Combine(otherDir.Path, "file.txt");
        JsonElement input = JsonSerializer.SerializeToElement(new { path, content = "x" });

        var (output, error) = ToolDispatch.Execute("write_file", input, tempDir.Path);

        Assert.Null(output);
        Assert.Equal("path_outside_cwd", error!.Code);
    }

    [Fact]
    public void GatedTools_AreExactlyWriteFileEditFileBash()
    {
        Assert.Equal(["write_file", "edit_file", "bash"], ToolDispatch.GatedTools);
        Assert.True(ToolDispatch.IsGated("write_file"));
        Assert.True(ToolDispatch.IsGated("edit_file"));
        Assert.True(ToolDispatch.IsGated("bash"));
    }

    [Fact]
    public void UngatedTools_AreExactlyReadFile()
    {
        Assert.Equal(["read_file"], ToolDispatch.UngatedTools);
        Assert.False(ToolDispatch.IsGated("read_file"));
    }
}
