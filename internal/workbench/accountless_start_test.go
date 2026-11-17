package workbench

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFreshProfileDoesNotRequireLocalAccountPassword(t *testing.T) {
	account, err := loadLocalAccount(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(account.lock)
	if account.view().Exists || !account.view().Unlocked {
		t.Fatalf("fresh profile account state=%+v; want accountless unlocked state", account.view())
	}
	epoch, allowed := account.accessFor(true)
	if epoch == nil || !allowed {
		t.Fatalf("fresh profile access epoch=%v allowed=%t; want live access", epoch, allowed)
	}
	if filepath.Base(account.path) != "account.json" {
		t.Fatalf("unexpected account path: %s", account.path)
	}
}

func TestAccountlessDesktopKeySurvivesRestartWithoutCreatingAccount(t *testing.T) {
	profile := t.TempDir()
	first, err := loadLocalAccount(profile)
	if err != nil {
		t.Fatal(err)
	}
	key, err := first.desktopKey()
	if err != nil || len(key) == 0 {
		t.Fatal("accountless desktop key unavailable", err)
	}
	defer clear(key)
	first.lock()
	second, err := loadLocalAccount(profile)
	if err != nil {
		t.Fatal(err)
	}
	defer second.lock()
	restored, err := second.desktopKey()
	defer clear(restored)
	if err != nil || !bytes.Equal(key, restored) {
		t.Fatal("desktop key changed across restart", err)
	}
	oldEpoch, _ := second.access()
	second.suspend()
	resumed, err := second.desktopKey()
	defer clear(resumed)
	if err != nil || !bytes.Equal(key, resumed) {
		t.Fatal("suspending accountless view lost desktop key", err)
	}
	newEpoch, allowed := second.access()
	if !allowed || newEpoch == oldEpoch || oldEpoch.Err() == nil || newEpoch.Err() != nil {
		t.Fatal("accountless view lifetime was not rotated")
	}
	if _, err := os.Stat(filepath.Join(profile, "account.json")); !os.IsNotExist(err) {
		t.Fatal("accountless startup created an account")
	}
	if err := second.create("Optional account", "fixture-password-123", ""); err != nil {
		t.Fatal(err)
	}
	sealed, err := second.desktopKey()
	defer clear(sealed)
	if err != nil || !bytes.Equal(key, sealed) {
		t.Fatal("optional password discarded existing encrypted data")
	}
	if _, err := os.Stat(filepath.Join(profile, accountlessKeyFile)); !os.IsNotExist(err) {
		t.Fatal("raw key remained after enabling password protection")
	}
}

func TestAccountlessCorruptKeyIsNotRegenerated(t *testing.T) {
	profile := t.TempDir()
	file := filepath.Join(profile, accountlessKeyFile)
	if err := writePrivate(file, []byte("invalid")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if account, err := loadLocalAccount(profile); err == nil {
		account.lock()
		t.Fatal("corrupt key accepted")
	}
	after, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("corrupt key overwritten", err)
	}
}
