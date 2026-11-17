// Package managedproxy owns Claude Eden's manual proxy nodes and generated core configuration.
package managedproxy

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sylvinus/agent-vm/internal/exitpolicy"
	"github.com/sylvinus/agent-vm/internal/host"
	"go.yaml.in/yaml/v3"
)

const Group = "EDEN"
const maxStoreSize = 32 << 20

type Store struct {
	Version      int                          `json:"version"`
	Port         int                          `json:"port"`
	Auth         string                       `json:"auth"`
	Revision     string                       `json:"revision"`
	Selected     string                       `json:"selected"`
	Nodes        []Node                       `json:"nodes"`
	ExitPolicies map[string]exitpolicy.Policy `json:"exit_policies,omitempty"`
}

func Token() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func Dir(profile string) string { return filepath.Join(profile, "proxy") }
func Socket(profile string) string {
	h := sha256.Sum256([]byte(filepath.Clean(profile)))
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), fmt.Sprintf("eden-%x", h[:8]), "api.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("eden-proxy-%d-%x", os.Getuid(), h[:8]), "control.sock")
}
func Load(profile string) (Store, error) {
	var s Store
	f, err := os.Open(filepath.Join(Dir(profile), "nodes.json"))
	if os.IsNotExist(err) {
		return Store{Version: 1, Port: 17896, Auth: Token(), Nodes: []Node{}}, nil
	}
	if err != nil {
		return s, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxStoreSize+1))
	if err != nil {
		return s, err
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	var extra any
	if len(b) > maxStoreSize || decoder.Decode(&s) != nil || decoder.Decode(&extra) != io.EOF {
		return s, errors.New("Claude Eden 代理配置无效，请恢复私有配置备份")
	}
	return s, s.Validate()
}

func (s Store) Validate() error {
	if s.Version != 1 || s.Port < 1024 || s.Port > 65535 || len(s.Auth) != 48 {
		return errors.New("Claude Eden 代理配置无效，请恢复私有配置备份")
	}
	seen := make(map[string]bool, len(s.Nodes))
	for _, node := range s.Nodes {
		if err := node.Validate(); err != nil {
			return err
		}
		if seen[node.ID] {
			return errors.New("代理节点 ID 重复，请恢复私有配置备份")
		}
		seen[node.ID] = true
	}
	if s.Selected != "" && !seen[s.Selected] {
		return errors.New("所选代理节点不存在，外网保持关闭")
	}
	return nil
}
func Write(path string, data []byte) error {
	if err := host.EnsurePrivateDir(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".proxy-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = host.ProtectFile(f.Name()); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (s Store) Save(profile string) error {
	if err := s.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if len(b) > maxStoreSize {
		return errors.New("代理节点配置总大小超过 32 MiB，请减少节点；原配置已保留")
	}
	return Write(filepath.Join(Dir(profile), "nodes.json"), b)
}
func (s Store) Endpoint() string { return fmt.Sprintf("socks5://eden:%s@127.0.0.1:%d", s.Auth, s.Port) }
func (s Store) Find(id string) (Node, bool) {
	for _, n := range s.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}
func (s *Store) SetNode(node Node) error {
	if err := node.Validate(); err != nil {
		return err
	}
	for i := range s.Nodes {
		if s.Nodes[i].ID == node.ID {
			s.Nodes[i] = node
			return nil
		}
	}
	s.Nodes = append(s.Nodes, node)
	return nil
}

// Numeric DoH endpoints bootstrap proxy server names without host DNS fallback.
func BootstrapDNSURLs() []string {
	return []string{"https://1.1.1.1/dns-query", "https://223.5.5.5/dns-query"}
}

func (s Store) Runtime(profile string) ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	// Windows AF_UNIX has 108 bytes including its trailing NUL. Reject long
	// temp paths before writing a config or starting a core that cannot bind.
	if runtime.GOOS == "windows" && len(Socket(profile)) >= 108 {
		return nil, errors.New("代理控制 socket 路径超过 Windows 的 107 字节上限，请使用更短且可写的临时目录；外网保持关闭")
	}
	if _, ok := s.Find(s.Selected); !ok {
		return nil, errors.New("请先选择有效的代理节点")
	}
	proxies := []map[string]any{}
	for _, node := range s.Nodes {
		proxies = append(proxies, node.coreConfig())
	}
	// Infrastructure DNS bootstraps only the core's server domains. Guest DNS
	// remains behind Eden Egress, and there is no host-system resolver fallback.
	dns := map[string]any{"enable": true, "ipv6": false, "use-hosts": false, "use-system-hosts": false, "default-nameserver": BootstrapDNSURLs(), "proxy-server-nameserver": BootstrapDNSURLs(), "nameserver": []string{"https://1.1.1.1/dns-query#" + Group}}
	config := map[string]any{"mixed-port": s.Port, "bind-address": "127.0.0.1", "allow-lan": false, "authentication": []string{"eden:" + s.Auth}, "skip-auth-prefixes": []string{}, "mode": "rule", "ipv6": false, "log-level": "warning", "find-process-mode": "off", "external-controller-unix": Socket(profile), "tun": map[string]any{"enable": false}, "dns": dns, "geo-auto-update": false, "profile": map[string]any{"store-selected": false}, "proxies": proxies, "proxy-groups": []any{map[string]any{"name": Group, "type": "select", "proxies": []string{s.Selected}}}, "rules": []string{"MATCH," + Group}}
	// Measure a request on the established connection rather than folding
	// protocol and TLS setup into the node's displayed HTTP latency.
	config["unified-delay"] = true
	return yaml.Marshal(config)
}
