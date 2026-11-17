package main

import (
	"bytes"
	"encoding/binary"
	"testing"

	agentvm "github.com/sylvinus/agent-vm"
)

// Resource wiring is a byte-only check. Do not start Wails or register fonts
// from ordinary host tests; GDI and actual controls are verified in the VM.
func TestDesktopAssetsUseRootEmbeddedResources(t *testing.T) {
	options := desktopAssets()
	for _, asset := range []struct {
		name      string
		got, want []byte
	}{
		{"icon", options.Icon, agentvm.DesktopIcon},
		{"regular font", options.FontRegular, agentvm.DesktopFontRegular},
		{"bold font", options.FontBold, agentvm.DesktopFontBold},
	} {
		if len(asset.got) == 0 || !bytes.Equal(asset.got, asset.want) {
			t.Fatalf("%s does not use the root embedded resource", asset.name)
		}
	}
	if len(options.Icon) < 6 || binary.LittleEndian.Uint16(options.Icon) != 0 || binary.LittleEndian.Uint16(options.Icon[2:]) != 1 {
		t.Fatal("desktop icon is not an ICO resource")
	}
	count := int(binary.LittleEndian.Uint16(options.Icon[4:]))
	if count == 0 || len(options.Icon) < 6+16*count {
		t.Fatal("desktop ICO directory is incomplete")
	}
	for index := range count {
		entry := options.Icon[6+16*index:]
		length := uint64(binary.LittleEndian.Uint32(entry[8:]))
		offset := uint64(binary.LittleEndian.Uint32(entry[12:]))
		if length == 0 || offset < uint64(6+16*count) || offset+length > uint64(len(options.Icon)) {
			t.Fatal("desktop ICO image data is incomplete")
		}
	}
	for _, font := range [][]byte{options.FontRegular, options.FontBold} {
		if len(font) < 12 || binary.BigEndian.Uint32(font) != 0x00010000 {
			t.Fatal("desktop font is not the packaged TrueType resource")
		}
	}
	if bytes.Equal(options.FontRegular, options.FontBold) {
		t.Fatal("regular and bold fonts must use their distinct packaged resources")
	}
}

func TestStartupFailureDialogDoesNotBlockUpdateFailureExit(t *testing.T) {
	for _, test := range []struct {
		args         []string
		updateLaunch bool
		show         bool
	}{
		{nil, false, true},
		{nil, true, false},
		{[]string{"--smoke-test", "fixture"}, false, false},
		{[]string{"--native-self-test", "fixture"}, false, false},
		{[]string{"--install-update"}, true, false},
	} {
		if startupFailureNeedsDialog(test.args, test.updateLaunch) != test.show {
			t.Fatal("startup failure dialog policy hid an update process exit or changed ordinary startup")
		}
	}
}
