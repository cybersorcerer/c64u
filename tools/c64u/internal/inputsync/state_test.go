package inputsync

import (
	"reflect"
	"testing"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

func kbd(transition string, names ...string) api.InputEvent {
	return api.InputEvent{Kind: "keyboard", Inputs: names, Transition: transition}
}

func TestDiff_PressRelease(t *testing.T) {
	prev := State{Keys: Set{"a": true, "left_shift": true}}
	cur := State{Keys: Set{"left_shift": true, "b": true}}
	want := []api.InputEvent{kbd("release", "a"), kbd("press", "b")}
	if got := Diff(prev, cur); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestDiff_OrderReleasePressTapMouse(t *testing.T) {
	prev := State{Joy: [3]Set{2: {"fire": true}}}
	cur := State{
		Keys: Set{"space": true},
		Taps: []Tap{{Kind: "keyboard", Inputs: []string{"left_shift", "2"}}},
		DX:   3,
	}
	want := []api.InputEvent{
		{Kind: "joystick", Port: 2, Inputs: []string{"fire"}, Transition: "release"},
		kbd("press", "space"),
		kbd("tap", "left_shift", "2"),
		{Kind: "mouse", Move: &api.MouseMove{X: 3}},
	}
	if got := Diff(prev, cur); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestDiff_MouseButtonsAndJoystickPort(t *testing.T) {
	cur := State{Joy: [3]Set{1: {"up": true}}, Buttons: Set{"left": true}}
	want := []api.InputEvent{
		{Kind: "joystick", Port: 1, Inputs: []string{"up"}, Transition: "press"},
		{Kind: "mouse", Inputs: []string{"left"}, Transition: "press"},
	}
	if got := Diff(State{}, cur); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestDiff_SplitsAtLimits(t *testing.T) {
	cur := State{Keys: Set{}}
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		cur.Keys[k] = true
	}
	got := Diff(State{}, cur)
	if len(got) != 2 || len(got[0].Inputs) != 8 || len(got[1].Inputs) != 2 {
		t.Errorf("got %+v, want press events of 8 and 2 keys", got)
	}

	all := State{Joy: [3]Set{2: {"up": true, "down": true, "left": true, "right": true, "fire": true, "fire2": true, "fire3": true}}}
	if got := Diff(State{}, all); len(got) != 1 || len(got[0].Inputs) != 7 {
		t.Errorf("got %+v, want one joystick event with 7 inputs", got)
	}
}

func TestDiff_ClampsMouse(t *testing.T) {
	got := Diff(State{}, State{DX: 300, DY: -500, WheelV: 99})
	want := []api.InputEvent{
		{Kind: "mouse", Move: &api.MouseMove{X: 127, Y: -127}},
		{Kind: "mouse", Wheel: &api.MouseWheel{Vertical: 64}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestDiff_TwoEqualTapsKept(t *testing.T) {
	cur := State{Taps: []Tap{{Kind: "keyboard", Inputs: []string{"l"}}, {Kind: "keyboard", Inputs: []string{"l"}}}}
	want := []api.InputEvent{kbd("tap", "l"), kbd("tap", "l")}
	if got := Diff(State{}, cur); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestDiff_NoChangeNoEvents(t *testing.T) {
	s := State{Keys: Set{"a": true}, Joy: [3]Set{2: {"fire": true}}, Buttons: Set{"left": true}}
	if got := Diff(s, s); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}
