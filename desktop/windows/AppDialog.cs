namespace Eden.Windows;

internal sealed class AppDialog : Form
{
    internal AppDialog(string message, string title, bool confirmation = false)
    {
        AutoScaleMode = AutoScaleMode.Dpi;
        AutoScaleDimensions = new SizeF(96, 96);
        Text = title;
        Font = Control.DefaultFont;
        BackColor = Color.White;
        ForeColor = Color.FromArgb(26, 28, 31);
        FormBorderStyle = FormBorderStyle.FixedDialog;
        StartPosition = FormStartPosition.CenterParent;
        ShowInTaskbar = false;
        MinimizeBox = MaximizeBox = false;
        AutoSize = true;
        AutoSizeMode = AutoSizeMode.GrowAndShrink;
        Padding = new Padding(24);
        var layout = new TableLayoutPanel { AutoSize = true, AutoSizeMode = AutoSizeMode.GrowAndShrink, ColumnCount = 1, Dock = DockStyle.Fill };
        var text = new Label { Text = message, AutoSize = true, MaximumSize = new Size(600, 0), Margin = Padding.Empty, UseMnemonic = false };
        var content = new Panel { AutoScroll = true, Width = Math.Clamp(text.PreferredSize.Width + 20, 320, 620), Margin = new Padding(0, 0, 0, 24) };
        text.MaximumSize = new Size(content.Width - 20, 0);
        content.Height = Math.Min(360, text.PreferredSize.Height);
        content.Controls.Add(text);
        var buttons = new FlowLayoutPanel { AutoSize = true, AutoSizeMode = AutoSizeMode.GrowAndShrink, FlowDirection = FlowDirection.RightToLeft, Dock = DockStyle.Fill, Margin = Padding.Empty };
        var accept = new Button { Text = "确定", AutoSize = true, MinimumSize = new Size(84, 34), DialogResult = DialogResult.OK };
        buttons.Controls.Add(accept);
        AcceptButton = accept;
        if (confirmation)
        {
            var cancel = new Button { Text = "取消", AutoSize = true, MinimumSize = new Size(84, 34), DialogResult = DialogResult.Cancel };
            buttons.Controls.Add(cancel);
            CancelButton = cancel;
        }
        else CancelButton = accept;
        layout.Controls.Add(content);
        layout.Controls.Add(buttons);
        Controls.Add(layout);
    }

    internal static DialogResult ShowMessage(IWin32Window? owner, string message, string title = "Claude Eden", bool confirmation = false)
    {
        using var dialog = new AppDialog(message, title, confirmation);
        return dialog.ShowDialog(owner);
    }

    protected override void OnHandleCreated(EventArgs e)
    {
        base.OnHandleCreated(e);
        new WindowChrome(BackColor, ForeColor).Apply(Handle, null);
    }
}
