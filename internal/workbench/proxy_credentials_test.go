package workbench

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/sylvinus/agent-vm/internal/eden"
	"github.com/sylvinus/agent-vm/internal/managedproxy"
	"golang.org/x/crypto/ssh"
)

func TestManualNodeCredentialEditsThroughAPI(t *testing.T) {
	s := testServer(t)
	validate := validateProxyCore
	t.Cleanup(func() { validateProxyCore = validate })
	validateProxyCore = func(_ context.Context, profile string, p managedproxy.Store) error {
		_, err := p.Runtime(profile)
		return err
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKeyWithPassphrase(key, "fixture", []byte("key-passphrase"))
	if err != nil {
		t.Fatal(err)
	}
	privateKey := string(pem.EncodeToMemory(block))
	call := func(action string, body any, wantError bool) {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		if w := request(s, "POST", "/api/proxy/"+action, s.origin, string(data), true); w.Code != 200 {
			t.Fatal(w.Code)
		}
		job := awaitJobIdle(t, &s.jobs)
		if (job.Error != "") != wantError {
			t.Fatalf("%s: unexpected error status %t", action, job.Error != "")
		}
	}
	node := map[string]any{"name": "credential fixture", "type": "http", "server": "127.0.0.1", "port": 12345, "username": "fixture", "password": "saved-password"}
	call("add", map[string]any{"node": node}, false)
	store, err := managedproxy.Load(s.profile)
	if err != nil {
		t.Fatal(err)
	}
	id := store.Nodes[len(store.Nodes)-1].ID
	read := func() managedproxy.Node {
		t.Helper()
		p, err := managedproxy.Load(s.profile)
		if err != nil {
			t.Fatal(err)
		}
		n, ok := p.Find(id)
		if !ok {
			t.Fatal("node disappeared")
		}
		return n
	}
	edit := func(wantError bool) { t.Helper(); call("edit", map[string]any{"node_id": id, "node": node}, wantError) }
	delete(node, "password")
	node["name"] = "renamed"
	edit(false)
	if read().Password != "saved-password" {
		t.Fatal("metadata edit lost password")
	}
	node["password"] = ""
	node["username"] = ""
	edit(false)
	if read().Password != "" || read().Username != "" {
		t.Fatal("explicit authentication removal failed")
	}
	node["username"] = "fixture"
	node["password"] = "http-only-password"
	edit(false)
	node["type"] = "ssh"
	node["auth_method"] = "password"
	delete(node, "password")
	edit(true)
	if read().Type != "http" {
		t.Fatal("failed protocol change replaced saved node")
	}
	node["password"] = "ssh-only-password"
	edit(false)
	delete(node, "password")
	edit(false)
	if read().Password != "ssh-only-password" {
		t.Fatal("SSH password omission did not retain secret")
	}
	node["auth_method"] = "private-key"
	edit(true)
	node["private_key"] = privateKey
	node["private_key_passphrase"] = "key-passphrase"
	edit(false)
	if read().Password != "" {
		t.Fatal("private-key authentication retained password")
	}
	delete(node, "private_key")
	delete(node, "private_key_passphrase")
	edit(false)
	if read().PrivateKey != privateKey || read().PrivateKeyPassphrase != "key-passphrase" {
		t.Fatal("omitted key credentials were lost")
	}
	w := request(s, "GET", "/api/proxy", s.origin, "", true)
	for _, secret := range []string{privateKey, "key-passphrase", "ssh-only-password", "http-only-password"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("status exposed a secret")
		}
	}
	node["private_key"] = ""
	edit(true)
	if read().PrivateKey != privateKey {
		t.Fatal("invalid clearing of a required key overwrote saved credentials")
	}
	delete(node, "private_key")
	node["type"] = "socks5"
	delete(node, "auth_method")
	node["password"] = "new-socks-password"
	edit(false)
	if n := read(); n.PrivateKey != "" || n.PrivateKeyPassphrase != "" || n.Password != "new-socks-password" {
		t.Fatal("protocol change retained hidden SSH secrets")
	}
}

func TestDeletingSelectedNodeClosesGateWithoutSelectingReplacement(t *testing.T) {
	s, gate := interfaceMonitorServer(t)
	grantTestGate(t, gate)
	p, err := managedproxy.Load(s.profile)
	if err != nil {
		t.Fatal(err)
	}
	replacement := p.Nodes[0]
	replacement.ID = managedproxy.Token()[:24]
	replacement.Name = "replacement"
	p.Nodes = append(p.Nodes, replacement)
	if err := p.Save(s.profile); err != nil {
		t.Fatal(err)
	}
	if w := request(s, "POST", "/api/proxy/delete", s.origin, `{"node_id":"`+p.Selected+`"}`, true); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if job := awaitJobIdle(t, &s.jobs); job.Error != "" {
		t.Fatal(job.Error)
	}
	if gate.Status().Mode != eden.GateClosed {
		t.Fatal("deleting selected node left authorization open")
	}
	after, err := managedproxy.Load(s.profile)
	if err != nil || after.Selected != "" || len(after.Nodes) != 1 || after.Nodes[0].ID != replacement.ID {
		t.Fatal("deletion applied an unapproved replacement", err)
	}
}
