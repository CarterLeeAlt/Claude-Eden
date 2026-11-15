package eden

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNoVNCClipboardUsesSelectedRoot(t *testing.T) {
	requireLinuxGuest(t)
	const marker = "sudo tee \"$workbench_novnc_root/workbench-clipboard.js\""
	services := browserServices()
	start := strings.Index(services, marker)
	if start < 0 {
		t.Fatal("noVNC clipboard provision step is missing")
	}
	patch := "sudo() { \"$@\"; }\n" + services[start:]
	const scripts = `<script type="module" src="app/ui.js"></script><script src="keep.js"></script>`
	const tag = `<script type="module" src="workbench-clipboard.js?revision=8"></script>`
	for _, rootName := range []string{"usr/share/novnc", "Debian noVNC fixture"} {
		for _, previous := range []struct{ name, tag string }{
			{"fresh", ""},
			{"unversioned", `<script type="module" src="workbench-clipboard.js"></script>`},
			{"earlier_revision", `<script type="module" src="workbench-clipboard.js?revision=3"></script>`},
		} {
			t.Run(rootName+"/"+previous.name, func(t *testing.T) {
				root := filepath.Join(t.TempDir(), "selected root", filepath.FromSlash(rootName))
				if err := os.MkdirAll(root, 0700); err != nil {
					t.Fatal(err)
				}
				html := filepath.Join(root, "vnc.html")
				if err := os.WriteFile(html, []byte("<!doctype html><html><body>"+scripts+previous.tag+"</body></html>"), 0600); err != nil {
					t.Fatal(err)
				}
				run := func() []byte {
					t.Helper()
					ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
					defer cancel()
					cmd := exec.CommandContext(ctx, "bash", "-se")
					cmd.Env = append(os.Environ(), "workbench_novnc_root="+root)
					cmd.Stdin = strings.NewReader(patch)
					if output, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("clipboard provision: %v\n%s", err, output)
					}
					body, err := os.ReadFile(html)
					if err != nil {
						t.Fatal(err)
					}
					return body
				}
				first := run()
				if strings.Count(string(first), tag) != 1 || strings.Count(string(first), "workbench-clipboard.js") != 1 || !bytes.Contains(first, []byte(scripts)) {
					t.Fatalf("clipboard tag or existing scripts changed unexpectedly: %s", first)
				}
				if second := run(); !bytes.Equal(first, second) {
					t.Fatalf("repeated clipboard provision changed HTML: %s", second)
				}
				server := httptest.NewServer(http.FileServer(http.Dir(root)))
				defer server.Close()
				for resource, want := range map[string][]byte{"/vnc.html": first, "/workbench-clipboard.js?revision=8": []byte(clipboardJS + "\n")} {
					response, err := server.Client().Get(server.URL + resource)
					if err != nil {
						t.Fatal(err)
					}
					body, readErr := io.ReadAll(response.Body)
					closeErr := response.Body.Close()
					if response.StatusCode != http.StatusOK || readErr != nil || closeErr != nil || !bytes.Equal(body, want) {
						t.Fatalf("selected noVNC root does not serve %s: status=%d read=%v close=%v", resource, response.StatusCode, readErr, closeErr)
					}
				}
			})
		}
		t.Run(rootName+"/missing_html", func(t *testing.T) {
			root := filepath.Join(t.TempDir(), filepath.FromSlash(rootName))
			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "bash", "-se")
			cmd.Env = append(os.Environ(), "workbench_novnc_root="+root)
			cmd.Stdin = strings.NewReader(patch)
			if output, err := cmd.CombinedOutput(); err == nil || !bytes.Contains(output, []byte("FileNotFoundError")) {
				t.Fatalf("missing selected HTML must fail visibly: %v\n%s", err, output)
			}
			if _, err := os.Stat(filepath.Join(root, "vnc.html")); !os.IsNotExist(err) {
				t.Fatalf("failed provision created or replaced HTML: %v", err)
			}
		})
	}
}
