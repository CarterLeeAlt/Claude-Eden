package desktop

import (
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestNativeTransportOriginAndPayloadBoundary(t *testing.T) {
	valid := &application.OriginInfo{Origin: "http://localhost:4567/#token", TopOrigin: "http://localhost:4567/#token", IsMainFrame: true}
	message := runtimeMessagePrefix + `{"id":"bd799ff2-597d-4e24-a0a4-7db302d6b981"}`
	if !validRuntimeMessage(message, valid, 4567) {
		t.Fatal("trusted top document rejected")
	}
	if !validRuntimeMessage(message, &application.OriginInfo{Origin: "http://localhost:4567/#token", TopOrigin: "http://localhost:4567/workbench", IsMainFrame: true}, 4567) {
		t.Fatal("trusted SPA history rewrite rejected")
	}
	for _, origin := range []*application.OriginInfo{nil, {Origin: valid.Origin, TopOrigin: "http://localhost:4568/", IsMainFrame: true}, {Origin: "https://localhost:4567/", TopOrigin: valid.TopOrigin, IsMainFrame: true}, {Origin: "http://evil:4567/", TopOrigin: valid.TopOrigin, IsMainFrame: true}, {Origin: "http://localhost:4567/frame", TopOrigin: valid.TopOrigin}} {
		if validRuntimeMessage(message, origin, 4567) {
			t.Fatalf("untrusted source accepted: %+v", origin)
		}
	}
	if validRuntimeMessage(message, valid, 0) || validRuntimeMessage("other:"+message, valid, 4567) || validRuntimeMessage(runtimeMessagePrefix+strings.Repeat("x", 30<<20), valid, 4567) {
		t.Fatal("runtime message boundary not enforced")
	}
}
