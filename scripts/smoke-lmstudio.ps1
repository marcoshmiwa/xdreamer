<#
.SYNOPSIS
Manual, non-CI smoke test (TECH-SPEC section 4): drives the published Native AOT Agent executable through a
full task lifecycle (Validation Criterion #11) against a real, locally running LM Studio instance.

.DESCRIPTION
Publishes the Agent project (Release, self-contained, Native AOT single-file) and invokes the
resulting executable directly -never `dotnet run` -since trimming-related reflection failures only
surface at that boundary. Sends one task message over the process's stdin, then acts as a minimal
orchestrator: prints every NDJSON message the agent emits on stdout, and for each permission_request
prompts you to allow or deny before writing the matching permission_response back to its stdin.

Not part of automated CI -coverage of the published, trimmed binary against a real LM Studio backend
is an accepted manual gap (TECH-SPEC section 6 finding #6). Run this locally before a release.

.PARAMETER BaseUrl
LM Studio's OpenAI-compatible endpoint.

.PARAMETER Model
The model name currently loaded in LM Studio. Required -LM Studio has no single default model.

.PARAMETER Instructions
The task instructions to send to the agent. Defaults to a task that exercises all four tools.

.PARAMETER Cwd
Working directory the agent operates in. Defaults to a fresh temp directory so nothing in your
repository can be touched by accident.

.EXAMPLE
./scripts/smoke-lmstudio.ps1 -Model "qwen2.5-coder-7b-instruct"
#>
param(
    [string]$BaseUrl = "http://localhost:1234/v1",
    [Parameter(Mandatory = $true)]
    [string]$Model,
    [string]$Instructions = "Create a file named smoke-test.txt containing the text 'hello from the agent', then read the file back and report its exact contents. Then run a shell command that prints the current date.",
    [string]$Cwd = (Join-Path ([System.IO.Path]::GetTempPath()) ("agent-smoke-" + [Guid]::NewGuid())),
    [int]$MaxTurns = 15,
    [int]$ContextLimitTokens = 8192
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot

if (-not (Test-Path $Cwd)) {
    New-Item -ItemType Directory -Path $Cwd | Out-Null
}

Write-Host "Publishing the Native AOT single-file executable (dotnet publish -c Release)..." -ForegroundColor Cyan
& dotnet publish (Join-Path $repoRoot "src\xDreamer.Agent\xDreamer.Agent.csproj") -c Release
if ($LASTEXITCODE -ne 0) {
    throw "dotnet publish failed with exit code $LASTEXITCODE. On Windows this usually means the VC++ build tools aren't on PATH -see the comment in src/xDreamer.Agent/xDreamer.Agent.csproj."
}

$exeCandidates = @(Get-ChildItem -Path (Join-Path $repoRoot "src\xDreamer.Agent\bin") -Recurse -File -Include "xDreamer.Agent.exe", "xDreamer.Agent" -ErrorAction SilentlyContinue |
    Where-Object { $_.DirectoryName -like "*\publish" } |
    Sort-Object LastWriteTime -Descending)
if ($exeCandidates.Count -eq 0) {
    throw "Could not find a published xDreamer.Agent executable under src/xDreamer.Agent/bin/**/publish/."
}
$exePath = $exeCandidates[0].FullName

Write-Host "Starting the published agent: $exePath" -ForegroundColor Cyan
Write-Host "Task cwd: $Cwd" -ForegroundColor Cyan

# Windows PowerShell 5.1's ProcessStartInfo predates StandardInputEncoding, and its StreamWriter
# default (UTF-8 *with* BOM) would prepend a BOM the agent's UTF-8 decoder doesn't strip, breaking
# the first line's JSON. Write raw UTF-8-no-BOM bytes directly to the stdin base stream instead.
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $exePath
$psi.RedirectStandardInput = $true
$psi.RedirectStandardOutput = $true
$psi.StandardOutputEncoding = $utf8NoBom
$psi.UseShellExecute = $false
$process = [System.Diagnostics.Process]::Start($psi)

function Send-NdjsonLine([string]$Json) {
    $bytes = $utf8NoBom.GetBytes($Json + "`n")
    $process.StandardInput.BaseStream.Write($bytes, 0, $bytes.Length)
    $process.StandardInput.BaseStream.Flush()
}

$taskMessage = @{
    type         = "task"
    task_id      = [Guid]::NewGuid().ToString()
    instructions = $Instructions
    cwd          = $Cwd
    config       = @{
        llm                  = @{ base_url = $BaseUrl; model = $Model }
        max_turns            = $MaxTurns
        context_limit_tokens = $ContextLimitTokens
    }
} | ConvertTo-Json -Depth 10 -Compress

Send-NdjsonLine $taskMessage
Write-Host ">> $taskMessage" -ForegroundColor DarkGray

$lastMessage = $null
while (-not $process.StandardOutput.EndOfStream) {
    $line = $process.StandardOutput.ReadLine()
    if ([string]::IsNullOrWhiteSpace($line)) { continue }
    Write-Host "<< $line" -ForegroundColor Gray

    $message = $line | ConvertFrom-Json
    $lastMessage = $message

    if ($message.type -eq "permission_request") {
        $answer = Read-Host "Allow $($message.tool) call $($message.id)? [y/N]"
        $decision = if ($answer -match '^[Yy]') { "allow" } else { "deny" }
        $response = @{ type = "permission_response"; id = $message.id; decision = $decision } | ConvertTo-Json -Compress
        Send-NdjsonLine $response
        Write-Host ">> $response" -ForegroundColor DarkGray
    }
    elseif ($message.type -eq "task_complete") {
        break
    }
}

$process.StandardInput.Close()
$process.WaitForExit()

Write-Host ""
if ($null -eq $lastMessage -or $lastMessage.type -ne "task_complete") {
    Write-Host "Smoke test FAILED: agent exited without emitting task_complete (exit code $($process.ExitCode))." -ForegroundColor Red
    exit 1
}
elseif ($lastMessage.result -eq "success") {
    Write-Host "Smoke test PASSED: task completed successfully." -ForegroundColor Green
    Write-Host "Summary: $($lastMessage.summary)"
}
else {
    Write-Host "Smoke test FAILED: $($lastMessage.error.code) - $($lastMessage.error.message)" -ForegroundColor Red
}

exit $process.ExitCode
