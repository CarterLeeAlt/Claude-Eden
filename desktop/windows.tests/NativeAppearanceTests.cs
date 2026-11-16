using System.Drawing;
using System.Drawing.Imaging;
using System.Runtime.InteropServices;
using System.Text;
using System.Windows.Forms;
using Eden.Windows;

internal static class NativeAppearanceTests
{
    [DllImport("gdi32.dll", CharSet = CharSet.Unicode)]
    private static extern int GetTextFaceW(IntPtr dc, int count, StringBuilder name);
    [DllImport("gdi32.dll", CharSet = CharSet.Unicode)]
    private static extern uint GetGlyphIndicesW(IntPtr dc, string text, int count, [Out] ushort[] indices, uint flags);
    [DllImport("gdi32.dll")]
    private static extern IntPtr SelectObject(IntPtr dc, IntPtr value);
    [DllImport("gdi32.dll")]
    private static extern bool DeleteObject(IntPtr value);
    [DllImport("user32.dll")]
    private static extern bool PostMessageW(IntPtr window, uint message, IntPtr key, IntPtr data);
    [DllImport("user32.dll")]
    private static extern IntPtr SendMessageW(IntPtr window, uint message, IntPtr key, IntPtr data);

    internal static void Run(string output)
    {
        var workspace = Environment.GetEnvironmentVariable("EDEN_WORKSPACE_ROOT") ?? throw new Exception("Use the workspace runner.");
        output = Path.GetFullPath(output);
        if (!output.StartsWith(Path.Combine(workspace, ".tmp") + Path.DirectorySeparatorChar, StringComparison.OrdinalIgnoreCase))
            throw new Exception("Native appearance screenshots must be inside the workspace .tmp directory.");
        Directory.CreateDirectory(output);
        Exception? failure = null;
        var thread = new Thread(() => {
            try { RunUI(output); } catch (Exception error) { failure = error; }
        });
        thread.SetApartmentState(ApartmentState.STA);
        thread.Start();
        if (!thread.Join(TimeSpan.FromSeconds(30))) throw new Exception("Native appearance test exceeded 30 seconds.");
        if (failure != null) throw failure;
        Console.WriteLine("Native appearance: private GDI/GDI+ fonts, glyph coverage, labels, menu, dialogs and Enter/Escape passed.");
    }

    private static void RunUI(string output)
    {
        Application.SetHighDpiMode(HighDpiMode.PerMonitorV2);
        Application.EnableVisualStyles();
        using var fonts = new AppFonts();
        Application.SetDefaultFont(fonts.Regular);
        const string text = "Claude Eden 2186 中文繁體日本語 Français Deutsch";
        foreach (var style in new[] { FontStyle.Regular, FontStyle.Bold })
        {
            using var font = new Font(fonts.Regular.FontFamily, 10.5f, style);
            using var bitmap = new Bitmap(1000, 120);
            using var graphics = Graphics.FromImage(bitmap);
            var dc = graphics.GetHdc();
            var handle = font.ToHfont();
            var previous = SelectObject(dc, handle);
            try
            {
                var name = new StringBuilder(64);
                if (GetTextFaceW(dc, name.Capacity, name) == 0 || name.ToString() != "Eden UI") throw new Exception("GDI fell back to a system font.");
                var glyphs = new ushort[text.Length];
                if (GetGlyphIndicesW(dc, text, text.Length, glyphs, 1) == uint.MaxValue || glyphs.Contains((ushort)0xffff)) throw new Exception("Bundled font lacks a required glyph.");
            }
            finally { SelectObject(dc, previous); DeleteObject(handle); graphics.ReleaseHdc(dc); }
        }

        using var owner = new Form { Text = "Claude Eden 字体验收", ClientSize = new Size(640, 170), StartPosition = FormStartPosition.CenterScreen, BackColor = Color.White, AutoScaleMode = AutoScaleMode.Dpi };
        owner.Controls.Add(new Label { Text = "正在启动 Claude Eden 工作台…\n中文 Noto Sans SC · 英文 Inter · 14 px", AutoSize = true, Location = new Point(24, 24) });
        owner.Show();
        Application.DoEvents();
        Capture(owner, output, "UI_CN_Noto_Sans_SC_EN_Inter_14px_Light_Native.png");
        using (var menu = new TrayMenu())
        {
            menu.Items.Add("打开 Claude Eden");
            menu.Items.Add("检查应用更新…");
            menu.Items.Add("退出 Claude Eden…");
            new WindowChrome(Color.White, Color.Black).ApplyMenu(menu);
            menu.Show(owner, new Point(16, 80));
            Application.DoEvents();
            foreach (ToolStripItem item in menu.Items)
                if (item.Font.FontFamily.Name != "Eden UI") throw new Exception("Tray item uses a system font.");
            Capture(menu, output, "UI_CN_Noto_Sans_SC_EN_Inter_14px_Light_Tray.png");
            menu.Close();
        }
        foreach (var (confirmation, key, expected, longText) in new[] {
            (false, 13, DialogResult.OK, false), (true, 13, DialogResult.OK, false), (true, 27, DialogResult.Cancel, false),
            (true, 27, DialogResult.Cancel, true),
        })
        {
            var message = "Claude Eden 2186\n请先保存编辑器中的文件并结束正在运行的工作。\n中文 Noto Sans SC；英文与数字 Inter。";
            if (longText) message = string.Join("\n", Enumerable.Repeat(message, 30));
            using var dialog = new AppDialog(message, "Claude Eden 提示", confirmation);
            Exception? callbackFailure = null;
            using var timer = new System.Windows.Forms.Timer { Interval = 200 };
            timer.Tick += (_, _) => {
                timer.Stop();
                try {
                    if (longText)
                    {
                        var content = dialog.Controls[0].Controls.OfType<Panel>().Single(panel => panel is not FlowLayoutPanel);
                        if (!content.VerticalScroll.Visible) throw new Exception("Long dialog text has no scroll path.");
                        SendMessageW(content.Handle, 0x20a, -120 << 16, IntPtr.Zero);
                        if (content.VerticalScroll.Value == 0) throw new Exception("Dialog did not scroll on wheel input.");
                    }
                    Capture(dialog, output, $"UI_CN_Noto_Sans_SC_EN_Inter_14px_Light_Dialog_{confirmation}_{key}_{(longText ? "Long" : "Short")}.png");
                    var control = dialog.ActiveControl ?? dialog;
                    if (!PostMessageW(control.Handle, 0x100, key, IntPtr.Zero) || !PostMessageW(control.Handle, 0x101, key, IntPtr.Zero)) throw new Exception("Could not send dialog key.");
                } catch (Exception error) { callbackFailure = error; dialog.Close(); }
            };
            dialog.Shown += (_, _) => timer.Start();
            var result = dialog.ShowDialog(owner);
            if (callbackFailure != null) throw callbackFailure;
            if (result != expected) throw new Exception($"Dialog key {key}: {result} instead of {expected}");
        }
        owner.Close();
    }

    private static void Capture(Control root, string output, string filename)
    {
        void Check(Control control)
        {
            if (control.Font.FontFamily.Name != "Eden UI") throw new Exception($"Unexpected font in {control.GetType().Name}: {control.Font.Name}");
            foreach (Control child in control.Controls)
            {
                if (control is not ScrollableControl { AutoScroll: true } && !control.ClientRectangle.Contains(child.Bounds)) throw new Exception($"Clipped {child.GetType().Name}: {child.Bounds} in {control.ClientRectangle}");
                Check(child);
            }
        }
        Check(root);
        using var bitmap = new Bitmap(root.Width, root.Height);
        root.DrawToBitmap(bitmap, new Rectangle(Point.Empty, bitmap.Size));
        bitmap.Save(Path.Combine(output, filename), ImageFormat.Png);
    }
}
