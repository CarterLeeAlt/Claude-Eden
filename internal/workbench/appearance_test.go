package workbench

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sylvinus/agent-vm/internal/uitheme"
)

func TestAppearanceDefaultUsesSharedClaudeEdenPalette(t *testing.T) {
	var expected AppearanceTheme
	if err := json.Unmarshal(uitheme.LightJSON, &expected); err != nil {
		t.Fatal(err)
	}
	a := defaultAppearance()
	if !reflect.DeepEqual(a.Light, expected) || a.UIFontSize != 14 || a.CodeFontSize != 13 {
		t.Fatalf("controller default differs from bundled UI: %+v", a)
	}
	if a.Light.Preset != "eden" || a.Light.Colors["button"] != "#a64b32" || !a.Light.OpaqueWindows {
		t.Fatal("default brand was replaced")
	}
	// Calls must not share a mutable colors map.
	a.Light.Colors["button"] = "#000000"
	if defaultAppearance().Light.Colors["button"] != "#a64b32" {
		t.Fatal("default palette leaked a mutation")
	}
}

func TestAppearanceSavedDerivedThemeDoesNotInheritBrandOverrides(t *testing.T) {
	s := &Server{profile: t.TempDir()}
	a := defaultAppearance()
	a.Light.Preset, a.Light.Surface, a.Light.Colors = "custom", "#ffffff", nil
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := writePrivate(filepath.Join(s.profile, "appearance.json"), a); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.getAppearance(w, httptest.NewRequest("GET", "/api/appearance", nil))
	var loaded Appearance
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &loaded) != nil || !reflect.DeepEqual(loaded, a) {
		t.Fatalf("saved custom theme changed: %s (stored %s)", w.Body.String(), b)
	}
}

func TestCodexAppearanceImportsOnlyAppearance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	data := `model = "unrelated"
[desktop]
sansFontSize = 16
codeFontSize = 13
appearanceLightCodeThemeId = "solarized"
appearanceDarkCodeThemeId = "ayu"
[desktop.appearanceLightChromeTheme]
accent = "#b58900"
surface = "#fdf6e3"
ink = "#657b83"
contrast = 45
opaqueWindows = false
[desktop.appearanceDarkChromeTheme]
accent = "#e6b450"
surface = "#10141c"
ink = "#bfbdb6"
contrast = 60
opaqueWindows = true
[desktop.appearanceDarkChromeTheme.fonts]
code = "Menlo"
`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".codex-global-state.json"), []byte(`{"reduced-motion-preference":"on","irrelevant":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := readCodexAppearance(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.Light.Preset != "solarized" || a.Light.Surface != "#fdf6e3" || a.UIFontSize != 16 || a.CodeFontSize != 13 || a.ReducedMotion != "on" || !a.FontSmoothing {
		t.Fatalf("unexpected appearance: %+v", a)
	}
	b, _ := json.Marshal(a)
	if bytes.Contains(b, []byte("secret")) || bytes.Contains(b, []byte("unrelated")) || bytes.Contains(b, []byte("darkTheme")) || bytes.Contains(b, []byte("fonts")) {
		t.Fatal("non-appearance data leaked")
	}
}

func TestAppearanceSaveDoesNotChangeEnvironmentAndRejectsBadInput(t *testing.T) {
	s := &Server{profile: t.TempDir(), preferences: Preferences{Workspace: "/project"}, jobs: jobManager{state: Job{Running: true, Kind: "start"}}}
	a := defaultAppearance()
	a.UIFontSize = 16
	b, _ := json.Marshal(a)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/appearance", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	s.saveAppearance(w, req)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if !s.jobs.snapshot().Running || s.jobs.snapshot().Kind != "start" || s.preferences.Workspace != "/project" {
		t.Fatal("appearance changed environment state")
	}
	w = httptest.NewRecorder()
	s.getAppearance(w, httptest.NewRequest("GET", "/api/appearance", nil))
	var saved Appearance
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil || !reflect.DeepEqual(saved, a) {
		t.Fatalf("appearance did not round trip: %s", w.Body.String())
	}
	a.UIFontSize = 400
	b, _ = json.Marshal(a)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/appearance", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	s.saveAppearance(w, req)
	if w.Code == 200 {
		t.Fatal("invalid appearance accepted")
	}
	stored, _ := os.ReadFile(filepath.Join(s.profile, "appearance.json"))
	if bytes.Contains(stored, []byte("400")) {
		t.Fatal("invalid settings replaced saved appearance")
	}
}

func TestAppearanceCustomThemesPersistAndInvalidUpdatesKeepFile(t *testing.T) {
	s := &Server{profile: t.TempDir()}
	a := defaultAppearance()
	a.Light.Colors = map[string]string{"chrome": "#123456", "buttonInk": "#abcdef"}
	a.CustomThemes = []SavedAppearanceTheme{{ID: "my-theme", Name: "我的配色", Light: a.Light}}
	save := func(a Appearance) *httptest.ResponseRecorder {
		b, _ := json.Marshal(a)
		req := httptest.NewRequest("POST", "/api/appearance", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.saveAppearance(w, req)
		return w
	}
	if w := save(a); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	original, err := os.ReadFile(filepath.Join(s.profile, "appearance.json"))
	if err != nil {
		t.Fatal(err)
	}
	restarted := &Server{profile: s.profile}
	w := httptest.NewRecorder()
	restarted.getAppearance(w, httptest.NewRequest("GET", "/api/appearance", nil))
	var loaded Appearance
	if err := json.Unmarshal(w.Body.Bytes(), &loaded); err != nil || !reflect.DeepEqual(loaded, a) {
		t.Fatalf("lost saved themes: %s", w.Body.String())
	}
	for _, mutate := range []func(*Appearance){
		func(a *Appearance) { a.Light.Colors["chrome"] = "red" },
		func(a *Appearance) { a.Light.Colors["unknown"] = "#ffffff" },
		func(a *Appearance) { a.CustomThemes[0].Name = "  " },
		func(a *Appearance) { a.CustomThemes = append(a.CustomThemes, a.CustomThemes[0]) },
		func(a *Appearance) { a.CustomThemes[0].Light.Colors = map[string]string{"line": "#abc"} },
	} {
		var invalid Appearance
		if err := json.Unmarshal(original, &invalid); err != nil {
			t.Fatal(err)
		}
		mutate(&invalid)
		if w := save(invalid); w.Code == 200 {
			t.Fatal("invalid theme accepted")
		}
		stored, _ := os.ReadFile(filepath.Join(s.profile, "appearance.json"))
		if !bytes.Equal(stored, original) {
			t.Fatal("invalid input replaced valid theme file")
		}
	}
	// Exercise the full supported library, including every editable color.
	for i := 1; i < 50; i++ {
		item := SavedAppearanceTheme{ID: fmt.Sprintf("theme-%d", i), Name: "配色", Light: a.Light}
		item.Light.Colors = make(map[string]string)
		for _, key := range appearanceColorKeys {
			item.Light.Colors[key] = "#123456"
		}
		a.CustomThemes = append(a.CustomThemes, item)
	}
	if w := save(a); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	a.CustomThemes = append(a.CustomThemes, SavedAppearanceTheme{ID: "overflow", Name: "超限", Light: a.Light})
	if w := save(a); w.Code == 200 {
		t.Fatal("library limit not enforced")
	}
}

func TestAppearanceFileKeepsDerivedDefaults(t *testing.T) {
	s := &Server{profile: t.TempDir()}
	expected := defaultAppearance()
	b, _ := json.Marshal(expected)
	if err := os.WriteFile(filepath.Join(s.profile, "appearance.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.getAppearance(w, httptest.NewRequest("GET", "/api/appearance", nil))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var loaded Appearance
	if err := json.Unmarshal(w.Body.Bytes(), &loaded); err != nil || !reflect.DeepEqual(loaded, expected) {
		t.Fatalf("appearance lost: %s", w.Body.String())
	}
}

func TestAppearanceRejectsUnsupportedFieldsWithoutChangingFile(t *testing.T) {
	for _, field := range []string{"mode", "darkTheme", "terminal", "fonts"} {
		t.Run(field, func(t *testing.T) {
			s := &Server{profile: t.TempDir()}
			b, err := json.Marshal(defaultAppearance())
			if err != nil {
				t.Fatal(err)
			}
			var value map[string]any
			if err := json.Unmarshal(b, &value); err != nil {
				t.Fatal(err)
			}
			if field == "mode" || field == "darkTheme" {
				value[field] = "unsupported"
			} else {
				value["lightTheme"].(map[string]any)[field] = map[string]any{}
			}
			invalid, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(s.profile, "appearance.json")
			if err := os.WriteFile(file, invalid, 0600); err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			s.getAppearance(w, httptest.NewRequest("GET", "/api/appearance", nil))
			if w.Code != 400 {
				t.Fatalf("unsupported saved field accepted: %s", w.Body.String())
			}
			w = httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/api/appearance", bytes.NewReader(invalid))
			req.Header.Set("Content-Type", "application/json")
			s.saveAppearance(w, req)
			if w.Code != 400 {
				t.Fatalf("unsupported input accepted: %s", w.Body.String())
			}
			stored, err := os.ReadFile(file)
			if err != nil || !bytes.Equal(stored, invalid) {
				t.Fatal("rejected read/write changed saved bytes", err)
			}
		})
	}
}
