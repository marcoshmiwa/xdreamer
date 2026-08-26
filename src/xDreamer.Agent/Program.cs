using System.Text.Json;
using Agent;
using Agent.Llm;
using Agent.Messages;
using Agent.Transport;

// Composition root: no task-specific argv — the process opens stdio and waits for the first task line.
// AgentLoop needs an ILlmClient at construction time, but config.llm only becomes known once that first
// line is parsed, so it's peeked here and replayed as AgentLoop's own "first line" — AgentLoop still owns
// parsing/validating it (Task 13); Program.cs only uses the peek to build the real LmStudioChatClient.
var stdio = new NdjsonStdio(Console.OpenStandardInput(), Console.OpenStandardOutput());

string? firstLine = await stdio.ReadLineAsync();
TaskMessage? task = TryParseTask(firstLine);

LlmConfig? llmConfig = task?.Config?.Llm;
ILlmClient llmClient = !string.IsNullOrWhiteSpace(llmConfig?.BaseUrl) && !string.IsNullOrWhiteSpace(llmConfig?.Model)
    ? new LmStudioChatClient(llmConfig!)
    : new LmStudioChatClient(new LlmConfig("http://127.0.0.1:0/", "unspecified", null));

bool firstLineReplayed = false;
Func<Task<string?>> readLine = () =>
{
    if (!firstLineReplayed)
    {
        firstLineReplayed = true;
        return Task.FromResult(firstLine);
    }

    return stdio.ReadLineAsync();
};

var loop = new AgentLoop(llmClient, readLine, stdio.WriteLine);
return await loop.RunAsync();

static TaskMessage? TryParseTask(string? line)
{
    if (line is null)
    {
        return null;
    }

    try
    {
        return JsonSerializer.Deserialize(line, JsonContext.Default.TaskMessage);
    }
    catch (JsonException)
    {
        return null;
    }
}
