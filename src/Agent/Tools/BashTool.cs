using System.ComponentModel;
using System.Diagnostics;
using Agent.Messages;

namespace Agent.Tools;

/// <summary>Gated tool: runs a shell command. Calls System.Diagnostics.Process directly — no interface
/// abstraction (DIP is scoped narrowly and does not extend to tool handlers, TECH-SPEC §3). A command that
/// exceeds timeout_ms resolves as success:true with output.timed_out:true, never error.code:"timeout"
/// (§6 audit finding #1) — exit_code already carries command outcome without signaling tool failure.</summary>
public static class BashTool
{
    private const int DefaultTimeoutMs = 120_000;

    public sealed record Input(string Command, string? Cwd, int? TimeoutMs);

    public sealed record Output(string Stdout, string Stderr, int ExitCode, bool TimedOut);

    public static (Output? Output, ToolError? Error) Execute(Input input)
    {
        if (string.IsNullOrWhiteSpace(input.Command))
        {
            return (null, new ToolError("spawn_error", "command must not be empty"));
        }

        var startInfo = new ProcessStartInfo
        {
            FileName = OperatingSystem.IsWindows() ? "cmd.exe" : "/bin/bash",
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        startInfo.ArgumentList.Add(OperatingSystem.IsWindows() ? "/c" : "-c");
        startInfo.ArgumentList.Add(input.Command);

        if (!string.IsNullOrEmpty(input.Cwd))
        {
            startInfo.WorkingDirectory = input.Cwd;
        }

        Process process;
        try
        {
            process = Process.Start(startInfo) ?? throw new InvalidOperationException("Process.Start returned null");
        }
        catch (Exception ex) when (ex is Win32Exception or ArgumentException or InvalidOperationException)
        {
            return (null, new ToolError("spawn_error", ex.Message));
        }

        Task<string> stdoutTask = process.StandardOutput.ReadToEndAsync();
        Task<string> stderrTask = process.StandardError.ReadToEndAsync();

        int timeoutMs = input.TimeoutMs.GetValueOrDefault(DefaultTimeoutMs);
        bool exited = process.WaitForExit(timeoutMs);

        if (!exited)
        {
            TryKill(process);
            process.WaitForExit();
            return (new Output(SafeResult(stdoutTask), SafeResult(stderrTask), process.ExitCode, TimedOut: true), null);
        }

        return (new Output(SafeResult(stdoutTask), SafeResult(stderrTask), process.ExitCode, TimedOut: false), null);
    }

    private static void TryKill(Process process)
    {
        try
        {
            process.Kill(entireProcessTree: true);
        }
        catch (InvalidOperationException)
        {
        }
    }

    private static string SafeResult(Task<string> task)
    {
        try
        {
            return task.GetAwaiter().GetResult();
        }
        catch (Exception)
        {
            return string.Empty;
        }
    }
}
