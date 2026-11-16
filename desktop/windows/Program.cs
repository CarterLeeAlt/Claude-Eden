using System.IO.Pipes;
using System.Security.Cryptography;
using System.Security.Principal;
using System.Text;
using System.Text.Json;
using System.Runtime.InteropServices;

namespace Eden.Windows;

internal static class Program
{
    public static string? SmokeOutput { get; private set; }
    [STAThread]
    private static int Main(string[] args)
    {
        if (args is ["--install-update"]) return UpdateInstaller.Run().GetAwaiter().GetResult();
        if (args is ["--smoke-test", var output] && Path.IsPathFullyQualified(output))
        {
            SmokeOutput = output;
            if (File.Exists(output)) File.Delete(output);
        }
        else if (args.Length != 0) return 2;
        ApplicationConfiguration.Initialize();
        using var fonts = new AppFonts();
        Application.SetDefaultFont(fonts.Regular);
        AppDomain.CurrentDomain.UnhandledException += (_, e) => Backend.Report(e.ExceptionObject is Exception error ? error.ToString() : "Unhandled native exception").GetAwaiter().GetResult();
        Application.ThreadException += (_, e) => {
            Backend.Report(e.Exception.ToString()).GetAwaiter().GetResult();
            if (AppDialog.ShowMessage(null, "程序发生错误：\n" + e.Exception + "\n\n是否退出 Claude Eden？", "Claude Eden 错误", confirmation: true) == DialogResult.OK) Application.Exit();
        };
        TaskScheduler.UnobservedTaskException += (_, e) => _ = Backend.Report(e.Exception.ToString());
        if (RuntimeInformation.OSArchitecture != Architecture.X64 || !OperatingSystem.IsWindowsVersionAtLeast(10))
        {
            AppDialog.ShowMessage(null, "此 Claude Eden 安装包需要 Windows 10 的 x64（Intel/AMD）系统。");
            return 1;
        }
        var kernel = NativeLibrary.Load("kernel32.dll");
        try
        {
            foreach (var api in new[] { "CreatePseudoConsole", "ResizePseudoConsole", "ClosePseudoConsole" })
            {
                if (NativeLibrary.TryGetExport(kernel, api, out _)) continue;
                AppDialog.ShowMessage(null, $"Windows 缺少终端所需的 {api} 接口。请更新到提供 ConPTY 的 Windows 10 版本（微软从 1809 起提供该接口）。");
                return 1;
            }
        }
        finally { NativeLibrary.Free(kernel); }
        var sid = WindowsIdentity.GetCurrent().User?.Value ?? Environment.UserName;
        var key = Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes(sid + "\0" + Path.GetFullPath(Backend.Root).TrimEnd(Path.DirectorySeparatorChar).ToUpperInvariant())))[..24];
        using var mutex = new Mutex(true, @"Local\Eden-" + key, out var owner);
        if (!owner)
        {
            if (SmokeOutput != null) return 3;
            try
            {
                using var client = new NamedPipeClientStream(".", "Eden-" + key, PipeDirection.Out, PipeOptions.CurrentUserOnly);
                client.Connect(3000); client.WriteByte(1);
            }
            catch (IOException) { AppDialog.ShowMessage(null, "Claude Eden 已在运行，但窗口暂时无法唤起。请从任务栏打开 Claude Eden。"); }
            catch (TimeoutException) { AppDialog.ShowMessage(null, "Claude Eden 正在启动，请稍候。"); }
            return 0;
        }
        using var form = new MainWindow();
        using var cancellation = new CancellationTokenSource();
        _ = Task.Run(async () =>
        {
            while (!cancellation.IsCancellationRequested)
            {
                try
                {
                    await using var pipe = new NamedPipeServerStream("Eden-" + key, PipeDirection.In, 1,
                        PipeTransmissionMode.Byte, PipeOptions.Asynchronous | PipeOptions.CurrentUserOnly);
                    await pipe.WaitForConnectionAsync(cancellation.Token);
                    var data = new byte[1];
                    if (await pipe.ReadAsync(data, cancellation.Token) == 1 && data[0] == 1 && form.IsHandleCreated)
                        form.BeginInvoke(form.RestoreWindow);
                }
                catch (OperationCanceledException) { break; }
                catch (IOException) { }
            }
        });
        Application.Run(form); cancellation.Cancel();
        if (SmokeOutput == null) return 0;
        try
        {
            using var report = JsonDocument.Parse(File.ReadAllText(SmokeOutput));
            return report.RootElement.TryGetProperty("success", out var success) && success.ValueKind == JsonValueKind.True ? 0 : 1;
        }
        catch (Exception error) when (error is IOException or UnauthorizedAccessException or JsonException) { return 1; }
    }
}
