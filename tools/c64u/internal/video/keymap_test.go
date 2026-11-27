//go:build darwin || windows || linux

package video

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/inputsync"
	"github.com/hajimehoshi/ebiten/v2"
)

func pressed(keys ...ebiten.Key) map[ebiten.Key]bool {
	m := map[ebiten.Key]bool{}
	for _, k := range keys {
		m[k] = true
	}
	return m
}

func ktap(names ...string) inputsync.Tap {
	return inputsync.Tap{Kind: "keyboard", Inputs: names}
}

func checkKeys(t *testing.T, f hostFrame, o keyOptions, wantHeld inputsync.Set, wantTaps []inputsync.Tap) {
	t.Helper()
	held, taps := translateKeys(f, o)
	if len(held) == 0 && len(wantHeld) == 0 {
		held, wantHeld = nil, nil
	}
	if !reflect.DeepEqual(held, wantHeld) {
		t.Errorf("held = %v, want %v", held, wantHeld)
	}
	if !reflect.DeepEqual(taps, wantTaps) {
		t.Errorf("taps = %v, want %v", taps, wantTaps)
	}
}

func TestKeymapNamesAreValid(t *testing.T) {
	check := func(table string, names []string) {
		event := "press:" + strings.Join(names, "+")
		if len(names) == 1 && names[0] == "restore" {
			event = "tap:restore"
		}
		if _, err := api.ParseInputEvent(event); err != nil {
			t.Errorf("%s: %v", table, err)
		}
	}
	for _, names := range controlKeys {
		check("controlKeys", names)
	}
	for _, name := range positionKeys {
		check("positionKeys", []string{name})
	}
	for r, names := range charKeys {
		check("charKeys "+string(r), names)
	}
}

func TestCharMode_QuoteIsShift2(t *testing.T) {
	f := hostFrame{Pressed: pressed(ebiten.KeyShiftLeft, ebiten.KeyDigit2), Chars: []rune{'"'}}
	checkKeys(t, f, keyOptions{}, nil, []inputsync.Tap{ktap("left_shift", "2")})
}

func TestCharMode_StarNotShifted(t *testing.T) {
	// German layout: * is Shift and the key right of Ü
	f := hostFrame{Pressed: pressed(ebiten.KeyShiftLeft, ebiten.KeyBracketRight), Chars: []rune{'*'}}
	checkKeys(t, f, keyOptions{}, nil, []inputsync.Tap{ktap("star")})
}

func TestCharMode_DeadKeySendsNothing(t *testing.T) {
	f := hostFrame{Pressed: pressed(ebiten.KeyBackquote)}
	checkKeys(t, f, keyOptions{}, nil, nil)
}

func TestCharMode_ShiftWithCursor(t *testing.T) {
	f := hostFrame{Pressed: pressed(ebiten.KeyShiftLeft, ebiten.KeyArrowDown)}
	checkKeys(t, f, keyOptions{}, inputsync.Set{"left_shift": true, "cursor_up_down": true}, nil)
}

func TestCharMode_ShiftWithMouseButton(t *testing.T) {
	f := hostFrame{Pressed: pressed(ebiten.KeyShiftLeft)}
	checkKeys(t, f, keyOptions{ExtraHeld: true}, inputsync.Set{"left_shift": true}, nil)
	checkKeys(t, f, keyOptions{}, nil, nil)
}

func TestCharMode_LeftAltIsCommodore(t *testing.T) {
	f := hostFrame{Pressed: pressed(ebiten.KeyAltLeft, ebiten.KeyDigit1), Chars: []rune{'¡'}}
	checkKeys(t, f, keyOptions{}, inputsync.Set{"commodore": true, "1": true}, nil)
}

func TestCharMode_CtrlCombination(t *testing.T) {
	f := hostFrame{Pressed: pressed(ebiten.KeyControlLeft, ebiten.KeyDigit9)}
	checkKeys(t, f, keyOptions{}, inputsync.Set{"ctrl": true, "9": true}, nil)
}

func TestCharMode_AltGrGivesLayoutChar(t *testing.T) {
	f := hostFrame{Pressed: pressed(ebiten.KeyAltRight, ebiten.KeyL), Chars: []rune{'@'}}
	checkKeys(t, f, keyOptions{}, nil, []inputsync.Tap{ktap("at")})
}

func TestCharMode_WindowsAltGrCtrlIgnored(t *testing.T) {
	f := hostFrame{Pressed: pressed(ebiten.KeyAltRight, ebiten.KeyControlLeft, ebiten.KeyQ), Chars: []rune{'@'}}
	checkKeys(t, f, keyOptions{AltGrFakesCtrl: true}, nil, []inputsync.Tap{ktap("at")})
}

func TestCharMode_UnknownCharIgnored(t *testing.T) {
	f := hostFrame{Pressed: pressed(ebiten.KeySemicolon), Chars: []rune{'ö'}}
	checkKeys(t, f, keyOptions{}, nil, nil)
}

func TestPositionMode_PhysicalKeys(t *testing.T) {
	// AZERTY: the key labelled A sits where US keyboards have Q
	f := hostFrame{Pressed: pressed(ebiten.KeyQ, ebiten.KeyShiftLeft), Chars: []rune{'A'}}
	checkKeys(t, f, keyOptions{Mode: modePosition}, inputsync.Set{"q": true, "left_shift": true}, nil)
}

func TestRestoreOnlyAsTap(t *testing.T) {
	f := hostFrame{Pressed: pressed(ebiten.KeyPageUp), JustPressed: pressed(ebiten.KeyPageUp)}
	checkKeys(t, f, keyOptions{}, nil, []inputsync.Tap{ktap("restore")})
	f.JustPressed = nil
	checkKeys(t, f, keyOptions{}, nil, nil)
}

func TestJoystickKeysExcluded(t *testing.T) {
	f := hostFrame{Pressed: pressed(ebiten.KeyArrowUp, ebiten.KeyControlLeft)}
	checkKeys(t, f, keyOptions{JoystickKeys: true}, nil, nil)
}
