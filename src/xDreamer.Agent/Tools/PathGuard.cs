namespace xDreamer.Agent.Tools;

/// <summary>Shared cwd-containment check (§6 audit finding #2), used only by WriteFileTool/EditFileTool.
/// read_file remains intentionally unrestricted — it has no path_outside_cwd error in its contract.</summary>
public static class PathGuard
{
    public static void EnsureWithinCwd(string path, string cwd)
    {
        string fullCwd = Path.GetFullPath(cwd).TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
        string fullPath = Path.GetFullPath(path, fullCwd);

        StringComparison comparison = OperatingSystem.IsWindows() ? StringComparison.OrdinalIgnoreCase : StringComparison.Ordinal;
        string cwdWithSeparator = fullCwd + Path.DirectorySeparatorChar;

        bool isWithinCwd = fullPath.Equals(fullCwd, comparison) || fullPath.StartsWith(cwdWithSeparator, comparison);
        if (!isWithinCwd)
        {
            throw new PathOutsideCwdException($"Path '{path}' resolves outside of cwd '{cwd}'");
        }
    }
}

public sealed class PathOutsideCwdException(string message) : Exception(message);
