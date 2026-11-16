using System.Diagnostics;
using Eden.Windows;

internal static class BackendStartupTests
{
    public static async Task Run()
    {
        var root = Path.Combine(Path.GetTempPath(), "eden-startup-" + Guid.NewGuid());
        Directory.CreateDirectory(root);
        try
        {
            foreach (var mode in new[] { "slow", "cancel", "deadline", "failure" })
            {
                using var cancellation = new CancellationTokenSource();
                var marker = Path.Combine(root, mode + ".pid");
                var elapsed = Stopwatch.StartNew();
                var task = Backend.JsonAt(Environment.ProcessPath!, ["desktop", "--startup-fixture", mode, marker],
                    cancellation.Token, mode == "deadline" ? TimeSpan.FromSeconds(1) : null);
                if (mode is "slow" or "cancel")
                {
                    using var fixtureDeadline = new CancellationTokenSource(TimeSpan.FromSeconds(10));
                    while (!File.Exists(marker)) await Task.Delay(20, fixtureDeadline.Token);
                    using var process = Process.GetProcessById(int.Parse(File.ReadAllText(marker)));
                    _ = process.SafeHandle;
                    if (mode == "cancel") cancellation.Cancel();
                    if (mode == "slow")
                    {
                        var result = await task;
                        if (result.Exit != 0 || result.Data.Text("status") != "ready" || elapsed.Elapsed < TimeSpan.FromMinutes(3))
                            throw new Exception("Slow desktop startup did not wait for actual readiness.");
                    }
                    else await ExpectCancellation(task);
                    if (!process.HasExited) throw new Exception("Completed/cancelled helper remains running.");
                }
                else if (mode == "deadline") await ExpectCancellation(task);
                else if ((await task).Exit != 7) throw new Exception("Helper failure exit code was lost.");
                Console.WriteLine($"PASS desktop helper {mode}: {elapsed.Elapsed.TotalSeconds:F2}s");
            }
        }
        finally { Directory.Delete(root, true); }
    }

    private static async Task ExpectCancellation(Task<(int Exit, System.Text.Json.JsonElement Data)> task)
    {
        try { await task; }
        catch (OperationCanceledException) { return; }
        throw new Exception("Helper cancellation was not propagated.");
    }
}
