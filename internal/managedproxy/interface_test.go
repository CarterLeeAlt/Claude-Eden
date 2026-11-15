package managedproxy

import (
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestBindingPreservesProxyChainAndReplacesStaleNodeBinding(t *testing.T) {
	data := []byte("interface-name: en0\nrules: ['MATCH,exit']\nproxies:\n - {name: entry, type: socks5, server: 203.0.113.1, port: 1080, password: secret, interface-name: en0}\n - {name: exit, type: http, server: 203.0.113.2, port: 443, dialer-proxy: entry}\n")
	bound, err := BindInterface(data, "en7")
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(bound, &config); err != nil {
		t.Fatal(err)
	}
	nodes := config["proxies"].([]any)
	if config["interface-name"] != "en7" || nodes[0].(map[string]any)["interface-name"] != "en7" || nodes[0].(map[string]any)["password"] != "secret" || nodes[1].(map[string]any)["dialer-proxy"] != "entry" || config["rules"].([]any)[0] != "MATCH,exit" {
		t.Fatal("binding changed the proxy chain or kept a stale interface")
	}
	if _, err := BindInterface(data, ""); err == nil {
		t.Fatal("unbound fallback accepted")
	}
}
