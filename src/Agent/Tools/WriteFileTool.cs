using System.Text;
using Agent.Messages;

namespace Agent.Tools;

/// <summary>Gated tool: writes a file's contents. Calls PathGuard.EnsureWithinCwd before writing (§6 audit
/// finding #2). Calls only File I/O — never constructs a tool_result message, that's AgentLoop's job (SRP).</summary>
public static class WriteFileTool
{
    public sealed record Input(string Path, string Content);

    public sealed record Output(int BytesWritten, bool Created);

    public static (Output? Output, ToolError? Error) Execute(Input input, string cwd)
    {
        try
        {
            PathGuard.EnsureWithinCwd(input.Path, cwd);
        }
        catch (PathOutsideCwdException ex)
        {
            return (null, new ToolError("path_outside_cwd", ex.Message));
        }

        bool created = !File.Exists(input.Path);
        byte[] bytes = Encoding.UTF8.GetBytes(input.Content);

        try
        {
            File.WriteAllBytes(input.Path, bytes);
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
        {
            return (null, new ToolError("write_error", ex.Message));
        }

        return (new Output(bytes.Length, created), null);
    }
}
