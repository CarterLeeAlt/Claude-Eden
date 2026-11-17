package eden

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// PortableDataDirectory derives the data root from the profile's location, not
// a saved machine/user/drive path. It only recognizes the packaged layout.
func PortableDataDirectory(profile string) string {
	profile = filepath.Clean(profile)
	data := filepath.Dir(profile)
	if filepath.Base(profile) != "profile" || filepath.Base(data) != "data" {
		return ""
	}
	app := filepath.Dir(data)
	for _, name := range []string{"portable.flag", "ClaudeEden.exe"} {
		info, err := os.Lstat(filepath.Join(app, name))
		if err != nil || !info.Mode().IsRegular() {
			return ""
		}
	}
	return data
}

const portablePathPrefix = "@data/"

// StorePortablePath makes only paths inside this package's data tree movable.
// External user projects keep their exact grant; moving the program must never
// silently grant a similarly named directory on another machine.
func StorePortablePath(profile, value string) (string, error) {
	data := PortableDataDirectory(profile)
	if data == "" || value == "" || !StorageContains(data, value) {
		return value, nil
	}
	relative, err := filepath.Rel(data, value)
	if err != nil {
		return "", err
	}
	if _, err = portableRelativePath(data, relative); err != nil {
		return "", err
	}
	return portablePathPrefix + filepath.ToSlash(relative), nil
}

func ResolvePortablePath(profile, value string) (string, error) {
	if !strings.HasPrefix(value, portablePathPrefix) {
		return value, nil
	}
	data := PortableDataDirectory(profile)
	if data == "" {
		return "", errors.New("相对便携数据路径缺少有效的程序目录")
	}
	return portableRelativePath(data, strings.TrimPrefix(value, portablePathPrefix))
}

// portableRelativePath never turns a link or a traversal into permission to
// access a host directory outside the portable data tree.
func portableRelativePath(data, value string) (string, error) {
	if !filepath.IsLocal(value) {
		return "", errors.New("便携数据路径必须是 data 内的相对路径")
	}
	target := filepath.Join(data, value)
	physical, err := StoragePhysicalPath(target)
	if err != nil {
		return "", err
	}
	root, err := StoragePhysicalPath(data)
	if err != nil || !sameStoragePath(root, data) || !StorageContains(root, physical) {
		return "", errors.New("便携数据路径不能通过目录链接访问 data 之外")
	}
	return target, nil
}

// EnvironmentRecordForStorage removes machine-specific paths from portable
// records. Callers continue to use resolved absolute paths in memory.
func EnvironmentRecordForStorage(profile string, record EnvironmentRecord) (EnvironmentRecord, error) {
	data := PortableDataDirectory(profile)
	if data == "" {
		return record, nil
	}
	relative, err := filepath.Rel(data, record.Directory)
	if err != nil {
		return EnvironmentRecord{}, err
	}
	if _, err = portableRelativePath(data, relative); err != nil {
		return EnvironmentRecord{}, err
	}
	record.Directory = filepath.ToSlash(relative)
	record.IdentityDirectory = ""
	record.Portable = true
	return record, nil
}
