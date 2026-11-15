package cli

import (
	"bytes"
	"strings"
	"testing"
)

// A program that embeds the shared commands names itself and says how it
// updates: Eden must not send its users to agent-vm's installer.
func TestEmbeddingProgramIdentity(t *testing.T) {
	savedCommand, savedHint := Command, UpdateHint
	t.Cleanup(func() { Command, UpdateHint = savedCommand, savedHint })
	Command, UpdateHint = "eden", "update Claude Eden with its installer"
	te, code, _ := setupRun(t, "--preinstall=none")
	if code != 0 || !strings.Contains(te.out(), "Starting eden setup...") || strings.Contains(te.out(), "Starting agent-vm setup...") {
		t.Fatalf("setup used the wrong embedding identity: %d %s", code, te.out())
	}

	var stdout, stderr bytes.Buffer
	io := IO{Stdout: &stdout, Stderr: &stderr}
	if code := versionCmd([]string{"--min", "99.0.0"}, io); code != 1 {
		t.Fatalf("an unmet minimum returned %d", code)
	}
	got := stderr.String()
	for _, want := range []string{"Error: eden ", "Update it:  update Claude Eden with its installer"} {
		if !strings.Contains(got, want) {
			t.Errorf("the version error lacks %q: %q", want, got)
		}
	}
	if strings.Contains(got, "agent-vm") {
		t.Errorf("the version error names agent-vm: %q", got)
	}

	stderr.Reset()
	if code := licensesCmd([]string{"x"}, io); code != 2 || !strings.Contains(stderr.String(), "Usage: eden licenses") {
		t.Errorf("licenses usage: %d %q", code, stderr.String())
	}
	for _, args := range [][]string{{"unknown"}, {"run"}, {"shell", "unexpected"}, {"doctor", "unexpected"}, {"list", "unexpected"}, {"stop", "one", "two"}} {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			environment := newStartEnv(t)
			if code := environment.run(args...); code != exitUsage || !strings.Contains(environment.out(), "eden ") || strings.Contains(environment.out(), "agent-vm") {
				t.Fatalf("embedding command usage: %d %s", code, environment.out())
			}
		})
	}
	t.Run("doctor-and-start-guidance", func(t *testing.T) {
		environment := newStartEnv(t)
		if code := environment.run("doctor"); code != 1 || !strings.Contains(environment.out(), "eden doctor") || !strings.Contains(environment.out(), "Run 'eden setup'") {
			t.Fatalf("doctor used the wrong recovery command: %d %s", code, environment.out())
		}
		if code := environment.run("run", "true"); code != 1 || !strings.Contains(environment.out(), "Run 'eden setup' first") || strings.Contains(environment.out(), "agent-vm") {
			t.Fatalf("start used the wrong recovery command: %d %s", code, environment.out())
		}
	})
}
