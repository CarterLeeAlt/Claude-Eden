package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

func WorkbenchEntry(raw string) (*url.URL, error) {
	entry, err := url.Parse(raw)
	if err != nil || !IsWorkbench(raw, portOf(entry)) || entry.Fragment == "" {
		return nil, errors.New("后台返回了无效的工作台地址")
	}
	return entry, nil
}

func portOf(entry *url.URL) int {
	if entry == nil {
		return 0
	}
	port, _ := strconv.Atoi(entry.Port())
	return port
}

func IsWorkbench(raw string, port int) bool {
	entry, err := url.Parse(raw)
	return err == nil && port > 0 && port <= 65535 && entry.Scheme == "http" && entry.Hostname() == "localhost" && entry.User == nil && portOf(entry) == port
}

func instanceKey(sid, root string) string {
	key := sha256.Sum256([]byte(sid + "\x00" + strings.ToUpper(strings.TrimRight(filepath.Clean(root), `\/`))))
	return strings.ToUpper(hex.EncodeToString(key[:12]))
}
