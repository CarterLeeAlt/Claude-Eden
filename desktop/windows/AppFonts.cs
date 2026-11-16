using System.Drawing.Text;
using System.Reflection;
using System.Runtime.InteropServices;

namespace Eden.Windows;

internal sealed class AppFonts : IDisposable
{
    private readonly PrivateFontCollection collection = new();
    private readonly List<IntPtr> buffers = [];
    private readonly List<IntPtr> registrations = [];
    internal Font Regular { get; }

    [DllImport("gdi32.dll", SetLastError = true)]
    private static extern IntPtr AddFontMemResourceEx(IntPtr font, uint size, IntPtr reserved, out uint count);
    [DllImport("gdi32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool RemoveFontMemResourceEx(IntPtr font);

    internal AppFonts()
    {
        foreach (var style in new[] { "Regular", "Bold" })
        {
            using var stream = Assembly.GetExecutingAssembly().GetManifestResourceStream($"Eden.Fonts.EdenUI-{style}.ttf")
                ?? throw new InvalidDataException("缺少内置界面字体，请重新安装 Claude Eden。");
            using var data = new MemoryStream();
            stream.CopyTo(data);
            var bytes = data.ToArray();
            var memory = Marshal.AllocCoTaskMem(bytes.Length);
            buffers.Add(memory);
            Marshal.Copy(bytes, 0, memory, bytes.Length);
            var registration = AddFontMemResourceEx(memory, (uint)bytes.Length, IntPtr.Zero, out var count);
            if (registration == IntPtr.Zero || count == 0)
                throw new InvalidDataException("无法加载内置界面字体，请重新安装 Claude Eden。");
            registrations.Add(registration);
            collection.AddMemoryFont(memory, bytes.Length);
        }
        var family = collection.Families.Single(font => font.Name == "Eden UI");
        Regular = new Font(family, 10.5f, FontStyle.Regular, GraphicsUnit.Point); // 14 px at 96 DPI.
    }

    public void Dispose()
    {
        Regular.Dispose();
        collection.Dispose();
        foreach (var registration in registrations) RemoveFontMemResourceEx(registration);
        foreach (var memory in buffers) Marshal.FreeCoTaskMem(memory);
    }
}
