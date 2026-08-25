using System.Text;

namespace Agent.Transport;

/// <summary>Line framing/reassembly over stdin/stdout for the NDJSON wire protocol: one JSON object per line.</summary>
public sealed class NdjsonStdio
{
    private readonly Stream _input;
    private readonly Stream _output;
    private readonly byte[] _readBuffer = new byte[4096];
    private readonly List<byte> _pending = [];
    private bool _inputEnded;

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
