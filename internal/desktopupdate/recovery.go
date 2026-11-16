package desktopupdate

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
	"strings"
	"time"

	"github.com/sylvinus/agent-vm/internal/installfiles"
)

func BackendPath(backup string) string { return filepath.Join(backup, "resources", "eden.exe") }

func copyFile(ctx context.Context, source, target string) (err error) {
	return copyFileOwned(ctx, source, target, nil)
}

func copyFileOwned(ctx context.Context, source, target string, created *bool) (err error) {
	if err = installfiles.PlainPath(source); err != nil {
		return err
	}
	if err = installfiles.PlainPath(target); err != nil {
		return err
	}
	input, err := installfiles.OpenReadLocked(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if created != nil {
		*created = true
	}
	defer func() { err = errors.Join(err, output.Close()) }()
	_, err = io.Copy(output, &contextReader{ctx, input})
	if err == nil {
		err = output.Sync()
	}
	return err
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.reader.Read(b)
}

// CopyTree snapshots only declared program paths when a valid ownership
// manifest exists. Legacy trees copy regular files but never follow links.
func CopyTree(ctx context.Context, source, target string) error {
	return copyTree(ctx, source, target, nil)
}

// created is set only after this invocation's exclusive mkdir succeeds; a
// colliding pre-existing directory never becomes cleanup authority.
func copyTree(ctx context.Context, source, target string, created *bool) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	source, e := filepath.Abs(source)
	if e != nil {
		return e
	}
	target, e = filepath.Abs(target)
	if e != nil {
		return e
	}
	lowerSource, lowerTarget := strings.ToLower(source), strings.ToLower(target)
	if lowerSource == lowerTarget || strings.HasPrefix(lowerTarget, lowerSource+string(filepath.Separator)) || strings.HasPrefix(lowerSource, lowerTarget+string(filepath.Separator)) {
		return errors.New("更新备份目录与程序目录重叠，无法安全备份。请使用完整安装包手动更新")
	}
	if e = installfiles.PlainPath(source); e != nil {
		return e
	}
	if e = installfiles.PlainPath(target); e != nil {
		return e
	}
	if e = os.Mkdir(target, 0700); e != nil {
		return e
	}
	if created != nil {
		*created = true
	}
	owned := installfiles.ReadManifest(filepath.Join(source, installfiles.ManifestName))
	if len(owned) > 0 {
		owned[strings.ToLower(installfiles.ManifestName)] = installfiles.File{Path: installfiles.ManifestName}
	}
	var size int64
	count := 0
	return filepath.WalkDir(source, func(name string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == source {
			return nil
		}
		if e = ctx.Err(); e != nil {
			return e
		}
		count++
		if count > 100000 {
			return errors.New("安装目录文件过多，无法安全备份")
		}
		if e = installfiles.PlainPath(name); e != nil {
			if !errors.Is(e, installfiles.ErrReparsePath) {
				return e
			}
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, e := filepath.Rel(source, name)
		if e != nil {
			return e
		}
		key := strings.ToLower(filepath.ToSlash(rel))
		if len(owned) > 0 {
			_, include := owned[key]
			if !include {
				for k := range owned {
					if strings.HasPrefix(k, key+"/") {
						include = true
						break
					}
				}
			}
			if !include {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		destination := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.Mkdir(destination, 0700)
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		size += info.Size()
		if size > 4<<30 {
			return errors.New("安装目录超过允许的备份大小")
		}
		return copyFile(ctx, name, destination)
	})
}

func OriginalIntact(target, backup string) bool {
	if _, e := os.Lstat(target + ".eden-installation.json"); !errors.Is(e, os.ErrNotExist) {
		return false
	}
	if installfiles.PlainPath(target) != nil || installfiles.PlainPath(backup) != nil {
		return false
	}
	for _, p := range []string{filepath.Join(backup, "Eden.exe"), filepath.Join(backup, installfiles.IdentityName)} {
		if _, e := os.Stat(p); e != nil {
			return false
		}
	}
	e := filepath.WalkDir(backup, func(source string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if e := installfiles.PlainPath(source); e != nil {
			return e
		}
		rel, e := filepath.Rel(backup, source)
		if e != nil {
			return e
		}
		installed := filepath.Join(target, rel)
		if e = installfiles.PlainPath(installed); e != nil {
			return e
		}
		before, e := installfiles.OpenReadLocked(source)
		if e != nil {
			return e
		}
		defer before.Close()
		after, e := installfiles.OpenReadLocked(installed)
		if e != nil {
			return e
		}
		defer after.Close()
		beforeHash, e := installfiles.Hash(before)
		if e != nil {
			return e
		}
		afterHash, e := installfiles.Hash(after)
		if e != nil {
			return e
		}
		if beforeHash != afterHash {
			return errors.New("installed program differs from snapshot")
		}
		return nil
	})
	return e == nil
}

func EnsureInstallationComplete(target string) error {
	receipt := target + ".eden-installation.json"
	_, e := os.Lstat(receipt)
	info, dirErr := os.Stat(target)
	if errors.Is(e, os.ErrNotExist) && dirErr == nil && info.IsDir() {
		return installfiles.PlainPath(target)
	}
	message := "安装目录或个人文件仍需恢复，已停止自动启动。请保留并检查 " + receipt
	var record struct {
		StagePath  string `json:"stagePath"`
		BackupPath string `json:"backupPath"`
	}
	if readSmallJSON(receipt, &record, 16<<20) == nil {
		for _, p := range []string{record.StagePath, record.BackupPath} {
			if p != "" {
				message += "。保留目录 " + p
			}
		}
	}
	return errors.New(message)
}

func profileEnvironment(configPath, confirmation string) []string {
	env := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "EDEN_UPDATE_CONFIRMATION") || (configPath != "" && strings.EqualFold(key, "EDEN_CONFIG")) {
			continue
		}
		env = append(env, entry)
	}
	if configPath != "" {
		env = append(env, "EDEN_CONFIG="+configPath)
	}
	if confirmation != "" {
		env = append(env, "EDEN_UPDATE_CONFIRMATION="+confirmation)
	}
	return env
}

// PrepareRollback invokes the exact backed-up controller and preserves the
// selected absolute profile. The broken candidate is never used for recovery.
func PrepareRollback(ctx context.Context, backup, configPath, publisher string) error {
	backend := BackendPath(backup)
	locked, e := installfiles.OpenReadLocked(backend)
	if e != nil {
		return e
	}
	defer locked.Close()
	if e = SamePublisher(backend, publisher); e != nil {
		return e
	}
	cmd := exec.Command(backend, "prepare-update", "--json", "--stop")
	hide(cmd)
	cmd.Dir = filepath.Dir(backup)
	cmd.Env = profileEnvironment(configPath, "")
	var output limitedBuffer
	output.limit = 65536
	var stderr limitedBuffer
	stderr.limit = 65536
	cmd.Stdout = &output
	cmd.Stderr = &stderr
	deadline, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if e = runProcess(deadline, cmd); e != nil {
		return fmt.Errorf("旧版后台无法确认环境已停止，原应用备份已保留：%w", e)
	}
	var result struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(bytes.TrimSpace(output.Bytes()), &result) != nil || result.Status != "prepared" {
		return errors.New("旧版后台无法确认环境已停止，原应用备份已保留")
	}
	return nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, errors.New("更新响应超过允许大小")
	}
	return b.Buffer.Write(p)
}

// cleanupDirectory is restricted to helper-owned snapshots and downloads. It
// does not run on installation trees; personal links are never traversed.
func cleanupDirectory(name string) {
	if name != "" && installfiles.PlainPath(name) == nil {
		_ = os.RemoveAll(name)
	}
}
func readSmallJSON(name string, value any, limit int64) error {
	if e := installfiles.PlainPath(name); e != nil {
		return e
	}
	f, e := installfiles.OpenReadLocked(name)
	if e != nil {
		return e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil {
		return e
	}
	if int64(len(b)) > limit {
		return errors.New("更新记录超过允许大小")
	}
	return json.Unmarshal(b, value)
}
func writeResultFile(name string, value any) error {
	if e := installfiles.PlainPath(name); e != nil {
		return e
	}
	b, e := json.Marshal(value)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(name), ".eden-update-*.tmp")
	if e != nil {
		return e
	}
	temporary := f.Name()
	defer os.Remove(temporary)
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	if e = errors.Join(e, f.Close()); e != nil {
		return e
	}
	if e = installfiles.PlainPath(name); e != nil {
		return e
	}
	return os.Rename(temporary, name)
}
