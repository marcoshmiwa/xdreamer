namespace Agent;

/// <summary>Loop controller: turns, tool dispatch, permission-gate correlation. Depends only on the stdio
/// delegates and (from Task 4 onward) ILlmClient — never on Console or HttpClient directly (DIP).</summary>
public sealed class AgentLoop
{
    private readonly Func<Task<string?>> _readLine;
    private readonly Action<string> _writeLine;

    public AgentLoop(Func<Task<string?>> readLine, Action<string> writeLine)
    {
        _readLine = readLine ?? throw new ArgumentNullException(nameof(readLine));
        _writeLine = writeLine ?? throw new ArgumentNullException(nameof(writeLine));
    }
}
