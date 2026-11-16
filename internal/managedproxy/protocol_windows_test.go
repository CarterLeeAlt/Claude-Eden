package managedproxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// These controlled upstreams only forward to the test's loopback target. A
// failed request reaching that target would expose a direct fallback by the core.
type protocolFixture struct {
	listener      net.Listener
	target        string
	node          Node
	ssh           *ssh.ServerConfig
	reject, stall atomic.Bool
	forwarded     atomic.Int32
	mu            sync.Mutex
	sockets       map[net.Conn]bool
	workers       sync.WaitGroup
	closed        chan struct{}
	once          sync.Once
}

func newProtocolFixture(t *testing.T, kind, auth, target string) *protocolFixture {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &protocolFixture{listener: l, target: target, sockets: make(map[net.Conn]bool), closed: make(chan struct{})}
	f.node = Node{ID: Token()[:24], Name: kind + " " + auth, Type: kind, Server: "127.0.0.1", Port: l.Addr().(*net.TCPAddr).Port}
	if auth != "none" {
		f.node.Username, f.node.Password = "fixture", "fixture-password"
	}
	if kind == "ssh" {
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		signer, err := ssh.NewSignerFromKey(key)
		if err != nil {
			t.Fatal(err)
		}
		f.ssh = &ssh.ServerConfig{}
		f.ssh.AddHostKey(signer)
		f.node.HostKey = []string{strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))}
		f.node.AuthMethod = "password"
		f.ssh.PasswordCallback = func(c ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if auth == "password" && !f.reject.Load() && c.User() == "fixture" && string(password) == "fixture-password" {
				return nil, nil
			}
			return nil, errors.New("fixture rejects authentication")
		}
		f.ssh.PublicKeyCallback = func(c ssh.ConnMetadata, public ssh.PublicKey) (*ssh.Permissions, error) {
			if auth != "password" && !f.reject.Load() && c.User() == "fixture" && bytes.Equal(public.Marshal(), signer.PublicKey().Marshal()) {
				return nil, nil
			}
			return nil, errors.New("fixture rejects key")
		}
		if auth != "password" {
			f.node.AuthMethod, f.node.Password = "private-key", ""
			var block *pem.Block
			if auth == "encrypted-key" {
				f.node.PrivateKeyPassphrase = "fixture-key-passphrase"
				block, err = ssh.MarshalPrivateKeyWithPassphrase(key, "fixture", []byte(f.node.PrivateKeyPassphrase))
			} else {
				block, err = ssh.MarshalPrivateKey(key, "fixture")
			}
			if err != nil {
				t.Fatal(err)
			}
			f.node.PrivateKey = string(pem.EncodeToMemory(block))
		}
	}
	f.workers.Add(1)
	go func() {
		defer f.workers.Done()
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			f.mu.Lock()
			f.sockets[conn] = true
			f.mu.Unlock()
			f.workers.Add(1)
			go func() {
				defer f.workers.Done()
				defer func() { conn.Close(); f.mu.Lock(); delete(f.sockets, conn); f.mu.Unlock() }()
				if f.stall.Load() {
					<-f.closed
					return
				}
				_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
				if kind == "ssh" {
					f.serveSSH(conn)
				} else {
					f.serveProxy(conn)
				}
			}()
		}
	}()
	t.Cleanup(f.close)
	return f
}

func (f *protocolFixture) close() {
	f.once.Do(func() {
		close(f.closed)
		f.listener.Close()
		f.mu.Lock()
		for c := range f.sockets {
			c.Close()
		}
		f.mu.Unlock()
		f.workers.Wait()
	})
}

func (f *protocolFixture) tunnel(client io.ReadWriteCloser) {
	remote, err := net.DialTimeout("tcp4", f.target, time.Second)
	if err != nil {
		return
	}
	defer remote.Close()
	f.forwarded.Add(1)
	done := make(chan struct{})
	go func() { io.Copy(remote, client); remote.Close(); close(done) }()
	io.Copy(client, remote)
	client.Close()
	<-done
}

func (f *protocolFixture) serveSSH(raw net.Conn) {
	conn, channels, requests, err := ssh.NewServerConn(raw, f.ssh)
	if err != nil {
		return
	}
	defer conn.Close()
	go ssh.DiscardRequests(requests)
	var workers sync.WaitGroup
	defer workers.Wait()
	for request := range channels {
		var payload struct {
			Host       string
			Port       uint32
			Origin     string
			OriginPort uint32
		}
		if request.ChannelType() != "direct-tcpip" || ssh.Unmarshal(request.ExtraData(), &payload) != nil || net.JoinHostPort(payload.Host, fmt.Sprint(payload.Port)) != f.target {
			_ = request.Reject(ssh.ConnectionFailed, "fixture cannot resolve target")
			continue
		}
		channel, incoming, err := request.Accept()
		if err != nil {
			continue
		}
		go ssh.DiscardRequests(incoming)
		workers.Add(1)
		go func() { defer workers.Done(); defer channel.Close(); f.tunnel(channel) }()
	}
}

func (f *protocolFixture) serveProxy(conn net.Conn) {
	reader := bufio.NewReader(conn)
	read := func(n int) ([]byte, error) { b := make([]byte, n); _, err := io.ReadFull(reader, b); return b, err }
	if f.node.Type == "http" {
		request, err := http.ReadRequest(reader)
		if err != nil {
			return
		}
		if f.reject.Load() || f.node.Username != "" && request.Header.Get("Proxy-Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte("fixture:fixture-password")) {
			fmt.Fprint(conn, "HTTP/1.1 407 Proxy Authentication Required\r\nContent-Length: 0\r\n\r\n")
			return
		}
		if request.Method != "CONNECT" || request.Host != f.target {
			fmt.Fprint(conn, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
			return
		}
		fmt.Fprint(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
	} else {
		header, err := read(2)
		if err != nil || header[0] != 5 {
			return
		}
		methods, err := read(int(header[1]))
		if err != nil {
			return
		}
		method := byte(0)
		if f.node.Username != "" {
			method = 2
		}
		if !bytes.Contains(methods, []byte{method}) {
			conn.Write([]byte{5, 255})
			return
		}
		conn.Write([]byte{5, method})
		if method == 2 {
			v, err := read(2)
			if err != nil {
				return
			}
			user, err := read(int(v[1]))
			if err != nil {
				return
			}
			v, err = read(1)
			if err != nil {
				return
			}
			password, err := read(int(v[0]))
			if err != nil {
				return
			}
			if f.reject.Load() || string(user) != "fixture" || string(password) != "fixture-password" {
				conn.Write([]byte{1, 1})
				return
			}
			conn.Write([]byte{1, 0})
		}
		v, err := read(4)
		if err != nil || v[0] != 5 || v[1] != 1 {
			return
		}
		var host string
		switch v[3] {
		case 1:
			b, err := read(4)
			if err != nil {
				return
			}
			host = net.IP(b).String()
		case 3:
			n, err := read(1)
			if err != nil {
				return
			}
			b, err := read(int(n[0]))
			if err != nil {
				return
			}
			host = string(b)
		default:
			return
		}
		port, err := read(2)
		if err != nil {
			return
		}
		if f.reject.Load() || net.JoinHostPort(host, fmt.Sprint(binary.BigEndian.Uint16(port))) != f.target {
			conn.Write([]byte{5, 4, 0, 1, 0, 0, 0, 0, 0, 0})
			return
		}
		conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
	}
	f.tunnel(conn)
}

func TestWindowsBundledManualProtocols(t *testing.T) {
	if os.Getenv("EDEN_PROXY_INTEGRATION") != "1" {
		t.Skip("requires bundled Mihomo beside the test binary")
	}
	for _, scenario := range []struct{ kind, auth string }{{"http", "none"}, {"http", "password"}, {"socks5", "none"}, {"socks5", "password"}, {"ssh", "password"}, {"ssh", "private-key"}, {"ssh", "encrypted-key"}} {
		t.Run(scenario.kind+"/"+scenario.auth, func(t *testing.T) {
			var targetRequests atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				targetRequests.Add(1)
				if r.Header.Get("Proxy-Authorization") != "" {
					t.Error("proxy authentication leaked to target")
				}
				if r.URL.Path == "/stream" {
					w.Header().Set("Content-Type", "text/plain")
					fmt.Fprint(w, "open\n")
					w.(http.Flusher).Flush()
					<-r.Context().Done()
					return
				}
				fmt.Fprint(w, "controlled-target")
			}))
			defer target.Close()
			fixture := newProtocolFixture(t, scenario.kind, scenario.auth, strings.TrimPrefix(target.URL, "http://"))
			defer fixture.close()
			profile := t.TempDir()
			store, err := Load(profile)
			if err != nil {
				t.Fatal(err)
			}
			reserved, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			store.Port = reserved.Addr().(*net.TCPAddr).Port
			reserved.Close()
			store.Nodes = []Node{fixture.node}
			store.Selected = fixture.node.ID
			store.Revision = Token()
			if err := store.Save(profile); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := Stop(profile); err != nil {
					t.Error(err)
				}
			}()
			start := func() {
				t.Helper()
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				if err := Start(ctx, profile, store); err != nil {
					t.Fatal(err)
				}
			}
			proxyURL, _ := url.Parse(fmt.Sprintf("http://eden:%s@127.0.0.1:%d", store.Auth, store.Port))
			transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
			request := func(address string) error {
				r, err := client.Get(address)
				if err != nil {
					return err
				}
				defer r.Body.Close()
				body, err := io.ReadAll(r.Body)
				if err != nil {
					return err
				}
				if r.StatusCode != 200 || string(body) != "controlled-target" {
					return errors.New("target did not return expected response")
				}
				return nil
			}
			fail := func(address string) {
				t.Helper()
				before := targetRequests.Load()
				if err := request(address); err == nil {
					t.Fatal("fault unexpectedly succeeded")
				}
				if targetRequests.Load() != before {
					t.Fatal("failed proxy request reached the target directly")
				}
			}
			start()
			if err := request(target.URL); err != nil {
				t.Fatal(err)
			}
			if fixture.forwarded.Load() != 1 {
				t.Fatal("request did not traverse selected upstream")
			}
			fail("http://unresolvable.invalid/")
			if err := Stop(profile); err != nil {
				t.Fatal(err)
			}
			fixture.reject.Store(true)
			start()
			fail(target.URL)
			if err := Stop(profile); err != nil {
				t.Fatal(err)
			}
			fixture.reject.Store(false)
			if scenario.kind == "ssh" {
				trusted := store.Nodes[0].HostKey
				_, unknownKey, err := ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				unknownSigner, err := ssh.NewSignerFromKey(unknownKey)
				if err != nil {
					t.Fatal(err)
				}
				store.Nodes[0].HostKey = []string{strings.TrimSpace(string(ssh.MarshalAuthorizedKey(unknownSigner.PublicKey())))}
				start()
				fail(target.URL)
				if err := Stop(profile); err != nil {
					t.Fatal(err)
				}
				store.Nodes[0].HostKey = trusted
				if scenario.auth == "encrypted-key" {
					store.Nodes[0].PrivateKeyPassphrase = "incorrect-passphrase"
					if err := Validate(t.Context(), profile, store); err == nil {
						t.Fatal("encrypted key accepted with an incorrect passphrase")
					}
					store.Nodes[0].PrivateKeyPassphrase = fixture.node.PrivateKeyPassphrase
				}
			}
			start()
			if err := request(target.URL); err != nil {
				t.Fatal("authentication recovery failed", err)
			}
			stream, err := client.Get(target.URL + "/stream")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := bufio.NewReader(stream.Body).ReadString('\n'); err != nil {
				t.Fatal(err)
			}
			beforeStop := time.Now()
			if err := Stop(profile); err != nil {
				t.Fatal(err)
			}
			_, err = io.ReadAll(stream.Body)
			stream.Body.Close()
			if err == nil || time.Since(beforeStop) > 2*time.Second {
				t.Fatal("stopping core did not promptly revoke existing connection", err)
			}
			fail(target.URL)
			start()
			var record windowsCoreRecord
			raw, err := os.ReadFile(windowsCoreRecordPath(profile))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &record); err != nil {
				t.Fatal(err)
			}
			process, err := os.FindProcess(int(record.PID))
			if err != nil {
				t.Fatal(err)
			}
			if err := process.Kill(); err != nil {
				t.Fatal(err)
			}
			process.Release()
			fail(target.URL)
			start()
			if err := request(target.URL); err != nil {
				t.Fatal("core crash recovery failed", err)
			}
			if err := Stop(profile); err != nil {
				t.Fatal(err)
			}
			fixture.stall.Store(true)
			start()
			fail(target.URL)
			fixture.close()
			fail(target.URL)
			t.Log("normal, resolution failure, authentication rejection/recovery, connection revocation, core crash/recovery, timeout and upstream stop passed")
		})
	}
}
