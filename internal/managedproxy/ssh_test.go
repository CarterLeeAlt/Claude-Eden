package managedproxy

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func testSSHKeyPair(t *testing.T) (string, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	wire := func(b []byte) []byte { return append(binary.BigEndian.AppendUint32(nil, uint32(len(b))), b...) }
	key := "ssh-ed25519 " + base64.StdEncoding.EncodeToString(append(wire([]byte("ssh-ed25519")), wire(public)...))
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), key
}

func TestSSHManualKeyAndPasswordNodes(t *testing.T) {
	key, hostKey := testSSHKeyPair(t)
	for _, auth := range []string{"password", "private-key"} {
		t.Run(auth, func(t *testing.T) {
			profile := t.TempDir()
			s, _ := Load(profile)
			n := Node{ID: Token()[:24], Name: "SSH", Type: "ssh", Server: "127.0.0.1", Port: 22, Username: "test", AuthMethod: auth, HostKey: []string{hostKey}}
			if auth == "password" {
				n.Password = "synthetic-password"
			} else {
				n.PrivateKey = key
			}
			if err := s.SetNode(n); err != nil {
				t.Fatal(err)
			}
			s.Selected = n.ID
			if err := s.Save(profile); err != nil {
				t.Fatal(err)
			}
			loaded, err := Load(profile)
			if err != nil {
				t.Fatal(err)
			}
			wire, err := loaded.Runtime(profile)
			if err != nil {
				t.Fatal(err)
			}
			var cfg struct{ Proxies []map[string]any }
			if err := yaml.Unmarshal(wire, &cfg); err != nil {
				t.Fatal(err)
			}
			p := cfg.Proxies[0]
			if p["type"] != "ssh" || p["udp"] != false || p["dialer-proxy"] != nil || p["username"] != "test" {
				t.Fatal("SSH transport boundary changed")
			}
			if auth == "password" && p["password"] != n.Password || auth == "private-key" && p["private-key"] != key {
				t.Fatal("SSH authentication changed")
			}
			if os.Getenv("EDEN_PROXY_INTEGRATION") == "1" {
				if err := Validate(context.Background(), profile, loaded); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestSSHRejectsFileKeysAndMixedAuthentication(t *testing.T) {
	n := Node{ID: Token()[:24], Name: "SSH", Type: "ssh", Server: "127.0.0.1", Port: 22, Username: "test", AuthMethod: "private-key"}
	for _, key := range []string{"", "~/.ssh/id_rsa", "C:\\private\\id_rsa", "/etc/private-key", "C:\\PRIVATE KEY\\id_rsa", "-----BEGIN PRIVATE KEY-----\ninvalid\n-----END PRIVATE KEY-----"} {
		n.PrivateKey = key
		if err := n.Validate(); err == nil || strings.Contains(err.Error(), key) && key != "" {
			t.Fatal("file key accepted or private input echoed")
		}
	}
	n.PrivateKey, _ = testSSHKeyPair(t)
	n.Password = "secret"
	if err := n.Validate(); err == nil {
		t.Fatal("both authentication methods accepted")
	}
	n.Password = ""
	n.Username = ""
	if err := n.Validate(); err == nil {
		t.Fatal("SSH accepted without username")
	}
	n.Username = "test"
	n.Type = "http"
	if err := n.Validate(); err == nil {
		t.Fatal("SSH credentials accepted by another protocol")
	}
}
