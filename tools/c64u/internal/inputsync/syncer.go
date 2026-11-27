package inputsync

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// Sender applies a batch of input events; *api.Client is one.
type Sender interface {
	MachineInput(events []api.InputEvent) (*api.Response, error)
}

// How long the syncer waits after a failed call before it tries again.
var retryDelay = 250 * time.Millisecond

// Mouse motion beyond one report is carried to the next batch, but only this
// much of it, so a fast swipe cannot leave the pointer drifting for seconds.
const maxMouseCarry = 254

// entry is one change waiting to be sent. Entries of one Update share a
// group, so the Shift and cursor of a Left arrow travel as one event.
type entry struct {
	kind       string
	port       int
	names      []string
	transition string
	group      int
	held       bool // a held input's press, release or short press, not a typed tap
}

// Syncer sends what changed since the last call, in the order it changed, one
// call at a time. Changes made while a call is in flight are merged: a press
// and release in between becomes a tap, and repeated taps of a held input
// collapse, so a slow device delays input but never queues it up.
type Syncer struct {
	sender Sender

	mu          sync.Mutex
	want        State   // held sets plus motion collected since the last send
	log         []entry // unsent changes in order
	group       int
	resync      bool // next call starts with release_all and the full state
	err         error
	unsupported bool

	wake     chan struct{}
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once

	// owned by the run goroutine
	pauseMove  bool
	retryAfter time.Time
}

// New starts a syncer that sends through s.
func New(s Sender) *Syncer {
	y := &Syncer{
		sender: s,
		wake:   make(chan struct{}, 1),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	go y.run()
	return y
}

// Probe sends release_all and waits for the answer, so a firmware without
// machine:input is known before the first keystroke instead of costing it.
// Call it once, before the first Update. Other errors are left to the first
// real call, which retries.
func (y *Syncer) Probe() {
	resp, err := y.sender.MachineInput([]api.InputEvent{{Kind: "release_all"}})
	if err == nil && resp.StatusCode == 404 {
		y.mu.Lock()
		y.unsupported = true
		y.mu.Unlock()
	}
}

// Update replaces the held keys, joystick inputs and mouse buttons, and adds
// the taps and motion of held to what is waiting to be sent.
func (y *Syncer) Update(held State) {
	y.mu.Lock()
	y.group++
	y.record("keyboard", 0, y.want.Keys, held.Keys)
	for port := 1; port <= 2; port++ {
		y.record("joystick", port, y.want.Joy[port], held.Joy[port])
	}
	y.record("mouse", 0, y.want.Buttons, held.Buttons)
	for _, tap := range held.Taps {
		y.group++
		y.log = append(y.log, entry{kind: tap.Kind, port: tap.Port, names: tap.Inputs, transition: "tap", group: y.group})
	}

	y.want.Keys = held.Keys.Clone()
	y.want.Buttons = held.Buttons.Clone()
	for port := range held.Joy {
		y.want.Joy[port] = held.Joy[port].Clone()
	}
	y.want.DX += held.DX
	y.want.DY += held.DY
	y.want.WheelV += held.WheelV
	y.want.WheelH += held.WheelH
	y.mu.Unlock()
	y.signal()
}

// record logs the releases, then the presses, that turn from into to.
func (y *Syncer) record(kind string, port int, from, to Set) {
	for _, name := range missing(from, to) {
		y.release(kind, port, name)
	}
	for _, name := range missing(to, from) {
		y.log = append(y.log, entry{kind: kind, port: port, names: []string{name}, transition: "press", group: y.group, held: true})
	}
}

// release turns an unsent press into a tap where it stands, so it keeps its
// place among the other changes, or logs a release.
func (y *Syncer) release(kind string, port int, name string) {
	same := func(e entry) bool {
		return e.held && e.kind == kind && e.port == port && len(e.names) == 1 && e.names[0] == name
	}
	for i := len(y.log) - 1; i >= 0; i-- {
		if !same(y.log[i]) || y.log[i].transition != "press" {
			continue
		}
		for j := 0; j < i; j++ {
			if same(y.log[j]) && y.log[j].transition == "tap" {
				// already tapped since the last call: drop the repeat
				y.log = append(y.log[:i], y.log[i+1:]...)
				return
			}
		}
		y.log[i].transition = "tap"
		return
	}
	y.log = append(y.log, entry{kind: kind, port: port, names: []string{name}, transition: "release", group: y.group, held: true})
}

// ReleaseAll drops everything held or waiting and has the device release
// every injected input. Use it when the window loses focus.
func (y *Syncer) ReleaseAll() {
	y.mu.Lock()
	y.want, y.log, y.resync = State{}, nil, true
	y.mu.Unlock()
	y.signal()
}

// Resync has the next call start with release_all and send everything held
// again. Use it after a machine reset, which drops every injected input while
// the host keys and buttons stay down.
func (y *Syncer) Resync() {
	y.mu.Lock()
	y.resync = true
	y.mu.Unlock()
	y.signal()
}

// Close stops sending and has the device release every injected input. The
// release goes out after any call in flight, so nothing lands behind it; Close
// waits at most timeout for that.
func (y *Syncer) Close(timeout time.Duration) {
	y.stopOnce.Do(func() { close(y.stop) })
	select {
	case <-y.done:
	case <-time.After(timeout):
	}
}

// Err is the error of the last call, nil once a call has succeeded again.
func (y *Syncer) Err() error {
	y.mu.Lock()
	defer y.mu.Unlock()
	return y.err
}

// Unsupported reports that the device answered 404: its firmware has no
// machine:input. The syncer sends nothing more after that.
func (y *Syncer) Unsupported() bool {
	y.mu.Lock()
	defer y.mu.Unlock()
	return y.unsupported
}

func (y *Syncer) signal() {
	select {
	case y.wake <- struct{}{}:
	default:
	}
}

func (y *Syncer) stopped() bool {
	select {
	case <-y.stop:
		return true
	default:
		return false
	}
}

func (y *Syncer) run() {
	defer close(y.done)
	y.loop()
	if !y.Unsupported() {
		y.sender.MachineInput([]api.InputEvent{{Kind: "release_all"}}) //nolint:errcheck
	}
}

func (y *Syncer) loop() {
	for {
		select {
		case <-y.wake:
		case <-y.stop:
			return
		}
		// Keep sending while there is something to send: motion beyond one
		// report and a retry after a failure need no new Update.
		for {
			if wait := time.Until(y.retryAfter); wait > 0 {
				select {
				case <-time.After(wait):
				case <-y.stop:
					return
				}
			}
			if y.stopped() {
				return
			}
			snap, events := y.snapshot()
			if len(events) == 0 {
				break
			}
			y.send(snap, events)
			if y.Unsupported() {
				return
			}
		}
	}
}

// snapshot takes the logged changes and the motion and turns them into events.
func (y *Syncer) snapshot() (State, []api.InputEvent) {
	y.mu.Lock()
	defer y.mu.Unlock()

	var events []api.InputEvent
	if y.resync {
		// After a failure nobody knows what the device holds, so start over.
		y.resync = false
		held := State{Keys: y.want.Keys, Buttons: y.want.Buttons, Joy: y.want.Joy}
		events = append([]api.InputEvent{{Kind: "release_all"}}, Diff(State{}, held)...)
	} else {
		events = eventsFrom(y.log)
	}
	y.log = nil

	var snap State
	// One move per batch; the firmware sends a report every 20ms at most. A
	// pause holds back the next motion there is, not merely the next batch.
	// Wheel steps queue a report each, so the wheel sends one step per batch,
	// waits out a pause like motion does, and drops the rest: a trackpad
	// scroll otherwise queued seconds of reports ahead of every click.
	moving := y.want.DX != 0 || y.want.DY != 0 || y.want.WheelV != 0 || y.want.WheelH != 0
	if y.pauseMove && moving {
		y.pauseMove = false
	} else if !y.pauseMove {
		snap.DX, snap.DY = clamp(y.want.DX, 127), clamp(y.want.DY, 127)
		snap.WheelV, snap.WheelH = clamp(y.want.WheelV, 1), clamp(y.want.WheelH, 1)
	}
	y.want.DX = clamp(y.want.DX-snap.DX, maxMouseCarry)
	y.want.DY = clamp(y.want.DY-snap.DY, maxMouseCarry)
	y.want.WheelV, y.want.WheelH = 0, 0

	return snap, append(events, Diff(State{}, snap)...)
}

// eventsFrom merges neighbouring entries of one group and transition into one
// event, within the API's per-event limits.
func eventsFrom(log []entry) []api.InputEvent {
	limits := map[string]int{"keyboard": api.MaxKeyboardInputs, "joystick": api.MaxJoystickInputs, "mouse": api.MaxMouseInputs}
	var events []api.InputEvent
	for i := 0; i < len(log); {
		e := log[i]
		names := append([]string(nil), e.names...)
		j := i + 1
		for e.held && j < len(log) && log[j].held && log[j].group == e.group && log[j].kind == e.kind &&
			log[j].port == e.port && log[j].transition == e.transition && len(names) < limits[e.kind] {
			names = append(names, log[j].names...)
			j++
		}
		sort.Strings(names)
		events = append(events, api.InputEvent{Kind: e.kind, Port: e.port, Inputs: names, Transition: e.transition})
		i = j
	}
	return events
}

// send delivers the events. A failure makes the next call start over from
// release_all: a press the device applied before the error, or a batch that
// got through before a later one failed, must not stay down.
func (y *Syncer) send(snap State, events []api.InputEvent) {
	var last *api.Response
	var err error
	for _, batch := range api.SplitInputBatches(events) {
		if y.stopped() {
			return // run sends release_all next
		}
		last, err = y.sender.MachineInput(batch)
		if err == nil && len(last.Errors) > 0 {
			err = sendError(last.Errors)
		}
		if err != nil {
			break
		}
	}

	y.mu.Lock()
	defer y.mu.Unlock()
	if err != nil {
		y.err = err
		if last != nil && last.StatusCode == 404 {
			y.unsupported = true
			return
		}
		y.retryAfter = time.Now().Add(retryDelay)
		// The taps are gone: a keystroke arriving late is worse than one lost.
		y.resync, y.log = true, nil
		y.want.DX = clamp(y.want.DX+snap.DX, maxMouseCarry)
		y.want.DY = clamp(y.want.DY+snap.DY, maxMouseCarry)
		return
	}

	y.err = nil
	if m, ok := last.Data["mouse"].(map[string]interface{}); ok {
		if pending, ok := m["pending"].(float64); ok && pending > 2 {
			y.pauseMove = true
		}
	}
}

type sendError []string

func (e sendError) Error() string { return strings.Join(e, "; ") }
