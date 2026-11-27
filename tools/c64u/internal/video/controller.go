//go:build darwin || windows || linux

package video

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/inputsync"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/petscii"
	"github.com/hajimehoshi/ebiten/v2"
)

// resetConfirmWindow is how long the second reset hotkey press has.
const resetConfirmWindow = time.Second

// closeTimeout bounds the release_all sent when the window closes.
const closeTimeout = 500 * time.Millisecond

// syncer is the part of inputsync.Syncer the controller uses.
type syncer interface {
	Update(inputsync.State)
	ReleaseAll()
	Close(time.Duration)
	Err() error
	Unsupported() bool
}

// tickInput is everything the window read from the host in one frame.
type tickInput struct {
	Frame        hostFrame
	Focused      bool
	Pads         map[ebiten.GamepadID]padFrame
	CursorX      int
	CursorY      int
	WheelX       float64
	WheelY       float64
	MouseButtons inputsync.Set
	Now          time.Time
}

// controller turns host input into the C64's input state, frame by frame, and
// handles the window hotkeys. It holds no Ebiten window, so it can be tested.
type controller struct {
	sync    syncer
	dev     Device
	goos    string
	keyName func(ebiten.Key) string

	mode       keyboardMode
	keyJoy     bool
	swapped    bool
	mouse      bool
	tracker    mouseTracker
	focused    bool
	resetArmed time.Time
	now        time.Time
	pads       []int // ports of the connected pads, for the title
	// swallowed are keys that were down while the hotkey modifier was; they
	// stay away from the C64 until released, so letting go of the modifier
	// first does not type the hotkey's letter.
	swallowed map[ebiten.Key]bool

	buffer    chan []byte // keyboard buffer writes on old firmware
	closeOnce sync.Once
}

func newController(s syncer, dev Device, goos string, keyName func(ebiten.Key) string) *controller {
	c := &controller{
		sync:      s,
		dev:       dev,
		goos:      goos,
		keyName:   keyName,
		focused:   true,
		buffer:    make(chan []byte, 64),
		swallowed: map[ebiten.Key]bool{},
	}
	go func() {
		for b := range c.buffer {
			c.dev.SendKeyBytes(b, 0) //nolint:errcheck
		}
	}()
	return c
}

func (c *controller) tick(in tickInput) {
	c.now = in.Now
	if !in.Focused {
		if c.focused {
			c.sync.ReleaseAll()
			c.mouse = false
		}
		c.focused = false
		return
	}
	c.focused = true

	action, mod := detectHotkey(in.Frame, c.keyName, c.goos)
	c.hotkey(action)
	frame := c.unswallowed(in.Frame, mod)

	if c.sync.Unsupported() {
		c.bufferKeys(frame)
		return
	}
	mouseHeld := c.mouse && len(in.MouseButtons) > 0
	keys, taps := translateKeys(frame, keyOptions{
		Mode:           c.mode,
		JoystickKeys:   c.keyJoy,
		AltGrFakesCtrl: c.goos == "windows",
		ExtraHeld:      mouseHeld,
	})
	st := inputsync.State{Keys: keys, Taps: taps, Joy: [3]inputsync.Set{1: {}, 2: {}}}

	ids := make([]ebiten.GamepadID, 0, len(in.Pads))
	for id := range in.Pads {
		ids = append(ids, id)
	}
	c.pads = c.pads[:0]
	for id, port := range assignPorts(ids, c.swapped) {
		joy, extra := padJoystick(in.Pads[id])
		merge(st.Joy[port], joy)
		merge(st.Keys, extra)
		c.pads = append(c.pads, port)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(c.pads)))
	if c.keyJoy {
		merge(st.Joy[c.keyJoyPort()], keyJoystick(frame))
	}

	if c.mouse {
		st.DX, st.DY, st.WheelV, st.WheelH = c.tracker.step(in.CursorX, in.CursorY, in.WheelX, in.WheelY)
		st.Buttons = in.MouseButtons.Clone()
		// The mouse sits on port 1; a joystick there would fight it.
		st.Joy[1] = inputsync.Set{}
	}
	c.sync.Update(st)
}

// unswallowed returns the frame without the keys that belong to a hotkey.
func (c *controller) unswallowed(f hostFrame, mod bool) hostFrame {
	if mod {
		for k, down := range f.Pressed {
			if down {
				c.swallowed[k] = true
			}
		}
	}
	for k := range c.swallowed {
		if !f.Pressed[k] {
			delete(c.swallowed, k)
		}
	}
	if len(c.swallowed) == 0 {
		return f
	}
	out := hostFrame{Pressed: map[ebiten.Key]bool{}, JustPressed: map[ebiten.Key]bool{}}
	for k, down := range f.Pressed {
		if down && !c.swallowed[k] {
			out.Pressed[k] = true
		}
	}
	for k, down := range f.JustPressed {
		if down && !c.swallowed[k] {
			out.JustPressed[k] = true
		}
	}
	// Characters cannot be told apart by key: drop them while a swallowed
	// key is still down, or its auto-repeat would type it.
	return out
}

func (c *controller) hotkey(action hotkeyAction) {
	switch action {
	case hkMode:
		if c.mode == modeCharacter {
			c.mode = modePosition
		} else {
			c.mode = modeCharacter
		}
	case hkJoystick:
		c.keyJoy = !c.keyJoy
	case hkSwapPorts:
		c.swapped = !c.swapped
	case hkMouse:
		c.mouse = !c.mouse
		c.tracker.reset()
	case hkMenu:
		go c.dev.MachineMenuButton() //nolint:errcheck
	case hkReset:
		if c.resetPending() {
			c.resetArmed = time.Time{}
			go c.dev.MachineReset() //nolint:errcheck
		} else {
			c.resetArmed = c.now
		}
	}
}

func (c *controller) resetPending() bool {
	return !c.resetArmed.IsZero() && c.now.Sub(c.resetArmed) <= resetConfirmWindow
}

func (c *controller) keyJoyPort() int {
	if c.swapped {
		return 1
	}
	return 2
}

// bufferKeys is the input path for firmware without machine:input: typed
// characters and special keys go into the KERNAL keyboard buffer.
func (c *controller) bufferKeys(f hostFrame) {
	send := func(b []byte) {
		select {
		case c.buffer <- b:
		default: // the device is behind; drop rather than block the frame
		}
	}
	if f.Pressed[ebiten.KeyAltLeft] || f.Pressed[ebiten.KeyAltRight] {
		for _, m := range cbmMap {
			if f.JustPressed[m.key] {
				send([]byte{m.petscii})
			}
		}
		return
	}
	for _, m := range keyMap {
		if f.JustPressed[m.key] {
			send([]byte{m.petscii})
		}
	}
	for _, r := range f.Chars {
		if r < 0x80 {
			if encoded, err := petscii.Encode(string(r)); err == nil {
				send(encoded)
			}
		}
	}
}

func (c *controller) captureMouse() bool { return c.mouse }

func (c *controller) title() string {
	if c.sync.Unsupported() {
		return "C64 Ultimate — keyboard buffer mode (firmware older than 1.2.0)"
	}
	if c.resetPending() {
		return fmt.Sprintf("C64 Ultimate — Press %s+R again to RESET", hotkeyModifier(c.goos))
	}

	keys := "character"
	if c.mode == modePosition {
		keys = "position"
	}
	var joy []string
	for _, port := range c.pads {
		joy = append(joy, fmt.Sprintf("pad→%d", port))
	}
	if len(joy) == 0 {
		joy = append(joy, "no pad")
	}
	if c.keyJoy {
		joy = append(joy, fmt.Sprintf("keys →%d", c.keyJoyPort()))
	} else {
		joy = append(joy, "keys off")
	}
	mouse := "free"
	if c.mouse {
		mouse = "captured"
	}
	title := fmt.Sprintf("C64 Ultimate — Keys: %s | Joy: %s | Mouse: %s", keys, strings.Join(joy, ", "), mouse)
	if err := c.sync.Err(); err != nil {
		title += " | input error: " + err.Error()
	}
	return title
}

// close releases every injected input and stops the buffer writer.
func (c *controller) close() {
	c.closeOnce.Do(func() {
		c.sync.Close(closeTimeout)
		close(c.buffer)
	})
}

// hotkeyHelp lists the window hotkeys for the console.
func hotkeyHelp(goos string) string {
	mod := hotkeyModifier(goos)
	lines := []string{
		"Window hotkeys:",
		mod + "+K  switch keyboard mode (character / position)",
		mod + "+J  keyboard joystick on/off (arrows, left Ctrl = fire)",
		mod + "+P  swap joystick ports",
		mod + "+L  capture / release the mouse (port 1)",
		mod + "+U  open / close the Ultimate menu",
		mod + "+R  reset the C64 (press twice within a second)",
		"Left Alt/Option is the C= key; right Alt/Option (AltGr) types your layout's symbols.",
		"Esc is RUN/STOP, Page Up is RESTORE.",
	}
	return strings.Join(lines, "\n")
}

func merge(dst, src inputsync.Set) {
	for k, v := range src {
		if v {
			dst[k] = true
		}
	}
}
