using System.Text;
using Agent.Transport;
using Xunit;

namespace Agent.Tests.Messages;

[Trait("Category", "Unit")]
public class NdjsonStdioTests
{
    [Fact]
    public async Task ReadLine_ReassemblesMessageSplitAcrossMultiplePartialReads()
    {
        string json = "{\"type\":\"task\",\"task_id\":\"abc-123\",\"instructions\":\"do something long enough to span several partial reads\"}";
        using var input = new SingleByteAtATimeStream(Encoding.UTF8.GetBytes(json + "\n"));
        using var output = new MemoryStream();
        var stdio = new NdjsonStdio(input, output);

        string? line = await stdio.ReadLineAsync(TestContext.Current.CancellationToken);

        Assert.Equal(json, line);
    }

    [Fact]
    public async Task ReadLine_ReturnsExactlyOneJsonObjectPerLine()
    {
        string first = "{\"type\":\"task\",\"task_id\":\"1\"}";
        string second = "{\"type\":\"permission_response\",\"id\":\"call-1\",\"decision\":\"allow\"}";
        using var input = new MemoryStream(Encoding.UTF8.GetBytes($"{first}\n{second}\n"));
        using var output = new MemoryStream();
        var stdio = new NdjsonStdio(input, output);

        string? firstLine = await stdio.ReadLineAsync(TestContext.Current.CancellationToken);
        string? secondLine = await stdio.ReadLineAsync(TestContext.Current.CancellationToken);
        string? thirdLine = await stdio.ReadLineAsync(TestContext.Current.CancellationToken);

        Assert.Equal(first, firstLine);
        Assert.Equal(second, secondLine);
        Assert.Null(thirdLine);
    }

    [Fact]
    public async Task ReadLine_StripsLeadingUtf8Bom()
    {
        // Some stdio clients (observed: PowerShell's Process.StandardInput) prepend a UTF-8 BOM
        // to the very first bytes of the stream regardless of how the caller writes to it.
        byte[] bom = [0xEF, 0xBB, 0xBF];
        string json = "{\"type\":\"task\"}";
        using var input = new MemoryStream([.. bom, .. Encoding.UTF8.GetBytes(json + "\n")]);
        using var output = new MemoryStream();
        var stdio = new NdjsonStdio(input, output);

        string? line = await stdio.ReadLineAsync(TestContext.Current.CancellationToken);

        Assert.Equal(json, line);
    }

    [Fact]
    public async Task ReadLine_NoBom_ReadsNormally()
    {
        string json = "{\"type\":\"task\"}";
        using var input = new MemoryStream(Encoding.UTF8.GetBytes(json + "\n"));
        using var output = new MemoryStream();
        var stdio = new NdjsonStdio(input, output);

        string? line = await stdio.ReadLineAsync(TestContext.Current.CancellationToken);

        Assert.Equal(json, line);
    }

    [Fact]
    public async Task ReadLine_ReturnsFinalLineEvenWithoutTrailingNewline()
    {
        using var input = new MemoryStream(Encoding.UTF8.GetBytes("""{"type":"task"}"""));
        using var output = new MemoryStream();
        var stdio = new NdjsonStdio(input, output);

        string? line = await stdio.ReadLineAsync(TestContext.Current.CancellationToken);

        Assert.Equal("""{"type":"task"}""", line);
    }

    [Fact]
    public async Task ReadLine_TrimsTrailingCarriageReturn_ForCrlfLineEndings()
    {
        using var input = new MemoryStream(Encoding.UTF8.GetBytes("{\"a\":1}\r\n"));
        using var output = new MemoryStream();
        var stdio = new NdjsonStdio(input, output);

        string? line = await stdio.ReadLineAsync(TestContext.Current.CancellationToken);

        Assert.Equal("{\"a\":1}", line);
    }

    [Fact]
    public void WriteLine_WritesUtf8JsonPlusNewlineToOutputStream()
    {
        using var input = new MemoryStream();
        using var output = new MemoryStream();
        var stdio = new NdjsonStdio(input, output);

        stdio.WriteLine("""{"type":"tool_result"}""");

        Assert.Equal("{\"type\":\"tool_result\"}\n", Encoding.UTF8.GetString(output.ToArray()));
    }

    /// <summary>Test double simulating a stdin pipe that only ever delivers one byte per read,
    /// regardless of the buffer size requested — forces reassembly across many partial reads.</summary>
    private sealed class SingleByteAtATimeStream(byte[] data) : Stream
    {
        private int _position;

        public override bool CanRead => true;
        public override bool CanSeek => false;
        public override bool CanWrite => false;
        public override long Length => data.Length;
        public override long Position { get => _position; set => throw new NotSupportedException(); }

        public override int Read(byte[] buffer, int offset, int count)
        {
            if (_position >= data.Length)
            {
                return 0;
            }

            buffer[offset] = data[_position];
            _position++;
            return 1;
        }

        public override Task<int> ReadAsync(byte[] buffer, int offset, int count, CancellationToken cancellationToken)
            => Task.FromResult(Read(buffer, offset, count));

        public override ValueTask<int> ReadAsync(Memory<byte> buffer, CancellationToken cancellationToken = default)
        {
            if (_position >= data.Length)
            {
                return ValueTask.FromResult(0);
            }

            buffer.Span[0] = data[_position];
            _position++;
            return ValueTask.FromResult(1);
        }

        public override void Flush() { }
        public override long Seek(long offset, SeekOrigin origin) => throw new NotSupportedException();
        public override void SetLength(long value) => throw new NotSupportedException();
        public override void Write(byte[] buffer, int offset, int count) => throw new NotSupportedException();
    }
}
