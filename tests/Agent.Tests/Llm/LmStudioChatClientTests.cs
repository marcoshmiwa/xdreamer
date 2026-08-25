using System.Net;
using System.Net.Sockets;
using System.Text.Json;
using Agent.Llm;
using Agent.Messages;
using Xunit;

namespace Agent.Tests.Llm;

[Trait("Category", "Integration")]
public class LmStudioChatClientTests : IClassFixture<MockLmStudioServer>
{
    private readonly MockLmStudioServer _mockServer;

    public LmStudioChatClientTests(MockLmStudioServer mockServer)
    {
        _mockServer = mockServer;
    }

    [Fact]
    public async Task CompleteAsync_SendsOpenAiCompatibleRequestBody()
    {
        _mockServer.SetResponse("""{"choices":[{"message":{"role":"assistant","content":"hello"}}]}""");
        var client = new LmStudioChatClient(new LlmConfig(_mockServer.BaseUrl, "test-model", null));

        var request = new ChatRequest(
            Messages: [new ChatMessage("user", "list the files in this repo")],
            Tools: [new ToolDefinition("read_file", "Read a file", JsonSerializer.SerializeToElement(new { type = "object" }))]);

        await client.CompleteAsync(request);

        Assert.NotNull(_mockServer.LastRequestBody);
        using var sentBody = JsonDocument.Parse(_mockServer.LastRequestBody!);
        JsonElement root = sentBody.RootElement;

        Assert.Equal("test-model", root.GetProperty("model").GetString());

        JsonElement messages = root.GetProperty("messages");
        Assert.Equal(1, messages.GetArrayLength());
        Assert.Equal("user", messages[0].GetProperty("role").GetString());
        Assert.Equal("list the files in this repo", messages[0].GetProperty("content").GetString());

        JsonElement tools = root.GetProperty("tools");
        Assert.Equal(1, tools.GetArrayLength());
        Assert.Equal("function", tools[0].GetProperty("type").GetString());
        Assert.Equal("read_file", tools[0].GetProperty("function").GetProperty("name").GetString());
    }

    [Fact]
    public async Task CompleteAsync_ParsesToolCallsFromResponse()
    {
        _mockServer.SetResponse("""
            {"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[
                {"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"src/Foo.cs\"}"}}
            ]}}]}
            """);
        var client = new LmStudioChatClient(new LlmConfig(_mockServer.BaseUrl, "test-model", null));

        var request = new ChatRequest(
            Messages: [new ChatMessage("user", "read Foo.cs")],
            Tools: [new ToolDefinition("read_file", "Read a file", JsonSerializer.SerializeToElement(new { type = "object" }))]);

        ChatResponse response = await client.CompleteAsync(request);

        Assert.Single(response.ToolCalls);
        RequestedToolCall call = response.ToolCalls[0];
        Assert.Equal("call_1", call.Id);
        Assert.Equal("read_file", call.ToolName);
        Assert.Equal("src/Foo.cs", call.Arguments.GetProperty("path").GetString());
    }

    [Fact]
    public async Task CompleteAsync_ServerUnreachable_ThrowsLlmUnreachableException()
    {
        int unusedPort = GetFreeTcpPort();
        var client = new LmStudioChatClient(new LlmConfig($"http://127.0.0.1:{unusedPort}/", "test-model", null));
        var request = new ChatRequest(Messages: [new ChatMessage("user", "hi")], Tools: []);

        await Assert.ThrowsAsync<LlmUnreachableException>(() => client.CompleteAsync(request));
    }

    [Fact]
    public async Task CompleteAsync_UsesBaseUrlAndModelFromConfig_NoHardcodedEndpoint()
    {
        using var secondServer = new MockLmStudioServer();
        secondServer.SetResponse("""{"choices":[{"message":{"role":"assistant","content":"from second server"}}]}""");
        _mockServer.SetResponse("""{"choices":[{"message":{"role":"assistant","content":"from first server"}}]}""");

        var firstClient = new LmStudioChatClient(new LlmConfig(_mockServer.BaseUrl, "model-one", null));
        var secondClient = new LmStudioChatClient(new LlmConfig(secondServer.BaseUrl, "model-two", null));
        var request = new ChatRequest(Messages: [new ChatMessage("user", "hi")], Tools: []);

        ChatResponse firstResponse = await firstClient.CompleteAsync(request);
        ChatResponse secondResponse = await secondClient.CompleteAsync(request);

        Assert.Equal("from first server", firstResponse.Content);
        Assert.Equal("from second server", secondResponse.Content);

        using var firstSentBody = JsonDocument.Parse(_mockServer.LastRequestBody!);
        using var secondSentBody = JsonDocument.Parse(secondServer.LastRequestBody!);
        Assert.Equal("model-one", firstSentBody.RootElement.GetProperty("model").GetString());
        Assert.Equal("model-two", secondSentBody.RootElement.GetProperty("model").GetString());
    }

    [Theory]
    [InlineData(0)]
    [InlineData(1)]
    public async Task CompleteAsync_AgainstEitherMockServerInstance_ProducesSameResult(int serverIndex)
    {
        using var serverA = new MockLmStudioServer();
        using var serverB = new MockLmStudioServer();
        serverA.SetResponse("""{"choices":[{"message":{"role":"assistant","content":"same result"}}]}""");
        serverB.SetResponse("""{"choices":[{"message":{"role":"assistant","content":"same result"}}]}""");

        // Same LmStudioChatClient code path either way — only config.llm.base_url differs (Validation Criterion #12).
        MockLmStudioServer selectedServer = serverIndex == 0 ? serverA : serverB;
        var client = new LmStudioChatClient(new LlmConfig(selectedServer.BaseUrl, "test-model", null));
        var request = new ChatRequest(Messages: [new ChatMessage("user", "hi")], Tools: []);

        ChatResponse response = await client.CompleteAsync(request);

        Assert.Equal("same result", response.Content);
    }

    private static int GetFreeTcpPort()
    {
        var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        int port = ((IPEndPoint)listener.LocalEndpoint).Port;
        listener.Stop();
        return port;
    }
}
