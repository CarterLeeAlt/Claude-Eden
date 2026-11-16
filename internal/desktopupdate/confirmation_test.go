package desktopupdate

import (
	"errors"
	"os"
	"testing"
)

// Ordinary startups have no confirmation token and never enter the native
// installation path. Keep this contract covered on every supported Go target.
func TestStartupConfirmationRejectsNilAndOrdinaryStartup(t *testing.T) {
	u := testUpdater(t, &fakeBackend{}, nil)
	t.Setenv("EDEN_UPDATE_CONFIRMATION", "")
	if err := u.MarkStartupFailed(nil); err == nil {
		t.Fatal("nil cause accepted")
	}
	if err := u.MarkStartupFailed(errors.New("ordinary startup")); err != nil {
		t.Fatal(err)
	}
	if err := u.MarkReady(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(u.config.Root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("ordinary startup wrote update data", err)
	}
}
