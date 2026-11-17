package desktop

import (
	"math"
	"strconv"
	"unicode/utf16"

	"github.com/google/uuid"
)

type inputState struct {
	owner     string
	composing bool
	resetting bool
	lastKey   uint32
	scan      uint32
	extended  bool
	repeat    bool
	high      uint16
	publish   func(InputEvent)
	modifiers func() (ctrl, alt, shift, meta, caps bool)
}

func validInput(request RemoteInputRequest) bool {
	if _, err := uuid.Parse(request.Owner); err != nil {
		return false
	}
	switch request.Operation {
	case "activate", "caret":
		return !math.IsNaN(request.X) && !math.IsInf(request.X, 0) && !math.IsNaN(request.Y) && !math.IsInf(request.Y, 0) && !math.IsNaN(request.Height) && !math.IsInf(request.Height, 0) && math.Abs(request.X) <= 32768 && math.Abs(request.Y) <= 32768 && request.Height > 0 && request.Height <= 4096
	case "cancel", "deactivate":
		return true
	}
	return false
}

func (s *inputState) emit(kind, text string, start, end int) {
	if s.owner != "" {
		s.publish(InputEvent{Owner: s.owner, Kind: kind, Text: text, SelectionStart: start, SelectionEnd: end})
	}
}

func (s *inputState) readKey(key uint32, data uintptr) {
	s.lastKey = key
	s.scan = uint32(data>>16) & 255
	s.extended = data&(1<<24) != 0
	s.repeat = data&(1<<30) != 0
}

func commandKey(key uint32) bool {
	return (key >= 0x70 && key <= 0x87) || (key >= 0x21 && key <= 0x28) || (key >= 0x10 && key <= 0x14) || key == 0x0d || key == 0x09 || key == 0x08 || key == 0x2e || key == 0x1b || key == 0x5b || key == 0x5c || key == 0x2d || key == 0x2c || key == 0x91 || key == 0x90 || key == 0x5d || key == 0x0c
}

func keyPair(key uint32) (string, string) {
	if key >= 0x41 && key <= 0x5a {
		return string(rune(key + 32)), "Key" + string(rune(key))
	}
	if key >= 0x30 && key <= 0x39 {
		return string(rune(key)), "Digit" + string(rune(key))
	}
	if key >= 0x60 && key <= 0x69 {
		return strconv.Itoa(int(key - 0x60)), "Numpad" + strconv.Itoa(int(key-0x60))
	}
	if key >= 0x70 && key <= 0x87 {
		value := "F" + strconv.Itoa(int(key-0x6f))
		return value, value
	}
	pairs := map[uint32][2]string{
		13: {"Enter", "Enter"}, 9: {"Tab", "Tab"}, 8: {"Backspace", "Backspace"}, 46: {"Delete", "Delete"}, 27: {"Escape", "Escape"},
		37: {"ArrowLeft", "ArrowLeft"}, 39: {"ArrowRight", "ArrowRight"}, 38: {"ArrowUp", "ArrowUp"}, 40: {"ArrowDown", "ArrowDown"},
		16: {"Shift", "ShiftLeft"}, 17: {"Control", "ControlLeft"}, 18: {"Alt", "AltLeft"}, 91: {"Meta", "MetaLeft"}, 92: {"Meta", "MetaRight"}, 20: {"CapsLock", "CapsLock"},
		36: {"Home", "Home"}, 35: {"End", "End"}, 33: {"PageUp", "PageUp"}, 34: {"PageDown", "PageDown"}, 32: {" ", "Space"}, 45: {"Insert", "Insert"}, 44: {"PrintScreen", "PrintScreen"},
		145: {"ScrollLock", "ScrollLock"}, 19: {"Pause", "Pause"}, 144: {"NumLock", "NumLock"}, 93: {"ContextMenu", "ContextMenu"}, 12: {"Clear", "Numpad5"},
		186: {";", "Semicolon"}, 187: {"=", "Equal"}, 188: {",", "Comma"}, 189: {"-", "Minus"}, 190: {".", "Period"}, 191: {"/", "Slash"}, 192: {"`", "Backquote"},
		219: {"[", "BracketLeft"}, 220: {"\\", "Backslash"}, 221: {"]", "BracketRight"}, 222: {"'", "Quote"}, 226: {"\\", "IntlBackslash"},
		106: {"*", "NumpadMultiply"}, 107: {"+", "NumpadAdd"}, 109: {"-", "NumpadSubtract"}, 110: {".", "NumpadDecimal"}, 111: {"/", "NumpadDivide"}, 108: {",", "NumpadComma"},
	}
	if pair, ok := pairs[key]; ok {
		return pair[0], pair[1]
	}
	return "Unidentified", "Unidentified"
}

func (s *inputState) key(key uint32, down bool, text string) {
	if s.owner == "" {
		return
	}
	value, code := keyPair(key)
	if text != "" {
		value = text
	}
	switch {
	case key == 13 && s.extended:
		code = "NumpadEnter"
	case key == 16 && s.scan == 0x36:
		code = "ShiftRight"
	case key == 17 && s.extended:
		code = "ControlRight"
	case key == 18 && s.extended:
		code = "AltRight"
	}
	if !s.extended && (key >= 33 && key <= 40 || key == 12 || key == 45 || key == 46) {
		if pad := map[uint32]string{0x47: "Numpad7", 0x48: "Numpad8", 0x49: "Numpad9", 0x4b: "Numpad4", 0x4c: "Numpad5", 0x4d: "Numpad6", 0x4f: "Numpad1", 0x50: "Numpad2", 0x51: "Numpad3", 0x52: "Numpad0", 0x53: "NumpadDecimal"}[s.scan]; pad != "" {
			code = pad
		}
	}
	ctrl, alt, shift, meta, caps := s.modifiers()
	kind := "keyup"
	if down {
		kind = "keydown"
	}
	s.publish(InputEvent{Owner: s.owner, Kind: kind, Key: value, Code: code, TextKey: text != "", Repeat: s.repeat, CtrlKey: ctrl, AltKey: alt, ShiftKey: shift, MetaKey: meta, ModifierCapsLock: caps})
}

func (s *inputState) character(unit uint16) {
	if s.composing || s.resetting {
		return
	}
	if unit >= 0xd800 && unit <= 0xdbff {
		s.high = unit
		return
	}
	if unit >= 0xdc00 && unit <= 0xdfff && s.high != 0 {
		s.emit("input", string(utf16.DecodeRune(rune(s.high), rune(unit))), 0, 0)
		s.high = 0
		return
	}
	s.high = 0
	if unit >= 32 && unit != 127 {
		if unit <= 127 {
			s.key(s.lastKey, true, string(rune(unit)))
		} else {
			s.emit("input", string(rune(unit)), 0, 0)
		}
	}
}
