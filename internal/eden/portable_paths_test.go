package eden

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func portablePathFixture(t *testing.T, app string) string {
	t.Helper()
	profile := filepath.Join(app, "data", "profile")
	if err := os.MkdirAll(profile, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ClaudeEden.exe", "portable.flag"} {
		if err := os.WriteFile(filepath.Join(app, name), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return profile
}

func TestPortableEnvironmentRecordSurvivesDirectoryMove(t *testing.T) {
	root := t.TempDir()
	before := filepath.Join(root, "first location")
	profile := portablePathFixture(t, before)
	transfer := DefaultEnvironmentDir(profile)
	if err := os.MkdirAll(transfer, 0700); err != nil {
		t.Fatal(err)
	}
	record, err := EnvironmentRecordForStorage(profile, EnvironmentRecord{Directory: transfer})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), before) || filepath.IsAbs(record.Directory) || !record.Portable {
		t.Fatalf("machine-specific portable record: %s", encoded)
	}
	if err := os.WriteFile(filepath.Join(profile, "environment.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	instance := filepath.Join(profile, "state", "lima", "existing-disk-id")
	if err := os.MkdirAll(instance, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instance, "lima.yaml"), []byte("guestIdentity: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	name := InstanceName(profile, transfer)
	if name != "existing-disk-id" {
		t.Fatalf("existing portable disk was replaced with %q", name)
	}
	after := filepath.Join(root, "另一目录 moved")
	if err := os.Rename(before, after); err != nil {
		t.Fatal(err)
	}
	newProfile := filepath.Join(after, "data", "profile")
	want := DefaultEnvironmentDir(newProfile)
	if got := EnvironmentDir(newProfile); got != want {
		t.Fatalf("moved directory=%q want=%q", got, want)
	}
	if got := InstanceName(newProfile, want); got != name {
		t.Fatalf("moved VM identity=%q want=%q", got, name)
	}
	if _, err := os.Stat(before); !os.IsNotExist(err) {
		t.Fatalf("old location was recreated: %v", err)
	}
}

func TestPortableEnvironmentRejectsEscapingStoredPaths(t *testing.T) {
	profile := portablePathFixture(t, filepath.Join(t.TempDir(), "portable"))
	for _, value := range []string{"../outside", "../../outside", filepath.Join(t.TempDir(), "absolute")} {
		b, err := json.Marshal(EnvironmentRecord{Directory: value, Portable: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(profile, "environment.json"), b, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadEnvironmentRecord(profile); err == nil {
			t.Fatalf("accepted escaping portable path %q", value)
		}
	}
}

func TestPortableStorageRemainsInsideDataWithoutMigrationLink(t *testing.T) {
	profile := portablePathFixture(t, filepath.Join(t.TempDir(), "portable"))
	loc, err := GetStorageLocation(profile)
	if err != nil {
		t.Fatal(err)
	}
	if !loc.Portable || loc.Custom || loc.Directory != filepath.Join(profile, "state") {
		t.Fatalf("unexpected portable storage: %+v", loc)
	}
	target := filepath.Join(t.TempDir(), "must-not-be-created")
	if err := ValidateStorageTarget(profile, target); err == nil {
		t.Fatal("portable storage accepted an external migration target")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("rejected target was touched: %v", err)
	}
}

func TestPortableEnvironmentRejectsExistingDirectoryLink(t *testing.T) {
	root := t.TempDir()
	profile := portablePathFixture(t, filepath.Join(root, "portable"))
	outside := filepath.Join(root, "outside-data")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(PortableDataDirectory(profile), "redirect")
	if err := linkStorage(link, outside); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(link); err != nil {
			t.Error(err)
		}
	})
	if _, err := EnvironmentRecordForStorage(profile, EnvironmentRecord{Directory: filepath.Join(link, "new-downloads")}); err == nil {
		t.Fatal("portable environment accepted a directory link outside data")
	}
	if _, err := os.Stat(filepath.Join(outside, "new-downloads")); !os.IsNotExist(err) {
		t.Fatalf("rejected linked target was touched: %v", err)
	}
}
