package desktop

import (
	"testing"
	"unsafe"
)

func TestNativeSelfTestABISizes(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("product Windows amd64 ABI")
	}
	if unsafe.Sizeof(nativeKeyboardInput{}) != 40 || unsafe.Offsetof(nativeKeyboardInput{}.VK) != 8 || unsafe.Offsetof(nativeKeyboardInput{}.Extra) != 24 {
		t.Fatal("SendInput union ABI changed")
	}
	if unsafe.Sizeof(inputProcessorProfile{}) != 88 || unsafe.Offsetof(inputProcessorProfile{}.CLSID) != 8 || unsafe.Offsetof(inputProcessorProfile{}.Layout) != 72 {
		t.Fatal("TSF profile ABI changed")
	}
	if unsafe.Sizeof(nativePlacement{}) != 44 {
		t.Fatal("WINDOWPLACEMENT ABI changed")
	}
}
