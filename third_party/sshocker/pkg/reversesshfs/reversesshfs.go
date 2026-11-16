package reversesshfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/lima-vm/sshocker/pkg/util"
	"github.com/sirupsen/logrus"
)

type Driver = string

const (
	DriverAuto              = Driver("auto")                // Default
	DriverBuiltin           = Driver("builtin")             // Legacy. Unrecommended.
	DriverOpensshSftpServer = Driver("openssh-sftp-server") // More robust and secure. Recommended.
)

type ReverseSSHFS struct {
	*ssh.SSHConfig
	Driver                  Driver
	OpensshSftpServerBinary string // used only when Driver == DriverOpensshSftpServer
	LocalPath               string
	Host                    string
	Port                    int
	RemotePath              string
	Readonly                bool
	sshCmd                  *exec.Cmd
	sshDone                 chan struct{}
	sshErr                  error
	serverDone              chan struct{}
	opensshSftpServerCmd    *exec.Cmd
	SSHFSAdditionalArgs     []string

	// ReadonlyNames makes a path read-only when any of its components is one of these names
	// (compared case-insensitively). Requires DriverBuiltin on Linux, macOS, or Windows.
	ReadonlyNames []string

	rooted atomic.Pointer[rootedHandlers]
}

// ExpectRemove makes the next removal of hostPath requested by the remote, within a few seconds,
// succeed without touching hostPath. It is used to relay a local deletion to the remote,
// by removing the path there, which emits an inotify event on the remote.
// It returns false, and the removal must not be relayed, when unsupported (not using DriverBuiltin
// on Linux, macOS, or Windows), or when the remote itself just removed or moved away hostPath.
func (rsf *ReverseSSHFS) ExpectRemove(hostPath string) bool {
	h := rsf.rooted.Load()
	if h == nil {
		return false
	}
	return h.expectRemove(hostPath)
}

func (rsf *ReverseSSHFS) Prepare(ctx context.Context) error {
	sshBinary := rsf.SSHConfig.Binary()
	sshArgs := rsf.SSHConfig.Args()
	if !path.IsAbs(rsf.RemotePath) {
		return fmt.Errorf("unexpected relative path: %q", rsf.RemotePath)
	}
	if rsf.Port != 0 {
		sshArgs = append(sshArgs, "-p", strconv.Itoa(rsf.Port))
	}
	sshArgs = append(sshArgs, rsf.Host, "--")
	sshArgs = append(sshArgs, "mkdir", "-p", addQuotes(rsf.RemotePath))
	sshCmd := exec.CommandContext(ctx, sshBinary, sshArgs...)
	sshCmd.WaitDelay = time.Second
	logrus.Debugf("executing ssh for preparing sshfs: %s %v", sshCmd.Path, sshCmd.Args)
	out, err := sshCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to mkdir %q (remote): %q: %w", rsf.RemotePath, string(out), errors.Join(ctx.Err(), err))
	}
	return nil
}

func DetectOpensshSftpServerBinary() string {
	if exe, err := exec.LookPath("sftp-server"); err == nil {
		return exe
	}
	homebrewSSHD := []string{
		"/usr/local/sbin/sshd",
		"/opt/homebrew/sbin/sshd",
	}
	for _, f := range homebrewSSHD {
		// sshd is like "/usr/local/Cellar/openssh/8.9p1/sbin/sshd"
		sshd, err := filepath.EvalSymlinks(f)
		if err != nil {
			continue
		}
		// local is like "/usr/local/Cellar/openssh"
		local := filepath.Dir(filepath.Dir(sshd))
		// sftpServer is like "/usr/local/Cellar/openssh/8.9p1/libexec/sftp-server"
		sftpServer := filepath.Join(local, "libexec", "sftp-server")
		if exe, err := exec.LookPath(sftpServer); err == nil {
			return exe
		}
	}
	if runtime.GOOS == "windows" {
		// unix path is like "/usr/lib/ssh/sftp-server"
		cygpathCmd := exec.Command("cygpath", "-w", "/usr/lib/ssh/sftp-server")
		// windows path is like `C:\msys64\usr\lib\ssh\sftp-server.exe`
		if out, err := cygpathCmd.Output(); err == nil {
			sftpServer := strings.TrimSpace(string(out))
			if exe, err := exec.LookPath(sftpServer); err == nil {
				return exe
			}
		}
	}
	candidates := []string{
		"/usr/libexec/sftp-server",         // macOS, OpenWrt
		"/usr/libexec/openssh/sftp-server", // Fedora
		"/usr/lib/sftp-server",             // Debian (symlink to openssh/sftp-server)
		"/usr/lib/openssh/sftp-server",     // Debian
		"/usr/lib/ssh/sftp-server",         // Alpine
	}
	for _, cand := range candidates {
		if exe, err := exec.LookPath(cand); err == nil {
			return exe
		}
	}
	return ""
}

func DetectDriver(explicitOpensshSftpServerBinary string) (Driver, string, error) {
	if explicitOpensshSftpServerBinary != "" {
		exe, err := exec.LookPath(explicitOpensshSftpServerBinary)
		if err != nil {
			return "", "", err
		}
		return DriverOpensshSftpServer, exe, nil
	}
	exe := DetectOpensshSftpServerBinary()
	if exe != "" {
		return DriverOpensshSftpServer, exe, nil
	}
	return DriverBuiltin, "", nil
}

// Start uses ctx for mount readiness; an established mount lives until Close.
func (rsf *ReverseSSHFS) Start(ctx context.Context) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	sshBinary := rsf.SSHConfig.Binary()
	sshArgs := rsf.SSHConfig.Args()
	if !filepath.IsAbs(rsf.LocalPath) && !path.IsAbs(rsf.LocalPath) {
		return fmt.Errorf("unexpected relative path: %q", rsf.LocalPath)
	}
	if runtime.GOOS == "windows" && path.IsAbs(rsf.LocalPath) {
		logrus.Infof("Accepting %q Unix path, assuming Cygwin/msys2 OpenSSH", rsf.LocalPath)
		// Convert MSYS2 path to Windows native path
		rsf.LocalPath = convertMSYS2Path(rsf.LocalPath)
		logrus.Infof("Converted path for native Windows OpenSSH: %q", rsf.LocalPath)
	}

	if !path.IsAbs(rsf.RemotePath) {
		return fmt.Errorf("unexpected relative path: %q", rsf.RemotePath)
	}
	if rsf.Port != 0 {
		sshArgs = append(sshArgs, "-p", strconv.Itoa(rsf.Port))
	}
	sshArgs = append(sshArgs, rsf.Host, "--")
	sshArgs = append(sshArgs, "sshfs", addQuotes(":"+rsf.LocalPath), addQuotes(rsf.RemotePath), "-o", "slave")
	if rsf.Readonly {
		sshArgs = append(sshArgs, "-o", "ro")
	}
	sshArgs = append(sshArgs, rsf.SSHFSAdditionalArgs...)
	rsf.sshCmd = exec.Command(sshBinary, sshArgs...)
	rsf.sshCmd.Stderr = os.Stderr
	driver := rsf.Driver
	opensshSftpServerBinary := rsf.OpensshSftpServerBinary
	switch driver {
	case DriverBuiltin, DriverOpensshSftpServer:
		// NOP
	case "", DriverAuto:
		if len(rsf.ReadonlyNames) > 0 {
			driver = DriverBuiltin
			break
		}
		var err error
		driver, opensshSftpServerBinary, err = DetectDriver(opensshSftpServerBinary)
		if err != nil {
			return fmt.Errorf("failed to choose driver automatically: %w", err)
		}
		logrus.Debugf("Chosen driver %q", driver)
	default:
		return fmt.Errorf("unknown driver %q", driver)
	}
	if len(rsf.ReadonlyNames) > 0 && driver != DriverBuiltin {
		return fmt.Errorf("ReadonlyNames requires driver %q, got %q", DriverBuiltin, driver)
	}
	var (
		builtinSftpServer interface{ Serve() error }
		rooted            *rootedHandlers
	)
	switch driver {
	case DriverBuiltin:
		stdinPipe, err := rsf.sshCmd.StdinPipe()
		if err != nil {
			return err
		}
		stdoutPipe, err := rsf.sshCmd.StdoutPipe()
		if err != nil {
			return err
		}
		stdio := &util.RWC{
			ReadCloser:  stdoutPipe,
			WriteCloser: stdinPipe,
		}
		builtinSftpServer, rooted, err = newRootedServer(stdio, rsf.LocalPath, rsf.Readonly, rsf.ReadonlyNames)
		rsf.rooted.Store(rooted)
		if err != nil {
			return err
		}
	case DriverOpensshSftpServer:
		if opensshSftpServerBinary == "" {
			opensshSftpServerBinary = DetectOpensshSftpServerBinary()
			if opensshSftpServerBinary == "" {
				return errors.New("no openssh sftp-server found")
			}
		}
		logrus.Debugf("Using OpenSSH SFTP Server %q", opensshSftpServerBinary)
		sftpServerArgs := []string{
			// `-e` available since OpenSSH 5.4p1 (2010) https://github.com/openssh/openssh-portable/commit/7bee06ab
			"-e",
			// `-d` available since OpenSSH 6.2p1 (2013) https://github.com/openssh/openssh-portable/commit/502ab0ef
			// NOTE: `-d` just chdirs the sftp server process to the specified directory.
			// This is expected to be used in conjunction with chroot (in future), however, macOS does not support unprivileged chroot.
			"-d", strings.ReplaceAll(rsf.LocalPath, "%", "%%"),
		}
		if rsf.Readonly {
			// `-R` available since OpenSSH 5.4p1 (2010) https://github.com/openssh/openssh-portable/commit/db7bf825
			sftpServerArgs = append(sftpServerArgs, "-R")
		}
		rsf.opensshSftpServerCmd = exec.Command(opensshSftpServerBinary, sftpServerArgs...)
		rsf.opensshSftpServerCmd.Stderr = os.Stderr
		var err error
		rsf.opensshSftpServerCmd.Stdin, err = rsf.sshCmd.StdoutPipe()
		if err != nil {
			return err
		}
		rsf.sshCmd.Stdin, err = rsf.opensshSftpServerCmd.StdoutPipe()
		if err != nil {
			return err
		}
	}
	logrus.Debugf("executing ssh for remote sshfs: %s %v", rsf.sshCmd.Path, rsf.sshCmd.Args)
	if err := rsf.sshCmd.Start(); err != nil {
		if rooted != nil {
			rooted.Close()
		}
		return err
	}
	rsf.sshDone = make(chan struct{})
	go func() {
		rsf.sshErr = rsf.sshCmd.Wait()
		close(rsf.sshDone)
	}()
	defer func() {
		if err != nil {
			err = errors.Join(err, rsf.Close())
		}
	}()
	logrus.Debugf("starting sftp server for %v", rsf.LocalPath)
	switch driver {
	case DriverBuiltin:
		rsf.serverDone = make(chan struct{})
		go func() {
			defer close(rsf.serverDone)
			if rooted != nil {
				defer rooted.Close()
			}
			if srvErr := builtinSftpServer.Serve(); srvErr != nil {
				if errors.Is(srvErr, io.EOF) {
					logrus.WithError(srvErr).Debugf("sftp server for %v exited with EOF (negligible)", rsf.LocalPath)
				} else {
					logrus.WithError(srvErr).Errorf("sftp server for %v exited", rsf.LocalPath)
				}
			}
		}()
	case DriverOpensshSftpServer:
		logrus.Debugf("executing OpenSSH SFTP Server: %s %v", rsf.opensshSftpServerCmd.Path, rsf.opensshSftpServerCmd.Args)
		if err := rsf.opensshSftpServerCmd.Start(); err != nil {
			return err
		}
	}
	logrus.Debugf("waiting for remote ready")
	if err := rsf.waitForRemoteReady(ctx); err != nil {
		return fmt.Errorf("failed to confirm mount %q: %w", rsf.RemotePath, err)
	}
	return nil
}

func addQuotes(input string) string {
	input = strconv.Quote(input)
	// exec.Command on Windows would escape wrapping double quotes in a way, which is not compatible
	// with passing arguments into bash shell over ssh, replacing with single quotes as a workaround.
	if runtime.GOOS == "windows" {
		input = strings.TrimPrefix(input, "\"")
		input = strings.TrimSuffix(input, "\"")
		return fmt.Sprintf(`'%s'`, input)
	}
	return input
}

func (rsf *ReverseSSHFS) waitForRemoteReady(ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(ctx)
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		select {
		case <-rsf.sshDone:
			cancel(fmt.Errorf("sshfs transport exited before mount readiness: %w", errors.Join(io.ErrUnexpectedEOF, rsf.sshErr)))
		case <-ctx.Done():
		}
	}()
	defer func() { cancel(nil); <-watchDone }()
	scriptName := "wait-for-remote-ready"
	scriptTemplate := `#!/bin/sh
set -eu
dir="{{.Dir}}"
LANG=C
LC_ALL=C
export LANG LC_ALL
# Spaces in file names are encoded as '\040' in the mount table.
mount | sed 's/\\040/ /g' | grep "on ${dir}" | grep -Eqw "fuse.sshfs|osxfuse"
`
	t, err := template.New(scriptName).Parse(scriptTemplate)
	if err != nil {
		return err
	}
	m := map[string]string{
		// rsf.RemotePath should have been verified during rsf.Prepare()
		"Dir": rsf.RemotePath,
	}
	var b bytes.Buffer
	if err := t.Execute(&b, m); err != nil {
		return err
	}
	script := b.String()
	logrus.Debugf("generated script %q with map %v: %q", scriptName, m, script)
	for {
		stdout, stderr, err := ssh.ExecuteScript(ctx, rsf.Host, rsf.Port, rsf.SSHConfig, script, scriptName)
		logrus.Debugf("executed script %q, stdout=%q, stderr=%q, err=%v", scriptName, stdout, stderr, err)
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		if err == nil {
			return nil
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return err
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return context.Cause(ctx)
		case <-timer.C:
		}
	}
}

func (rsf *ReverseSSHFS) Close() error {
	if rsf.sshCmd != nil {
		logrus.Debugf("killing processes for remote sshfs: %s %v", rsf.sshCmd.Path, rsf.sshCmd.Args)
	}
	var errs []error
	for _, cmd := range []*exec.Cmd{rsf.sshCmd, rsf.opensshSftpServerCmd} {
		if cmd == nil || cmd.Process == nil {
			continue
		}
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			errs = append(errs, err)
		}
		if cmd == rsf.sshCmd && rsf.sshDone != nil {
			<-rsf.sshDone
		} else {
			_ = cmd.Wait()
		}
	}
	if rsf.serverDone != nil {
		<-rsf.serverDone
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing reverse sshfs: %w", errors.Join(errs...))
	}
	return nil
}

// convertMSYS2Path converts an MSYS2 style path (e.g., /c/Users/...) to a Windows native path
func convertMSYS2Path(localPath string) string {
	if len(localPath) >= 3 && localPath[0] == '/' && localPath[2] == '/' {
		driveLetter := strings.ToUpper(string(localPath[1]))
		// Explicitly force backslashes, bypassing WSL/Linux environment quirks
		remainingPath := strings.ReplaceAll(localPath[2:], "/", "\\")
		return driveLetter + ":" + remainingPath
	}
	return localPath
}
