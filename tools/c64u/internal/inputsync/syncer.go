package inputsync

import (
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

// Syncer sends the difference between the wanted state and the last state the
// device acknowledged, one call at a time. Updates made while a call is in
// flight are merged, so a slow device delays input but never queues it up.
type Syncer struct {
	sender Sender

	mu          sync.Mutex
	want        State // held sets plus taps and motion collected since the last send
	seen        State // every held input since the last snapshot, to catch short presses
	releaseAll  bool
	err         error
	unsupported bool

	wake     chan struct{}
	stop     chan struct{}
	stopOnce sync.Once

	// owned by the run goroutine
	lastSent   State
	pauseMove  bool
	retryAfter time.Time
}

// New starts a syncer that sends through s.
func New(s Sender) *Syncer {
	y := &Syncer{
		sender: s,
		wake:   make(chan struct{}, 1),
		stop:   make(chan struct{}),
	}
	go y.run()
	return y
}

// Update replaces the held keys, joystick inputs and mouse buttons, and adds
// the taps and motion of held to what is waiting to be sent.
func (y *Syncer) Update(held State) {
	y.mu.Lock()
	y.want.Keys = held.Keys.Clone()
	y.want.Buttons = held.Buttons.Clone()
	for port := range held.Joy {
		y.want.Joy[port] = held.Joy[port].Clone()
	}
	y.want.Taps = append(y.want.Taps, held.Taps...)
	y.want.DX += held.DX
	y.want.DY += held.DY
	y.want.WheelV += held.WheelV
	y.want.WheelH += held.WheelH

	y.seen.Keys = union(y.seen.Keys, held.Keys)
	y.seen.Buttons = union(y.seen.Buttons, held.Buttons)
	for port := range held.Joy {
		y.seen.Joy[port] = union(y.seen.Joy[port], held.Joy[port])
	}
	y.mu.Unlock()
	y.signal()
}

// ReleaseAll drops everything held or waiting and has the device release
// every injected input. Use it when the window loses focus.
func (y *Syncer) ReleaseAll() {
	y.mu.Lock()
	y.want, y.seen = State{}, State{}
	y.releaseAll = true
	y.mu.Unlock()
	y.signal()
}

// Close stops sending and releases every injected input, waiting at most
// timeout for the device to answer.
func (y *Syncer) Close(timeout time.Duration) {
	y.stopOnce.Do(func() { close(y.stop) })
	if y.Unsupported() {
		return
	}
	done := make(chan struct{})
	go func() {
		y.sender.MachineInput([]api.InputEvent{{Kind: "release_all"}}) //nolint:errcheck
		close(done)
	}()
	select {
	case <-done:
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
			if !y.send(snap, events) {
				if y.Unsupported() {
					return
				}
			}
		}
	}
}

// snapshot takes what is waiting and turns it into events. Taps and motion
// leave the wanted state here; only a motion remainder stays behind.
func (y *Syncer) snapshot() (State, []api.InputEvent) {
	y.mu.Lock()
	defer y.mu.Unlock()

	snap := State{
		Keys:    y.want.Keys.Clone(),
		Buttons: y.want.Buttons.Clone(),
		WheelV:  y.want.WheelV,
		WheelH:  y.want.WheelH,
	}
	for port := range y.want.Joy {
		snap.Joy[port] = y.want.Joy[port].Clone()
	}

	// A press that came and went since the last snapshot is a tap.
	for _, name := range missing(y.seen.Keys, union(y.want.Keys, y.lastSent.Keys)) {
		snap.Taps = append(snap.Taps, Tap{Kind: "keyboard", Inputs: []string{name}})
	}
	for port := 1; port <= 2; port++ {
		for _, name := range missing(y.seen.Joy[port], union(y.want.Joy[port], y.lastSent.Joy[port])) {
			snap.Taps = append(snap.Taps, Tap{Kind: "joystick", Port: port, Inputs: []string{name}})
		}
	}
	for _, name := range missing(y.seen.Buttons, union(y.want.Buttons, y.lastSent.Buttons)) {
		snap.Taps = append(snap.Taps, Tap{Kind: "mouse", Inputs: []string{name}})
	}
	snap.Taps = append(snap.Taps, y.want.Taps...)
	y.want.Taps = nil

	// One move per batch; the firmware sends a report every 20ms at most.
	// A pause holds back the next motion there is, not merely the next batch.
	if y.pauseMove && (y.want.DX != 0 || y.want.DY != 0) {
		y.pauseMove = false
	} else if !y.pauseMove {
		snap.DX, snap.DY = clamp(y.want.DX, 127), clamp(y.want.DY, 127)
	}
	y.want.DX = clamp(y.want.DX-snap.DX, maxMouseCarry)
	y.want.DY = clamp(y.want.DY-snap.DY, maxMouseCarry)
	y.want.WheelV, y.want.WheelH = 0, 0

	y.seen = State{Keys: y.want.Keys.Clone(), Buttons: y.want.Buttons.Clone()}
	for port := range y.want.Joy {
		y.seen.Joy[port] = y.want.Joy[port].Clone()
	}

	var events []api.InputEvent
	base := y.lastSent
	if y.releaseAll {
		y.releaseAll = false
		events = append(events, api.InputEvent{Kind: "release_all"})
		base = State{}
		y.lastSent = State{}
	}
	return snap, append(events, Diff(base, snap)...)
}

// send delivers the events and reports whether all of them were accepted.
func (y *Syncer) send(snap State, events []api.InputEvent) bool {
	var last *api.Response
	var err error
	for _, batch := range api.SplitInputBatches(events) {
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
			return false
		}
		y.retryAfter = time.Now().Add(retryDelay)
		// The taps are gone: a keystroke arriving late is worse than one lost.
		// Motion is kept, capped, and presses since the snapshot start afresh.
		y.want.DX = clamp(y.want.DX+snap.DX, maxMouseCarry)
		y.want.DY = clamp(y.want.DY+snap.DY, maxMouseCarry)
		y.seen = State{}
		if len(events) > 0 && events[0].Kind == "release_all" {
			y.releaseAll = true
		}
		return false
	}

	y.err = nil
	y.lastSent = State{Keys: snap.Keys, Buttons: snap.Buttons, Joy: snap.Joy}
	if m, ok := last.Data["mouse"].(map[string]interface{}); ok {
		if pending, ok := m["pending"].(float64); ok && pending > 2 {
			y.pauseMove = true
		}
	}
	return true
}

type sendError []string

func (e sendError) Error() string { return strings.Join(e, "; ") }

func union(a, b Set) Set {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	u := make(Set, len(a)+len(b))
	for k, v := range a {
		if v {
			u[k] = true
		}
	}
	for k, v := range b {
		if v {
			u[k] = true
		}
	}
	return u
}
