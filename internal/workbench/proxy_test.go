package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sylvinus/agent-vm/internal/managedproxy"
)

func TestFailedNodeEditKeepsWorkingNodeAndSecretsPrivate(t *testing.T) {
	s := testServer(t)
	p, _ := managedproxy.Load(s.profile)
	p.Nodes[0].Username, p.Nodes[0].Password = "qa", "private-node-password"
	if err := p.Save(s.profile); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(s.profile, "proxy", "nodes.json")
	before, _ := os.ReadFile(file)
	validate := validateProxyCore
	t.Cleanup(func() { validateProxyCore = validate })
	validateProxyCore = func(context.Context, string, managedproxy.Store) error { return errors.New("fixture rejected node") }
	body := `{"node_id":"` + p.Selected + `","node":{"name":"new","type":"socks5","server":"127.0.0.1","port":1080,"username":"qa","password":"new-private-password"}}`
	if w := request(s, "POST", "/api/proxy/edit", s.origin, body, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if job := awaitJobIdle(t, &s.jobs); job.Error == "" {
		t.Fatal("failed validation was not reported")
	}
	if after, _ := os.ReadFile(file); string(after) != string(before) {
		t.Fatal("failed edit replaced working data")
	}
	w := request(s, "GET", "/api/proxy", s.origin, "", true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, secret := range []string{"private-node-password", "new-private-password", p.Auth} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("status leaked proxy credentials")
		}
	}
	if w = request(s, "POST", "/api/proxy/stop", s.origin, `{}`, false); w.Code != 401 {
		t.Fatal("proxy mutation accepted without authentication")
	}
}

func TestManualNodeAPIAndRemovedEntries(t *testing.T) {
	s := testServer(t)
	validate, start := validateProxyCore, startProxyCore
	t.Cleanup(func() { validateProxyCore, startProxyCore = validate, start })
	validateProxyCore = func(_ context.Context, profile string, p managedproxy.Store) error {
		_, err := p.Runtime(profile)
		return err
	}
	startProxyCore = func(context.Context, string, managedproxy.Store) error { return nil }
	for _, action := range []string{"import", "refresh", "route"} {
		if w := request(s, "POST", "/api/proxy/"+action, s.origin, `{}`, true); w.Code != 404 {
			t.Fatal("removed proxy entry accepted", action, w.Code)
		}
	}
	for _, body := range []string{
		`{"node":{"name":"bad","type":"http","server":"example.com","port":80,"dialer-proxy":"DIRECT"}}`,
		`{"node":{"name":"bad","type":"http","server":"example.com","port":80},"content":"proxies: []"}`,
		`{"node":{"name":"bad","type":"http","server":"example.com","port":80},"node_ids":["one","two"]}`,
	} {
		if w := request(s, "POST", "/api/proxy/add", s.origin, body, true); w.Code != 400 {
			t.Fatal("extra core configuration accepted", w.Code)
		}
	}
	call := func(action, body string) {
		t.Helper()
		if w := request(s, "POST", "/api/proxy/"+action, s.origin, body, true); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		if job := awaitJobIdle(t, &s.jobs); job.Error != "" {
			t.Fatal(job.Error)
		}
	}
	before, _ := managedproxy.Load(s.profile)
	call("add", `{"node":{"name":"manual","type":"http","server":"127.0.0.1","port":1080,"username":"qa","password":"secret-http-password"}}`)
	p, _ := managedproxy.Load(s.profile)
	if len(p.Nodes) != 2 || p.Selected != before.Selected || p.Revision != before.Revision {
		t.Fatal("saving a node silently changed the active proxy")
	}
	id := p.Nodes[1].ID
	call("edit", `{"node_id":"`+id+`","node":{"name":"renamed","type":"http","server":"127.0.0.1","port":1081,"username":"qa"}}`)
	p, _ = managedproxy.Load(s.profile)
	if p.Nodes[1].Password != "secret-http-password" || p.Nodes[1].Name != "renamed" {
		t.Fatal("edit lost omitted credentials")
	}
	w := request(s, "GET", "/api/proxy", s.origin, "", true)
	var view struct{ Nodes []map[string]any }
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil || len(view.Nodes) != 2 || view.Nodes[1]["has_password"] != true || strings.Contains(w.Body.String(), "secret-http-password") {
		t.Fatal("node editor metadata leaked or lost credentials", err)
	}
	call("select", `{"node_id":"`+id+`"}`)
	p, _ = managedproxy.Load(s.profile)
	if p.Selected != id || p.Revision == before.Revision {
		t.Fatal("explicit selection was not applied")
	}
	call("delete", `{"node_id":"`+id+`"}`)
	p, _ = managedproxy.Load(s.profile)
	if p.Selected != "" || len(p.Nodes) != 1 {
		t.Fatal("deleting active node selected an unapproved replacement")
	}
	cfg, _ := loadConfig(s.configPath)
	if cfg.RequireManagedProxy(s.profile) == nil {
		t.Fatal("deleting active node retained network authorization")
	}
}

func TestSettingsCannotWriteProxyEndpoint(t *testing.T) {
	s := testServer(t)
	for _, body := range []string{`{"proxy_url":"socks5://127.0.0.1:7890"}`, `{"proxy_username":"user"}`, `{"proxy_password":"secret"}`} {
		if w := request(s, "POST", "/api/settings", s.origin, body, true); w.Code != 400 {
			t.Fatal("removed proxy settings accepted", w.Code)
		}
	}
	if w := request(s, "POST", "/api/proxy/migrate", s.origin, `{}`, true); w.Code != 404 {
		t.Fatal("removed migration entry accepted", w.Code)
	}
}
