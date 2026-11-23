//go:build darwin || windows || (linux && cgo)

package video

import (
	"reflect"
	"testing"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/inputsync"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestPadJoystick_DpadAndStick(t *testing.T) {
	joy, _ := padJoystick(padFrame{AxisX: 0.4})
	if len(joy) != 0 {
		t.Errorf("stick at 0.4 gave %v, want nothing inside the dead zone", joy)
	}
	joy, _ = padJoystick(padFrame{AxisX: 0.6})
	if !reflect.DeepEqual(joy, inputsync.Set{"right": true}) {
		t.Errorf("stick at 0.6 gave %v, want right", joy)
	}
	joy, _ = padJoystick(padFrame{AxisY: -0.8, Buttons: map[ebiten.StandardGamepadButton]bool{ebiten.StandardGamepadButtonRightBottom: true}})
	if !reflect.DeepEqual(joy, inputsync.Set{"up": true, "fire": true}) {
		t.Errorf("got %v, want up and fire", joy)
	}
	joy, _ = padJoystick(padFrame{Buttons: map[ebiten.StandardGamepadButton]bool{
		ebiten.StandardGamepadButtonLeftLeft: true, ebiten.StandardGamepadButtonLeftBottom: true, ebiten.StandardGamepadButtonRightRight: true,
	}})
	if !reflect.DeepEqual(joy, inputsync.Set{"left": true, "down": true, "fire2": true}) {
		t.Errorf("got %v, want left, down and fire2", joy)
	}
}

func TestPadJoystick_ExtraKeys(t *testing.T) {
	_, keys := padJoystick(padFrame{Buttons: map[ebiten.StandardGamepadButton]bool{ebiten.StandardGamepadButtonRightLeft: true}})
	if !reflect.DeepEqual(keys, inputsync.Set{"space": true}) {
		t.Errorf("X gave %v, want space", keys)
	}
	_, keys = padJoystick(padFrame{Buttons: map[ebiten.StandardGamepadButton]bool{ebiten.StandardGamepadButtonCenterRight: true}})
	if !reflect.DeepEqual(keys, inputsync.Set{"run_stop": true}) {
		t.Errorf("Menu gave %v, want run_stop", keys)
	}
}

func TestAssignPorts(t *testing.T) {
	if got := assignPorts([]ebiten.GamepadID{3, 1}, false); !reflect.DeepEqual(got, map[ebiten.GamepadID]int{1: 2, 3: 1}) {
		t.Errorf("got %v", got)
	}
	if got := assignPorts([]ebiten.GamepadID{3, 1}, true); !reflect.DeepEqual(got, map[ebiten.GamepadID]int{1: 1, 3: 2}) {
		t.Errorf("swapped: got %v", got)
	}
	if got := assignPorts([]ebiten.GamepadID{5}, false); !reflect.DeepEqual(got, map[ebiten.GamepadID]int{5: 2}) {
		t.Errorf("one pad: got %v", got)
	}
	if got := assignPorts([]ebiten.GamepadID{0, 1, 2}, false); len(got) != 2 {
		t.Errorf("three pads: got %v, want the third ignored", got)
	}
}

func TestKeyJoystick(t *testing.T) {
	got := keyJoystick(hostFrame{Pressed: pressed(ebiten.KeyArrowLeft, ebiten.KeyControlLeft)})
	if !reflect.DeepEqual(got, inputsync.Set{"left": true, "fire": true}) {
		t.Errorf("got %v, want left and fire", got)
	}
}

func TestMouseTracker_HalvesAndCarries(t *testing.T) {
	var m mouseTracker
	if dx, dy, _, _ := m.step(0, 0, 0, 0); dx != 0 || dy != 0 {
		t.Errorf("first step moved %d,%d", dx, dy)
	}
	if dx, _, _, _ := m.step(3, 0, 0, 0); dx != 1 {
		t.Errorf("3 host pixels gave dx %d, want 1", dx)
	}
	if dx, _, _, _ := m.step(4, 0, 0, 0); dx != 1 {
		t.Errorf("the carried pixel plus one gave dx %d, want 1", dx)
	}
	if dx, _, _, _ := m.step(0, 0, 0, 0); dx != -2 {
		t.Errorf("4 pixels back gave dx %d, want -2", dx)
	}
	if _, _, wv, _ := m.step(0, 0, 0, 0.5); wv != 0 {
		t.Errorf("half a detent gave %d", wv)
	}
	if _, _, wv, _ := m.step(0, 0, 0, 0.5); wv != 1 {
		t.Errorf("two halves gave %d, want 1", wv)
	}
	m.reset()
	if dx, _, _, _ := m.step(100, 0, 0, 0); dx != 0 {
		t.Errorf("first step after reset moved %d", dx)
	}
}
