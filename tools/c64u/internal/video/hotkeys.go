//go:build darwin || windows || (linux && cgo)

package video

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// hotkeyAction is what a window hotkey asks for.
type hotkeyAction int

const (
	hkNone hotkeyAction = iota
	hkMode
	hkJoystick
	hkSwapPorts
	hkMouse
	hkMenu
	hkReset
)

var hotkeyLetters = map[string]hotkeyAction{
	"k": hkMode,
	"j": hkJoystick,
	"p": hkSwapPorts,
	"m": hkMouse,
	"u": hkMenu,
	"r": hkReset,
}

// hotkeyModifier names the modifier for the window title and help.
func hotkeyModifier(goos string) string {
	if goos == "darwin" {
		return "Cmd"
	}
	return "Ctrl+Shift"
}

// detectHotkey reports the hotkey pressed this frame, and whether the hotkey
// modifier is held at all; while it is, no keyboard input reaches the C64.
// The modifier is a key the C64 lacks: Cmd on macOS, Ctrl+Shift elsewhere,
// where the Windows key belongs to the system. Letters follow the label in the
// current layout, so Mod+M sits on the M of an AZERTY keyboard too.
func detectHotkey(f hostFrame, keyName func(ebiten.Key) string, goos string) (hotkeyAction, bool) {
	var held bool
	if goos == "darwin" {
		held = f.Pressed[ebiten.KeyMetaLeft] || f.Pressed[ebiten.KeyMetaRight]
	} else {
		held = (f.Pressed[ebiten.KeyControlLeft] || f.Pressed[ebiten.KeyControlRight]) &&
			(f.Pressed[ebiten.KeyShiftLeft] || f.Pressed[ebiten.KeyShiftRight])
	}
	if !held {
		return hkNone, false
	}
	for key, down := range f.JustPressed {
		if !down {
			continue
		}
		letter := strings.ToLower(keyName(key))
		if letter == "" {
			letter = positionKeys[key] // the US label
		}
		if action, ok := hotkeyLetters[letter]; ok {
			return action, true
		}
	}
	return hkNone, true
}
