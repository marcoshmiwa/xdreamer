using System.Text.Json;
using System.Text.Json.Serialization;
using System.Text.Json.Serialization.Metadata;
using Agent.Messages;

namespace Agent.Tools;

/// <summary>Switch expression mapping tool name → handler function, plus the gated-tool static lookup
/// (§2: no ITool interface — the tool set is spec-fixed at exactly four, not extensible).</summary>
public static partial class ToolDispatch
{
    private static readonly string[] GatedToolNamesArray = ["write_file", "edit_file", "bash"];
    private static readonly string[] UngatedToolNamesArray = ["read_file"];

    public static IReadOnlyCollection<string> GatedTools { get; } = GatedToolNamesArray;

    public static IReadOnlyCollection<string> UngatedTools { get; } = UngatedToolNamesArray;

    public static bool IsGated(string toolName) => Array.IndexOf(GatedToolNamesArray, toolName) >= 0;

    public static (JsonElement? Output, ToolError? Error) Execute(string toolName, JsonElement input, string cwd)
    {
        return toolName switch
        {
            "read_file" => Run(ReadFileTool.Execute(Deserialize(input, ToolDispatchJsonContext.Default.ReadFileToolInput)), ToolDispatchJsonContext.Default.ReadFileToolOutput),
            "write_file" => Run(WriteFileTool.Execute(Deserialize(input, ToolDispatchJsonContext.Default.WriteFileToolInput), cwd), ToolDispatchJsonContext.Default.WriteFileToolOutput),
            "edit_file" => Run(EditFileTool.Execute(Deserialize(input, ToolDispatchJsonContext.Default.EditFileToolInput), cwd), ToolDispatchJsonContext.Default.EditFileToolOutput),
            "bash" => Run(BashTool.Execute(Deserialize(input, ToolDispatchJsonContext.Default.BashToolInput)), ToolDispatchJsonContext.Default.BashToolOutput),
            _ => throw new ArgumentException($"Unknown tool: {toolName}", nameof(toolName)),
        };
    }

    private static T Deserialize<T>(JsonElement input, JsonTypeInfo<T> typeInfo) => input.Deserialize(typeInfo)!;

    private static (JsonElement? Output, ToolError? Error) Run<T>((T? Output, ToolError? Error) result, JsonTypeInfo<T> typeInfo)
        where T : class
        => result.Output is null
            ? (null, result.Error)
            : (JsonSerializer.SerializeToElement(result.Output, typeInfo), null);

    [JsonSourceGenerationOptions(WriteIndented = false)]
    [JsonSerializable(typeof(ReadFileTool.Input), TypeInfoPropertyName = "ReadFileToolInput")]
    [JsonSerializable(typeof(ReadFileTool.Output), TypeInfoPropertyName = "ReadFileToolOutput")]
    [JsonSerializable(typeof(WriteFileTool.Input), TypeInfoPropertyName = "WriteFileToolInput")]
    [JsonSerializable(typeof(WriteFileTool.Output), TypeInfoPropertyName = "WriteFileToolOutput")]
    [JsonSerializable(typeof(EditFileTool.Input), TypeInfoPropertyName = "EditFileToolInput")]
    [JsonSerializable(typeof(EditFileTool.Output), TypeInfoPropertyName = "EditFileToolOutput")]
    [JsonSerializable(typeof(BashTool.Input), TypeInfoPropertyName = "BashToolInput")]
    [JsonSerializable(typeof(BashTool.Output), TypeInfoPropertyName = "BashToolOutput")]
    private partial class ToolDispatchJsonContext : JsonSerializerContext
    {
    }
}
