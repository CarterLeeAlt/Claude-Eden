package desktop

import "testing"

func TestInputKeyboardSemantics(t *testing.T) {
	for _, caps := range []bool{false, true} {
		for _, shift := range []bool{false, true} {
			var events []InputEvent
			state := inputState{owner: "owner", publish: func(value InputEvent) { events = append(events, value) }, modifiers: func() (bool, bool, bool, bool, bool) { return false, false, shift, false, caps }}
			state.readKey(0x41, 0x1e<<16)
			character := 'a'
			if caps != shift {
				character = 'A'
			}
			state.character(uint16(character))
			state.key(0x41, false, "")
			if len(events) != 2 || events[0].Key != string(character) || events[0].Code != "KeyA" || !events[0].TextKey || events[1].Key != "a" || events[1].Kind != "keyup" {
				t.Fatalf("printable down/up changed: %+v", events)
			}
			for _, event := range events {
				if event.ModifierCapsLock != caps || event.ShiftKey != shift || event.CtrlKey || event.AltKey || event.MetaKey {
					t.Fatalf("modifier state changed: %+v", event)
				}
			}
			for _, key := range []uint32{20, 37, 16, 13, 9, 0x87} {
				events = nil
				state.key(key, true, "")
				state.key(key, false, "")
				if len(events) != 2 || events[0].Kind != "keydown" || events[0].TextKey || events[1].Kind != "keyup" || !commandKey(key) {
					t.Fatalf("command down/up changed for %d: %+v", key, events)
				}
			}
		}
	}
}

func TestInputPhysicalKeysAndUnicode(t *testing.T) {
	var events []InputEvent
	state := inputState{owner: "owner", publish: func(event InputEvent) { events = append(events, event) }, modifiers: func() (bool, bool, bool, bool, bool) { return true, true, true, true, true }}
	for _, test := range []struct {
		key      uint32
		scan     uint32
		extended bool
		code     string
	}{
		{13, 0x1c, true, "NumpadEnter"}, {16, 0x36, false, "ShiftRight"}, {17, 0x1d, true, "ControlRight"}, {18, 0x38, true, "AltRight"},
		{37, 0x4b, false, "Numpad4"}, {37, 0x4b, true, "ArrowLeft"}, {45, 0x52, false, "Numpad0"}, {46, 0x53, false, "NumpadDecimal"},
	} {
		data := uintptr(test.scan)<<16 | 1<<30
		if test.extended {
			data |= 1 << 24
		}
		state.readKey(test.key, data)
		state.key(test.key, true, "")
		event := events[len(events)-1]
		if event.Code != test.code || !event.Repeat || !event.CtrlKey || !event.AltKey || !event.ShiftKey || !event.MetaKey || !event.ModifierCapsLock {
			t.Fatalf("physical key changed: %+v", event)
		}
	}
	events = nil
	state.character(0xd83d)
	state.character(0xde00)
	state.character('中')
	if len(events) != 2 || events[0].Kind != "input" || events[0].Text != "😀" || events[1].Text != "中" {
		t.Fatalf("unicode damaged: %+v", events)
	}
	events = nil
	state.composing = true
	state.character('a')
	state.resetting = true
	state.composing = false
	state.character('a')
	state.owner = ""
	state.key(13, true, "")
	if len(events) != 0 {
		t.Fatal("composition/reset/unowned input leaked")
	}
}
