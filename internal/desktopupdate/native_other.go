//go:build !windows

package desktopupdate

import (
	"context"
	"errors"
	"os/exec"
)

func Publisher(path string) (string, error) {
	return "", errors.New("Windows 更新签名仅支持 Windows")
}
func ProductVersion(path string) (string, string, error) {
	return "", "", errors.New("Windows 版本资源仅支持 Windows")
}
func CurrentStartTicks() (int64, error) { return 0, errors.New("自动更新仅支持 Windows") }

type parentProcess struct{}

func openParent(pid int, ticks int64, image string) (*parentProcess, error) {
	return nil, errors.New("自动更新仅支持 Windows")
}
func (p *parentProcess) Close() {}
func (p *parentProcess) Wait(ctx context.Context) error {
	return errors.New("自动更新仅支持 Windows")
}
func installationLock() (func(), error) { return nil, errors.New("自动更新仅支持 Windows") }
func hide(cmd *exec.Cmd)                {}
func isSharingViolation(err error) bool { return false }
func installerCommand(path, target, diagnostics string) *exec.Cmd {
	return exec.Command(path, "/S", "/UPDATE", "/D="+target)
}
func runProcess(ctx context.Context, cmd *exec.Cmd) error {
	return errors.New("自动更新仅支持 Windows")
}
func showRecovery(message string) {}
