//go:build !windows && !linux && !darwin

package eden

import "errors"

func localStorageVolume(string) error { return errors.New("此平台不支持自定义环境存储") }
