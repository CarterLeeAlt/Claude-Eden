package desktopupdate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestLaunchReadRetriesOnlyExplicitSharingErrors(t *testing.T) {
	for _, tc := range []struct {
		code error
		want bool
	}{
		{windows.ERROR_SHARING_VIOLATION, true}, {windows.ERROR_LOCK_VIOLATION, true},
		{windows.ERROR_ACCESS_DENIED, false}, {windows.ERROR_FILE_NOT_FOUND, false},
		{windows.ERROR_INVALID_DATA, false}, {errors.New("sharing violation"), false},
	} {
		if got := isSharingViolation(fmt.Errorf("read receipt: %w", tc.code)); got != tc.want {
			t.Fatal(tc.code, got)
		}
	}
	for _, release := range []bool{true, false} {
		t.Run(fmt.Sprint("release-", release), func(t *testing.T) {
			name := filepath.Join(t.TempDir(), "launch.json")
			writeTest(t, name, `{"version":"0.1.8"}`)
			p, err := windows.UTF16PtrFromString(name)
			if err != nil {
				t.Fatal(err)
			}
			h, err := windows.CreateFile(p, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if h != 0 {
					if err := windows.CloseHandle(h); err != nil {
						t.Error(err)
					}
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- waitForLaunch(ctx, name, "0.1.8", make(chan error, 1)) }()
			started := time.Now()
			select {
			case err := <-done:
				t.Fatal("did not retry sharing conflict", err)
			case <-time.After(650 * time.Millisecond):
			}
			if release {
				if err := windows.CloseHandle(h); err != nil {
					t.Fatal(err)
				}
				h = 0
			}
			err = <-done
			if release {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "持续被占用") || time.Since(started) < 3*time.Second {
				t.Fatal("sharing retry did not end at its operation boundary", err)
			}
		})
	}
}

func TestLaunchReadPermissionErrorIsImmediate(t *testing.T) {
	name := filepath.Join(t.TempDir(), "launch.json")
	writeTest(t, name, `{"version":"0.1.8"}`)
	original, err := windows.GetNamedSecurityInfo(name, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	oldACL, _, err := original.DACL()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := windows.SetNamedSecurityInfo(name, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, oldACL, nil); err != nil {
			t.Error(err)
		}
	}()
	// Deny only file data, retaining READ_CONTROL/WRITE_DAC for restoration.
	sd, err := windows.SecurityDescriptorFromString("D:P(D;;0x1;;;WD)(A;;FA;;;OW)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(name, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	started := time.Now()
	err = waitForLaunch(ctx, name, "0.1.8", make(chan error, 1))
	if !errors.Is(err, os.ErrPermission) || time.Since(started) >= time.Second {
		t.Fatal("permission error swallowed or retried", err)
	}
}
