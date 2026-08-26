using System.Text.Json;
using xDreamer.Agent.Messages;
using Xunit;

namespace xDreamer.Agent.Tests.Messages;

[Trait("Category", "Unit")]
public class WireMessagesTests
{
    [Fact]
    public void TaskMessage_RoundTripsThroughJsonContext()
    {
        var original = new TaskMessage(
            TaskId: "task-1",
            Instructions: "implement feature X",
            Cwd: "/repo",
            Config: new TaskConfig(
                Llm: new LlmConfig(BaseUrl: "http://localhost:1234/v1", Model: "local-model", Temperature: 0.2),
                MaxTurns: 20,
                ContextLimitTokens: 8192));

        var json = JsonSerializer.Serialize(original, JsonContext.Default.TaskMessage);
        var roundTripped = JsonSerializer.Deserialize(json, JsonContext.Default.TaskMessage);

        Assert.NotNull(roundTripped);
        Assert.Equal("task", roundTripped!.Type);
        Assert.Equal(original.TaskId, roundTripped.TaskId);
        Assert.Equal(original.Instructions, roundTripped.Instructions);
        Assert.Equal(original.Cwd, roundTripped.Cwd);
        Assert.Equal(original.Config!.MaxTurns, roundTripped.Config!.MaxTurns);
        Assert.Equal(original.Config!.ContextLimitTokens, roundTripped.Config!.ContextLimitTokens);
        Assert.Equal(original.Config!.Llm!.BaseUrl, roundTripped.Config!.Llm!.BaseUrl);
        Assert.Equal(original.Config!.Llm!.Model, roundTripped.Config!.Llm!.Model);
        Assert.Equal(original.Config!.Llm!.Temperature, roundTripped.Config!.Llm!.Temperature);
        Assert.Contains("\"type\":\"task\"", json);
    }

    [Fact]
    public void ToolCallMessage_RoundTripsThroughJsonContext()
    {
        var input = JsonSerializer.SerializeToElement(new { path = "src/Foo.cs", offset = 10, limit = 100 });
        var original = new ToolCallMessage(Id: "call-1", Tool: "read_file", Input: input);

        var json = JsonSerializer.Serialize(original, JsonContext.Default.ToolCallMessage);
        var roundTripped = JsonSerializer.Deserialize(json, JsonContext.Default.ToolCallMessage);

        Assert.NotNull(roundTripped);
        Assert.Equal("tool_call", roundTripped!.Type);
        Assert.Equal(original.Id, roundTripped.Id);
        Assert.Equal(original.Tool, roundTripped.Tool);
        Assert.Equal("src/Foo.cs", roundTripped.Input.GetProperty("path").GetString());
        Assert.Equal(10, roundTripped.Input.GetProperty("offset").GetInt32());
        Assert.Equal(100, roundTripped.Input.GetProperty("limit").GetInt32());
    }

    [Fact]
    public void PermissionRequestMessage_RoundTripsThroughJsonContext()
    {
        var input = JsonSerializer.SerializeToElement(new { path = "src/Foo.cs", content = "new contents" });
        var original = new PermissionRequestMessage(Id: "call-2", Tool: "write_file", Input: input);

        var json = JsonSerializer.Serialize(original, JsonContext.Default.PermissionRequestMessage);
        var roundTripped = JsonSerializer.Deserialize(json, JsonContext.Default.PermissionRequestMessage);

        Assert.NotNull(roundTripped);
        Assert.Equal("permission_request", roundTripped!.Type);
        Assert.Equal(original.Id, roundTripped.Id);
        Assert.Equal(original.Tool, roundTripped.Tool);
        Assert.Equal("src/Foo.cs", roundTripped.Input.GetProperty("path").GetString());
        Assert.Equal("new contents", roundTripped.Input.GetProperty("content").GetString());
    }

    [Fact]
    public void PermissionResponseMessage_RoundTripsThroughJsonContext()
    {
        var original = new PermissionResponseMessage(Id: "call-2", Decision: "deny", Reason: "not safe");

        var json = JsonSerializer.Serialize(original, JsonContext.Default.PermissionResponseMessage);
        var roundTripped = JsonSerializer.Deserialize(json, JsonContext.Default.PermissionResponseMessage);

        Assert.NotNull(roundTripped);
        Assert.Equal("permission_response", roundTripped!.Type);
        Assert.Equal(original.Id, roundTripped.Id);
        Assert.Equal(original.Decision, roundTripped.Decision);
        Assert.Equal(original.Reason, roundTripped.Reason);
    }

    [Fact]
    public void ToolResultMessage_RoundTripsThroughJsonContext()
    {
        var output = JsonSerializer.SerializeToElement(new { content = "file body", truncated = false });
        var success = new ToolResultMessage(Id: "call-1", Tool: "read_file", Success: true, Output: output, Error: null);

        var successJson = JsonSerializer.Serialize(success, JsonContext.Default.ToolResultMessage);
        var successRoundTripped = JsonSerializer.Deserialize(successJson, JsonContext.Default.ToolResultMessage);

        Assert.NotNull(successRoundTripped);
        Assert.Equal("tool_result", successRoundTripped!.Type);
        Assert.True(successRoundTripped.Success);
        Assert.Null(successRoundTripped.Error);
        Assert.Equal("file body", successRoundTripped.Output!.Value.GetProperty("content").GetString());

        var failure = new ToolResultMessage(
            Id: "call-2", Tool: "write_file", Success: false, Output: null,
            Error: new ToolError(Code: "permission_denied", Message: "denied by orchestrator"));

        var failureJson = JsonSerializer.Serialize(failure, JsonContext.Default.ToolResultMessage);
        var failureRoundTripped = JsonSerializer.Deserialize(failureJson, JsonContext.Default.ToolResultMessage);

        Assert.NotNull(failureRoundTripped);
        Assert.False(failureRoundTripped!.Success);
        Assert.Null(failureRoundTripped.Output);
        Assert.Equal("permission_denied", failureRoundTripped.Error!.Code);
        Assert.Equal("denied by orchestrator", failureRoundTripped.Error!.Message);
    }

    [Fact]
    public void TaskCompleteMessage_RoundTripsThroughJsonContext()
    {
        var success = new TaskCompleteMessage(TaskId: "task-1", Result: "success", Summary: "done", Error: null);

        var successJson = JsonSerializer.Serialize(success, JsonContext.Default.TaskCompleteMessage);
        var successRoundTripped = JsonSerializer.Deserialize(successJson, JsonContext.Default.TaskCompleteMessage);

        Assert.NotNull(successRoundTripped);
        Assert.Equal("task_complete", successRoundTripped!.Type);
        Assert.Equal("task-1", successRoundTripped.TaskId);
        Assert.Equal("success", successRoundTripped.Result);
        Assert.Equal("done", successRoundTripped.Summary);
        Assert.Null(successRoundTripped.Error);

        var failure = new TaskCompleteMessage(
            TaskId: null, Result: "failure", Summary: null,
            Error: new TaskCompleteError(Code: "malformed_message", Message: "first message was not type: task"));

        var failureJson = JsonSerializer.Serialize(failure, JsonContext.Default.TaskCompleteMessage);
        var failureRoundTripped = JsonSerializer.Deserialize(failureJson, JsonContext.Default.TaskCompleteMessage);

        Assert.NotNull(failureRoundTripped);
        Assert.Null(failureRoundTripped!.TaskId);
        Assert.Equal("failure", failureRoundTripped.Result);
        Assert.Null(failureRoundTripped.Summary);
        Assert.Equal("malformed_message", failureRoundTripped.Error!.Code);
    }
}
