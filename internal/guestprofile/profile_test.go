package guestprofile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFixedDefaultAndExplicitRandomProfiles(t *testing.T) {
	p, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := Defaults()
	want.Hostname = p.Hostname
	if p.Options != want || p.ExtensionSuffix != "" {
		t.Fatalf("default changed: %+v", p)
	}
	seen := map[string]bool{}
	for range 100 {
		p, err := New(Options{Mode: "random"})
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
		if p.Username == "developer" || p.BrowserUsername == "browser" || seen[p.Hostname] || seen[p.ExtensionSuffix] {
			t.Fatal("random profile reused default or generated identity")
		}
		seen[p.Hostname], seen[p.ExtensionSuffix] = true, true
	}
}

func TestCreationAndReload(t *testing.T) {
	dir := t.TempDir()
	o := Options{Mode: "manual", Hostname: "X", Username: "alex", BrowserUsername: "web", WorkHome: "/home/projects", BrowserHome: "/home/browsing", AudioName: "speakers", AudioDescription: "Desktop Speakers", DisplayWidth: 1920, DisplayHeight: 1080, BrowserWidth: 1280, BrowserHeight: 720, ExtensionName: "My Tabs", WebGL: "disabled", Canvas: "blocked", GraphicsBackend: "auto"}
	p, err := Create(dir, o)
	if err != nil {
		t.Fatal(err)
	}
	if p.Options != o {
		t.Fatalf("manual values changed: %+v", p.Options)
	}
	next, err := Create(dir, Options{Mode: "random"})
	if err != nil || *p != *next {
		t.Fatal("restart changed profile", err)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var copy Profile
	if err := json.Unmarshal(b, &copy); err != nil || copy != *p {
		t.Fatal("serialization changed profile", err)
	}
	if !o.Matches(p.Options) || !(Options{}).Matches(p.Options) || (Options{Username: "different"}).Matches(p.Options) {
		t.Fatal("creation options comparison is incorrect")
	}
}

func TestInvalidOptions(t *testing.T) {
	for _, o := range []Options{{Mode: "unknown"}, {Hostname: "bad\nname"}, {Hostname: "-abc"}, {Hostname: "0123"}, {Username: "root"}, {Username: "debian"}, {Username: "a;id"}, {Username: "same", BrowserUsername: "same"}, {AudioName: "a;id"}, {AudioDescription: "x\nload-module bad"}, {ExtensionName: "\"; id"}, {DisplayWidth: 1920}, {BrowserWidth: 8000, BrowserHeight: 600}, {DisplayWidth: 800, DisplayHeight: 600, BrowserWidth: 1280, BrowserHeight: 720}, {WebGL: "fake-gpu"}, {Canvas: "unknown"}, {WorkHome: "/root/private"}, {BrowserHome: "/home/a/../b"}, {WorkHome: "/home/same", BrowserHome: "/home/same"}, {Username: "alex", BrowserUsername: "web", WorkHome: "/home/web"}, {WorkHome: "/home/debian"}, {AudioModel: "missing"}, {AudioModel: "ur22c", AudioDescription: "other"}, {GraphicsBackend: "metal"}, {GraphicsBackend: "d3d11"}, {GraphicsBackend: "opengl", WebGL: "disabled"}} {
		if _, err := New(o); err == nil {
			t.Fatalf("accepted invalid options %+v", o)
		}
	}
}

func TestMissingAndCorruptProfiles(t *testing.T) {
	dir := t.TempDir()
	if p, err := Load(dir); err != nil || p != nil {
		t.Fatal("absent record changed", err)
	}
	path := filepath.Join(dir, Filename)
	for _, raw := range []string{`{}`, `{"version":999}`, `{"unknown":true}`, `{} {}`} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Create(dir, Options{}); err == nil {
			t.Fatal("replaced corrupt profile")
		}
		b, err := os.ReadFile(path)
		if err != nil || string(b) != raw {
			t.Fatal("overwrote corrupt record", err)
		}
	}
}

func TestConcurrentCreateUsesOneCompleteProfile(t *testing.T) {
	dir := t.TempDir()
	start := make(chan struct{})
	results := make(chan *Profile, 24)
	for range cap(results) {
		go func() {
			<-start
			p, err := Create(dir, Options{Mode: "random"})
			if err != nil {
				t.Errorf("concurrent creation: %v", err)
			}
			results <- p
		}()
	}
	close(start)
	first := <-results
	for i := 1; i < cap(results); i++ {
		next := <-results
		if first == nil || next == nil || *first != *next {
			t.Error("installers received different profiles")
		}
	}
}

func TestAudioCatalogAndGeneratedDevice(t *testing.T) {
	models := AudioModels()
	if len(models) != 12 {
		t.Fatal("audio catalog is incomplete")
	}
	seen := map[string]bool{}
	for _, m := range models {
		if m.ID == "" || seen[m.ID] || m.Source == "" {
			t.Fatal("invalid catalog provenance")
		}
		seen[m.ID] = true
		p, err := New(Options{Mode: "manual", AudioModel: m.ID})
		if err != nil {
			t.Fatal(err)
		}
		if p.AudioDescription != m.Label || p.AudioModel != m.ID {
			t.Fatal("selected audio model changed")
		}
	}
}
