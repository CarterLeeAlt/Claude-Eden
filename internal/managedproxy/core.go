package managedproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/sylvinus/agent-vm/internal/diagnostics"
	"github.com/sylvinus/agent-vm/internal/host"
)

// Preserve causes for errors.Is/As without exposing private socket paths,
// request URLs, credentials or response contents in displayed/logged errors.
type coreError struct {
	stage string
	err   error
}

func (e *coreError) Error() string {
	message := "Claude Eden 代理核心操作失败（" + e.stage + "）"
	var nested *coreError
	var errno syscall.Errno
	var network net.Error
	switch {
	case e.err == nil:
		return message
	case errors.As(e.err, &nested):
		return message + ": " + nested.Error()
	case errors.Is(e.err, context.Canceled):
		return message + ": context canceled"
	case errors.Is(e.err, context.DeadlineExceeded):
		return message + ": context deadline exceeded"
	case errors.As(e.err, &errno):
		var socketError *net.OpError
		if runtime.GOOS != "windows" && errors.As(e.err, &socketError) && socketError.Net == "unix" &&
			(errno == syscall.EINVAL || errno == syscall.ENAMETOOLONG) {
			return fmt.Sprintf("%s: errno %d (%s)；请将 TMPDIR 设置为更短且可写的临时目录", message, errno, errno)
		}
		return fmt.Sprintf("%s: errno %d", message, errno)
	case errors.As(e.err, &network) && network.Timeout():
		return message + ": timeout"
	default:
		return fmt.Sprintf("%s: %T", message, e.err)
	}
}

func (e *coreError) Unwrap() error { return e.err }

// CorePath deliberately never searches PATH or another application's bundle.
func CorePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	name := "mihomo"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	p := filepath.Join(filepath.Dir(exe), name)
	if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() && (runtime.GOOS == "windows" || st.Mode()&0111 != 0) {
		return p, nil
	}
	return "", errors.New("Claude Eden 随附代理核心缺失，请重新安装完整应用")
}
func Validate(ctx context.Context, profile string, s Store) error {
	core, err := CorePath()
	if err != nil {
		return err
	}
	b, err := s.Runtime(profile)
	if err != nil {
		return err
	}
	dir := Dir(profile)
	if err = host.EnsurePrivateDir(dir); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "validate-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = host.ProtectFile(f.Name()); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	f.Close()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, core, "-t", "-d", dir, "-f", f.Name())
	cmd.Env = coreEnvironment(dir)
	output := diagnostics.New(profile).Stream("proxy-validation", "error")
	defer output.Close()
	cmd.Stdout, cmd.Stderr = output, output
	if e := cmd.Run(); e != nil {
		return errors.New("随附的 Mihomo 核心未通过配置校验，请核对节点协议和参数；可在设置的“日志与诊断”中导出诊断包查看校验记录；原配置已保留")
	}
	return nil
}
func API(ctx context.Context, profile, method, path string, body any, reply any) error {
	tr := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", Socket(profile))
	}}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return &coreError{stage: "api.encode", err: err}
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://eden-proxy"+path, bytes.NewReader(data))
	if err != nil {
		return &coreError{stage: "api.request", err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return &coreError{stage: "api.transport", err: err}
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return &coreError{stage: fmt.Sprintf("api.http-%d", res.StatusCode)}
	}
	if reply != nil {
		if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(reply); err != nil {
			return &coreError{stage: "api.decode", err: err}
		}
	}
	return nil
}
func Running(ctx context.Context, profile string, s Store) bool {
	return checkRunning(ctx, profile, s) == nil
}

func checkRunning(ctx context.Context, profile string, s Store) error {
	var cfg struct {
		Port         int    `json:"mixed-port"`
		Interface    string `json:"interface-name"`
		UnifiedDelay bool   `json:"unified-delay"`
	}
	var group struct {
		Now string `json:"now"`
	}
	bound, err := BoundInterface(filepath.Join(Dir(profile), "config.yaml"))
	if err != nil || bound == "" {
		return &coreError{stage: "readiness.bound-interface", err: err}
	}
	if err := API(ctx, profile, "GET", "/configs", nil, &cfg); err != nil {
		return &coreError{stage: "readiness.configs", err: err}
	}
	if cfg.Port != s.Port {
		return &coreError{stage: "readiness.port"}
	}
	if cfg.Interface != bound {
		return &coreError{stage: "readiness.interface"}
	}
	if !cfg.UnifiedDelay {
		return &coreError{stage: "readiness.unified-delay"}
	}
	if err := API(ctx, profile, "GET", "/proxies/"+Group, nil, &group); err != nil {
		return &coreError{stage: "readiness.group", err: err}
	}
	if group.Now != s.Selected {
		return &coreError{stage: "readiness.selection"}
	}
	return nil
}
