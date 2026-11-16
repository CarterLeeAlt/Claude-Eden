// Package desktopupdate implements the desktop update transaction independently
// of the windowing framework. Only typed public state crosses the UI boundary.
package desktopupdate

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sylvinus/agent-vm/internal/installfiles"
)

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

func numeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func parts(s string) []string {
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	for _, p := range strings.Split(m[4], ".") {
		if len(p) > 1 && p[0] == '0' && numeric(p) {
			return nil
		}
	}
	return m[1:]
}
func compareNumber(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}

// IsNewer accepts exactly the backend's canonical three-part versions, without
// build metadata or a v prefix, and compares arbitrarily large numeric parts.
func IsNewer(candidate, current string) bool {
	a, b := parts(candidate), parts(current)
	if a == nil || b == nil {
		return false
	}
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return compareNumber(a[i], b[i]) > 0
		}
	}
	if a[3] == "" || b[3] == "" {
		return a[3] == "" && b[3] != ""
	}
	x, y := strings.Split(a[3], "."), strings.Split(b[3], ".")
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] == y[i] {
			continue
		}
		xn, yn := numeric(x[i]), numeric(y[i])
		if xn != yn {
			return !xn
		}
		if xn {
			return compareNumber(x[i], y[i]) > 0
		}
		return x[i] > y[i]
	}
	return len(x) > len(y)
}

func DownloadPath(raw, root string) (string, error) {
	if raw == "" || !filepath.IsAbs(raw) {
		return "", errors.New("更新安装包路径必须是绝对路径")
	}
	p, e := filepath.Abs(raw)
	if e != nil {
		return "", e
	}
	r, e := filepath.Abs(root)
	if e != nil {
		return "", e
	}
	dir := filepath.Dir(p)
	if !strings.EqualFold(filepath.Dir(dir), filepath.Clean(r)) || !strings.HasPrefix(filepath.Base(dir), "download-") || filepath.Base(p) != "Eden-setup.exe" {
		return "", errors.New("更新安装包不属于 Claude Eden 的私有下载目录")
	}
	if e = installfiles.PlainPath(p); e != nil {
		return "", e
	}
	return p, nil
}
