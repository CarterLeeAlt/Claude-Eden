package desktopupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sylvinus/agent-vm/internal/host"
	"github.com/sylvinus/agent-vm/internal/installfiles"
)

type launchConfirmation struct {
	Version string `json:"version"`
	Error   string `json:"error,omitempty"`
}

// MarkStartupFailed reports a real failure or explicit startup cancellation of
// the update candidate. Ordinary startups and already-ready processes do not
// have an outstanding confirmation token and leave the update result alone.
func (u *Updater) MarkStartupFailed(cause error) error {
	if cause == nil {
		return errors.New("缺少工作台启动失败原因")
	}
	message := strings.ToValidUTF8(cause.Error(), "�")
	if errors.Is(cause, context.Canceled) {
		message = "工作台启动已取消"
	}
	if strings.TrimSpace(message) == "" {
		message = "工作台启动失败，后台未返回错误详情"
	}
	for {
		encoded, _ := json.Marshal(message)
		if len(encoded) <= 8192 {
			break
		}
		message = strings.ToValidUTF8(message[:len(message)/2], "�")
	}
	acknowledged, err := u.writeConfirmation(message)
	if err != nil {
		// A disk/permission error must not leave the helper waiting for an
		// acknowledgement that this failed candidate could not publish. Use
		// the existing safe application-exit callback, never a time limit.
		if confirmationPattern.MatchString(os.Getenv("EDEN_UPDATE_CONFIRMATION")) && u.config.Quit != nil {
			u.config.Quit()
		}
		return err
	}
	if acknowledged {
		u.change("failed", "新版工作台启动失败："+message, false, "")
	}
	return nil
}

func (u *Updater) MarkReady() error {
	acknowledged, err := u.writeConfirmation("")
	if err != nil {
		return err
	}
	if acknowledged {
		u.change("current", "Claude Eden 已更新至 "+u.config.Version, false, "")
	}
	return nil
}

// Serialize terminal acknowledgements so a delayed callback cannot replace
// an already accepted ready/failure result from this candidate process.
func (u *Updater) writeConfirmation(message string) (bool, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	token := os.Getenv("EDEN_UPDATE_CONFIRMATION")
	if !confirmationPattern.MatchString(token) {
		return false, nil
	}
	if err := installfiles.PlainPath(u.config.Root); err != nil {
		return false, err
	}
	if err := host.EnsurePrivateDir(u.config.Root); err != nil {
		return false, err
	}
	if err := installfiles.PlainPath(u.config.Root); err != nil {
		return false, err
	}
	record := launchConfirmation{Version: u.config.Version, Error: message}
	for {
		encoded, err := json.Marshal(record)
		if err != nil {
			return false, err
		}
		if len(encoded) <= 16384 {
			break
		}
		if len(record.Error) <= 16 {
			return false, errors.New("工作台确认版本信息超过允许大小")
		}
		record.Error = strings.ToValidUTF8(record.Error[:len(record.Error)/2], "�")
		if record.Error == "" {
			record.Error = "启动失败"
		}
	}
	if err := writeResultFile(filepath.Join(u.config.Root, "launch-"+token+".json"), record); err != nil {
		return false, err
	}
	if err := os.Unsetenv("EDEN_UPDATE_CONFIRMATION"); err != nil {
		return false, err
	}
	return true, nil
}

func waitForLaunch(ctx context.Context, marker, version string, exited chan error) error {
	var sharingSince time.Time
	for {
		var confirmation launchConfirmation
		err := readSmallJSON(marker, &confirmation, 16384)
		if err == nil {
			if confirmation.Version != version {
				return errors.New("新版工作台启动确认版本不匹配")
			}
			if confirmation.Error != "" {
				return fmt.Errorf("新版工作台启动失败：%s", confirmation.Error)
			}
			return nil
		}
		if isSharingViolation(err) {
			if sharingSince.IsZero() {
				sharingSince = time.Now()
			}
			if time.Since(sharingSince) >= 3*time.Second {
				return fmt.Errorf("工作台启动回执持续被占用：%w", err)
			}
		} else {
			sharingSince = time.Time{}
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("无法读取工作台启动回执：%w", err)
			}
		}
		select {
		case exitErr := <-exited:
			exited <- exitErr
			return errors.New("新版工作台未能完成启动确认：目标进程已退出")
		default:
		}
		if err = sleep(ctx, 500*time.Millisecond); err != nil {
			return err
		}
	}
}
