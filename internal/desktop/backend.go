package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

type Backend struct {
	Executable  string
	ConfigPath  string
	Directory   string
	prefix      []string
	environment []string
}

type bootstrapReply struct {
	Status  string `json:"status"`
	URL     string `json:"url"`
	Message string `json:"message"`
}

func NewBackend(executable, config string) (*Backend, error) {
	if config == "" {
		config = os.Getenv("EDEN_CONFIG")
	}
	if config == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		config = filepath.Join(home, ".eden", "config.json")
	}
	absolute, err := filepath.Abs(config)
	if err != nil {
		return nil, err
	}
	program, err := filepath.Abs(executable)
	if err != nil {
		return nil, err
	}
	return &Backend{Executable: program, ConfigPath: absolute, Directory: filepath.Dir(filepath.Dir(program))}, nil
}

func (b *Backend) Root() string { return filepath.Dir(b.ConfigPath) }

func (b *Backend) Command(ctx context.Context, arguments ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, b.Executable, append(append([]string{}, b.prefix...), arguments...)...)
	cmd.Dir = b.Directory
	cmd.Env = append(append(os.Environ(), b.environment...), "EDEN_CONFIG="+b.ConfigPath)
	if len(arguments) > 0 && arguments[0] == "desktop" {
		cmd.Env = append(cmd.Env, "EDEN_DESKTOP_OWNER_PID="+strconv.Itoa(os.Getpid()))
	}
	hideProcess(cmd)
	// Cancellation ends this short-lived CLI helper only, never the controller
	// or the VM it may already have started. Inherited pipe handles are bounded.
	cmd.WaitDelay = 2 * time.Second
	return cmd
}

type boundedOutput struct {
	bytes.Buffer
	limit    int
	exceeded bool
	onLimit  func()
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if b.Len()+n > b.limit {
		b.exceeded = true
		if b.onLimit != nil {
			b.onLimit()
		}
		if remaining := b.limit - b.Len(); remaining > 0 {
			_, _ = b.Buffer.Write(p[:remaining])
		}
		return n, errors.New("后台响应超过允许大小")
	}
	return b.Buffer.Write(p)
}

// JSON leaves desktop startup unbounded: readiness or explicit cancellation,
// not a disk-speed timeout, determines when startup has completed.
func (b *Backend) JSON(ctx context.Context, deadline time.Duration, limit int, result any, arguments ...string) (int, error) {
	if deadline > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, deadline)
		defer cancel()
	}
	cmd := b.Command(ctx, arguments...)
	output := &boundedOutput{limit: limit, onLimit: func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}}
	cmd.Stdout, cmd.Stderr = output, io.Discard
	err := cmd.Run()
	if ctx.Err() != nil {
		return -1, ctx.Err()
	}
	if output.exceeded {
		return -1, errors.New("后台响应超过允许大小")
	}
	code := 0
	if err != nil {
		var exited *exec.ExitError
		if !errors.As(err, &exited) {
			return -1, fmt.Errorf("无法启动 Claude Eden 后台：%w", err)
		}
		code = exited.ExitCode()
	}
	if err := json.Unmarshal(output.Bytes(), result); err != nil {
		return code, errors.New("Claude Eden 后台未返回有效状态，请查看工作台日志")
	}
	return code, nil
}

func (b *Backend) fileCommand(ctx context.Context, args ...string) (FileResult, error) {
	var result FileResult
	code, err := b.JSON(ctx, 30*time.Minute, 65536, &result, args...)
	if err != nil {
		return result, err
	}
	if code != 0 {
		if result.Message == "" {
			result.Message = "本机文件操作未完成"
		}
		return result, errors.New(result.Message)
	}
	return result, nil
}

func (b *Backend) Report(message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if len(message) > 4096 {
		message = message[:4096]
	}
	data, err := json.Marshal(struct {
		Source  string `json:"source"`
		Message string `json:"message"`
	}{"native", message})
	if err != nil {
		return
	}
	cmd := b.Command(ctx, "diagnostic-event")
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	_ = cmd.Run() // Diagnostic failure cannot replace or recurse on the original error.
}
