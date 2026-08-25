using System.Text.Json.Serialization;
using Agent.Messages;

namespace Agent.Tools;

/// <summary>Gated tool: replaces an exact substring within a file. Calls PathGuard.EnsureWithinCwd before
/// writing (§6 audit finding #2). replace_all defaults to false when omitted — an ambiguous single-replace
/// attempt (old_string occurs more than once) is rejected rather than guessed at (§6 audit finding #8).</summary>
public static class EditFileTool
{
    public sealed record Input(
        [property: JsonPropertyName("path")] string Path,
        [property: JsonPropertyName("old_string")] string OldString,
        [property: JsonPropertyName("new_string")] string NewString,
        [property: JsonPropertyName("replace_all")] bool? ReplaceAll);

    public sealed record Output(
        [property: JsonPropertyName("replacements_made")] int ReplacementsMade);

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

        if (!File.Exists(input.Path))
        {
            return (null, new ToolError("old_string_not_found", $"File not found: {input.Path}"));
        }

        string content;
        try
        {
            content = File.ReadAllText(input.Path);
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
        {
            return (null, new ToolError("write_error", ex.Message));
        }

        int occurrences = CountOccurrences(content, input.OldString);
        if (occurrences == 0)
        {
            return (null, new ToolError("old_string_not_found", $"old_string not found in {input.Path}"));
        }

        bool replaceAll = input.ReplaceAll ?? false;
        if (!replaceAll && occurrences > 1)
        {
            return (null, new ToolError(
                "old_string_not_unique",
                $"old_string occurs {occurrences} times in {input.Path}; pass replace_all to replace them all"));
        }

        string newContent = replaceAll
            ? content.Replace(input.OldString, input.NewString, StringComparison.Ordinal)
            : ReplaceFirst(content, input.OldString, input.NewString);
        int replacementsMade = replaceAll ? occurrences : 1;

        try
        {
            File.WriteAllText(input.Path, newContent);
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException)
        {
            return (null, new ToolError("write_error", ex.Message));
        }

        return (new Output(replacementsMade), null);
    }

    private static int CountOccurrences(string content, string target)
    {
        if (target.Length == 0)
        {
            return 0;
        }

        int count = 0;
        int index = 0;
        while ((index = content.IndexOf(target, index, StringComparison.Ordinal)) >= 0)
        {
            count++;
            index += target.Length;
        }

        return count;
    }

    private static string ReplaceFirst(string content, string oldValue, string newValue)
    {
        int index = content.IndexOf(oldValue, StringComparison.Ordinal);
        return index < 0 ? content : content[..index] + newValue + content[(index + oldValue.Length)..];
    }
}
