//go:build !windows

package installfiles

import (
	"errors"
	"os"
)

func isReparse(name string) (bool, error) {
	s, e := os.Lstat(name)
	if e != nil {
		return false, e
	}
	return s.Mode()&os.ModeSymlink != 0, nil
}
func OpenReadLocked(name string) (*os.File, error) {
	return nil, errors.New("安装事务仅支持 Windows")
}
func OpenVerified(root, relative string) (*os.File, error) {
	return nil, errors.New("校验句柄删除仅支持 Windows")
}
func DeleteVerified(f *os.File) error { return errors.New("校验句柄删除仅支持 Windows") }
func renameNoReplace(from, to string) error {
	if exists(to) {
		return os.ErrExist
	}
	return os.Rename(from, to)
}
func replaceFile(from, to string) error { return os.Rename(from, to) }
