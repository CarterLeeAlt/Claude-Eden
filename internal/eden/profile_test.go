package eden

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigChangeRequiresStop(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "lima", "test"), 0700)
	c := DefaultConfig()
	if err := c.RecordRunning(dir, "test"); err != nil {
		t.Fatal(err)
	}
	if err := c.CheckRunning(dir); err != nil {
		t.Fatal(err)
	}
	c.Timezone = "Asia/Tokyo"
	if c.CheckRunning(dir) == nil {
		t.Fatal("running profile silently changed")
	}
}

func TestRunningRecordRejectsReusedPID(t *testing.T) {
	dir := t.TempDir()
	instance := filepath.Join(dir, "lima", "test")
	os.MkdirAll(instance, 0700)
	c := DefaultConfig()
	if err := c.RecordRunning(dir, "test"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(instance, "eden-running.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record runningProfile
	if err := json.Unmarshal(b, &record); err != nil {
		t.Fatal(err)
	}
	if record.Started == "" {
		t.Skip("process start time is unavailable on this platform")
	}
	if !record.alive() {
		t.Fatal("current owner was not recognized")
	}
	legacy := runningProfile{PID: record.PID}
	if !legacy.alive() {
		t.Fatal("legacy PID-only record was rejected")
	}
	reused := runningProfile{PID: record.PID, Started: record.Started + "0", Fingerprint: record.Fingerprint}
	if reused.alive() {
		t.Fatal("record from an earlier process with the same PID was treated as running")
	}
	b, _ = json.Marshal(reused)
	if err := os.WriteFile(filepath.Join(instance, "eden-running.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	c.Timezone = "Asia/Tokyo"
	if err := c.CheckRunning(dir); err != nil {
		t.Fatalf("stale record blocked a configuration change: %v", err)
	}
}

// A PID reused by a process whose start time cannot be read, such as a
// protected Windows service after a reboot, is not the recorded owner (#167).
func TestRunningRecordRejectsUnreadableStartTime(t *testing.T) {
	if processStartToken(os.Getpid()) == "" {
		t.Skip("process start time is unavailable on this platform")
	}
	// PID zero has no queryable process start time. An exited child's PID can
	// still identify a retained process object or be reused by another process.
	const unavailable = 0
	if processStartToken(unavailable) != "" {
		t.Fatal("reserved PID unexpectedly has a process start time")
	}
	if ProcessStartMatches(unavailable, "1") {
		t.Fatal("an unreadable start time matched a recorded owner")
	}
	if !ProcessStartMatches(unavailable, "") {
		t.Fatal("a legacy record without a start time lost its PID-only check")
	}
}
