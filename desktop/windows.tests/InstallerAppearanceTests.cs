using System.Diagnostics;
using System.Drawing;
using System.Drawing.Imaging;
using System.Runtime.InteropServices;
using System.Text;
using System.Text.Json;

internal static class InstallerAppearanceTests
{
    private delegate bool EnumWindow(IntPtr window, IntPtr state);
    [DllImport("user32.dll")] private static extern bool EnumWindows(EnumWindow callback, IntPtr state);
    [DllImport("user32.dll")] private static extern bool EnumChildWindows(IntPtr parent, EnumWindow callback, IntPtr state);
    [DllImport("user32.dll")] private static extern uint GetWindowThreadProcessId(IntPtr window, out uint process);
    [DllImport("user32.dll")] private static extern bool IsWindowVisible(IntPtr window);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] private static extern int GetWindowTextW(IntPtr window, StringBuilder text, int size);
    [DllImport("user32.dll")] private static extern IntPtr GetDlgItem(IntPtr parent, int id);
    [DllImport("user32.dll")] private static extern IntPtr SendMessageW(IntPtr window, uint message, IntPtr arg, IntPtr data);
    [DllImport("user32.dll")] private static extern bool PostMessageW(IntPtr window, uint message, IntPtr arg, IntPtr data);
    [DllImport("user32.dll")] private static extern bool GetWindowRect(IntPtr window, out RECT rect);
    [DllImport("user32.dll")] private static extern bool PrintWindow(IntPtr window, IntPtr dc, uint flags);
    [DllImport("gdi32.dll", CharSet = CharSet.Unicode)] private static extern int GetObjectW(IntPtr font, int size, out LOGFONT value);
    [StructLayout(LayoutKind.Sequential)] private struct RECT { public int Left, Top, Right, Bottom; }
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)] private struct LOGFONT {
        public int Height, Width, Escapement, Orientation, Weight;
        public byte Italic, Underline, StrikeOut, CharacterSet, OutPrecision, ClipPrecision, Quality, PitchAndFamily;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 32)] public string Face;
    }
    private sealed record ControlFont(string Text, string Face, int Height, int Weight);

    internal static void Run(string executable, string output)
    {
        var workspace = Environment.GetEnvironmentVariable("EDEN_WORKSPACE_ROOT") ?? throw new Exception("Use the workspace runner.");
        var temporary = Path.Combine(workspace, ".tmp") + Path.DirectorySeparatorChar;
        executable = Path.GetFullPath(executable); output = Path.GetFullPath(output);
        if (!executable.StartsWith(temporary, StringComparison.OrdinalIgnoreCase) || !output.StartsWith(temporary, StringComparison.OrdinalIgnoreCase))
            throw new Exception("Use a compile-only installer fixture and output inside workspace .tmp.");
        Directory.CreateDirectory(output);
        var target = Path.Combine(output, "unused-install-target");
        if (Directory.Exists(target)) throw new Exception("Installer preview target must not exist.");
        using var process = Process.Start(new ProcessStartInfo(executable) { UseShellExecute = false, ArgumentList = { "/D=" + target } }) ?? throw new Exception("Could not start installer preview.");
        var records = new Dictionary<string, ControlFont[]>();
        var errors = new List<string>();
        IntPtr main = IntPtr.Zero;
        try
        {
            Wait(() => (main = Windows(process.Id).FirstOrDefault()) != IntPtr.Zero, process);
            for (int page = 0; page < 4; page++)
            {
                var label = "Page" + page;
                records[label] = Inspect(main);
                Capture(main, output, label);
                if (page == 3) break;
                var next = GetDlgItem(main, 1);
                if (Text(next).Contains("Install", StringComparison.OrdinalIgnoreCase) || Text(next).Contains("安装"))
                    throw new Exception("Refusing to activate an Install button during font preview.");
                var before = string.Join("\n", records[label].Select(item => item.Text));
                if (!PostMessageW(next, 0xf5, IntPtr.Zero, IntPtr.Zero)) throw new Exception("Could not activate installer Next.");
                Wait(() => string.Join("\n", Inspect(main).Select(item => item.Text)) != before, process);
            }
            PostMessageW(main, 0x10, IntPtr.Zero, IntPtr.Zero);
            IntPtr confirmation = IntPtr.Zero;
            Wait(() => (confirmation = Windows(process.Id).FirstOrDefault(window => window != main)) != IntPtr.Zero, process);
            records["Cancel"] = Inspect(confirmation);
            if (!records["Cancel"].Any(item => item.Text.Contains("退出") || item.Text.Contains("quit", StringComparison.OrdinalIgnoreCase)))
                throw new Exception("Cancel confirmation is missing its question text.");
            Capture(confirmation, output, "Cancel");
            var yes = GetDlgItem(confirmation, 6);
            if (yes == IntPtr.Zero) throw new Exception("Cancel confirmation has no Yes button.");
            PostMessageW(yes, 0xf5, IntPtr.Zero, IntPtr.Zero);
            if (!process.WaitForExit(10000)) throw new Exception("Installer preview did not exit after cancel.");
            if (Directory.Exists(target)) throw new Exception("Preview unexpectedly created an install directory.");
            foreach (var (stage, controls) in records)
                foreach (var control in controls)
                    if (control.Face != "Eden UI") errors.Add($"{stage}: {control.Text} uses {control.Face}");
            File.WriteAllText(Path.Combine(output, "fonts.json"), JsonSerializer.Serialize(records, new JsonSerializerOptions { WriteIndented = true }));
            if (errors.Count > 0) throw new Exception(string.Join("\n", errors));
            Console.WriteLine("Installer appearance: four real pages, private fonts, cancel dialog and clean exit passed; installation was not started.");
        }
        finally { if (!process.HasExited) { process.Kill(true); process.WaitForExit(); } }
    }

    private static void Wait(Func<bool> condition, Process process)
    {
        var limit = Stopwatch.StartNew();
        while (!condition())
        {
            if (process.HasExited || limit.Elapsed > TimeSpan.FromSeconds(10)) throw new Exception("Installer preview stopped or timed out.");
            Thread.Sleep(50);
        }
        Thread.Sleep(150); // Allow the just-created native page to finish its first paint.
    }
    private static List<IntPtr> Windows(int process)
    {
        var windows = new List<IntPtr>();
        EnumWindows((window, _) => { GetWindowThreadProcessId(window, out var owner); if (owner == process && IsWindowVisible(window)) windows.Add(window); return true; }, IntPtr.Zero);
        return windows;
    }
    private static string Text(IntPtr window) { var text = new StringBuilder(4096); GetWindowTextW(window, text, text.Capacity); return text.ToString(); }
    private static ControlFont[] Inspect(IntPtr parent)
    {
        var values = new List<ControlFont>();
        EnumChildWindows(parent, (window, _) => {
            var text = Text(window);
            if (!IsWindowVisible(window) || text.Length == 0) return true;
            var font = SendMessageW(window, 0x31, IntPtr.Zero, IntPtr.Zero);
            if (font != IntPtr.Zero && GetObjectW(font, Marshal.SizeOf<LOGFONT>(), out var value) > 0)
                values.Add(new(text, value.Face, value.Height, value.Weight));
            return true;
        }, IntPtr.Zero);
        if (values.Count == 0) throw new Exception("No visible installer text controls were inspected.");
        return values.ToArray();
    }
    private static void Capture(IntPtr window, string output, string stage)
    {
        GetWindowRect(window, out var rect);
        using var bitmap = new Bitmap(rect.Right - rect.Left, rect.Bottom - rect.Top);
        using var graphics = Graphics.FromImage(bitmap);
        var dc = graphics.GetHdc();
        try { if (!PrintWindow(window, dc, 2)) throw new Exception("Could not capture installer window."); }
        finally { graphics.ReleaseHdc(dc); }
        bitmap.Save(Path.Combine(output, $"UI_CN_Noto_Sans_SC_EN_Inter_14px_Light_Installer_{stage}.png"), ImageFormat.Png);
    }
}
