using System.Text.Json.Serialization;

namespace xDreamer.Agent.Messages;

[JsonSourceGenerationOptions(WriteIndented = false)]
[JsonSerializable(typeof(TaskMessage))]
[JsonSerializable(typeof(ToolCallMessage))]
[JsonSerializable(typeof(PermissionRequestMessage))]
[JsonSerializable(typeof(PermissionResponseMessage))]
[JsonSerializable(typeof(ToolResultMessage))]
[JsonSerializable(typeof(TaskCompleteMessage))]
public partial class JsonContext : JsonSerializerContext
{
}
