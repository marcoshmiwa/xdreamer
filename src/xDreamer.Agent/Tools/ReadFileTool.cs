using System.Text.Json.Serialization;
using xDreamer.Agent.Messages;

namespace xDreamer.Agent.Tools;

/// <summary>Ungated tool: reads a file's contents. Calls only File.Exists/Directory.Exists/File.ReadAllText —
/// never constructs a tool_result message, that's AgentLoop's job alone (SRP). Intentionally unrestricted
/// by PathGuard: read_file's contract has no path_outside_cwd error code (§6 audit finding #2).</summary>
public static class ReadFileTool
{
    public sealed record Input(
        [property: JsonPropertyName("path")] string Path,
        [property: JsonPropertyName("offset")] int? Offset,
        [property: JsonPropertyName("limit")] int? Limit);

    public sealed record Output(
        [property: JsonPropertyName("content")] string Content,
        [property: JsonPropertyName("truncated")] bool Truncated);

    public static (Output? Output, ToolError? Error) Execute(Input input)
    {
        if (Directory.Exists(input.Path))
        {
            return (null, new ToolError("is_directory", $"Path is a directory: {input.Path}"));
        }

        if (!File.Exists(input.Path))
        {
            return (null, new ToolError("not_found", $"File not found: {input.Path}"));
        }

        string[] lines;
        try
        {
            lines = File.ReadAllLines(input.Path);
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
        {
            return (null, new ToolError("read_error", ex.Message));
        }

        int offset = input.Offset.GetValueOrDefault(0);
        if (offset < 0)
        {
            offset = 0;
        }

        int available = Math.Max(0, lines.Length - offset);
        int take = input.Limit.HasValue ? Math.Min(input.Limit.Value, available) : available;
        bool truncated = offset + take < lines.Length;

        string content = string.Join('\n', lines.Skip(offset).Take(take));
        return (new Output(content, truncated), null);
    }
}
