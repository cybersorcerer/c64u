//go:build darwin || windows || (linux && cgo)

package video

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func usNames(k ebiten.Key) string { return "" }

func TestHotkey_MacUsesCmd(t *testing.T) {
	f := hostFrame{Pressed: pressed(ebiten.KeyMetaLeft, ebiten.KeyK), JustPressed: pressed(ebiten.KeyK)}
	if a, mod := detectHotkey(f, usNames, "darwin"); a != hkMode || !mod {
		t.Errorf("got %v,%v, want hkMode,true", a, mod)
	}
	f = hostFrame{Pressed: pressed(ebiten.KeyMetaLeft)}
	if a, mod := detectHotkey(f, usNames, "darwin"); a != hkNone || !mod {
		t.Errorf("Cmd alone: got %v,%v, want hkNone,true", a, mod)
	}
}

func TestHotkey_AllLetters(t *testing.T) {
	want := map[ebiten.Key]hotkeyAction{
		ebiten.KeyK: hkMode, ebiten.KeyJ: hkJoystick, ebiten.KeyP: hkSwapPorts,
		ebiten.KeyL: hkMouse, ebiten.KeyU: hkMenu, ebiten.KeyR: hkReset,
	}
	for key, action := range want {
		f := hostFrame{Pressed: pressed(ebiten.KeyMetaRight, key), JustPressed: pressed(key)}
		if a, _ := detectHotkey(f, usNames, "darwin"); a != action {
			t.Errorf("Cmd+%v: got %v, want %v", key, a, action)
		}
	}
}

func TestHotkey_LayoutName(t *testing.T) {
	// Dvorak puts L where US has P, so the label must win over the position.
	dvorak := func(k ebiten.Key) string {
		if k == ebiten.KeyP {
			return "l"
		}
		return ""
	}
	f := hostFrame{Pressed: pressed(ebiten.KeyMetaLeft, ebiten.KeyP), JustPressed: pressed(ebiten.KeyP)}
	if a, _ := detectHotkey(f, dvorak, "darwin"); a != hkMouse {
		t.Errorf("got %v, want hkMouse from the layout name", a)
	}
}

func TestHotkey_OtherOSCtrlShift(t *testing.T) {
	f := hostFrame{Pressed: pressed(ebiten.KeyControlLeft, ebiten.KeyU), JustPressed: pressed(ebiten.KeyU)}
	if a, mod := detectHotkey(f, usNames, "linux"); a != hkNone || mod {
		t.Errorf("Ctrl alone: got %v,%v, want hkNone,false", a, mod)
	}
	f.Pressed[ebiten.KeyShiftLeft] = true
	if a, mod := detectHotkey(f, usNames, "linux"); a != hkMenu || !mod {
		t.Errorf("Ctrl+Shift+U: got %v,%v, want hkMenu,true", a, mod)
	}
	f = hostFrame{Pressed: pressed(ebiten.KeyMetaLeft, ebiten.KeyU), JustPressed: pressed(ebiten.KeyU)}
	if a, mod := detectHotkey(f, usNames, "windows"); a != hkNone || mod {
		t.Errorf("Windows key on Windows: got %v,%v, want no hotkey", a, mod)
	}
}

// Review finding: Windows reports a fake left Ctrl with AltGr, so AltGr+Shift
// must not count as the Ctrl+Shift modifier.
func TestHotkey_WindowsAltGrShiftIsNoModifier(t *testing.T) {
	f := hostFrame{Pressed: pressed(ebiten.KeyAltRight, ebiten.KeyControlLeft, ebiten.KeyShiftLeft, ebiten.KeyQ), JustPressed: pressed(ebiten.KeyQ)}
	if a, mod := detectHotkey(f, usNames, "windows"); a != hkNone || mod {
		t.Errorf("got %v,%v, want hkNone,false", a, mod)
	}
}

// The macOS window menu takes Cmd+H, Cmd+Q, Cmd+M and Ctrl+Cmd+F before the
// window sees them; Cmd+M minimized the window instead of capturing the mouse.
func TestHotkey_AvoidMacMenuKeys(t *testing.T) {
	for _, letter := range []string{"h", "q", "m", "f"} {
		if a, ok := hotkeyLetters[letter]; ok {
			t.Errorf("hotkey %v sits on Cmd+%s, which the macOS menu takes", a, letter)
		}
	}
}
