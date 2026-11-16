package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sylvinus/agent-vm/internal/diagnostics"
	"github.com/sylvinus/agent-vm/internal/eden"
	"github.com/sylvinus/agent-vm/internal/usererror"
)

type claudeDesktopState struct {
	Installed         bool    `json:"installed"`
	Version           string  `json:"version"`
	Running           bool    `json:"running"`
	Failed            bool    `json:"failed"`
	Managed           bool    `json:"managed"`
	NativeUserSession bool    `json:"native_user_session,omitempty"`
	DisplayScale      float64 `json:"display_scale,omitempty"`
	FailureReason     string  `json:"failure_reason,omitempty"`
}

const claudeDesktopSnapshot = `python3 - <<'WORKBENCH_DESKTOP_STATE'
import json, subprocess
from pathlib import Path
def run(*args):
    return subprocess.run(args, capture_output=True, text=True).stdout.strip()
package=run('dpkg-query','-W','-f=${Status}\n${Version}','claude-desktop').splitlines()
installed=bool(package and package[0]=='install ok installed')
state=run('systemctl','is-active','workbench-claude-desktop.service')
result=run('systemctl','show','--property=Result','--value','workbench-claude-desktop.service') if state=='failed' else ''
print(json.dumps({'installed':installed,'version':package[1] if installed else '', 'running':state=='active','failed':state=='failed','managed':Path('/run/workbench-desktop/keyring-ready').exists(),'failure_reason':'oom-kill' if result=='oom-kill' else ''}))
WORKBENCH_DESKTOP_STATE`

func (s *Server) claudeDesktopStatus(w http.ResponseWriter, r *http.Request) {
	if !s.vmRunning() {
		http.Error(w, "请先启动 Claude Eden 环境", 409)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	b, err := s.desktopStatus.read(ctx, s.lifetimeContext(), func(ctx context.Context) ([]byte, error) {
		ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		return s.guestQuick(ctx, claudeDesktopSnapshot)
	})
	var state claudeDesktopState
	if err != nil {
		_ = diagnostics.New(s.profile).Record("warn", "claude-desktop", fmt.Sprintf("read state: %v", err))
		http.Error(w, usererror.Wrap("读取 Claude Desktop 状态", err, "请确认环境仍在运行；重新连接后再打开应用。持续失败时导出诊断包。").Error(), 502)
		return
	}
	if json.Unmarshal(b, &state) != nil {
		http.Error(w, "读取 Claude Desktop 状态失败：环境返回的应用状态格式无效。请更新环境组件；仍失败时导出诊断包。", 502)
		return
	}
	jsonReply(w, state)
}

func (s *Server) claudeDesktopAction(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	if action != "install" && action != "update" && action != "start" && action != "stop" {
		http.NotFound(w, r)
		return
	}
	var input struct{}
	if !readJSON(w, r, &input) {
		return
	}
	if !s.vmRunning() {
		http.Error(w, "请先启动 Claude Eden 环境", 409)
		return
	}
	s.launch(w, "claude-desktop-"+action, func() error {
		s.desktopStatus.invalidate()
		defer s.desktopStatus.invalidate()
		switch action {
		case "install", "update":
			s.mu.Lock()
			allowed := s.toolsNetworkAllowedLocked()
			s.mu.Unlock()
			if !allowed {
				return errors.New("工作台正在退出，请稍后重试安装")
			}
			monitorContext, stopMonitor := context.WithCancel(s.operationContext())
			done := make(chan struct{})
			go func() {
				defer close(done)
				ticker := time.NewTicker(10 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-monitorContext.Done():
						return
					case <-ticker.C:
						s.monitorSafety()
					}
				}
			}()
			defer func() {
				stopMonitor()
				s.mu.Lock()
				if s.monitorCancel != nil {
					s.monitorCancel()
				}
				s.mu.Unlock()
				<-done
			}()
			s.setMessage("正在下载并安装 Claude Desktop，可在操作详情查看进度…")
			script := eden.ClaudeDesktopInstallScript()
			if action == "update" {
				script = "set -e\nsudo systemctl stop workbench-claude-desktop.service\n" + script
			}
			return RunMaintenanceScript(s.operationContext(), s.configPath, script, s.jobs.execution(), s.jobs.execution())
		case "start":
			return s.startClaudeDesktop()
		default:
			s.setMessage("正在关闭 Claude Desktop…")
			return s.runDesktopScript("sudo systemctl stop workbench-claude-desktop.service", 30*time.Second)
		}
	})
}

func (s *Server) startClaudeDesktop() error {
	defer func() { s.desktopViews.reset(); s.desktopViews.background(s.changeDesktopVisibility) }()
	s.desktopStatus.invalidate()
	defer s.desktopStatus.invalidate()
	s.setMessage("正在打开 Claude Desktop…")
	key, err := s.account.desktopKey()
	if err != nil {
		return err
	}
	defer clear(key)
	script := `test "$(dpkg-query -W -f='${Status}' claude-desktop 2>/dev/null)" = 'install ok installed' || { echo '请先安装 Claude Desktop。'; exit 1; }
` + eden.ClaudeDesktopProvisionScript()
	if err = s.runDesktopScript(script, 0); err != nil {
		return err
	}
	if err = s.runDesktopOperation("/usr/local/bin/workbench-claude-start", bytes.NewReader(key), 0); err != nil {
		// Cancellation owns the pending session as well as the SSH request.
		cleanup, finish := context.WithTimeout(context.Background(), 30*time.Second)
		defer finish()
		_, stopErr := s.guest(cleanup, "sudo systemctl stop workbench-claude-desktop.service")
		return errors.Join(err, stopErr)
	}
	return nil
}

func (s *Server) runDesktopScript(script string, timeout time.Duration) error {
	script, err := s.linuxTemplate(script)
	if err != nil {
		return err
	}
	return s.runDesktopOperation("/bin/bash -se", strings.NewReader(script), timeout)
}

func (s *Server) runDesktopOperation(program string, input io.Reader, timeout time.Duration) error {
	ctx, cancel := context.WithCancel(s.operationContext())
	defer cancel()
	runtimeLimit := "infinity"
	if timeout > 0 {
		var finish context.CancelFunc
		ctx, finish = context.WithTimeout(ctx, timeout)
		defer finish()
		runtimeLimit = strconv.FormatInt(int64(timeout.Seconds()), 10)
	}
	// A guest cgroup owns apt and all descendants, including across SSH loss.
	// The application's persistent service is separate from this operation.
	command := fmt.Sprintf("sudo systemd-run --quiet --pipe --wait --collect --unit=workbench-desktop-operation --property=User=developer --property=RuntimeMaxSec=%s --property=KillMode=control-group --setenv=HOME=/home/developer --working-directory=/home/developer %s", runtimeLimit, program)
	command, err := s.linuxTemplate(command)
	if err != nil {
		return err
	}
	cmd := s.sshCommand(ctx, "-T", s.sshTarget(), "--", command)
	cmd.Stdin = input
	cmd.Stdout, cmd.Stderr = s.jobs.execution(), s.jobs.execution()
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			cleanup, finish := context.WithTimeout(context.Background(), 5*time.Second)
			defer finish()
			_, _ = s.guest(cleanup, "sudo systemctl stop workbench-desktop-operation.service")
			return ctx.Err()
		}
		return executionFailure("执行 Claude Desktop 操作", err, s.jobs.snapshot().Log)
	}
	return nil
}

func (s *Server) claudeDesktopCredentials(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	b, err := s.guestQuick(ctx, `set -eu
systemctl is-active --quiet workbench-claude-desktop.service
password=$(cat "$HOME/.config/workbench/desktop-password")
printf '%s\n' "$password"
if test -f /run/workbench-desktop/base-display-scale; then cat /run/workbench-desktop/base-display-scale
elif test -f /run/workbench-desktop/display-scale; then cat /run/workbench-desktop/display-scale
else echo; fi
if test -f /run/workbench-desktop/xsettings.pid; then echo dynamic; fi`)
	password, scale, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	if err != nil || len(password) != 8 {
		http.Error(w, "Claude Desktop 尚未启动", 409)
		return
	}
	// With a settings manager the viewer chooses the density of its own screen.
	scale, dynamic, _ := strings.Cut(scale, "\n")
	density := 2.0
	if scale == "1.5" {
		density = 1.5
	}
	jsonReply(w, map[string]any{"password": password, "display_scale": density, "dynamic_scale": dynamic == "dynamic"})
}

// The zero value is a viewer that leaves the display's density alone.
func validDisplayDensity(scale float64) bool {
	return scale == 0 || scale >= 0.75 && scale <= 4
}

func (s *Server) claudeDesktopResize(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Width  int     `json:"width"`
		Height int     `json:"height"`
		Scale  float64 `json:"scale"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	if input.Width < 320 || input.Height < 240 || input.Width > 8192 || input.Height > 8192 || !validDisplayDensity(input.Scale) {
		http.Error(w, "画面尺寸无效", 400)
		return
	}
	scale := ""
	if input.Scale != 0 {
		scale = strconv.FormatFloat(input.Scale, 'f', -1, 64)
	}
	script := desktopResizeScript(input.Width, input.Height, scale)
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	output, err := s.guestQuick(ctx, script)
	if err != nil {
		http.Error(w, "Claude Desktop 画面尺寸未能调整", 502)
		return
	}
	reply := map[string]any{"ok": true}
	if applied, _, _ := strings.Cut(string(output), "\n"); scale != "" && strings.TrimSpace(applied) == "scale "+scale {
		reply["scale"] = input.Scale
	}
	jsonReply(w, reply)
}

// Fixed commands and validated numbers only; no host shell executes input.
// scale is empty for a viewer that leaves the display's density alone.
func desktopResizeScript(wid, hei int, scale string) string {
	return fmt.Sprintf(`set -eu
export DISPLAY=:78 XAUTHORITY=/run/workbench-desktop/Xauthority
exec 9>/run/workbench-desktop/resize.lock
flock 9
requested=%s
manager=
if test -r /run/workbench-desktop/xsettings.pid; then
  manager=$(cat /run/workbench-desktop/xsettings.pid)
  if test "$(cat "/proc/$manager/comm" 2>/dev/null)" != xsettingsd; then manager=; fi
fi
# A fixed-density session ignores the request, and its viewer sizes for that
# density. A viewer that names none sized its display for the configured one.
scale=
if test -n "$manager"; then
  scale=${requested:-$(cat /run/workbench-desktop/base-display-scale 2>/dev/null || echo 2)}
  if test -n "$requested"; then echo "scale $scale"; fi
fi
printf '%%s %%s %%s\n' %d %d "$scale" > /run/workbench-desktop/desktop-size
if test -n "$manager" && test "$(cat /run/workbench-desktop/display-scale 2>/dev/null)" != "$scale"; then
  dpi=$(awk -v scale="$scale" 'BEGIN {printf "%%.0f", scale * 96 * 1024}')
  # A session started by an earlier version has no list of its other settings.
  { printf 'Xft/DPI %%s\nGdk/WindowScalingFactor 1\nGdk/UnscaledDPI %%s\n' "$dpi" "$dpi"
    cat /run/workbench-desktop/xsettings.fixed 2>/dev/null || printf 'Gtk/DecorationLayout ":"\n'; } > /run/workbench-desktop/xsettings.conf.new
  mv /run/workbench-desktop/xsettings.conf.new /run/workbench-desktop/xsettings.conf
  kill -HUP "$manager"
  printf '%%s\n' "$scale" > /run/workbench-desktop/display-scale
fi
name=workbench-%dx%d
if ! xrandr --query | grep -q " $name "; then
  xrandr --newmode "$name" %.3f %d %d %d %d %d %d %d %d +HSync -VSync
  xrandr --addmode VNC-0 "$name"
fi
xrandr --output VNC-0 --mode "$name"
for old in $(xrandr --query | awk '/^ +workbench-/ {print $1}'); do
  if [ "$old" != "$name" ]; then xrandr --delmode VNC-0 "$old"; xrandr --rmmode "$old"; fi
done`, scale, wid, hei, wid, hei, float64((wid+32)*(hei+8)*60)/1000000, wid, wid+8, wid+16, wid+32, hei, hei+1, hei+2, hei+8)
}

func (s *Server) claudeDesktopVNC(w http.ResponseWriter, r *http.Request) {
	if !s.vmRunning() {
		http.Error(w, "环境尚未启动", 409)
		return
	}
	view, err := s.desktopViews.connect(s.changeDesktopVisibility)
	if err != nil {
		http.Error(w, "无法恢复 Claude Desktop 画面，请重新连接", http.StatusBadGateway)
		return
	}
	defer view.close()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	// No extra localhost port or public VNC listener on the host. SSH is bound
	// to this profile's single guest and carries the authenticated RFB session.
	cmd := s.sshCommand(ctx, "-T", s.sshTarget(), "--", "exec socat STDIO UNIX-CONNECT:/run/workbench-desktop/vnc.sock")
	reader, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, "无法连接应用画面", 502)
		return
	}
	writer, err := cmd.StdinPipe()
	if err != nil {
		_ = reader.Close()
		http.Error(w, "无法连接应用画面", 502)
		return
	}
	if err = cmd.Start(); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		http.Error(w, "无法连接应用画面", 502)
		return
	}
	defer func() { cancel(); writer.Close(); reader.Close(); _ = cmd.Wait() }()
	s.streamVNC(w, r, reader, writer, func() { cancel(); _ = writer.Close(); _ = reader.Close() }, view.setVisible)
}
