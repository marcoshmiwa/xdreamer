using System.Net;
using System.Net.Sockets;
using System.Text.Json;
using Agent;
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
    public void Constructor_BlankBaseUrl_ThrowsArgumentException()
    {
        Assert.Throws<ArgumentException>(() => new LmStudioChatClient(new LlmConfig("   ", "test-model", null)));
    }

    [Fact]
    public void Constructor_BlankModel_ThrowsArgumentException()
    {
        Assert.Throws<ArgumentException>(() => new LmStudioChatClient(new LlmConfig("http://localhost:1234/v1", "   ", null)));
    }

    [Fact]
    public async Task CompleteAsync_NonSuccessStatusCode_ThrowsLlmUnreachableException()
    {
        using var server = new MockLmStudioServer();
        server.SetStatusCode(500);
        var client = new LmStudioChatClient(new LlmConfig(server.BaseUrl, "test-model", null));
        var request = new ChatRequest(Messages: [new ChatMessage("user", "hi")], Tools: []);

        await Assert.ThrowsAsync<LlmUnreachableException>(() => client.CompleteAsync(request));
    }

    [Fact]
    public async Task CompleteAsync_ResponseWithNoChoices_ReturnsNullContentAndEmptyToolCalls()
    {
        using var server = new MockLmStudioServer();
        server.SetResponse("""{"choices":[]}""");
        var client = new LmStudioChatClient(new LlmConfig(server.BaseUrl, "test-model", null));
        var request = new ChatRequest(Messages: [new ChatMessage("user", "hi")], Tools: []);

        ChatResponse response = await client.CompleteAsync(request);

        Assert.Null(response.Content);
        Assert.Empty(response.ToolCalls);
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

    [Fact]
    public async Task AgentLoop_LlmUnreachable_EmitsTaskCompleteFailureLlmUnreachable_ZeroRetries()
    {
        using var refusingServer = new ConnectionCountingRefusingServer();
        var llmClient = new LmStudioChatClient(new LlmConfig(refusingServer.BaseUrl, "test-model", null));

        var taskMessage = new TaskMessage(
            "task-1", "do stuff", Path.GetTempPath(),
            new TaskConfig(new LlmConfig(refusingServer.BaseUrl, "test-model", null), MaxTurns: 5, ContextLimitTokens: 8192));
        string taskLine = JsonSerializer.Serialize(taskMessage, JsonContext.Default.TaskMessage);

        var incoming = new Queue<string?>([taskLine]);
        var written = new List<string>();
        var loop = new AgentLoop(
            llmClient,
            () => Task.FromResult(incoming.Count > 0 ? incoming.Dequeue() : null),
            written.Add);

        int exitCode = await loop.RunAsync();

        Assert.NotEqual(0, exitCode);
        var complete = JsonSerializer.Deserialize(written[^1], JsonContext.Default.TaskCompleteMessage);
        Assert.Equal("failure", complete!.Result);
        Assert.Equal("llm_unreachable", complete.Error!.Code);

        // Poll briefly: the TCP accept on the server side is asynchronous relative to the client
        // observing the connection failure, but zero retries means exactly one attempt was ever made.
        for (int i = 0; i < 50 && refusingServer.ConnectionAttempts == 0; i++)
        {
            await Task.Delay(10, TestContext.Current.CancellationToken);
        }

        Assert.Equal(1, refusingServer.ConnectionAttempts);
    }

    private static int GetFreeTcpPort()
    {
        var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        int port = ((IPEndPoint)listener.LocalEndpoint).Port;
        listener.Stop();
        return port;
    }

    /// <summary>Accepts a raw TCP connection and immediately resets it (no HTTP response), counting
    /// attempts — used to prove LmStudioChatClient makes exactly one connection attempt, never retries.</summary>
    private sealed class ConnectionCountingRefusingServer : IDisposable
    {
        private readonly TcpListener _listener;
        private readonly CancellationTokenSource _cts = new();
        private readonly Task _acceptLoop;

        public string BaseUrl { get; }

        public int ConnectionAttempts;

        public ConnectionCountingRefusingServer()
        {
            _listener = new TcpListener(IPAddress.Loopback, 0);
            _listener.Start();
            int port = ((IPEndPoint)_listener.LocalEndpoint).Port;
            BaseUrl = $"http://127.0.0.1:{port}/";
            _acceptLoop = Task.Run(AcceptLoopAsync);
        }

        private async Task AcceptLoopAsync()
        {
            while (!_cts.IsCancellationRequested)
            {
                TcpClient client;
                try
                {
                    client = await _listener.AcceptTcpClientAsync(_cts.Token).ConfigureAwait(false);
                }
                catch (Exception)
                {
                    return;
                }

                Interlocked.Increment(ref ConnectionAttempts);
                client.Client.LingerState = new LingerOption(true, 0);
                client.Close();
            }
        }

        public void Dispose()
        {
            _cts.Cancel();
            _listener.Stop();
            try
            {
                _acceptLoop.Wait(TimeSpan.FromSeconds(1));
            }
            catch (AggregateException)
            {
            }

            _cts.Dispose();
        }
    }
}
