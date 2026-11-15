//go:build !windows

package workbench

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

// The authenticated loopback SSH endpoint runs the shipped guest command in an
// isolated home. It never reads a real account or developer configuration.
func cliTestSSH(t *testing.T, s *Server, home string, env []string) {
	cliTestSSHRewrite(t, s, home, env, nil)
}

func cliTestSSHRewrite(t *testing.T, s *Server, home string, env []string, rewrite func(string) string) {
	t.Helper()
	runtimeSource, err := os.ReadFile("../eden/client_runtime.py")
	if err != nil {
		t.Fatal(err)
	}
	runtimeDirectory := filepath.Join(home, ".local/share/workbench/cli-channel-code")
	if err := os.MkdirAll(runtimeDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDirectory, "client_runtime.py"), runtimeSource, 0600); err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostPublic, hostKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatal(err)
	}
	clientKey, _ := ssh.NewPublicKey(public)
	config := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if !bytes.Equal(key.Marshal(), clientKey.Marshal()) {
			return nil, errors.New("unknown test client")
		}
		return nil, nil
	}}
	config.AddHostKey(hostSigner)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); listener.Close() })
	dir := s.instanceDir()
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(dir, "fixture-key")
	block, err := ssh.MarshalPrivateKey(private, "workbench-test")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(keyFile, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	pub, _ := ssh.NewPublicKey(hostPublic)
	known := filepath.Join(dir, "fixture-known-hosts")
	if err = os.WriteFile(known, append([]byte(fmt.Sprintf("[127.0.0.1]:%d ", port)), ssh.MarshalAuthorizedKey(pub)...), 0600); err != nil {
		t.Fatal(err)
	}
	settings := fmt.Sprintf("Host *\n HostName 127.0.0.1\n Port %d\n User developer\n StrictHostKeyChecking yes\n UserKnownHostsFile %q\n IdentityFile %q\n", port, known, keyFile)
	if err = os.WriteFile(filepath.Join(dir, "ssh.config"), []byte(settings), 0600); err != nil {
		t.Fatal(err)
	}
	if err = writePrivate(filepath.Join(dir, "eden-running.json"), map[string]int{"pid": os.Getpid()}); err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				server, channels, requests, err := ssh.NewServerConn(conn, config)
				if err != nil {
					conn.Close()
					return
				}
				defer server.Close()
				stop := context.AfterFunc(ctx, func() { server.Close() })
				defer stop()
				go ssh.DiscardRequests(requests)
				for target := range channels {
					if target.ChannelType() != "session" {
						_ = target.Reject(ssh.UnknownChannelType, "session required")
						continue
					}
					ch, reqs, err := target.Accept()
					if err != nil {
						return
					}
					func() {
						defer ch.Close()
						withPTY := false
						for req := range reqs {
							if req.Type == "pty-req" {
								withPTY = true
								_ = req.Reply(true, nil)
								continue
							}
							if req.Type != "exec" {
								_ = req.Reply(false, nil)
								continue
							}
							var in struct{ Command string }
							if ssh.Unmarshal(req.Payload, &in) != nil {
								return
							}
							_ = req.Reply(true, nil)
							if rewrite != nil {
								in.Command = rewrite(in.Command)
							}
							cmd := exec.CommandContext(ctx, "/bin/sh", "-c", in.Command)
							cmd.Dir, cmd.Env = home, env
							if withPTY {
								terminal, err := startTerminal(cmd, 100, 36)
								if err != nil {
									return
								}
								defer terminal.Close()
								go func() { _, _ = io.Copy(ch, terminal) }()
								go func() { _, _ = io.Copy(terminal, ch); _ = terminal.Kill(); _ = terminal.Close() }()
								_ = terminal.Wait()
								_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
								return
							}
							cmd.Stdout, cmd.Stderr = ch, ch.Stderr()
							input, err := cmd.StdinPipe()
							if err != nil {
								return
							}
							go func() { _, _ = io.Copy(input, ch); _ = input.Close() }()
							status := uint32(0)
							if err := cmd.Run(); err != nil {
								status = 1
							}
							_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
							return
						}
					}()
				}
			}()
		}
	}()
}
