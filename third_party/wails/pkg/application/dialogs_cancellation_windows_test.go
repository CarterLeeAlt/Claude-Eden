//go:build windows && !server

package application

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wailsapp/wails/v3/internal/go-common-file-dialog/cfd"
)

// This fixture implements the dialog boundary without creating COM objects or
// HWNDs. Both single and multiple cancellation must become an empty selection.
type cancellationDialogFixture struct {
	cfd.OpenMultipleFilesDialog
	path     string
	paths    []string
	err      error
	released int
}

func (d *cancellationDialogFixture) ShowAndGetResult() (string, error) {
	return d.path, d.err
}
func (d *cancellationDialogFixture) ShowAndGetResults() ([]string, error) {
	return d.paths, d.err
}
func (d *cancellationDialogFixture) Release() error { d.released++; return nil }

func TestWindowsFileDialogCancellationReturnsEmptySelection(t *testing.T) {
	for _, multiple := range []bool{false, true} {
		for _, cancellation := range []error{cfd.ErrorCancelled, fmt.Errorf("dialog: %w", cfd.ErrorCancelled)} {
			dialog := &cancellationDialogFixture{err: cancellation}
			result, err := showCfdDialog(func() (cfd.Dialog, error) { return dialog, nil }, multiple, nil)
			if err != nil || dialog.released != 1 {
				t.Fatalf("cancellation leaked an error or dialog: %v, released=%d", err, dialog.released)
			}
			if multiple {
				if paths, ok := result.([]string); !ok || len(paths) != 0 {
					t.Fatalf("cancelled multi-selection = %#v", result)
				}
			} else if result != "" {
				t.Fatalf("cancelled single-selection = %#v; must not become '.'", result)
			}
		}
	}
}

func TestWindowsFileDialogSuccessAndErrorsRemainDistinct(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "selected.txt")
	for _, multiple := range []bool{false, true} {
		dialog := &cancellationDialogFixture{path: path, paths: []string{path}}
		result, err := showCfdDialog(func() (cfd.Dialog, error) { return dialog, nil }, multiple, nil)
		var want any = path
		if multiple {
			want = []string{path}
		}
		if err != nil || !reflect.DeepEqual(result, want) || dialog.released != 1 {
			t.Fatalf("selection changed: %#v, %v", result, err)
		}
		dialog.err = errors.New("dialog fixture failure")
		if _, err = showCfdDialog(func() (cfd.Dialog, error) { return dialog, nil }, multiple, nil); err != dialog.err || dialog.released != 2 {
			t.Fatal("non-cancellation error was hidden or dialog was not released")
		}
		if _, err = showCfdDialog(func() (cfd.Dialog, error) { return nil, dialog.err }, multiple, nil); err != dialog.err {
			t.Fatal("dialog creation error was hidden")
		}
	}
}

func TestWindowsFileDialogDefaultExtensionMatchesWin32Contract(t *testing.T) {
	if windowsDialogDefaultExtension(nil) != "" {
		t.Fatal("missing file filter invented a default extension")
	}
	for _, test := range []struct{ pattern, extension string }{
		{"*.zip", "zip"}, {"*.json", "json"}, {"*.txt", "txt"}, {"*.tar.gz;*.zip", "tar.gz"},
		{" *.png ; *.jpg", "png"}, {"*.*", ""}, {"*", ""}, {"*.?xt", ""}, {"README", ""},
	} {
		actual := windowsDialogDefaultExtension([]FileFilter{{Pattern: test.pattern}})
		if actual != test.extension {
			t.Fatalf("filter %q produced extension %q, want %q", test.pattern, actual, test.extension)
		}
	}
}
