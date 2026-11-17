package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixtureRejectsMissingOrEscapingConfigurationBeforeWrites(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()
	t.Chdir(workspace)
	for _, tc := range []struct {
		name, root, config string
	}{
		{"missing-both", "", ""},
		{"missing-config", workspace, ""},
		{"missing-workspace", "", filepath.Join(workspace, "profile", "config.json")},
		{"workspace-root", workspace, workspace},
		{"outside", workspace, filepath.Join(outside, "profile", "config.json")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EDEN_WORKSPACE_ROOT", tc.root)
			t.Setenv("EDEN_CONFIG", tc.config)
			if err := run(); err == nil {
				t.Fatal("unsafe fixture invocation accepted")
			}
			for _, root := range []string{workspace, outside} {
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 0 {
					t.Fatalf("invalid invocation wrote into %s: %v, %v", root, entries, err)
				}
			}
		})
	}
}

func TestFixtureAcceptsWorkspaceRelativeConfiguration(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	t.Setenv("EDEN_WORKSPACE_ROOT", workspace)
	t.Setenv("EDEN_CONFIG", filepath.Join("profile", "config.json"))
	got, err := fixtureConfig()
	if want := filepath.Join(workspace, "profile", "config.json"); err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}
