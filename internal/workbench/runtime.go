package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"al.essio.dev/pkg/shellescape"

	"github.com/sylvinus/agent-vm/internal/diagnostics"
	"github.com/sylvinus/agent-vm/internal/eden"
	"github.com/sylvinus/agent-vm/internal/guestos"
	"github.com/sylvinus/agent-vm/internal/host"
	"github.com/sylvinus/agent-vm/internal/hostimage"
	"github.com/sylvinus/agent-vm/internal/managedproxy"
	"github.com/sylvinus/agent-vm/internal/vmname"
)

var loadConfig = eden.Load
var defaultConfig = eden.DefaultConfig

type guestCredentials struct {
	Token        string  `json:"token"`
	VNCPassword  string  `json:"-"`
	DisplayScale float64 `json:"display_scale"`
	DynamicScale bool    `json:"dynamic_scale"`
}

func (s *Server) workspaceLocked() string {
	return s.preferences.Workspace
}
func (s *Server) workspace() string      { s.mu.Lock(); defer s.mu.Unlock(); return s.workspaceLocked() }
func (s *Server) environmentDir() string { return environmentWorkspace(s.profile) }
func (s *Server) sshTarget() string {
	return "lima-" + eden.InstanceName(s.profile, s.environmentDir())
}

func (s *Server) environmentPrepared() bool {
	_, err := os.Stat(filepath.Join(s.profile, "state", ".agent-vm-base-version"))
	return err == nil
}
func (s *Server) sshCommand(ctx context.Context, args ...string) *exec.Cmd {
	return s.sshCommandOptions(ctx, false, args...)
}
func (s *Server) sshCommandWithForwarding(ctx context.Context, args ...string) *exec.Cmd {
	return s.sshCommandOptions(ctx, true, args...)
}
func (s *Server) sshCommandOptions(ctx context.Context, maintenance bool, args ...string) *exec.Cmd {
	cfg := filepath.Join(s.instanceDir(), "ssh.config")
	base := []string{"-q", "-e", "none", "-F", cfg, "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", "-o", "PermitLocalCommand=no", "-o", "PasswordAuthentication=no", "-o", "KbdInteractiveAuthentication=no", "-o", "ProxyCommand=none", "-o", "ProxyJump=none", "-o", "ConnectTimeout=3", "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=2", "-o", "ForwardAgent=no", "-o", "IdentityAgent=none", "-o", "ForwardX11=no", "-o", "ClearAllForwardings=yes", "-o", "ControlPath=none"}
	if maintenance {
		for index, value := range base {
			if value == "ClearAllForwardings=yes" {
				base[index] = "ClearAllForwardings=no"
			}
		}
	}
	base = append(base, args...)
	cmd := exec.CommandContext(ctx, sshExecutable(), base...)
	cmd.Env = sshEnvironment()
	host.HideCommandWindow(cmd)
	return cmd
}
func (s *Server) guest(ctx context.Context, script string) ([]byte, error) {
	return s.guestControl(ctx, script, "")
}

func (s *Server) guestControl(ctx context.Context, script, diagnosticSource string) ([]byte, error) {
	// SSH enters the user's default shell. Run control scripts in Bash so
	// zsh initialization cannot change their redirection or error behavior.
	command := "exec /bin/bash --noprofile --norc -c " + shellescape.Quote(script)
	cmd := s.sshCommand(ctx, "-T", s.sshTarget(), "--", command)
	var stderr fileControlStderr
	if diagnosticSource != "" {
		cmd.Stderr = &stderr
		defer func() {
			if len(stderr.data) > 0 {
				_ = diagnostics.New(s.profile).Record("warn", diagnosticSource, "guest control stderr: "+string(stderr.data))
			}
		}()
	}
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(pipe, (64<<10)+1))
	if readErr != nil || len(data) > 64<<10 {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, errors.New("guest control response exceeded its limit")
	}
	if err = cmd.Wait(); err != nil {
		return nil, err
	}
	return data, nil
}
func (s *Server) attach() error {
	return s.attachContext(s.lifetimeContext())
}

func (s *Server) attachContext(parent context.Context) error {
	if err := s.requireStorageReady(); err != nil {
		return err
	}
	if err := s.requireCurrentLinuxTransferMount(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	if !s.vmRunning() {
		return errors.New("环境尚未启动")
	}
	b, err := s.guest(ctx, guestConnectionSnapshot)
	if err != nil {
		return fmt.Errorf("读取环境连接信息失败：%w", errors.Join(ctx.Err(), err))
	}
	credentials, manifest, err := parseGuestConnection(b)
	s.mu.Lock()
	s.guestManifest = manifest
	s.guestUpdateRequired = errors.Is(err, errGuestUpdateRequired)
	s.guestUpdateError = ""
	if err != nil {
		s.ready = false
		s.guestUpdateError = err.Error()
	}
	s.mu.Unlock()
	if err != nil {
		if errors.Is(err, errGuestUpdateRequired) {
			_ = s.closeProxyGate("环境组件需要更新，外网保持关闭")
		}
		return err
	}
	if _, err = s.guestAPI(ctx, credentials.Token, "GET", "/health", nil); err != nil {
		return err
	}
	s.mu.Lock()
	home := browserHomeURL(s.preferences)
	s.mu.Unlock()
	if err = s.setBrowserHome(ctx, credentials.Token, home); err != nil {
		return err
	}
	s.mu.Lock()
	s.credentials = credentials
	s.ready = true
	s.environmentGeneration++
	s.mu.Unlock()
	s.restoreProjectShares()
	s.desktopViews.background(s.changeDesktopVisibility)
	s.browserViews.background(s.changeBrowserVisibility)
	return nil
}
func (s *Server) monitor(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	skipped := 0
	for ctx.Err() == nil {
		timing := s.startMonitorTiming()
		s.mu.Lock()
		ready, closing, token, generation := s.ready, s.closing, s.credentials.Token, s.environmentGeneration
		s.mu.Unlock()
		if closing {
			timing.result = "closing"
		} else if s.busy() {
			timing.result = "job_running"
		} else if ready {
			// A read-only sample must not reject interactive work. Reattachment
			// changes the generation, so a late failure cannot invalidate it.
			probe, cancel := s.controlContext()
			_, err := s.guestAPI(probe, token, "GET", "/health", nil)
			cancel()
			if err != nil {
				s.mu.Lock()
				if !s.closing && s.ready && generation == s.environmentGeneration && !s.busy() {
					s.ready = false
				}
				s.mu.Unlock()
			}
			timing.stage("environment_health")
		} else if s.op.TryLock() {
			s.mu.Lock()
			attach := !s.ready && !s.closing
			s.mu.Unlock()
			if attach && !s.busy() {
				_ = s.attach()
				timing.stage("environment_health")
			}
			s.op.Unlock()
		} else {
			timing.result = "operation_lock_busy"
		}
		if timing.result == "operation_lock_busy" {
			skipped++
		} else {
			skipped = 0
		}
		timing.stages = append(timing.stages, fmt.Sprintf("consecutive_lock_skips=%d", skipped))
		s.recordMonitorTiming("environment", timing)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) runEden(args ...string) error {
	if len(args) > 0 && args[0] == "up" {
		if err := s.requireCurrentLinuxTransferMount(); err != nil {
			return err
		}
	}
	ctx := s.operationContext()
	// Setup and startup can both install dependencies. Downloads own their
	// inactivity timeouts; the operation context still permits an explicit stop.
	if len(args) != 1 || (args[0] != "setup" && args[0] != "up") {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, s.exe, args...)
	host.HideCommandWindow(cmd)
	cmd.Dir = s.environmentDir()
	if cmd.Dir == "" {
		cmd.Dir = s.profile
	} else if len(args) > 0 && args[0] == "stop" {
		if info, err := os.Stat(cmd.Dir); err != nil || !info.IsDir() {
			cmd.Dir = s.profile
		}
	}
	cmd.Env = append(os.Environ(), eden.ConfigEnv+"="+s.configPath)
	execution := s.jobs.execution()
	if len(args) == 1 && args[0] == "setup" {
		cmd.Env = append(cmd.Env, "EDEN_SETUP_PROGRESS=1")
	}
	cmd.Env = append(cmd.Env, eden.JobEnv+"="+execution.id)
	cmd.Stdout, cmd.Stderr = execution, execution
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return context.Canceled
		}
		action := map[string]string{"setup": "准备 Linux 环境", "up": "启动环境", "down": "停止环境", "stop": "停止环境"}[args[0]]
		if action == "" {
			action = "执行环境操作"
		}
		if p := s.jobs.snapshot().SetupProgress; p != nil && p.Label != "" {
			action = strings.TrimPrefix(p.Label, "正在")
		}
		// CLI failures can include the original QEMU stderr before a generic
		// "Failed to start base VM" footer. Surface that reason in job.error.
		for _, line := range strings.Split(s.jobs.snapshot().Log, "\n") {
			if _, reason, ok := strings.Cut(line, "Error: QEMU 启动失败："); ok {
				return fmt.Errorf("%s失败：虚拟机未能启动。QEMU 启动失败：%s。请查看操作详情；仍无法启动时，在设置中导出诊断包。", action, reason)
			}
		}
		diagnostics.Log("error", "environment-operation", fmt.Sprintf("operation=%s error=%v context=%v", args[0], err, ctx.Err()))
		return executionFailure(action, errors.Join(ctx.Err(), err), s.jobs.snapshot().Log)
	}
	return nil
}
func (s *Server) startEnvironment(syncTimezone bool) error {
	s.desktopViews.reset()
	s.browserViews.reset()
	s.controlSession.close()
	s.desktopStatus.invalidate()
	defer s.desktopStatus.invalidate()
	if err := s.requireStorageReady(); err != nil {
		return err
	}
	if err := s.requireCurrentLinuxTransferMount(); err != nil {
		return err
	}
	pending, err := guestos.LoadInstallation(s.profile)
	if err != nil {
		return err
	}
	linuxSystem, err := s.startupLinuxSystem()
	if err != nil {
		return err
	}
	if pending != nil {
		prepared, err := hostimage.Load(filepath.Join(s.profile, "state", "guest-media"))
		if err != nil || pending.Media == "" || prepared != pending.Source {
			return errors.New("请从首次安装入口完成 Linux 镜像准备")
		}
	}
	if _, err := validateWorkspace(s.environmentDir(), s.profile); err != nil {
		return err
	}
	cfg, err := loadConfig(s.configPath)
	if err != nil {
		return err
	}
	if err = cfg.RequireManagedProxy(s.profile); err != nil {
		_ = s.closeProxyGate(err.Error())
		return err
	}
	{
		p, e := managedproxy.Load(s.profile)
		if e != nil {
			return e
		}
		if p.Selected == "" || p.Endpoint() != cfg.ProxyURL || p.Revision != cfg.ProxyRevision {
			return errors.New("内置代理配置未应用，请重新选择节点")
		}
		name, e := physicalProxyInterface(s.operationContext())
		if e != nil {
			_ = s.closeProxyGate(e.Error())
			return e
		}
		bound, _ := managedproxy.BoundInterface(filepath.Join(managedproxy.Dir(s.profile), "config.yaml"))
		if bound != name || !managedproxy.Running(s.operationContext(), s.profile, p) {
			// Every core start changes the report generation, including recovery.
			if e = s.activateProxy(p, cfg); e != nil {
				return e
			}
			cfg, e = loadConfig(s.configPath)
			if e != nil {
				return e
			}
		}
	}

	attachErr := s.attach()
	if errors.Is(attachErr, errGuestUpdateRequired) {
		return s.synchronizeEnvironment()
	}
	if attachErr == nil {
		client, gateErr := s.gateClient()
		if gateErr == nil {
			state, e := client.Status(context.Background())
			if e == nil {
				if state.Mode == eden.GateOpen && state.Report != nil && state.Report.Authorizes(state.RunID, cfg.Fingerprint(), time.Now()) == nil {
					s.publishGate(state)
					return nil
				}
				return s.checkNetworkIfWanted()
			}
		}
		s.setMessage("正在重启环境以加载 VM 外网络门槛…")
		if err := s.stopEnvironment(); err != nil {
			return err
		}
	}
	s.mu.Lock()
	preferences := s.preferences
	auto := preferences.AutoTimezone || preferences.AutoLanguage
	s.mu.Unlock()
	s.setMessage("正在验证代理出口…")
	loc, probeErr := probeEnvironmentLocation(s.operationContext(), cfg, preferences)
	if s.operationContext().Err() != nil {
		return s.operationContext().Err()
	}
	if err := s.finishLocationProbe(cfg, loc, probeErr); err != nil {
		return err
	}
	if probeErr != nil && !s.environmentPrepared() {
		return probeErr
	}
	if cfg.ExitPolicy.NeedsBinding() && loc.IP != "" && !s.vmRunning() {
		bound := cfg.ExitPolicy.BindMissing(loc.exitLocation())
		if bound != cfg.ExitPolicy {
			cfg.ExitPolicy = bound
			if err = s.saveExitPolicyConfig(cfg); err != nil {
				return err
			}
		}
	}
	if auto && syncTimezone && !s.vmRunning() && probeErr == nil {
		observed := loc.exitLocation()
		within, _ := cfg.ExitPolicy.Evaluate(loc.IP, &observed)
		updated := cfg
		if within == eden.CheckPassed {
			updated = autoRegionSettings(cfg, preferences, loc)
		}
		if updated != cfg {
			// Changing a live hostagent's profile is forbidden. Stop before replacing it.
			s.setMessage("正在应用出口所在地的时区与语言建议…")
			if err = s.stopEnvironment(); err != nil {
				return err
			}
			cfg = updated
			if err = writePrivate(s.configPath, cfg); err != nil {
				return err
			}
		}
	} else if probeErr != nil {
		s.setMessage("代理出口无法通过检查，正在启动离线环境…")
	}
	if _, err = os.Stat(filepath.Join(s.profile, "state", ".agent-vm-base-version")); os.IsNotExist(err) {
		s.setMessage("首次准备 Linux 环境与开发工具，可能需要数分钟…")
		job := s.jobs.execution()
		job.setSetupProgress(&SetupProgress{Phase: "image", System: linuxSystem, Label: "正在准备 " + guestProgressName(linuxSystem) + " 镜像"})
		if err = s.runEden("setup"); err != nil {
			return err
		}
		job.setSetupProgress(&SetupProgress{Phase: "services", Label: "正在启动浏览器和编辑器"})
		defer job.setSetupProgress(nil)
	}
	s.setMessage("正在启动浏览器、编辑器与共享文件…")
	if err = s.runEden("up"); err != nil {
		return err
	}
	ctx := s.operationContext()
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err = s.attachContext(ctx); err == nil {
			return s.checkNetworkIfWanted()
		}
		if !startupConnectionPending(err) {
			return err
		}
		// Transport retries cannot conceal a process that has really stopped.
		if !s.vmRunning() {
			return errors.New("环境进程在连接就绪前退出，请查看操作详情")
		}
		probe, cancel := context.WithTimeout(ctx, 5*time.Second)
		output, serviceErr := s.guest(probe, "sudo python3 /usr/local/lib/workbench-wait-services.py --check 2>&1")
		probeErr := probe.Err()
		cancel()
		if serviceErr != nil && !startupConnectionPending(errors.Join(probeErr, serviceErr)) {
			return fmt.Errorf("环境服务启动失败：%s：%w", strings.TrimSpace(string(output)), serviceErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func startupConnectionPending(err error) bool {
	var networkError net.Error
	var sshExit *exec.ExitError
	return errors.Is(err, errBrowserStarting) || errors.Is(err, context.DeadlineExceeded) ||
		errors.As(err, &networkError) || errors.As(err, &sshExit) && sshExit.ExitCode() == 255
}
func (s *Server) stopEnvironment() (returned error) {
	s.desktopStatus.invalidate()
	defer s.desktopStatus.invalidate()
	explicitStop := s.jobs.snapshot().Kind == "stop"
	defer func() {
		if explicitStop && returned == nil {
			returned = managedproxy.Stop(s.profile)
		}
	}()
	s.setMessage("正在停止 Claude Eden 环境…")
	if (explicitStop || s.jobs.snapshot().Kind == "storage") && eden.InstanceJobID(filepath.Join(s.profile, "state"), vmname.Building) != "" {
		if err := s.runEden("stop", "--keep-proxy", vmname.Building); err != nil {
			return err
		}
	}
	if client, err := s.gateClient(); err == nil {
		if state, e := client.Close(context.Background(), "环境正在停止"); e == nil {
			s.publishGate(state)
		}
	}
	s.mu.Lock()
	s.ready = false
	s.credentials = guestCredentials{}
	s.guestManifest = nil
	s.guestUpdateRequired = false
	s.guestUpdateError = ""
	s.safety.Error = ""
	s.mu.Unlock()
	s.terminals.closeAll()
	if _, err := os.Stat(filepath.Join(s.instanceDir(), "lima.yaml")); os.IsNotExist(err) {
		s.closeProjectShares()
		return nil
	}
	// Internal restarts keep the already validated core alive. Explicit stop
	// releases it above only after the environment has stopped successfully.
	if err := s.runEden("stop", "--keep-proxy"); err != nil {
		return err
	}
	// Guest applications may flush shared files during their shutdown. Keep
	// SFTP alive through VM exit, and preserve it if stopping fails.
	s.closeProjectShares()
	return nil
}
func (s *Server) apply(v savedProfile, validate bool) error {
	old, err := loadConfig(s.configPath)
	if err != nil {
		return err
	}
	// Network endpoints are generated by the node manager, including restore.
	v.Config.ProxyURL, v.Config.ProxyRevision = old.ProxyURL, old.ProxyRevision
	s.mu.Lock()
	oldPrefs := s.preferences
	resumeVersion := s.safety.pauseVersion
	if execution := s.jobs.execution(); execution.id != "" {
		resumeVersion = execution.pauseVersion
	}
	wasReady := vmAlive(s.profile, s.environmentDir())
	s.mu.Unlock()
	changed := old != v.Config || oldPrefs.AutoTimezone != v.Preferences.AutoTimezone || oldPrefs.AutoLanguage != v.Preferences.AutoLanguage
	if changed && validate && old.RequireManagedProxy(s.profile) == nil {
		s.setMessage("正在验证新出口，现有环境保持运行…")
		loc, e := probeEnvironmentLocation(s.operationContext(), v.Config, v.Preferences)
		if err := s.finishLocationProbe(v.Config, loc, e); err != nil {
			return err
		}
		if e != nil {
			if v.Preferences.AutoTimezone || v.Preferences.AutoLanguage || loc.IP == "" {
				return e
			}
		} else {
			v.Config = autoRegionSettings(v.Config, v.Preferences, loc)
		}
		v.Config.ExitPolicy = v.Config.ExitPolicy.BindMissing(loc.exitLocation())
		observed := loc.exitLocation()
		if result, message := v.Config.ExitPolicy.Evaluate(loc.IP, &observed); result != eden.CheckPassed {
			if result == eden.CheckUnknown && e != nil && !v.Preferences.AutoTimezone && !v.Preferences.AutoLanguage {
				return errors.New("时区和语言已选择手动设置，但所有出口仍须确认国家或地区。请重试位置检测或切换节点；静态 IP 和不限地区不能跳过大陆出口检查，原配置保持不变")
			}
			return errors.New(message)
		}
	}
	if changed {
		if err := s.resumeNetworkIntentAt(resumeVersion); err != nil {
			return err
		}
		if s.operationContext().Err() != nil {
			return s.operationContext().Err()
		}
		if err = writePrivate(filepath.Join(s.profile, "previous-profile.json"), savedProfile{old, oldPrefs}); err != nil {
			return err
		}
		if err = s.stopEnvironment(); err != nil {
			return err
		}
	}
	// A project is a live directory grant in the same VM, never a lifecycle change.
	if v.Preferences.Workspace != "" && s.vmRunning() {
		s.setMessage("正在连接项目文件夹…")
		if _, err = s.connectProject(v.Preferences.Workspace, true); err != nil {
			return err
		}
	}
	if v.Config.RequireManagedProxy(s.profile) == nil {
		err = s.saveExitPolicyConfig(v.Config)
	} else {
		err = writePrivate(s.configPath, v.Config)
	}
	if err != nil {
		return err
	}
	if err = savePreferences(s.profile, v.Preferences); err != nil {
		_ = writePrivate(s.configPath, old)
		return err
	}
	s.mu.Lock()
	s.preferences = v.Preferences
	token, ready := s.credentials.Token, s.ready
	s.mu.Unlock()
	s.jobs.execution().settingsSaved()
	if !changed && ready && browserHomeURL(oldPrefs) != browserHomeURL(v.Preferences) {
		ctx, cancel := s.controlContext()
		defer cancel()
		if err = s.setBrowserHome(ctx, token, browserHomeURL(v.Preferences)); err != nil {
			return fmt.Errorf("设置已保存，但浏览器暂时未能应用默认网站，请恢复连接后重试：%w", err)
		}
	}
	if changed && wasReady {
		s.setMessage("正在重新连接客机…")
		if err = s.startEnvironment(false); err != nil {
			var network networkCheckError
			if errors.As(err, &network) {
				return fmt.Errorf("设置已保存，环境已启动。外网仍关闭，修复以下问题后可重新检测：%w", err)
			}
			return fmt.Errorf("设置已保存，环境尚未恢复就绪。请核对客机状态后重试启动：%w", err)
		}
	}
	return nil
}

// Interface recovery uses the ordinary cancellable job queue. Location changes
// remain governed by the saved exit policy and never replace its baseline.
func (s *Server) monitorNetworkTick() {
	timing := s.startMonitorTiming()
	defer s.recordMonitorTiming("network", timing)
	if s.storageRecoveryStatus().Required {
		timing.result = "storage_recovery_pending"
		return
	}
	s.monitorProxyInterface()
	timing.stage("physical_interface")
	s.checkProxyHealth()
	timing.stage("proxy_health")
	s.monitorSafety()
	timing.stage("safety")
	// Sampling runs independently of interactive operations. Only admission of
	// a recovery job needs the environment mutation lock.
	if !s.op.TryLock() {
		timing.result = "operation_lock_busy"
		return
	}
	defer s.op.Unlock()
	s.recoverNetwork()
	timing.stage("recovery")
}

func (s *Server) monitorNetwork(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	skipped := 0
	for ctx.Err() == nil {
		if s.op.TryLock() {
			s.op.Unlock()
			skipped = 0
			s.monitorNetworkTick()
		} else {
			skipped++
			timing := s.startMonitorTiming()
			timing.result = "operation_lock_busy"
			timing.stages = append(timing.stages, fmt.Sprintf("consecutive_lock_skips=%d", skipped))
			s.recordMonitorTiming("network", timing)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func vmAlive(profile, workspace string) bool {
	if workspace == "" {
		return false
	}
	b, e := os.ReadFile(filepath.Join(profile, "state", "lima", eden.InstanceName(profile, workspace), "eden-running.json"))
	if e != nil {
		return false
	}
	var record struct {
		PID     int    `json:"pid"`
		Started string `json:"started"`
	}
	if json.Unmarshal(b, &record) != nil || record.PID < 1 {
		return false
	}
	return processExists(record.PID) && eden.ProcessStartMatches(record.PID, record.Started)
}
func (s *Server) vmRunning() bool {
	return vmAlive(s.profile, s.environmentDir())
}
