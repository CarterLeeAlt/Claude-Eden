package managedproxy

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sylvinus/agent-vm/internal/host"
	"go.yaml.in/yaml/v3"
)

func TestManualNodesPersistAndGenerateOnlySelectedProxyRoute(t *testing.T) {
	profile := t.TempDir()
	s, err := Load(profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, protocol := range []string{"http", "socks5", "ssh"} {
		n := Node{ID: Token()[:24], Name: "节点 " + protocol, Type: protocol, Server: "127.0.0.1", Port: 1080, Username: "test", Password: "private-node-password"}
		if protocol == "ssh" {
			n.AuthMethod = "password"
			n.Port = 22
		}
		if err := s.SetNode(n); err != nil {
			t.Fatal(err)
		}
	}
	s.Selected, s.Revision = s.Nodes[1].ID, Token()
	if err := s.Save(profile); err != nil {
		t.Fatal(err)
	}
	if err := host.CheckPrivateFile(filepath.Join(Dir(profile), "nodes.json")); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(profile)
	if err != nil || !reflect.DeepEqual(loaded, s) {
		t.Fatal("manual nodes changed after reload", err)
	}
	for _, selected := range loaded.Nodes {
		loaded.Selected = selected.ID
		wire, err := loaded.Runtime(profile)
		if err != nil {
			t.Fatal(err)
		}
		var cfg struct {
			Port           int                   `yaml:"mixed-port"`
			Bind           string                `yaml:"bind-address"`
			LAN            bool                  `yaml:"allow-lan"`
			IPv6           bool                  `yaml:"ipv6"`
			Authentication []string              `yaml:"authentication"`
			SkipAuth       []string              `yaml:"skip-auth-prefixes"`
			Tun            struct{ Enable bool } `yaml:"tun"`
			Proxies        []map[string]any      `yaml:"proxies"`
			Groups         []struct {
				Type    string
				Proxies []string
			} `yaml:"proxy-groups"`
			Rules []string `yaml:"rules"`
			DNS   struct {
				Default []string `yaml:"default-nameserver"`
				Proxy   []string `yaml:"proxy-server-nameserver"`
				Names   []string `yaml:"nameserver"`
			} `yaml:"dns"`
		}
		if err := yaml.Unmarshal(wire, &cfg); err != nil {
			t.Fatal(err)
		}
		if cfg.Port != loaded.Port || cfg.Bind != "127.0.0.1" || cfg.LAN || cfg.IPv6 || cfg.Tun.Enable || len(cfg.SkipAuth) != 0 || !reflect.DeepEqual(cfg.Authentication, []string{"eden:" + loaded.Auth}) {
			t.Fatal("generated listener isolation changed")
		}
		if len(cfg.Groups) != 1 || cfg.Groups[0].Type != "select" || !reflect.DeepEqual(cfg.Groups[0].Proxies, []string{selected.ID}) || !reflect.DeepEqual(cfg.Rules, []string{"MATCH," + Group}) {
			t.Fatal("traffic can choose an unselected route")
		}
		if !reflect.DeepEqual(cfg.DNS.Default, BootstrapDNSURLs()) || !reflect.DeepEqual(cfg.DNS.Proxy, BootstrapDNSURLs()) || !reflect.DeepEqual(cfg.DNS.Names, []string{"https://1.1.1.1/dns-query#" + Group}) {
			t.Fatal("DNS escaped its defined proxy/bootstrap paths")
		}
		if len(cfg.Proxies) != 3 {
			t.Fatal("node list was not loaded for explicit switching and delay probes")
		}
		for _, node := range cfg.Proxies {
			if node["udp"] != false || node["dialer-proxy"] != nil || node["type"] == "direct" {
				t.Fatal("node opened extra outbound paths")
			}
		}
		if strings.Contains(string(wire), "DIRECT") || strings.Contains(string(wire), "fallback") || strings.Contains(string(wire), "proxy-providers") {
			t.Fatal("unowned core route generated")
		}
	}
	loaded.Selected = ""
	if _, err := loaded.Runtime(profile); err == nil {
		t.Fatal("missing selected node generated a usable route")
	}
}

func TestNodeStoreRejectsUnknownFieldsAndInvalidProtocols(t *testing.T) {
	profile := t.TempDir()
	s, _ := Load(profile)
	n := Node{ID: Token()[:24], Name: "test", Type: "http", Server: "example.com", Port: 8080}
	if err := s.SetNode(n); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(profile); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(Dir(profile), "nodes.json")
	original, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(map[string]any){
		func(v map[string]any) { v["sources"] = []any{} },
		func(v map[string]any) { v["chain"] = []string{n.ID} },
		func(v map[string]any) { v["nodes"].([]any)[0].(map[string]any)["dialer-proxy"] = "DIRECT" },
		func(v map[string]any) {
			v["nodes"].([]any)[0].(map[string]any)["config"] = map[string]any{"type": "direct"}
		},
		func(v map[string]any) { v["nodes"].([]any)[0].(map[string]any)["type"] = "vless" },
		func(v map[string]any) { v["selected"] = "missing-node" },
	} {
		var value map[string]any
		if err := json.Unmarshal(original, &value); err != nil {
			t.Fatal(err)
		}
		change(value)
		wire, _ := json.Marshal(value)
		if err := Write(file, wire); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(profile); err == nil {
			t.Fatal("invalid/extra node settings were accepted")
		}
		if after, err := os.ReadFile(file); err != nil || !bytes.Equal(after, wire) {
			t.Fatal("rejected input was rewritten", err)
		}
	}
	if err := Write(file, append(original, []byte(" {}")...)); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(profile); err == nil {
		t.Fatal("multiple JSON values accepted")
	}
}

func TestInvalidNodeUpdateKeepsSavedData(t *testing.T) {
	profile := t.TempDir()
	s, _ := Load(profile)
	n := Node{ID: Token()[:24], Name: "fixed", Type: "socks5", Server: "127.0.0.1", Port: 1080}
	if err := s.SetNode(n); err != nil {
		t.Fatal(err)
	}
	s.Selected = n.ID
	if err := s.Save(profile); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(Dir(profile), "nodes.json"))
	for _, protocol := range []string{"direct", "vmess", "vless", "trojan", "ss", "ssr", "hysteria", "hysteria2", "tuic", "anytls", "mieru", "HTTP"} {
		bad := n
		bad.Type = protocol
		if err := s.SetNode(bad); err == nil {
			t.Fatalf("unsupported type %q accepted", protocol)
		}
	}
	for _, server := range []string{"", "http://proxy.example", "user@proxy.example", "proxy.example:1080", "proxy.example/path", "proxy example", "proxy\x00example"} {
		bad := n
		bad.Server = server
		if err := s.SetNode(bad); err == nil {
			t.Fatal("malformed server accepted")
		}
	}
	for _, port := range []int{0, -1, 65536} {
		bad := n
		bad.Port = port
		if err := s.SetNode(bad); err == nil {
			t.Fatal("invalid server port accepted")
		}
	}
	if after, _ := os.ReadFile(filepath.Join(Dir(profile), "nodes.json")); !bytes.Equal(before, after) {
		t.Fatal("failed edit overwrote node store")
	}
}
