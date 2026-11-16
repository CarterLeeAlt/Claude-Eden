package desktopupdate

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sylvinus/agent-vm/internal/installfiles"
)

// RunInstaller is the --install-update process entry, before any Wails runtime
// starts. EOF, cancellation or anything other than COMMIT revokes preparation.
func RunInstaller(ctx context.Context, root, executable string, input io.Reader, output io.Writer) (err error) {
	return RunInstallerWithDiagnostics(ctx, root, executable, input, output, "")
}

// RunInstallerWithDiagnostics additionally enables NSIS's path-free phase
// report. The caller chooses an absolute private diagnostic file.
func RunInstallerWithDiagnostics(ctx context.Context, root, executable string, input io.Reader, output io.Writer, diagnostics string) (err error) {
	if root == "" || !filepath.IsAbs(root) {
		return errors.New("更新私有目录无效")
	}
	if err = installfiles.PlainPath(root); err != nil {
		return err
	}
	rootInfo, e := os.Stat(root)
	if e != nil {
		return e
	}
	if !rootInfo.IsDir() {
		return errors.New("更新私有目录无效")
	}
	var plan InstallPlan
	backup := ""
	backupOwned := false
	committed, installerStarted, succeeded := false, false, false
	var launched *exec.Cmd
	var launchedExit chan error
	result := func(success bool, message string) {
		_ = writeResultFile(filepath.Join(root, "last-result.json"), struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}{success, message})
	}
	recoverOriginal := func(recoveryCtx context.Context, detail string) error {
		unlock, e := installationLock()
		if e != nil {
			return e
		}
		defer unlock()
		if e = EnsureInstallationComplete(plan.Target); e != nil {
			return e
		}
		if OriginalIntact(plan.Target, backup) {
			if e = SamePublisher(filepath.Join(plan.Target, "Eden.exe"), plan.Publisher); e != nil {
				return e
			}
			result(false, "更新未完成，原应用已保留："+detail)
			return reopenRecovered(&backup, func() error { return startApplication(plan.Target, plan.ConfigPath, "", plan.Publisher) })
		}
		if !installerStarted {
			return errors.New("原应用在安装前发生变化，备份已保留，请核对安装目录后重试")
		}
		if e = PrepareRollback(recoveryCtx, backup, plan.ConfigPath, plan.Publisher); e != nil {
			return e
		}
		if launched != nil {
			select {
			case <-launchedExit:
			default:
				if e = launched.Process.Kill(); e != nil {
					return e
				}
				<-launchedExit
			}
		}
		if e = restoreApplicationFiles(recoveryCtx, plan, backup); e != nil {
			return e
		}
		result(false, "更新未完成，已恢复原应用："+detail)
		return reopenRecovered(&backup, func() error { return startApplication(plan.Target, plan.ConfigPath, "", plan.Publisher) })
	}
	defer func() {
		if succeeded {
			cleanupDirectory(backup)
			cleanupDirectory(filepath.Dir(plan.Archive))
			_ = os.Remove(filepath.Join(root, "launch-"+plan.Confirmation+".json"))
		} else if !committed && backupOwned {
			cleanupDirectory(backup)
		}
		if err == nil {
			return
		}
		detail := err.Error()
		if !committed || !backupOwned {
			result(false, "更新未完成："+detail)
			return
		}
		recoveryCtx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
		defer cancel()
		recoveryErr := recoverOriginal(recoveryCtx, detail)
		if recoveryErr == nil {
			return
		}
		detail += "。自动恢复未完成。" + recoveryErr.Error()
		if _, e := os.Lstat(plan.Target + ".eden-installation.json"); e == nil {
			detail += "。恢复目录记录在 " + plan.Target + ".eden-installation.json"
		}
		if backup != "" {
			detail += "。程序备份保留在 " + backup
		}
		result(false, "更新未完成："+detail)
		showRecovery(detail)
	}()
	protocolCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if closer, ok := input.(io.Closer); ok {
		stopClose := context.AfterFunc(protocolCtx, func() { _ = closer.Close() })
		defer func() { stopClose(); _ = closer.Close() }()
	}
	reader := bufio.NewReaderSize(input, 16387)
	line, e := readLine(protocolCtx, reader, 16384)
	if e != nil {
		return e
	}
	if e = json.Unmarshal([]byte(line), &plan); e != nil {
		return errors.New("安装计划无效")
	}
	if !confirmationPattern.MatchString(plan.Confirmation) || !filepath.IsAbs(plan.Target) || strings.ContainsAny(plan.Target, "\"\x00\r\n") || filepath.Clean(plan.Target) != plan.Target || filepath.Dir(plan.Target) == plan.Target {
		return errors.New("安装目标无效")
	}
	if plan.ConfigPath == "" || !filepath.IsAbs(plan.ConfigPath) {
		return errors.New("更新配置文件必须是绝对路径")
	}
	if err = installfiles.PlainPath(plan.Target); err != nil {
		return err
	}
	// Do not retain an installation-directory current-directory handle.
	if err = os.Chdir(root); err != nil {
		return err
	}
	identity, e := os.ReadFile(filepath.Join(plan.Target, installfiles.IdentityName))
	if e != nil {
		return e
	}
	if strings.TrimSpace(string(identity)) != installfiles.Identity {
		return errors.New("请先使用 Claude Eden 安装程序安装应用，再自动更新")
	}
	original := filepath.Join(plan.Target, "Eden.exe")
	publisher, e := Publisher(executable)
	if e != nil {
		return e
	}
	if plan.Publisher != publisher {
		return errors.New("安装计划发布者不匹配")
	}
	if err = SamePublisher(original, publisher); err != nil {
		return err
	}
	_, installedVersion, e := ProductVersion(original)
	if e != nil {
		return e
	}
	if !IsNewer(plan.Version, installedVersion) {
		return errors.New("更新版本必须高于当前安装版本")
	}
	archiveLock, e := VerifyArchiveLocked(root, plan.Archive, plan.Digest, plan.Version, publisher)
	if e != nil {
		return e
	}
	defer archiveLock.Close()
	parent, e := openParent(plan.ParentPID, plan.ParentStartTicks, original)
	if e != nil {
		return e
	}
	defer parent.Close()
	// Watch the control pipe during snapshotting as well as at READY. Closing
	// the parent endpoint cancels reversible preparation promptly; a premature
	// COMMIT is not consumed until all verification and backup steps succeed.
	preparationCtx, revoke := context.WithCancel(protocolCtx)
	defer revoke()
	type commandResult struct {
		line string
		err  error
	}
	command := make(chan commandResult, 1)
	go func() {
		line, e := readLine(protocolCtx, reader, 16384)
		if e != nil || line != "COMMIT" {
			revoke()
		}
		command <- commandResult{line, e}
	}()
	backup = filepath.Join(root, "previous-"+plan.Confirmation)
	if _, e = os.Lstat(backup); !errors.Is(e, os.ErrNotExist) {
		return errors.New("更新备份目录已存在")
	}
	if err = copyTree(preparationCtx, plan.Target, backup, &backupOwned); err != nil {
		return err
	}
	if err = SamePublisher(BackendPath(backup), publisher); err != nil {
		return err
	}
	if _, err = fmt.Fprintln(output, "READY"); err != nil {
		return err
	}
	if flusher, ok := output.(interface{ Flush() error }); ok {
		if err = flusher.Flush(); err != nil {
			return err
		}
	}
	var requested commandResult
	select {
	case requested = <-command:
	case <-protocolCtx.Done():
		return protocolCtx.Err()
	}
	if requested.err != nil {
		return requested.err
	}
	if requested.line != "COMMIT" {
		return errors.New("安装准备已撤销，原应用已保留")
	}
	if e = protocolCtx.Err(); e != nil {
		return e
	}
	committed = true
	// The committed helper owns the transaction, independent of the exiting
	// parent window. Bound individual operations, never candidate startup.
	transactionCtx := context.WithoutCancel(ctx)
	parentCtx, parentCancel := context.WithTimeout(transactionCtx, 45*time.Second)
	e = parent.Wait(parentCtx)
	parentCancel()
	if e != nil {
		return fmt.Errorf("原应用未能退出：%w", e)
	}
	if err = verifyArchive(archiveLock, plan.Archive, plan.Digest, plan.Version, publisher); err != nil {
		return err
	}
	if diagnostics != "" {
		if !filepath.IsAbs(diagnostics) || strings.ContainsAny(diagnostics, "\"\x00\r\n") {
			return errors.New("安装诊断路径无效")
		}
		if err = installfiles.PlainPath(diagnostics); err != nil {
			return err
		}
	}
	installer := installerCommand(plan.Archive, plan.Target, diagnostics)
	installerStarted = true
	installCtx, installCancel := context.WithTimeout(transactionCtx, 10*time.Minute)
	e = runProcess(installCtx, installer)
	installCancel()
	if e != nil {
		return fmt.Errorf("Windows 安装程序未完成安装：%w", e)
	}
	if err = EnsureInstallationComplete(plan.Target); err != nil {
		return err
	}
	installedLock, e := installfiles.OpenReadLocked(original)
	if e != nil {
		return e
	}
	defer installedLock.Close()
	if err = SamePublisher(original, publisher); err != nil {
		return err
	}
	_, installedVersion, e = ProductVersion(original)
	if e != nil {
		return e
	}
	if installedVersion != plan.Version {
		return errors.New("安装后版本不匹配")
	}
	marker := filepath.Join(root, "launch-"+plan.Confirmation+".json")
	if err = installfiles.PlainPath(marker); err != nil {
		return err
	}
	if e = os.Remove(marker); e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	candidate := exec.Command(original)
	hide(candidate)
	candidate.Dir = plan.Target
	candidate.Env = profileEnvironment(plan.ConfigPath, plan.Confirmation)
	launched, launchedExit, err = startTrackedApplication(candidate)
	if err != nil {
		return fmt.Errorf("新版工作台进程未能创建：%w", err)
	}
	_ = installedLock.Close()
	// A slow startup is not a failed installation. Only a real ready/error
	// acknowledgement or the exact candidate's exit ends this lifecycle wait.
	if err = waitForLaunch(transactionCtx, marker, plan.Version, launchedExit); err != nil {
		return err
	}
	succeeded = true
	result(true, "Claude Eden 已更新至 "+plan.Version)
	return nil
}

func restoreApplicationFiles(ctx context.Context, plan InstallPlan, backup string) error {
	restored, failed := plan.Target+".restore-"+plan.Confirmation, plan.Target+".failed-"+plan.Confirmation
	if err := CopyTree(ctx, backup, restored); err != nil {
		return err
	}
	receipt := plan.Target + ".eden-installation.json"
	preserve := func(op, source, target string) error {
		return installfiles.Run(ctx, installfiles.Options{Operation: op, Source: source, Target: target, Receipt: receipt, Backup: failed, LegacyTarget: true, KeepInPlace: []string{"EdenData"}})
	}
	if err := preserve("prepare", plan.Target, restored); err != nil {
		return err
	}
	if err := preserve("detach", plan.Target, failed); err != nil {
		return err
	}
	err := preserve("move", failed, restored)
	if err == nil {
		err = preserve("attach", restored, plan.Target)
	}
	if err != nil {
		// Return every already-moved personal object before restoring the
		// candidate. Keep the receipt if any reversal fails.
		if reverse := preserve("restore", restored, failed); reverse != nil {
			return errors.Join(err, reverse)
		}
		if reverse := preserve("attach", failed, plan.Target); reverse != nil {
			return errors.Join(err, reverse)
		}
		if reverse := preserve("complete", "", plan.Target); reverse != nil {
			return errors.Join(err, reverse)
		}
		return err
	}
	if err := preserve("cleanup", failed, plan.Target); err != nil {
		return err
	}
	return EnsureInstallationComplete(plan.Target)
}

func startApplication(target, configPath, confirmation, publisher string) error {
	image := filepath.Join(target, "Eden.exe")
	locked, e := installfiles.OpenReadLocked(image)
	if e != nil {
		return e
	}
	defer locked.Close()
	if e = SamePublisher(image, publisher); e != nil {
		return e
	}
	cmd := exec.Command(image)
	hide(cmd)
	cmd.Dir = target
	cmd.Env = profileEnvironment(configPath, confirmation)
	_, _, e = startTrackedApplication(cmd)
	return e
}

// Ownership starts only after CreateProcess succeeds. Recovery receives either
// the exact started command and its single Wait result, or no process at all.
func startTrackedApplication(cmd *exec.Cmd) (*exec.Cmd, chan error, error) {
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	return cmd, exited, nil
}

// A failed relaunch is an incomplete recovery. Retain the verified snapshot
// until the old application process has actually been created successfully.
func reopenRecovered(backup *string, reopen func() error) error {
	if err := reopen(); err != nil {
		return err
	}
	cleanupDirectory(*backup)
	*backup = ""
	return nil
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
