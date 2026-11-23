//go:build darwin || windows || (linux && cgo)

package video

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/inputsync"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/petscii"
	"github.com/hajimehoshi/ebiten/v2"
)

type fakeSync struct {
	mu          sync.Mutex
	updates     []inputsync.State
	releaseAll  int
	closed      time.Duration
	unsupported bool
	err         error
}

func (f *fakeSync) Update(s inputsync.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, s)
}
func (f *fakeSync) ReleaseAll() { f.mu.Lock(); f.releaseAll++; f.mu.Unlock() }
func (f *fakeSync) Close(d time.Duration) {
	f.mu.Lock()
	f.closed = d
	f.mu.Unlock()
}
func (f *fakeSync) Err() error        { f.mu.Lock(); defer f.mu.Unlock(); return f.err }
func (f *fakeSync) Unsupported() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.unsupported }

func (f *fakeSync) last() inputsync.State {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.updates) == 0 {
		return inputsync.State{}
	}
	return f.updates[len(f.updates)-1]
}

type fakeDevice struct {
	mu          sync.Mutex
	keyBytes    [][]byte
	menu, reset int
}

func (d *fakeDevice) MachineInput([]api.InputEvent) (*api.Response, error) {
	return &api.Response{}, nil
}
func (d *fakeDevice) SendKeyBytes(b []byte, _ time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.keyBytes = append(d.keyBytes, b)
	return nil
}
func (d *fakeDevice) MachineMenuButton() (*api.Response, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.menu++
	return &api.Response{}, nil
}
func (d *fakeDevice) MachineReset() (*api.Response, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.reset++
	return &api.Response{}, nil
}
func (d *fakeDevice) count(n *int) int { d.mu.Lock(); defer d.mu.Unlock(); return *n }

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func newTestController(t *testing.T) (*controller, *fakeSync, *fakeDevice) {
	s, d := &fakeSync{}, &fakeDevice{}
	c := newController(s, d, "darwin", func(ebiten.Key) string { return "" })
	t.Cleanup(c.close)
	return c, s, d
}

var t0 = time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)

func cmd(key ebiten.Key, at time.Time) tickInput {
	return tickInput{Focused: true, Now: at, Frame: hostFrame{Pressed: pressed(ebiten.KeyMetaLeft, key), JustPressed: pressed(key)}}
}

func TestController_Title(t *testing.T) {
	c, s, _ := newTestController(t)
	c.tick(tickInput{Focused: true, Now: t0})
	if want := "C64 Ultimate — Keys: character | Joy: no pad, keys off | Mouse: free"; c.title() != want {
		t.Errorf("title = %q, want %q", c.title(), want)
	}
	s.mu.Lock()
	s.err = errors.New("connection refused")
	s.mu.Unlock()
	if !strings.HasSuffix(c.title(), " | input error: connection refused") {
		t.Errorf("title = %q, want the input error appended", c.title())
	}
}

func TestController_FocusLossReleasesAll(t *testing.T) {
	c, s, _ := newTestController(t)
	c.tick(tickInput{Focused: true, Now: t0, Frame: hostFrame{Pressed: pressed(ebiten.KeySpace)}})
	c.tick(tickInput{Focused: false, Now: t0})
	c.tick(tickInput{Focused: false, Now: t0})
	if s.releaseAll != 1 {
		t.Errorf("ReleaseAll called %d times, want 1", s.releaseAll)
	}
}

func TestController_ModeToggle(t *testing.T) {
	c, _, _ := newTestController(t)
	c.tick(cmd(ebiten.KeyK, t0))
	if !strings.Contains(c.title(), "Keys: position") {
		t.Errorf("title = %q, want position mode", c.title())
	}
	c.tick(cmd(ebiten.KeyJ, t0))
	if !strings.Contains(c.title(), "keys →2") {
		t.Errorf("title = %q, want the keyboard joystick on port 2", c.title())
	}
	c.tick(cmd(ebiten.KeyP, t0))
	if !strings.Contains(c.title(), "keys →1") {
		t.Errorf("title = %q, want the keyboard joystick on port 1 after the swap", c.title())
	}
}

func TestController_ResetNeedsSecondPress(t *testing.T) {
	c, _, d := newTestController(t)
	c.tick(cmd(ebiten.KeyR, t0))
	if !strings.Contains(c.title(), "Press Cmd+R again to RESET") {
		t.Errorf("title = %q, want the reset confirmation", c.title())
	}
	c.tick(cmd(ebiten.KeyR, t0.Add(1500*time.Millisecond)))
	time.Sleep(20 * time.Millisecond)
	if n := d.count(&d.reset); n != 0 {
		t.Fatalf("reset after a 1.5s gap, want none")
	}
	c.tick(cmd(ebiten.KeyR, t0.Add(2000*time.Millisecond)))
	waitFor(t, "the reset", func() bool { return d.count(&d.reset) == 1 })
}

func TestController_MenuHotkey(t *testing.T) {
	c, _, d := newTestController(t)
	c.tick(cmd(ebiten.KeyU, t0))
	waitFor(t, "the menu button", func() bool { return d.count(&d.menu) == 1 })
}

func TestController_ModifierSuppressesKeys(t *testing.T) {
	c, s, _ := newTestController(t)
	c.tick(tickInput{Focused: true, Now: t0, Frame: hostFrame{
		Pressed: pressed(ebiten.KeyMetaLeft, ebiten.KeySpace, ebiten.KeyA), JustPressed: pressed(ebiten.KeyA), Chars: []rune{'a'},
	}})
	if got := s.last(); len(got.Keys) != 0 || len(got.Taps) != 0 {
		t.Errorf("sent %v / %v while Cmd was held", got.Keys, got.Taps)
	}
}

func TestController_GamepadsAndMouseBeatsPort1(t *testing.T) {
	c, s, _ := newTestController(t)
	fire := padFrame{Buttons: map[ebiten.StandardGamepadButton]bool{ebiten.StandardGamepadButtonRightBottom: true}}
	pads := map[ebiten.GamepadID]padFrame{0: fire, 1: fire}

	c.tick(tickInput{Focused: true, Now: t0, Pads: pads})
	got := s.last()
	if !got.Joy[2]["fire"] || !got.Joy[1]["fire"] {
		t.Fatalf("Joy = %v, want fire on both ports", got.Joy)
	}

	c.tick(cmd(ebiten.KeyM, t0))
	if !c.captureMouse() {
		t.Fatal("Cmd+M did not capture the mouse")
	}
	c.tick(tickInput{Focused: true, Now: t0, Pads: pads, MouseButtons: inputsync.Set{"left": true}})
	got = s.last()
	if len(got.Joy[1]) != 0 {
		t.Errorf("Joy[1] = %v while the mouse is captured, want nothing", got.Joy[1])
	}
	if !got.Joy[2]["fire"] || !reflect.DeepEqual(got.Buttons, inputsync.Set{"left": true}) {
		t.Errorf("got Joy[2] %v, Buttons %v; want fire on port 2 and the left button", got.Joy[2], got.Buttons)
	}
}

func TestController_FocusLossFreesMouse(t *testing.T) {
	c, _, _ := newTestController(t)
	c.tick(cmd(ebiten.KeyM, t0))
	c.tick(tickInput{Focused: false, Now: t0})
	if c.captureMouse() {
		t.Error("the mouse stayed captured after the window lost focus")
	}
}

func TestController_FallbackOnUnsupported(t *testing.T) {
	c, s, d := newTestController(t)
	s.mu.Lock()
	s.unsupported = true
	s.mu.Unlock()
	c.tick(tickInput{Focused: true, Now: t0, Frame: hostFrame{Pressed: pressed(ebiten.KeyA), Chars: []rune{'A'}}})
	want, _ := petscii.Encode("A")
	waitFor(t, "the keyboard buffer write", func() bool {
		d.mu.Lock()
		defer d.mu.Unlock()
		return len(d.keyBytes) == 1 && reflect.DeepEqual(d.keyBytes[0], want)
	})
	if want := "C64 Ultimate — keyboard buffer mode (firmware older than 1.2.0)"; c.title() != want {
		t.Errorf("title = %q, want %q", c.title(), want)
	}
}

func TestController_CloseSendsRelease(t *testing.T) {
	s, d := &fakeSync{}, &fakeDevice{}
	c := newController(s, d, "darwin", func(ebiten.Key) string { return "" })
	c.close()
	if s.closed != 500*time.Millisecond {
		t.Errorf("Close timeout = %v, want 500ms", s.closed)
	}
}

// Review finding: keys of a hotkey must not reach the C64 when the modifier
// is let go first.
func TestController_HotkeyKeysNotLeaked(t *testing.T) {
	c, s, _ := newTestController(t)
	c.tick(cmd(ebiten.KeyK, t0)) // position mode, so a held K would be sent
	c.tick(tickInput{Focused: true, Now: t0, Frame: hostFrame{Pressed: pressed(ebiten.KeyK), Chars: []rune{'k'}}})
	if got := s.last(); len(got.Keys) != 0 || len(got.Taps) != 0 {
		t.Errorf("sent %v / %v after Cmd was let go first", got.Keys, got.Taps)
	}
	c.tick(tickInput{Focused: true, Now: t0})
	c.tick(tickInput{Focused: true, Now: t0, Frame: hostFrame{Pressed: pressed(ebiten.KeyK)}})
	if got := s.last(); !got.Keys["k"] {
		t.Errorf("K pressed again after its release was not sent: %v", got.Keys)
	}
}
