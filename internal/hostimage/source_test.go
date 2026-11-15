package hostimage

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSourceRejectsUntrustedURLsAndUnsupportedSystems(t *testing.T) {
	base := Plan{Host: Identity{System: "debian-13", Version: "13", Arch: "amd64"}, Version: "13", Format: "qcow2", Digest: "sha256:" + strings.Repeat("a", 64), URL: "https://cloud.debian.org/system.img"}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"http://cloud.debian.org/a", "https://cloud.debian.org.evil.test/a", "https://user@cloud.debian.org/a", "https://cloud.debian.org:443/a", "https://cloud-images.ubuntu.com/a"} {
		plan := base
		plan.URL = url
		if err := plan.Validate(); err == nil {
			t.Errorf("accepted %q", url)
		}
	}
	for _, system := range []string{"arch-linux", "debian-12", "ubuntu-22.04", "ubuntu-24.04"} {
		plan := base
		plan.Host.System = system
		if err := plan.Validate(); err == nil {
			t.Fatalf("accepted unsupported source %q", system)
		}
	}
	for _, change := range []func(*Plan){
		func(p *Plan) { p.Version, p.Host.Version = "12", "12" },
		func(p *Plan) { p.Host.Arch = "arm64" },
	} {
		plan := base
		change(&plan)
		if err := plan.Validate(); err == nil {
			t.Fatalf("accepted unsupported identity %+v", plan)
		}
	}
}

func TestSourceKeyAndSpaceAllowCompletedConcurrentDownload(t *testing.T) {
	cache := t.TempDir()
	plan := Plan{Host: Identity{System: "debian-13", Version: "13", Arch: "amd64"}, Version: "13", Format: "qcow2", URL: "https://cloud.debian.org/example.img", Digest: "sha256:" + strings.Repeat("a", 64), Bytes: 10}
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.CachedPath(cache)+".partial", []byte("123456"), 0600); err != nil {
		t.Fatal(err)
	}
	if remaining := plan.RemainingBytes(cache); remaining != 4 {
		t.Fatalf("remaining=%d", remaining)
	}
	ready := plan
	ready.Path = plan.CachedPath(cache)
	ready.Modified = 123
	if ready.Key() != plan.Key() {
		t.Fatal("a completed cache invalidated the selected image")
	}
	if remaining := ready.RemainingBytes(cache); remaining != 0 {
		t.Fatalf("ready remaining=%d", remaining)
	}
	plan.Digest = ""
	if err := plan.Validate(); err == nil {
		t.Fatal("accepted media without the pinned catalog digest")
	}
}

func TestDownloadResumesAndRejectsWrongRange(t *testing.T) {
	for _, wrong := range []bool{false, true} {
		t.Run(fmt.Sprint(wrong), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Range") != "bytes=3-" {
					t.Error("did not request resume")
				}
				start := 3
				if wrong {
					start = 2
				}
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-5/6", start))
				w.WriteHeader(http.StatusPartialContent)
				fmt.Fprint(w, "def")
			}))
			defer server.Close()
			file, err := os.CreateTemp(t.TempDir(), "partial")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			file.WriteString("abc")
			var progress bytes.Buffer
			err = download(t.Context(), Plan{Host: Identity{System: "debian-13"}, URL: server.URL, Bytes: 6}, file, 3, &progress, nil)
			if wrong {
				if err == nil {
					t.Fatal("accepted wrong range")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(file.Name())
			if string(data) != "abcdef" {
				t.Fatalf("got %q", data)
			}
			if !strings.Contains(progress.String(), "EDEN_GUEST_BYTES debian-13 download 6 6") {
				t.Fatalf("missing actual resumed byte progress: %s", progress.String())
			}
		})
	}
}

func TestPrepareRejectsModifiedSourceAndPreservesIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "system.img")
	os.WriteFile(path, []byte("original"), 0600)
	info, _ := os.Stat(path)
	digest := sha256.Sum256([]byte("original"))
	plan := Plan{Host: Identity{System: "debian-13", Version: "13", Arch: "amd64"}, Version: "13", Format: "qcow2", Path: path, Bytes: 8, Modified: info.ModTime().UnixNano(), Digest: "sha256:" + hex.EncodeToString(digest[:])}
	os.WriteFile(path, []byte("tampered"), 0600)
	if _, err := Prepare(t.Context(), filepath.Join(dir, "cache"), plan, io.Discard, nil); err == nil {
		t.Fatal("accepted changed source")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "tampered" {
		t.Fatal("changed user's source")
	}
}

func TestPrepareRecoversOversizedPartialWithoutDiscardingIt(t *testing.T) {
	cache := t.TempDir()
	content := []byte("valid image")
	digest := sha256.Sum256(content)
	plan := Plan{
		Host:    Identity{System: "debian-13", Version: "13", Arch: "amd64"},
		Version: "13", Format: "qcow2", Bytes: int64(len(content)),
		URL: "https://cloud.debian.org/fixture.img", Digest: "sha256:" + hex.EncodeToString(digest[:]),
	}
	partial := plan.CachedPath(cache) + ".partial"
	bad := []byte("oversized incomplete response")
	if err := os.WriteFile(partial, bad, 0600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Range") != "" {
			t.Error("retried using the oversized prefix")
		}
		_, _ = w.Write(content)
	}))
	defer server.Close()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{
		SerialNumber: big.NewInt(1), DNSNames: []string{"cloud.debian.org"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	// StartTLS trusts this certificate only in the test client; the request still
	// verifies the catalog hostname and certificate chain over the real TLS path.
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}
	server.StartTLS()
	transport := server.Client().Transport.(*http.Transport)
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return new(net.Dialer).DialContext(ctx, network, server.Listener.Addr().String())
	}
	defer transport.CloseIdleConnections()
	for attempt := 0; attempt < 2; attempt++ {
		ready, err := Prepare(t.Context(), cache, plan, io.Discard, transport)
		if err != nil || ready.Digest != plan.Digest || ready.Path != plan.CachedPath(cache) {
			t.Fatalf("retry did not prepare the original image: %+v %v", ready, err)
		}
	}
	retained, err := filepath.Glob(partial + ".invalid-*")
	if err != nil || len(retained) != 1 || requests != 1 {
		t.Fatalf("retained=%v requests=%d error=%v", retained, requests, err)
	}
	data, err := os.ReadFile(retained[0])
	if err != nil || !bytes.Equal(data, bad) {
		t.Fatal("discarded the failed response", err)
	}
}
