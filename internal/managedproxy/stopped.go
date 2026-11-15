package managedproxy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// ConfirmStopped performs no cleanup. Linux and Windows remove their private
// process record only after stopping the identified process.
func ConfirmStopped(ctx context.Context, profile string) error {
	path := filepath.Join(Dir(profile), "managed-process.json")
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("无法确认此配置的代理进程已经退出")
}
