// Package inputsync keeps the C64 Ultimate's injected inputs in step with what
// the host holds down. It knows nothing about windows or keyboards: callers
// describe the wanted state in the REST API's own names, and the package works
// out which machine:input events turn the last state sent into that one.
package inputsync

import (
	"sort"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// Set holds input names that are down.
type Set map[string]bool

// Clone returns an independent copy; a nil set stays nil.
func (s Set) Clone() Set {
	if s == nil {
		return nil
	}
	c := make(Set, len(s))
	for k, v := range s {
		c[k] = v
	}
	return c
}

// Tap is a press and release that happen as one event: a typed character, or
// a key that went down and up again between two sends.
type Tap struct {
	Kind   string // keyboard, joystick or mouse
	Port   int    // joystick only
	Inputs []string
}

// State is what the machine should see.
type State struct {
	Keys    Set    // held C64 keys
	Joy     [3]Set // held joystick inputs; index 1 and 2 are the ports
	Buttons Set    // held mouse buttons
	Taps    []Tap  // in order
	DX, DY  int    // mouse motion since the last send
	WheelV  int    // wheel detents since the last send
	WheelH  int
}

// Diff returns the events that turn prev into cur: releases first, then
// presses, then taps, then one mouse move and one wheel report. Releasing
// first lets a modifier come up before a tap that must not carry it.
func Diff(prev, cur State) []api.InputEvent {
	var events []api.InputEvent
	for _, transition := range []string{"release", "press"} {
		from, to := prev, cur
		if transition == "press" {
			from, to = cur, prev
		}
		// names held in from but not in to
		events = appendChunks(events, "keyboard", 0, transition, missing(from.Keys, to.Keys), api.MaxKeyboardInputs)
		for port := 1; port <= 2; port++ {
			events = appendChunks(events, "joystick", port, transition, missing(from.Joy[port], to.Joy[port]), api.MaxJoystickInputs)
		}
		events = appendChunks(events, "mouse", 0, transition, missing(from.Buttons, to.Buttons), api.MaxMouseInputs)
	}
	for _, tap := range cur.Taps {
		events = append(events, api.InputEvent{Kind: tap.Kind, Port: tap.Port, Inputs: tap.Inputs, Transition: "tap"})
	}
	if cur.DX != 0 || cur.DY != 0 {
		events = append(events, api.InputEvent{Kind: "mouse", Move: &api.MouseMove{X: clamp(cur.DX, 127), Y: clamp(cur.DY, 127)}})
	}
	if cur.WheelV != 0 || cur.WheelH != 0 {
		events = append(events, api.InputEvent{Kind: "mouse", Wheel: &api.MouseWheel{Vertical: clamp(cur.WheelV, 64), Horizontal: clamp(cur.WheelH, 64)}})
	}
	return events
}

// missing returns the names in a that b lacks, sorted.
func missing(a, b Set) []string {
	var names []string
	for name, down := range a {
		if down && !b[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func appendChunks(events []api.InputEvent, kind string, port int, transition string, names []string, limit int) []api.InputEvent {
	for len(names) > 0 {
		n := min(limit, len(names))
		events = append(events, api.InputEvent{Kind: kind, Port: port, Inputs: names[:n], Transition: transition})
		names = names[n:]
	}
	return events
}

func clamp(v, limit int) int {
	return max(-limit, min(limit, v))
}
