package workbench

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sylvinus/agent-vm/internal/guestprofile"
)

func sampleDevice() *deviceObservation {
	v := &deviceObservation{UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/142.0.0.0 Safari/537.36", Platform: "Linux x86_64"}
	v.Canvas.State, v.Canvas.Checksum = "passed", "1234abcd"
	v.WebGL.State, v.WebGL.Renderer, v.WebGL.Version = "passed", "ANGLE (Intel, Intel UHD Graphics, OpenGL)", "WebGL 2.0"
	v.Audio.State = "permission_required"
	return v
}
func deviceCheckState(items []deviceCheckItem, id string) string {
	for _, item := range items {
		if item.ID == id {
			return item.State
		}
	}
	return "missing"
}

func TestGraphicsEvidenceDistinguishesFallback(t *testing.T) {
	p, err := guestprofile.New(guestprofile.Options{GraphicsBackend: "opengl"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ renderer, backend, class, state string }{
		{"ANGLE (Intel, Intel UHD Graphics, OpenGL)", "opengl", "driver-reported", "passed"},
		{"ANGLE (Intel, Intel UHD Graphics, Direct3D11)", "opengl", "driver-reported", "unknown"},
		{"llvmpipe (LLVM 18)", "opengl", "software", "failed"},
		{"llvmpipe (LLVM 18)", "auto", "software", "warning"},
		{"SwiftShader Device", "software", "software", "passed"},
		{"virtio GPU", "auto", "virtual-gpu", "passed"},
		{"ANGLE (Intel, Intel UHD Graphics, OpenGL)", "software", "driver-reported", "failed"},
		{"", "auto", "unobserved", "unknown"},
	} {
		v := sampleDevice()
		v.WebGL.Renderer = tc.renderer
		p.GraphicsBackend = tc.backend
		items, class := evaluateDevice(p, v)
		if class != tc.class || deviceCheckState(items, "graphics-backend") != tc.state {
			t.Fatalf("incorrect graphics evidence: %+v %s %+v", tc, class, items)
		}
	}
	v := sampleDevice()
	v.WebGL.State = "failed"
	items, _ := evaluateDevice(p, v)
	if deviceCheckState(items, "webgl-render") != "failed" {
		t.Fatal("incorrect pixels passed")
	}
	items, class := evaluateDevice(p, nil)
	if class != "unobserved" || deviceCheckState(items, "browser-evidence") != "unknown" {
		t.Fatal("missing evidence passed")
	}
}

func TestDevicePermissionMalformedAndUntrustedEvidence(t *testing.T) {
	v := sampleDevice()
	items, _ := evaluateDevice(nil, v)
	if deviceCheckState(items, "browser-audio") != "unknown" || deviceCheckState(items, "user-agent") != "unknown" {
		t.Fatal("unavailable audio or UA evidence passed")
	}
	for _, mutate := range []func(*deviceObservation){func(v *deviceObservation) { v.WebGL.Renderer = strings.Repeat("x", 513) }, func(v *deviceObservation) { v.Canvas.State = "other" }, func(v *deviceObservation) { v.Audio.State = "other" }, func(v *deviceObservation) { v.Screen.Width = -1 }, func(v *deviceObservation) { v.Canvas.Checksum = "INVALID" }} {
		v := sampleDevice()
		mutate(v)
		items, _ := evaluateDevice(nil, v)
		if deviceCheckState(items, "browser-evidence") != "unknown" {
			t.Fatal("malformed evidence accepted")
		}
	}
	s := testServer(t)
	if r := request(s, "POST", "/api/environment/device-check", s.origin, `{"target":"browser"}`, true); r.Code != 403 {
		t.Fatal("untrusted window collected device data", r.Code)
	}
	if r := guestProfileRequest(s, "POST", "/api/environment/device-check", `{"target":"other"}`); r.Code != 400 {
		t.Fatal("invalid target accepted", r.Code)
	}
	if r := guestProfileRequest(s, "POST", "/api/environment/device-check", `{"target":"browser"}`); r.Code != 409 {
		t.Fatal("stopped environment probed", r.Code)
	}
}

func TestDeviceChecksSavedOptionsAndIdentityConsistency(t *testing.T) {
	p, err := guestprofile.New(guestprofile.Options{WebGL: "disabled", Canvas: "blocked"})
	if err != nil {
		t.Fatal(err)
	}
	v := sampleDevice()
	if err := json.Unmarshal([]byte(`{"brands":[{"brand":"Chromium","version":"142"}],"platform":"Linux"}`), &v.ClientHints); err != nil {
		t.Fatal(err)
	}
	items, _ := evaluateDevice(p, v)
	if deviceCheckState(items, "user-agent") != "passed" || deviceCheckState(items, "webgl-render") != "failed" || deviceCheckState(items, "canvas-render") != "failed" {
		t.Fatal("saved options not compared with observed values")
	}
	v.WebGL.State, v.WebGL.Renderer, v.Canvas.State, v.Canvas.Checksum = "unavailable", "", "blocked", ""
	items, _ = evaluateDevice(p, v)
	if deviceCheckState(items, "webgl-render") != "passed" || deviceCheckState(items, "canvas-render") != "passed" {
		t.Fatal("matching disabled options rejected")
	}
	v.ClientHints.Platform = "Windows"
	items, _ = evaluateDevice(p, v)
	if deviceCheckState(items, "user-agent") != "failed" {
		t.Fatal("inconsistent OS passed")
	}
	v.ClientHints.Platform = "Linux"
	v.ClientHints.Brands[0].Version = "141"
	items, _ = evaluateDevice(p, v)
	if deviceCheckState(items, "user-agent") != "failed" {
		t.Fatal("inconsistent browser version passed")
	}
}
