package desktopupdate

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/sylvinus/agent-vm/internal/appupdate"
	"github.com/sylvinus/agent-vm/internal/host"
	"github.com/sylvinus/agent-vm/internal/installfiles"
)

// ErrStopRequired tells the updater to retry the lifecycle preparation with an
// explicit stop request. Other preparation errors must not commit installation.
var ErrStopRequired = errors.New("environment stop required")

type Backend interface {
	// Update invokes emit synchronously in stream order and does not retain it
	// after returning. The implementation must honor context cancellation.
	Update(context.Context, bool, func(appupdate.State)) error
	PrepareUpdate(context.Context, bool) error
}
type Config struct {
	Root, Target, Executable, Version, ConfigPath string
	Backend                                       Backend
	Changed                                       func(State)
	// Quit is required to install; Reconnect resumes a revoked preparation.
	Quit, Reconnect func()
}
type State struct {
	Phase          string `json:"phase"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion,omitempty"`
	ReceivedBytes  int64  `json:"receivedBytes,omitempty"`
	TotalBytes     int64  `json:"totalBytes,omitempty"`
	Message        string `json:"message"`
	Retryable      bool   `json:"retryable"`
	RetryAt        string `json:"retryAt"`
}
type InstallPlan struct {
	Archive          string `json:"archive"`
	Digest           string `json:"digest"`
	Version          string `json:"version"`
	Publisher        string `json:"publisher"`
	Target           string `json:"target"`
	ParentPID        int    `json:"parentPid"`
	ParentStartTicks int64  `json:"parentStartTicks"`
	Confirmation     string `json:"confirmation"`
	ConfigPath       string `json:"configPath"`
}
type Updater struct {
	config            Config
	mu                sync.Mutex
	state             State
	prepared          *InstallPlan
	cancel            context.CancelFunc
	done              chan struct{}
	committed, closed bool
}

func New(c Config) (*Updater, error) {
	if c.Root == "" || c.Target == "" || c.Executable == "" || c.Version == "" || c.ConfigPath == "" || c.Backend == nil {
		return nil, errors.New("桌面更新配置不完整")
	}
	for _, p := range []*string{&c.Root, &c.Target, &c.Executable, &c.ConfigPath} {
		if *p != "" {
			absolute, e := filepath.Abs(*p)
			if e != nil {
				return nil, e
			}
			*p = absolute
		}
	}
	u := &Updater{config: c, state: State{Phase: "idle", CurrentVersion: c.Version}}
	var result struct {
		Success *bool  `json:"success"`
		Message string `json:"message"`
	}
	if readSmallJSON(filepath.Join(c.Root, "last-result.json"), &result, 16384) == nil {
		u.state.Message = result.Message
		if result.Success != nil && !(*result.Success) {
			u.state.Phase = "failed"
			u.state.Retryable = true
		}
	}
	return u, nil
}
func (u *Updater) Status() State { u.mu.Lock(); defer u.mu.Unlock(); return u.state }
func (u *Updater) publish(state State) {
	if u.config.Changed != nil {
		u.config.Changed(state)
	}
}
func (u *Updater) change(phase, message string, retryable bool, retryAt string) {
	u.mu.Lock()
	u.state.Phase = phase
	u.state.Message = message
	u.state.Retryable = retryable
	u.state.RetryAt = retryAt
	s := u.state
	u.mu.Unlock()
	u.publish(s)
}
func (u *Updater) Handle(operation string) (State, error) {
	u.mu.Lock()
	if u.closed {
		u.mu.Unlock()
		return u.Status(), errors.New("桌面更新已关闭")
	}
	switch operation {
	case "status":
	case "cancel":
		if !u.committed && u.cancel != nil {
			u.cancel()
		}
	case "check", "install":
		deadline, _ := time.Parse(time.RFC3339, u.state.RetryAt)
		if u.cancel == nil && !u.committed && !deadline.After(time.Now()) && (operation != "check" || u.prepared == nil) {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
			u.cancel = cancel
			u.done = make(chan struct{})
			done := u.done
			go func() {
				defer func() { cancel(); u.mu.Lock(); u.cancel = nil; close(done); u.mu.Unlock() }()
				if operation == "install" {
					u.mu.Lock()
					prepared := u.prepared != nil
					u.mu.Unlock()
					if prepared {
						u.install(ctx)
						return
					}
				}
				u.run(ctx, operation == "check")
			}()
		}
	default:
		u.mu.Unlock()
		return u.Status(), errors.New("更新操作无效")
	}
	s := u.state
	u.mu.Unlock()
	return s, nil
}
func (u *Updater) run(ctx context.Context, checkOnly bool) {
	if err := installfiles.PlainPath(u.config.Root); err != nil {
		u.change("failed", err.Error(), false, "")
		return
	}
	u.change("checking", "正在检查更新，优先使用主机网络", false, "")
	var last appupdate.State
	err := u.config.Backend.Update(ctx, checkOnly, func(s appupdate.State) {
		last = s
		u.mu.Lock()
		u.state = State{s.Phase, u.config.Version, s.LatestVersion, s.ReceivedBytes, s.TotalBytes, s.Message, s.Retryable, s.RetryAt}
		state := u.state
		u.mu.Unlock()
		u.publish(state)
	})
	var archive string
	defer func() {
		if archive != "" {
			cleanupDirectory(filepath.Dir(archive))
		}
	}()
	if err == nil && !checkOnly && last.Phase == "downloaded" {
		archive, err = DownloadPath(last.ArchivePath, u.config.Root)
		if err == nil && !IsNewer(last.LatestVersion, u.config.Version) {
			err = errors.New("安装包版本无效或不高于当前版本")
		}
		if err == nil {
			u.change("verifying", "正在验证 Windows 签名和安装包内容", false, "")
			var publisher string
			publisher, err = Publisher(u.config.Executable)
			if err == nil {
				var f *os.File
				f, err = VerifyArchiveLocked(u.config.Root, archive, last.SHA256, last.LatestVersion, publisher)
				if f != nil {
					f.Close()
				}
			}
			if err == nil {
				err = ctx.Err()
			}
			if err == nil {
				var ticks int64
				ticks, err = CurrentStartTicks()
				if err == nil {
					u.mu.Lock()
					u.prepared = &InstallPlan{archive, last.SHA256, last.LatestVersion, publisher, u.config.Target, os.Getpid(), ticks, newConfirmation(), u.config.ConfigPath}
					u.mu.Unlock()
					archive = ""
					u.change("ready", "更新已下载并验证，可安装重启", true, "")
				}
			}
		}
	}
	if err != nil {
		message := err.Error()
		if ctx.Err() != nil {
			message = "更新已暂停，可稍后重试"
		}
		u.change("failed", message, true, last.RetryAt)
		return
	}
	if !checkOnly && last.Phase == "downloaded" {
		u.install(ctx)
	}
}

func (u *Updater) install(parent context.Context) {
	if u.config.Quit == nil {
		u.change("failed", "缺少应用退出回调，安装准备已停止", false, "")
		return
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	u.mu.Lock()
	plan := *u.prepared
	u.mu.Unlock()
	u.change("installing", "正在准备安装更新", false, "")
	committed := false
	helperOwned := false
	helperPath := filepath.Join(u.config.Root, "helper-"+newConfirmation()+".exe")
	var helper *exec.Cmd
	var input io.WriteCloser
	var output io.ReadCloser
	var exited chan error
	defer func() {
		if input != nil {
			input.Close()
		}
		if !committed && helper != nil && helper.Process != nil {
			select {
			case <-exited:
			case <-time.After(5 * time.Second):
				_ = helper.Process.Kill()
				<-exited
			}
		}
		if output != nil {
			output.Close()
		}
		if !committed && helperOwned {
			_ = os.Remove(helperPath)
		}
	}()
	err := func() error {
		if e := installfiles.PlainPath(plan.Target); e != nil {
			return e
		}
		if e := installfiles.PlainPath(u.config.Root); e != nil {
			return e
		}
		if e := host.EnsurePrivateDir(u.config.Root); e != nil {
			return e
		}
		if e := installfiles.PlainPath(u.config.Root); e != nil {
			return e
		}
		if e := copyFileOwned(ctx, u.config.Executable, helperPath, &helperOwned); e != nil {
			return e
		}
		helperLock, e := installfiles.OpenReadLocked(helperPath)
		if e != nil {
			return e
		}
		defer helperLock.Close()
		if e := SamePublisher(helperPath, plan.Publisher); e != nil {
			return e
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		helper = exec.Command(helperPath, "--install-update")
		hide(helper)
		helper.Dir = u.config.Root
		helper.Env = profileEnvironment(plan.ConfigPath, "")
		input, e = helper.StdinPipe()
		if e != nil {
			return e
		}
		output, e = helper.StdoutPipe()
		if e != nil {
			return e
		}
		// The helper reports actionable errors through last-result.json. Do not
		// let an unconsumed stderr pipe block its lifecycle.
		helper.Stderr = io.Discard
		if e = helper.Start(); e != nil {
			return e
		}
		exited = make(chan error, 1)
		go func() { exited <- helper.Wait() }()
		protocolInput, protocolOutput := input, output
		abort := context.AfterFunc(ctx, func() { protocolInput.Close(); protocolOutput.Close() })
		defer abort()
		if e = json.NewEncoder(input).Encode(plan); e != nil {
			return e
		}
		ready, e := readLine(ctx, output, 16384)
		if e != nil {
			return e
		}
		if ready != "READY" {
			return errors.New("安装程序准备失败，原应用已保留")
		}
		if e = u.config.Backend.PrepareUpdate(ctx, false); errors.Is(e, ErrStopRequired) {
			e = u.config.Backend.PrepareUpdate(ctx, true)
		}
		if e != nil {
			return e
		}
		select {
		case e = <-exited:
			exited <- e
			return errors.New("安装程序已退出，原应用已保留")
		default:
		}
		if e = u.commit(ctx, input); e != nil {
			return e
		}
		// A complete COMMIT line transfers ownership even if closing the pipe
		// subsequently fails. Never report a revocation after the helper can act.
		_ = input.Close()
		input = nil
		committed = true
		u.change("installing", "正在安装并重启 Claude Eden", false, "")
		u.config.Quit()
		return nil
	}()
	if err != nil {
		message := err.Error()
		if ctx.Err() != nil {
			message = "安装准备超时或已取消，原应用已保留"
		}
		u.change("failed", message, true, "")
		if u.config.Reconnect != nil {
			u.config.Reconnect()
		}
	}
}

// commit and Handle("cancel") share one ordering point. A cancellation accepted
// before the write revokes preparation; after the complete line, the helper owns
// the transaction and a UI cancellation cannot take it back.
func (u *Updater) commit(ctx context.Context, input io.Writer) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if e := ctx.Err(); e != nil {
		return e
	}
	if u.closed {
		return errors.New("桌面更新已关闭")
	}
	n, e := io.WriteString(input, "COMMIT\n")
	if n == len("COMMIT\n") {
		u.committed = true
		return nil
	}
	if e != nil {
		return e
	}
	return io.ErrShortWrite
}

var confirmationPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func newConfirmation() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func (u *Updater) Close() {
	u.mu.Lock()
	u.closed = true
	cancel, done, committed := u.cancel, u.done, u.committed
	u.mu.Unlock()
	if !committed && cancel != nil {
		cancel()
		<-done
	}
}

func readLine(ctx context.Context, input io.Reader, limit int) (string, error) {
	type result struct {
		text string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		reader, ok := input.(*bufio.Reader)
		if !ok {
			reader = bufio.NewReaderSize(input, limit+3)
		}
		b, e := reader.ReadSlice('\n')
		if errors.Is(e, bufio.ErrBufferFull) || len(b) > limit+2 {
			done <- result{"", errors.New("更新响应超过允许大小")}
			return
		}
		if e != nil {
			done <- result{"", e}
			return
		}
		text := string(b[:len(b)-1])
		if len(text) > 0 && text[len(text)-1] == '\r' {
			text = text[:len(text)-1]
		}
		if len(text) > limit {
			done <- result{"", errors.New("更新响应超过允许大小")}
			return
		}
		done <- result{text, nil}
	}()
	select {
	case r := <-done:
		return r.text, r.err
	case <-ctx.Done():
		if closer, ok := input.(io.Closer); ok {
			closer.Close()
		}
		return "", ctx.Err()
	}
}
