using System.Text;

namespace xDreamer.Agent.Transport;

/// <summary>Line framing/reassembly over stdin/stdout for the NDJSON wire protocol: one JSON object per line.</summary>
public sealed class NdjsonStdio
{
    private static readonly byte[] Utf8Bom = [0xEF, 0xBB, 0xBF];

    private readonly Stream _input;
    private readonly Stream _output;
    private readonly byte[] _readBuffer = new byte[4096];
    private readonly List<byte> _pending = [];
    private bool _inputEnded;
    private bool _bomChecked;

    public NdjsonStdio(Stream input, Stream output)
    {
        _input = input;
        _output = output;
    }

    /// <summary>Reads the next complete NDJSON line, reassembling it across however many partial reads the
    /// underlying stream needed to deliver it. Returns null at end of stream once no pending data remains.</summary>
    public async Task<string?> ReadLineAsync(CancellationToken cancellationToken = default)
    {
        while (true)
        {
            StripLeadingBomOnce();

            int newlineIndex = _pending.IndexOf((byte)'\n');
            if (newlineIndex >= 0)
            {
                string line = DecodeLine(_pending, newlineIndex);
                _pending.RemoveRange(0, newlineIndex + 1);
                return line;
            }

            if (_inputEnded)
            {
                if (_pending.Count == 0)
                {
                    return null;
                }

                string remainder = DecodeLine(_pending, _pending.Count);
                _pending.Clear();
                return remainder;
            }

            int read = await _input.ReadAsync(_readBuffer.AsMemory(0, _readBuffer.Length), cancellationToken).ConfigureAwait(false);
            if (read == 0)
            {
                _inputEnded = true;
                continue;
            }

            for (int i = 0; i < read; i++)
            {
                _pending.Add(_readBuffer[i]);
            }
        }
    }

    /// <summary>Some stdio clients (observed: PowerShell's Process.StandardInput, whose default StreamWriter
    /// emits a UTF-8 preamble on first flush regardless of how the caller writes to it) prepend a UTF-8 BOM
    /// to the very first bytes of the stream. Strip it once, transparently, rather than requiring every
    /// orchestrator to avoid it.</summary>
    private void StripLeadingBomOnce()
    {
        if (_bomChecked)
        {
            return;
        }

        if (_pending.Count < Utf8Bom.Length && !_inputEnded)
        {
            return;
        }

        _bomChecked = true;
        if (_pending.Count >= Utf8Bom.Length
            && _pending[0] == Utf8Bom[0] && _pending[1] == Utf8Bom[1] && _pending[2] == Utf8Bom[2])
        {
            _pending.RemoveRange(0, Utf8Bom.Length);
        }
    }

    /// <summary>Writes one NDJSON line (a single JSON object followed by a newline) to the output stream.</summary>
    public void WriteLine(string json)
    {
        byte[] bytes = Encoding.UTF8.GetBytes(json + "\n");
        _output.Write(bytes, 0, bytes.Length);
        _output.Flush();
    }

    private static string DecodeLine(List<byte> buffer, int length)
    {
        if (length > 0 && buffer[length - 1] == (byte)'\r')
        {
            length--;
        }

        byte[] bytes = new byte[length];
        buffer.CopyTo(0, bytes, 0, length);
        return Encoding.UTF8.GetString(bytes);
    }
}
