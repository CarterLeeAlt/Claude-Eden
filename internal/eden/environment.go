package eden

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/sylvinus/agent-vm/internal/paths"
	"github.com/sylvinus/agent-vm/internal/vmname"
)

type EnvironmentRecord struct {
	Directory         string `json:"directory"`
	IdentityDirectory string `json:"identity_directory,omitempty"`
	Portable          bool   `json:"portable,omitempty"`
}

func LoadEnvironmentRecord(profile string) (EnvironmentRecord, error) {
	var saved EnvironmentRecord
	data, err := os.ReadFile(filepath.Join(profile, "environment.json"))
	if errors.Is(err, os.ErrNotExist) {
		return saved, nil
	}
	if err != nil {
		return saved, err
	}
	if json.Unmarshal(data, &saved) != nil {
		return saved, errors.New("环境工作目录记录无效，请保留原配置并导出诊断记录")
	}
	if saved.Portable {
		root := PortableDataDirectory(profile)
		if root == "" || saved.IdentityDirectory != "" {
			return saved, errors.New("便携环境记录必须位于随包 data/profile 内")
		}
		resolved, err := portableRelativePath(root, saved.Directory)
		if err != nil {
			return saved, err
		}
		saved.Directory = resolved
		return saved, nil
	}
	if !filepath.IsAbs(saved.Directory) ||
		saved.IdentityDirectory != "" && !filepath.IsAbs(saved.IdentityDirectory) {
		return saved, errors.New("环境工作目录记录无效，请保留原配置并导出诊断记录")
	}
	return saved, nil
}

// DefaultEnvironmentDir keeps download transfers outside the default profile,
// including independent profiles stored below that profile's directory.
func DefaultEnvironmentDir(profile string) string {
	path := filepath.Clean(profile) + "-downloads"
	if home, err := os.UserHomeDir(); err == nil {
		defaultProfile := filepath.Join(home, ".eden")
		if !paths.Equal(profile, defaultProfile) && StorageContains(defaultProfile, profile) {
			if relative, err := filepath.Rel(defaultProfile, profile); err == nil {
				path = filepath.Join(defaultProfile+"-downloads", relative)
			}
		}
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

// EnvironmentDir is the internal transfer directory, independent of the selected
// project. A saved record keeps the chosen transfer location stable.
func EnvironmentDir(profile string) string {
	if saved, err := LoadEnvironmentRecord(profile); err == nil && saved.Directory != "" {
		return saved.Directory
	}
	return DefaultEnvironmentDir(profile)
}

func EnvironmentIdentityDir(profile string) string {
	if saved, err := LoadEnvironmentRecord(profile); err == nil && saved.IdentityDirectory != "" {
		return saved.IdentityDirectory
	}
	return EnvironmentDir(profile)
}

// InstanceName is shared by the controller and each Eden CLI subprocess.
// It uses the same spelling as the CLI's AbsDir, including Windows drives.
func InstanceName(profile, workspace string) string {
	// There is exactly one work environment per portable profile. Its identity
	// must not change when the application moves to a different drive or user.
	if PortableDataDirectory(profile) != "" {
		// Keep an already-created disk's directory name as its portable ID.
		// Never derive a replacement ID from the new installation path.
		home := filepath.Join(profile, "state", "lima")
		entries, _ := os.ReadDir(home)
		name := ""
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == "base" || entry.Name() == "build" || strings.HasPrefix(entry.Name(), "_") {
				continue
			}
			if _, err := os.Stat(filepath.Join(home, entry.Name(), "lima.yaml")); err != nil {
				continue
			}
			if name != "" {
				return "work"
			}
			name = entry.Name()
		}
		if name != "" {
			return name
		}
		return "work"
	}
	if saved, err := LoadEnvironmentRecord(profile); err == nil && saved.IdentityDirectory != "" &&
		paths.Equal(paths.Host(filepath.Clean(workspace)), paths.Host(filepath.Clean(saved.Directory))) {
		workspace = saved.IdentityDirectory
	}
	home := filepath.Join(profile, "state", "lima")
	_, err := os.Stat(filepath.Join(profile, locationFile))
	logical := err == nil
	if logical {
		if real, e := filepath.EvalSymlinks(profile); e == nil {
			home = filepath.Join(real, "state", "lima")
		}
	}
	dir := paths.Host(filepath.Clean(workspace))
	limit := vmname.HomeLimit(home, logical)
	// A hashed project name needs at least one prefix byte plus nine suffix
	// bytes. Eden owns only one environment per profile, so a fixed short
	// name is unambiguous when that minimum does not fit. Never truncate the
	// hash or pretend the socket limit can hold a ten-byte name.
	if limit < 10 {
		return "work"
	}
	expected := limit.Name(dir)
	if _, err := os.Stat(filepath.Join(home, expected)); err == nil {
		return expected
	}
	// A profile restored from a different path can contain an instance made
	// under a previous socket limit. Only accept a unique, valid truncation
	// of this exact workspace's name, confirmed by a persistent Lima config.
	entries, _ := os.ReadDir(home)
	var old string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !strings.HasSuffix(name, expected[len(expected)-9:]) || vmname.Limit(len(name)).Name(dir) != name {
			continue
		}
		if _, err := os.Stat(filepath.Join(home, name, "lima.yaml")); err != nil {
			continue
		}
		if old != "" {
			return expected
		}
		old = name
	}
	if old != "" {
		return old
	}
	return expected
}
