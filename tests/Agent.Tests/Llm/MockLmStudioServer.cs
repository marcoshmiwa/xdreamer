using System.Net;
using System.Net.Sockets;
using System.Text;

namespace Agent.Tests.Llm;

/// <summary>In-process HttpListener-based mock implementing POST /chat/completions, for testing
/// LmStudioChatClient without a real LM Studio process. One instance per test class via
/// IClassFixture&lt;T&gt;, disposed after (TECH-SPEC §4 Mocking Boundaries).</summary>
public sealed class MockLmStudioServer : IDisposable
{
    private readonly HttpListener _listener;
    private readonly CancellationTokenSource _cts = new();
    private readonly Task _acceptLoop;
    private readonly Queue<string> _queuedResponses = new();
    private string _responseJson = "{\"choices\":[{\"message\":{\"role\":\"assistant\",\"content\":\"ok\"}}]}";

    public string BaseUrl { get; }

    public string? LastRequestBody { get; private set; }

    public MockLmStudioServer()
    {
        int port = GetFreeTcpPort();
        BaseUrl = $"http://127.0.0.1:{port}/";

        _listener = new HttpListener();
        _listener.Prefixes.Add(BaseUrl);
        _listener.Start();
        _acceptLoop = Task.Run(AcceptLoopAsync);
    }

    /// <summary>Sets the JSON body the mock returns for every subsequent request (once any enqueued
    /// responses are exhausted).</summary>
    public void SetResponse(string json) => _responseJson = json;

    /// <summary>Queues one JSON response to be returned for the next request only, before falling back
    /// to SetResponse's fixed body — lets a test script a distinct response per AgentLoop turn.</summary>
    public void EnqueueResponse(string json) => _queuedResponses.Enqueue(json);

    private async Task AcceptLoopAsync()
    {
        while (!_cts.IsCancellationRequested)
        {
            HttpListenerContext context;
            try
            {
                context = await _listener.GetContextAsync().ConfigureAwait(false);
            }
            catch (Exception) when (_cts.IsCancellationRequested || !_listener.IsListening)
            {
                return;
            }

            using var reader = new StreamReader(context.Request.InputStream, Encoding.UTF8);
            LastRequestBody = await reader.ReadToEndAsync().ConfigureAwait(false);

            string responseJson = _queuedResponses.Count > 0 ? _queuedResponses.Dequeue() : _responseJson;
            byte[] responseBytes = Encoding.UTF8.GetBytes(responseJson);
            context.Response.ContentType = "application/json";
            context.Response.ContentLength64 = responseBytes.Length;
            await context.Response.OutputStream.WriteAsync(responseBytes).ConfigureAwait(false);
            context.Response.OutputStream.Close();
        }
    }

    private static int GetFreeTcpPort()
    {
        var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        int port = ((IPEndPoint)listener.LocalEndpoint).Port;
        listener.Stop();
        return port;
    }

    public void Dispose()
    {
        _cts.Cancel();
        _listener.Stop();
        _listener.Close();
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
