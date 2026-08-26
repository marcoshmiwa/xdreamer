using System.Text.Json;
using xDreamer.Agent;
using xDreamer.Agent.Llm;
using xDreamer.Agent.Messages;
using xDreamer.Agent.Tests.Llm;
using xDreamer.Agent.Tests.Tools;
using Xunit;

namespace xDreamer.Agent.Tests;

/// <summary>The one true end-to-end test (TECH-SPEC §4): mock LLM + real temp dir + real process exec,
/// fake stdio. Drives a full task -> (tool_call|permission_request) -> permission_response -> tool_result
/// -> task_complete sequence against all four tools and the permission gate (Validation Criterion #11).</summary>
[Trait("Category", "Integration")]
public class AgentLoopIntegrationTests
{
    [Fact]
    public async Task FullTaskLifecycle_AllFourTools_PermissionGate_EndsInTaskComplete()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string readPath = Path.Combine(tempDir.Path, "notes.txt");
        File.WriteAllText(readPath, "hello world");
        string writePath = Path.Combine(tempDir.Path, "newfile.txt");

        using var mockServer = new MockLmStudioServer();
        mockServer.EnqueueResponse(ToolCallResponseJson("call_read", "read_file", new { path = readPath }));
        mockServer.EnqueueResponse(ToolCallResponseJson("call_write", "write_file", new { path = writePath, content = "written content" }));
        mockServer.EnqueueResponse(ToolCallResponseJson("call_edit", "edit_file", new { path = writePath, old_string = "written", new_string = "edited" }));
        mockServer.EnqueueResponse(ToolCallResponseJson("call_bash", "bash", new { command = "echo hi" }));
        mockServer.SetResponse(FinalResponseJson("All done"));

        var llmClient = new LmStudioChatClient(new LlmConfig(mockServer.BaseUrl, "test-model", null));

        var incoming = new Queue<string?>(
        [
            TaskLine(tempDir.Path),
            PermissionResponseLine("call_write", "allow"),
            PermissionResponseLine("call_edit", "allow"),
            PermissionResponseLine("call_bash", "allow"),
        ]);
        var written = new List<string>();

        // Construct new AgentLoop(mockLlmClient, fakeReadLine, fakeWriteLine) — zero real stdio involved (TECH-SPEC §3 DIP example).
        var loop = new AgentLoop(
            llmClient,
            () => Task.FromResult(incoming.Count > 0 ? incoming.Dequeue() : null),
            written.Add);

        int exitCode = await loop.RunAsync();

        Assert.Equal(0, exitCode);

        List<JsonDocument> messages = written.Select(line => JsonDocument.Parse(line)).ToList();
        string[] types = messages.Select(m => m.RootElement.GetProperty("type").GetString()!).ToArray();
        Assert.Equal(
            ["tool_call", "tool_result", "permission_request", "tool_result", "permission_request", "tool_result", "permission_request", "tool_result", "task_complete"],
            types);

        JsonElement readResult = messages[1].RootElement;
        Assert.True(readResult.GetProperty("success").GetBoolean());
        Assert.Equal("hello world", readResult.GetProperty("output").GetProperty("content").GetString());

        Assert.True(messages[3].RootElement.GetProperty("success").GetBoolean());
        Assert.True(messages[5].RootElement.GetProperty("success").GetBoolean());
        Assert.True(messages[7].RootElement.GetProperty("success").GetBoolean());

        JsonElement complete = messages[8].RootElement;
        Assert.Equal("success", complete.GetProperty("result").GetString());
        Assert.Equal("All done", complete.GetProperty("summary").GetString());

        Assert.Equal("edited content", File.ReadAllText(writePath));
    }

    [Fact]
    public async Task FullTaskLifecycle_AssertsOnCapturedOutput_NoRealStdioInvolved()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string readPath = Path.Combine(tempDir.Path, "notes.txt");
        File.WriteAllText(readPath, "content");

        using var mockServer = new MockLmStudioServer();
        mockServer.EnqueueResponse(ToolCallResponseJson("call_read", "read_file", new { path = readPath }));
        mockServer.SetResponse(FinalResponseJson("done"));
        var llmClient = new LmStudioChatClient(new LlmConfig(mockServer.BaseUrl, "test-model", null));

        var incoming = new Queue<string?>([TaskLine(tempDir.Path)]);
        var written = new List<string>();

        // Every byte the agent reads/writes flows through these two in-memory delegates — never Console.
        var loop = new AgentLoop(
            llmClient,
            () => Task.FromResult(incoming.Count > 0 ? incoming.Dequeue() : null),
            written.Add);

        int exitCode = await loop.RunAsync();

        Assert.Equal(0, exitCode);
        Assert.Equal(3, written.Count);
        Assert.Contains(written, line => JsonDocument.Parse(line).RootElement.GetProperty("type").GetString() == "tool_call");
        Assert.Contains(written, line => JsonDocument.Parse(line).RootElement.GetProperty("type").GetString() == "tool_result");
        JsonElement complete = JsonDocument.Parse(written[^1]).RootElement;
        Assert.Equal("task_complete", complete.GetProperty("type").GetString());
        Assert.Equal("success", complete.GetProperty("result").GetString());
    }

    [Fact]
    public async Task WriteFile_NoFileWrittenBeforePermissionResponseReceived()
    {
        using var tempDir = new ReadFileToolTests.TempDirectory();
        string writePath = Path.Combine(tempDir.Path, "newfile.txt");

        using var mockServer = new MockLmStudioServer();
        mockServer.EnqueueResponse(ToolCallResponseJson("call_write", "write_file", new { path = writePath, content = "hello" }));
        mockServer.SetResponse(FinalResponseJson("done"));
        var llmClient = new LmStudioChatClient(new LlmConfig(mockServer.BaseUrl, "test-model", null));

        string taskLine = TaskLine(tempDir.Path);
        bool taskLineSent = false;
        var permissionResponseGate = new TaskCompletionSource();
        var written = new List<string>();

        // The second ReadLine call (for the permission_response) blocks until the test explicitly
        // releases it, so we can inspect filesystem state while the AgentLoop is genuinely paused
        // in AwaitingPermission — not just after the fact.
        Func<Task<string?>> readLine = async () =>
        {
            if (!taskLineSent)
            {
                taskLineSent = true;
                return taskLine;
            }

            await permissionResponseGate.Task;
            return PermissionResponseLine("call_write", "allow");
        };

        var loop = new AgentLoop(llmClient, readLine, written.Add);
        Task<int> runTask = loop.RunAsync();

        await WaitUntilAsync(() => written.Any(line => JsonDocument.Parse(line).RootElement.GetProperty("type").GetString() == "permission_request"));

        Assert.False(File.Exists(writePath));

        permissionResponseGate.SetResult();
        int exitCode = await runTask;

        Assert.Equal(0, exitCode);
        Assert.True(File.Exists(writePath));
    }

    private static async Task WaitUntilAsync(Func<bool> condition)
    {
        for (int i = 0; i < 500 && !condition(); i++)
        {
            await Task.Delay(10);
        }

        Assert.True(condition(), "Condition was not met within the timeout.");
    }

    private static string TaskLine(string cwd)
    {
        var message = new TaskMessage(
            TaskId: "task-1",
            Instructions: "read notes.txt, write it to newfile.txt, edit it, then run a shell command",
            Cwd: cwd,
            Config: new TaskConfig(new LlmConfig("http://unused/", "test-model", null), MaxTurns: 10, ContextLimitTokens: 8192));
        return JsonSerializer.Serialize(message, JsonContext.Default.TaskMessage);
    }

    private static string PermissionResponseLine(string id, string decision)
        => JsonSerializer.Serialize(new PermissionResponseMessage(id, decision, null), JsonContext.Default.PermissionResponseMessage);

    private static string ToolCallResponseJson(string callId, string toolName, object argumentsObject)
    {
        string argumentsJson = JsonSerializer.Serialize(argumentsObject);
        return JsonSerializer.Serialize(new
        {
            choices = new[]
            {
                new
                {
                    message = new
                    {
                        role = "assistant",
                        content = (string?)null,
                        tool_calls = new[]
                        {
                            new { id = callId, type = "function", function = new { name = toolName, arguments = argumentsJson } },
                        },
                    },
                },
            },
        });
    }

    private static string FinalResponseJson(string content)
        => JsonSerializer.Serialize(new { choices = new[] { new { message = new { role = "assistant", content } } } });
}
