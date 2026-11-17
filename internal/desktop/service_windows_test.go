package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDesktopServiceRejectsInvalidRequestsBeforeNativeCalls(t *testing.T) {
	service := &DesktopService{}
	ctx := context.Background()
	if _, err := service.SaveRecoveryKey("invalid"); err == nil {
		t.Fatal("invalid recovery key accepted")
	}
	if _, err := service.ExportTheme(ThemeExportRequest{Text: `{"format":"other","version":2}`}); err == nil {
		t.Fatal("invalid theme accepted")
	}
	if _, err := service.DesktopFileAction(ctx, FileActionRequest{ID: "invalid"}); err == nil {
		t.Fatal("invalid file action accepted")
	}
	if _, err := service.BrowserDownloadAction(ctx, DownloadRequest{ID: "bd799ff2-597d-4e24-a0a4-7db302d6b981", Operation: "execute"}); err == nil {
		t.Fatal("invalid download operation accepted")
	}
	if _, err := service.UploadHostFiles(ctx, UploadRequest{Target: "other"}); err == nil {
		t.Fatal("invalid upload target accepted")
	}
	if _, err := service.AppUpdate("other"); err == nil {
		t.Fatal("invalid update operation accepted")
	}
	if status, err := service.AccountDevice("status"); err != nil || status.Supported {
		t.Fatal("device status changed")
	}
	if _, err := service.AccountDevice("login"); err == nil {
		t.Fatal("unsupported device operation accepted")
	}
	if _, err := service.AppReference("other"); err == nil {
		t.Fatal("unlisted external reference accepted")
	}
}

func TestFileSelectionsAreOpaqueBoundedAndExpire(t *testing.T) {
	service := &DesktopService{shell: &Shell{clipboard: &clipboardSync{readers: make(map[string]selectedFiles)}}, selections: make(map[string]selectedFiles)}
	root := t.TempDir()
	file := filepath.Join(root, "selected.txt")
	if err := os.WriteFile(file, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, paths := range [][]string{nil, make([]string, 101), {"relative.txt"}, {root}, {filepath.Join(root, "missing.txt")}} {
		if _, err := service.rememberFiles(paths); err == nil {
			t.Fatalf("invalid selection accepted: %v", paths)
		}
	}
	if len(service.shell.clipboard.readers) != 0 {
		t.Fatal("rejected file selections retained tentative cache leases")
	}
	for range 17 {
		selection, err := service.rememberFiles([]string{file})
		if err != nil {
			t.Fatal(err)
		}
		if selection.Count != 1 || validID(selection.FileSelection) != nil || strings.Contains(selection.FileSelection, root) {
			t.Fatal("selection leaked paths or lost identity")
		}
	}
	if len(service.selections) != 16 {
		t.Fatal("selection bound not enforced")
	}
	if len(service.shell.clipboard.readers) != 16 {
		t.Fatal("file selections did not retain exactly their live clipboard file leases")
	}
	service.selections["expired"] = selectedFiles{[]string{file}, time.Now().Add(-time.Minute)}
	if _, err := service.rememberFiles([]string{file}); err != nil {
		t.Fatal(err)
	}
	if _, exists := service.selections["expired"]; exists {
		t.Fatal("expired selection retained")
	}
	service.selections["expired"] = selectedFiles{[]string{file}, time.Now().Add(-time.Minute)}
	_, err := service.UploadHostFiles(context.Background(), UploadRequest{Target: "terminal", ID: "0123456789abcdef", Selection: "expired"})
	if err == nil {
		t.Fatal("expired upload selection accepted")
	}
	if _, exists := service.selections["expired"]; exists {
		t.Fatal("used selection was not consumed")
	}
}

func TestThemeAndAppearanceValidation(t *testing.T) {
	if err := validateTheme(`{"format":"eden-theme","version":2}`); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"null", `{"format":"eden-theme","version":1}`, `{"format":"eden-theme","version":2.5}`, strings.Repeat("x", 256*1024+1), string([]byte{0xff})} {
		if validateTheme(text) == nil {
			t.Fatal("invalid theme accepted")
		}
	}
	root := t.TempDir()
	path := filepath.Join(root, "theme.json")
	if err := os.WriteFile(path, []byte("\xef\xbb\xbf{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if text, err := readTheme(path); err != nil || text != "{}" {
		t.Fatal("UTF-8 BOM import damaged")
	}
	if err := os.WriteFile(path, []byte{0xff}, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readTheme(path); err == nil {
		t.Fatal("invalid UTF-8 import accepted")
	}
	for _, value := range []string{"#123456", "#ABCdef"} {
		if _, err := parseColour(value); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"red", "#12345", "#1234567", "#GGGGGG"} {
		if _, err := parseColour(value); err == nil {
			t.Fatal("invalid appearance accepted")
		}
	}
	if safeFilename(`..\folder\download:bad?.txt`) != "download_bad_.txt" {
		t.Fatal("download name was not sanitized")
	}
}

func TestUploadSelectionLeaseLivesUntilHelperFinishes(t *testing.T) {
	t.Setenv("EDEN_DESKTOP_DRIVER_FIXTURE", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"success", "failure", "block"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("EDEN_DESKTOP_DRIVER_MODE", mode)
			root := t.TempDir()
			backend, err := NewBackend(executable, filepath.Join(root, "config.json"))
			if err != nil {
				t.Fatal(err)
			}
			backend.Directory = root
			// OS clipboard collection is the external boundary and is excluded
			// here; the actual token, lease and backend helper paths run normally.
			clipboard := &clipboardSync{readers: make(map[string]selectedFiles), collecting: true}
			service := &DesktopService{shell: &Shell{backend: backend, clipboard: clipboard}, selections: make(map[string]selectedFiles)}
			file := filepath.Join(root, "selected.txt")
			if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			selection, err := service.rememberFiles([]string{file})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := service.UploadHostFiles(ctx, UploadRequest{Target: "terminal", ID: "0123456789abcdef", Selection: selection.FileSelection})
				done <- err
			}()
			deadline := time.Now().Add(3 * time.Second)
			active := false
			for time.Now().Before(deadline) {
				clipboard.mu.Lock()
				lease, exists := clipboard.readers[selection.FileSelection]
				active = exists && lease.expires.IsZero()
				clipboard.mu.Unlock()
				if active {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if !active {
				t.Fatal("upload did not promote its lease to the helper's full lifetime")
			}
			if mode == "block" {
				cancel()
			}
			select {
			case err := <-done:
				if (err == nil) != (mode == "success") {
					t.Fatalf("helper result changed: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("upload helper did not finish or honour cancellation")
			}
			if len(clipboard.readers) != 0 || len(service.selections) != 0 {
				t.Fatal("consumed token or completed/cancelled upload lease was retained")
			}
		})
	}
}
