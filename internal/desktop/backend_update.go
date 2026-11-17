package desktop

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/sylvinus/agent-vm/internal/appupdate"
	"github.com/sylvinus/agent-vm/internal/desktopupdate"
)

func (b *Backend) Update(ctx context.Context, checkOnly bool, changed func(appupdate.State)) error {
	operation := "download"
	if checkOnly {
		operation = "check"
	}
	command := b.Command(ctx, "app-update", operation)
	output, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	command.Stderr = io.Discard
	if err = command.Start(); err != nil {
		return err
	}
	// A malformed/oversized stream must terminate only this helper and still
	// reap it. The independently running controller remains untouched.
	defer func() {
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 4096), 65537)
	var last appupdate.State
	for scanner.Scan() {
		if len(scanner.Bytes()) > 65536 {
			return errors.New("更新响应超过允许大小")
		}
		if err = json.Unmarshal(scanner.Bytes(), &last); err != nil {
			return errors.New("更新响应格式无效")
		}
		changed(last)
	}
	if err = scanner.Err(); err != nil {
		return err
	}
	if err = command.Wait(); ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		if last.Message != "" {
			return errors.New(last.Message)
		}
		return err
	}
	return nil
}

func (b *Backend) PrepareUpdate(ctx context.Context, stop bool) error {
	args := []string{"prepare-update", "--json"}
	if stop {
		args = append(args, "--stop")
	}
	var result bootstrapReply
	code, err := b.JSON(ctx, 3*time.Minute, 65536, &result, args...)
	if err != nil {
		return err
	}
	if result.Status == "stop_required" {
		return desktopupdate.ErrStopRequired
	}
	if code != 0 || result.Status != "prepared" {
		if result.Message == "" {
			result.Message = "环境未能安全停止"
		}
		return errors.New(result.Message)
	}
	return nil
}
