//go:build darwin || windows || (linux && cgo)

package video

import (
	"strings"
	"unicode"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/inputsync"
	"github.com/hajimehoshi/ebiten/v2"
)

// keyboardMode decides how host keys become C64 keys.
type keyboardMode int

const (
	// modeCharacter sends what the host layout types: " on a German keyboard
	// becomes Shift+2 on the C64.
	modeCharacter keyboardMode = iota
	// modePosition sends the C64 key in the same place, whatever its label.
	modePosition
)

// hostFrame is what the host keyboard did in one frame. Ebiten names keys by
// their US position; Chars are the characters the OS layout produced.
type hostFrame struct {
	Pressed     map[ebiten.Key]bool
	JustPressed map[ebiten.Key]bool
	Chars       []rune
}

type keyOptions struct {
	Mode keyboardMode
	// JoystickKeys hands the arrows and left Ctrl to the keyboard joystick.
	JoystickKeys bool
	// AltGrFakesCtrl: Windows reports a left Ctrl along with AltGr.
	AltGrFakesCtrl bool
	// ExtraHeld: a mouse button is down, so a held Shift belongs to it.
	ExtraHeld bool
}

// controlKeys are held as long as the host key is, in both modes. The right
// Alt is missing on purpose: it is AltGr, which the layout needs for @ [ ] and
// the like.
var controlKeys = map[ebiten.Key][]string{
	ebiten.KeyEnter:        {"return"},
	ebiten.KeyNumpadEnter:  {"return"},
	ebiten.KeySpace:        {"space"},
	ebiten.KeyBackspace:    {"inst_del"},
	ebiten.KeyDelete:       {"inst_del"},
	ebiten.KeyHome:         {"clr_home"},
	ebiten.KeyShiftLeft:    {"left_shift"},
	ebiten.KeyShiftRight:   {"right_shift"},
	ebiten.KeyControlLeft:  {"ctrl"},
	ebiten.KeyControlRight: {"ctrl"},
	ebiten.KeyTab:          {"ctrl"},
	ebiten.KeyAltLeft:      {"commodore"},
	ebiten.KeyEscape:       {"run_stop"},
	ebiten.KeyArrowRight:   {"cursor_left_right"},
	ebiten.KeyArrowDown:    {"cursor_up_down"},
	ebiten.KeyArrowLeft:    {"left_shift", "cursor_left_right"},
	ebiten.KeyArrowUp:      {"left_shift", "cursor_up_down"},
	ebiten.KeyF1:           {"f1"},
	ebiten.KeyF3:           {"f3"},
	ebiten.KeyF5:           {"f5"},
	ebiten.KeyF7:           {"f7"},
	ebiten.KeyF2:           {"left_shift", "f1"},
	ebiten.KeyF4:           {"left_shift", "f3"},
	ebiten.KeyF6:           {"left_shift", "f5"},
	ebiten.KeyF8:           {"left_shift", "f7"},
}

// positionKeys maps a host key to the C64 key in roughly the same place.
var positionKeys = map[ebiten.Key]string{
	ebiten.KeyMinus:        "plus",
	ebiten.KeyEqual:        "minus",
	ebiten.KeyBackquote:    "arrow_left",
	ebiten.KeyBracketLeft:  "at",
	ebiten.KeyBracketRight: "star",
	ebiten.KeyBackslash:    "pound",
	ebiten.KeySemicolon:    "colon",
	ebiten.KeyQuote:        "semicolon",
	ebiten.KeyComma:        "comma",
	ebiten.KeyPeriod:       "period",
	ebiten.KeySlash:        "slash",
	ebiten.KeyEnd:          "equals",
	ebiten.KeyPageDown:     "arrow_up",
}

// charKeys maps a typed character to the C64 keys that produce it.
var charKeys = map[rune][]string{
	'+': {"plus"}, '-': {"minus"}, '*': {"star"}, '/': {"slash"}, '=': {"equals"},
	',': {"comma"}, '.': {"period"}, ':': {"colon"}, ';': {"semicolon"},
	'@': {"at"}, '£': {"pound"},
	'<': {"left_shift", "comma"}, '>': {"left_shift", "period"}, '?': {"left_shift", "slash"},
	'[': {"left_shift", "colon"}, ']': {"left_shift", "semicolon"},
	'^': {"arrow_up"}, '↑': {"arrow_up"}, '_': {"arrow_left"}, '←': {"arrow_left"},
	'π': {"left_shift", "arrow_up"},
}

func init() {
	letters := []ebiten.Key{
		ebiten.KeyA, ebiten.KeyB, ebiten.KeyC, ebiten.KeyD, ebiten.KeyE, ebiten.KeyF, ebiten.KeyG,
		ebiten.KeyH, ebiten.KeyI, ebiten.KeyJ, ebiten.KeyK, ebiten.KeyL, ebiten.KeyM, ebiten.KeyN,
		ebiten.KeyO, ebiten.KeyP, ebiten.KeyQ, ebiten.KeyR, ebiten.KeyS, ebiten.KeyT, ebiten.KeyU,
		ebiten.KeyV, ebiten.KeyW, ebiten.KeyX, ebiten.KeyY, ebiten.KeyZ,
	}
	for i, key := range letters {
		name := string(rune('a' + i))
		positionKeys[key] = name
		charKeys[rune('a'+i)] = []string{name}
		charKeys[rune('A'+i)] = []string{"left_shift", name}
	}
	digits := []ebiten.Key{
		ebiten.KeyDigit0, ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4,
		ebiten.KeyDigit5, ebiten.KeyDigit6, ebiten.KeyDigit7, ebiten.KeyDigit8, ebiten.KeyDigit9,
	}
	for i, key := range digits {
		name := string(rune('0' + i))
		positionKeys[key] = name
		charKeys[rune('0'+i)] = []string{name}
	}
	// Shift+1..9 on the C64
	for i, r := range `!"#$%&'()` {
		charKeys[r] = []string{"left_shift", string(rune('1' + i))}
	}
}

func isCtrlKey(k ebiten.Key) bool {
	return k == ebiten.KeyControlLeft || k == ebiten.KeyControlRight
}

func isShiftKey(k ebiten.Key) bool {
	return k == ebiten.KeyShiftLeft || k == ebiten.KeyShiftRight
}

func isJoystickKey(k ebiten.Key) bool {
	switch k {
	case ebiten.KeyArrowUp, ebiten.KeyArrowDown, ebiten.KeyArrowLeft, ebiten.KeyArrowRight, ebiten.KeyControlLeft:
		return true
	}
	return false
}

// isAlnumName is a C64 letter or digit key.
func isAlnumName(name string) bool {
	return len(name) == 1 && strings.IndexFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) < 0
}

// translateKeys turns one frame of host keyboard input into the C64 keys to
// hold and the keystrokes to tap.
func translateKeys(f hostFrame, o keyOptions) (inputsync.Set, []inputsync.Tap) {
	held := inputsync.Set{}
	ignoreCtrl := o.AltGrFakesCtrl && f.Pressed[ebiten.KeyAltRight]

	var shifts []string
	other, combo := false, false
	for key, down := range f.Pressed {
		names, ok := controlKeys[key]
		if !down || !ok || (o.JoystickKeys && isJoystickKey(key)) || (ignoreCtrl && isCtrlKey(key)) {
			continue
		}
		if isShiftKey(key) {
			shifts = append(shifts, names...)
			continue
		}
		if key == ebiten.KeyAltLeft || key == ebiten.KeyTab || isCtrlKey(key) {
			combo = true
		}
		other = true
		for _, n := range names {
			held[n] = true
		}
	}
	// Typed characters bring their own Shift; the host's goes only with keys
	// that type nothing, or it would turn * into Shift+* on the C64.
	if o.Mode == modePosition || other || o.ExtraHeld {
		for _, n := range shifts {
			held[n] = true
		}
	}

	var taps []inputsync.Tap
	if f.JustPressed[ebiten.KeyPageUp] {
		taps = append(taps, inputsync.Tap{Kind: "keyboard", Inputs: []string{"restore"}})
	}

	switch {
	case o.Mode == modePosition:
		for key, down := range f.Pressed {
			if name, ok := positionKeys[key]; down && ok {
				held[name] = true
			}
		}
	case combo:
		// C= and CTRL combinations: the OS would type ¡ or ø, so take the key.
		for key, down := range f.Pressed {
			if name, ok := positionKeys[key]; down && ok && isAlnumName(name) {
				held[name] = true
			}
		}
	default:
		for _, r := range f.Chars {
			if names, ok := charKeys[r]; ok {
				taps = append(taps, inputsync.Tap{Kind: "keyboard", Inputs: names})
			}
		}
	}
	return held, taps
}
