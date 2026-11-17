package desktop

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

// These tests never open the OS clipboard, create an HWND or change input state.
func TestClipboardBitmapRoundTrip(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 120, G: 80, B: 20, A: 255})
	img.SetNRGBA(2, 1, color.NRGBA{R: 1, G: 2, B: 3, A: 255})
	dib := imageToDIB(img)
	data, err := dibToPNG(dib)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for y := range 2 {
		for x := range 3 {
			got := color.NRGBAModel.Convert(decoded.At(x, y)).(color.NRGBA)
			want := img.NRGBAAt(x, y)
			want.A = 255
			if got != want {
				t.Fatalf("pixel %d,%d = %+v, want %+v", x, y, got, want)
			}
		}
	}
	if _, err := validatePNG(data); err != nil {
		t.Fatal(err)
	}
	trimmed, err := trimClipboardPNG(append(bytes.Clone(data), make([]byte, 16)...))
	if err != nil || !bytes.Equal(trimmed, data) {
		t.Fatal("GlobalSize padding damaged PNG")
	}
	if _, err := validatePNG([]byte("not png")); err == nil {
		t.Fatal("invalid PNG accepted")
	}
}

func TestClipboardBitmapValidation(t *testing.T) {
	valid := imageToDIB(image.NewNRGBA(image.Rect(0, 0, 2, 2)))
	for _, data := range [][]byte{nil, make([]byte, 39), valid[:40], valid[:len(valid)-1]} {
		if _, err := dibToPNG(data); err == nil {
			t.Fatal("truncated DIB accepted")
		}
	}
	for _, test := range []struct {
		offset int
		value  uint32
	}{{0, 10000}, {4, 0}, {4, 0x7fffffff}, {8, 0}, {8, 0x80000000}, {16, 99}, {32, 10000}} {
		data := bytes.Clone(valid)
		binary.LittleEndian.PutUint32(data[test.offset:], test.value)
		if _, err := dibToPNG(data); err == nil {
			t.Fatalf("malformed DIB offset %d accepted", test.offset)
		}
	}
	data := bytes.Clone(valid)
	binary.LittleEndian.PutUint32(data[8:], 0xfffffffe)
	if _, err := dibToPNG(data); err != nil {
		t.Fatalf("top-down DIB rejected: %v", err)
	}
}

func TestClipboardPNGAllocationPaddingAndTruncation(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	data := encoded.Bytes()
	for _, padding := range []int{0, 1, 16, 64} {
		actual, err := trimClipboardPNG(append(bytes.Clone(data), make([]byte, padding)...))
		if err != nil || !bytes.Equal(actual, data) {
			t.Fatalf("padding %d: %v", padding, err)
		}
	}
	for length := range len(data) {
		if _, err := trimClipboardPNG(data[:length]); err == nil {
			t.Fatalf("truncated PNG length %d accepted", length)
		}
	}
	oversizedChunk := bytes.Clone(data)
	binary.BigEndian.PutUint32(oversizedChunk[8:], 0xffffffff)
	invalidEnd := bytes.Clone(data)
	binary.BigEndian.PutUint32(invalidEnd[len(data)-12:], 1)
	for _, invalid := range [][]byte{nil, []byte("not png"), oversizedChunk, invalidEnd} {
		if _, err := trimClipboardPNG(invalid); err == nil {
			t.Fatal("invalid PNG chunk allocation accepted")
		}
	}
}

func TestClipboardCacheReferences(t *testing.T) {
	root := t.TempDir()
	owned := []string{filepath.Join(root, "cache", "one"), filepath.Join(root, "cache", "two")}
	for _, selected := range [][]string{{owned[0]}, {filepath.Join(root, "cache")}, {root}, {filepath.Join(root, "cache", "child", "three")}} {
		if !pathsReference(selected, owned) {
			t.Fatal("live selection lost cache ownership")
		}
	}
	for _, selected := range [][]string{nil, {filepath.Join(root, "cache-other", "one")}} {
		if pathsReference(selected, owned) {
			t.Fatal("unrelated selection retained cache ownership")
		}
	}
	if pathsReference([]string{root}, nil) {
		t.Fatal("empty ownership matched")
	}
}

func TestClipboardUnicodeTextDecoding(t *testing.T) {
	for _, text := range []string{"", "Claude Eden 中文 😀", strings.Repeat("x", maxText)} {
		var data []byte
		for _, unit := range append(utf16.Encode([]rune(text)), 0, 123) {
			data = binary.LittleEndian.AppendUint16(data, unit)
		}
		actual, err := decodeClipboardText(data)
		if err != nil || actual != text {
			t.Fatalf("Unicode text length %d: %v", len(text), err)
		}
	}
	if _, err := decodeClipboardText([]byte{1}); err == nil {
		t.Fatal("odd UTF-16 allocation accepted")
	}
	if _, err := decodeClipboardText(bytes.Repeat([]byte{'x', 0}, maxText+1)); err == nil {
		t.Fatal("oversized text accepted")
	}
}

func TestClipboardCollectionRespectsSelectionsAndActiveReaders(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "cache", "one")}
	now := time.Now()
	clipboard := &clipboardSync{copies: map[string][]string{"cache": paths}, pending: map[string]bool{}, readers: make(map[string]selectedFiles)}
	if len(clipboard.collectCandidates(paths, now)) != 0 {
		t.Fatal("OS clipboard file references were collected")
	}
	clipboard.pending["cache"] = true
	if len(clipboard.collectCandidates(nil, now)) != 0 {
		t.Fatal("in-flight receive was collected")
	}
	delete(clipboard.pending, "cache")
	clipboard.retainReader("selection", paths, now.Add(time.Minute))
	if len(clipboard.collectCandidates(nil, now)) != 0 {
		t.Fatal("unconsumed file selection was collected after clipboard changed")
	}
	clipboard.retainReader("upload", paths, time.Time{})
	if len(clipboard.collectCandidates(nil, now.Add(2*time.Minute))) != 0 {
		t.Fatal("active upload expired with its original file selection")
	}
	if _, retained := clipboard.readers["selection"]; retained {
		t.Fatal("expired selection lease remained live")
	}
	clipboard.releaseReader("upload")
	if ids := clipboard.collectCandidates(nil, now); len(ids) != 1 || ids[0] != "cache" {
		t.Fatal("unreferenced cache did not become collectable")
	}
}

func TestClipboardCollectionRechecksLateReferencesBeforeDeletion(t *testing.T) {
	paths := []string{filepath.Join(t.TempDir(), "cache", "one")}
	now := time.Now()
	for _, reference := range []string{"selection", "upload", "pending", "receive", "clipboard"} {
		t.Run(reference, func(t *testing.T) {
			clipboard := &clipboardSync{copies: map[string][]string{"cache": paths}, pending: map[string]bool{}, readers: make(map[string]selectedFiles)}
			candidates := clipboard.collectCandidates(nil, now)
			if len(candidates) != 1 {
				t.Fatal("fixture did not first observe an unused cache")
			}
			var selected []string
			switch reference {
			case "selection":
				clipboard.retainReader("selection", paths, now.Add(time.Minute))
			case "upload":
				clipboard.retainReader("upload", paths, time.Time{})
			case "pending":
				clipboard.pending["cache"] = true
			case "receive":
				clipboard.receiving = 1
			case "clipboard":
				selected = paths
			}
			// A nil backend makes an accidental deletion fail at the real boundary.
			clipboard.collectUnused(context.Background(), func() (clipboardSnapshot, error) { return clipboardSnapshot{paths: selected}, nil })
			if len(clipboard.copies) != 1 {
				t.Fatal("a reference acquired after candidate collection was deleted")
			}
		})
	}
}

func TestClipboardCollectionKeepsOwnershipWhileTakingSnapshot(t *testing.T) {
	clipboard := &clipboardSync{copies: map[string][]string{"cache": {filepath.Join(t.TempDir(), "cache", "one")}}, pending: map[string]bool{}, readers: map[string]selectedFiles{}}
	clipboard.collectUnused(context.Background(), func() (clipboardSnapshot, error) {
		if clipboard.mu.TryLock() {
			clipboard.mu.Unlock()
			t.Fatal("clipboard snapshot was taken outside the reader/deletion critical section")
		}
		return clipboardSnapshot{}, errors.New("controlled unreadable clipboard")
	})
	if len(clipboard.copies) != 1 {
		t.Fatal("an unreadable clipboard released its owned files")
	}
}

func TestClipboardReaderPromotionIsAtomicWithExpiry(t *testing.T) {
	now := time.Now()
	for _, expiry := range []time.Time{now.Add(time.Second), now, now.Add(-time.Second), {}} {
		clipboard := &clipboardSync{readers: map[string]selectedFiles{"selection": {paths: []string{"fixture"}, expires: expiry}}}
		want := expiry.IsZero() || expiry.After(now)
		if clipboard.promoteReader("selection", now) != want {
			t.Fatal("reader expiry changed during promotion")
		}
		reader, exists := clipboard.readers["selection"]
		if exists != want || exists && !reader.expires.IsZero() {
			t.Fatal("reader was not promoted or expired atomically")
		}
	}
	clipboard := &clipboardSync{readers: map[string]selectedFiles{}}
	if clipboard.promoteReader("collected", now) {
		t.Fatal("already collected reader was resurrected")
	}
}

func TestClipboardCacheDeletionAndReceivePreserveBackendResults(t *testing.T) {
	t.Setenv("EDEN_DESKTOP_DRIVER_FIXTURE", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"success", "failure", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("EDEN_DESKTOP_DRIVER_MODE", mode)
			root := t.TempDir()
			backend, err := NewBackend(executable, filepath.Join(root, "config.json"))
			if err != nil {
				t.Fatal(err)
			}
			backend.Directory = root
			paths := []string{filepath.Join(root, "cache", "one")}
			clipboard := &clipboardSync{backend: backend, copies: map[string][]string{"cache": paths}, pending: map[string]bool{}, readers: map[string]selectedFiles{}, loaded: true}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancel" {
				cancel()
			}
			clipboard.collectUnused(ctx, func() (clipboardSnapshot, error) { return clipboardSnapshot{}, nil })
			_, retained := clipboard.copies["cache"]
			if retained != (mode != "success") {
				t.Fatal("cache ownership no longer matches the backend deletion result")
			}
			response, err := clipboard.receive(ctx, "local", "fixture-files")
			if (err == nil) != (mode == "success") || clipboard.receiving != 0 {
				t.Fatal("receive lost its error or retained an active receive", err)
			}
			if mode == "success" {
				if len(response.Paths) == 0 || !clipboard.pending[response.CacheID] || len(clipboard.collectCandidates(nil, time.Now())) != 0 {
					t.Fatal("newly received cache was not pinned before receive ended")
				}
			} else if clipboard.loaded {
				t.Fatal("failed receive did not invalidate the cache listing")
			}
		})
	}
}
