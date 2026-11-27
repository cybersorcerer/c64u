package inputsync

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// fakeSender records every batch and answers as configured.
type fakeSender struct {
	mu      sync.Mutex
	calls   [][]api.InputEvent
	times   []time.Time
	delay   time.Duration
	fail    int     // the next fail calls return an error
	status  int     // non-zero: answer with this HTTP status
	pending float64 // non-zero: report this mouse.pending once
}

func (f *fakeSender) MachineInput(events []api.InputEvent) (*api.Response, error) {
	f.mu.Lock()
	delay := f.delay
	f.mu.Unlock()
	time.Sleep(delay)

	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, events)
	f.times = append(f.times, time.Now())
	if f.fail > 0 {
		f.fail--
		return nil, errors.New("connection refused")
	}
	resp := &api.Response{StatusCode: 200, Data: map[string]interface{}{}}
	if f.status != 0 {
		resp.StatusCode = f.status
		resp.Errors = []string{"HTTP 404: Not Found"}
	}
	if f.pending != 0 {
		resp.Data["mouse"] = map[string]interface{}{"pending": f.pending}
		f.pending = 0
	}
	return resp, nil
}

func (f *fakeSender) snapshot() [][]api.InputEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]api.InputEvent(nil), f.calls...)
}

func (f *fakeSender) set(fn func(f *fakeSender)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

// eventually polls cond for up to two seconds.
func eventually(t *testing.T, what string, cond func() bool) {
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

func callCount(f *fakeSender) func() bool {
	return func() bool { return len(f.snapshot()) > 0 }
}

func newSyncer(t *testing.T, f *fakeSender) *Syncer {
	y := New(f)
	t.Cleanup(func() { y.Close(100 * time.Millisecond) })
	return y
}

// held replays the recorded calls and returns the keyboard and port-2
// joystick inputs the device would be holding afterwards.
func held(calls [][]api.InputEvent) Set {
	down := Set{}
	for _, batch := range calls {
		for _, ev := range batch {
			for _, name := range ev.Inputs {
				switch ev.Transition {
				case "press":
					down[ev.Kind+":"+name] = true
				case "release":
					delete(down, ev.Kind+":"+name)
				}
			}
			if ev.Kind == "release_all" {
				down = Set{}
			}
		}
	}
	return down
}

func TestSyncer_SendsDiff(t *testing.T) {
	f := &fakeSender{}
	y := newSyncer(t, f)
	y.Update(State{Keys: Set{"a": true}})
	eventually(t, "a call", callCount(f))
	want := [][]api.InputEvent{{kbd("press", "a")}}
	if got := f.snapshot(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSyncer_ShortPressBecomesTap(t *testing.T) {
	f := &fakeSender{delay: 50 * time.Millisecond}
	y := newSyncer(t, f)
	y.Update(State{Keys: Set{"x": true}}) // occupies the sender
	time.Sleep(10 * time.Millisecond)
	y.Update(State{Keys: Set{"x": true, "a": true}})
	y.Update(State{Keys: Set{"x": true}})
	y.Update(State{Keys: Set{"x": true}, Joy: [3]Set{2: {"fire": true}}})
	y.Update(State{Keys: Set{"x": true}})
	eventually(t, "two calls", func() bool { return len(f.snapshot()) >= 2 })
	got := f.snapshot()[1]
	want := []api.InputEvent{
		kbd("tap", "a"),
		{Kind: "joystick", Port: 2, Inputs: []string{"fire"}, Transition: "tap"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("second call = %+v, want %+v", got, want)
	}
}

func TestSyncer_NoBacklog(t *testing.T) {
	f := &fakeSender{delay: 50 * time.Millisecond}
	y := newSyncer(t, f)
	for i := range 100 {
		s := State{Joy: [3]Set{2: {}}}
		if i%2 == 0 {
			s.Joy[2]["fire"] = true
		}
		y.Update(s)
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	calls := f.snapshot()
	if len(calls) > 8 {
		t.Errorf("%d calls for 200 ms of input, want at most 8", len(calls))
	}
	if held(calls)["joystick:fire"] {
		t.Error("fire is still held after the last update released it")
	}
}

func TestSyncer_ErrorRetriesFromReleaseAll(t *testing.T) {
	f := &fakeSender{fail: 1}
	y := newSyncer(t, f)
	y.Update(State{Keys: Set{"a": true}})
	eventually(t, "the failed call", callCount(f))
	eventually(t, "Err to be set", func() bool { return y.Err() != nil })
	eventually(t, "the retry", func() bool { return len(f.snapshot()) >= 2 })
	eventually(t, "Err to clear", func() bool { return y.Err() == nil })

	calls := f.snapshot()
	if want := []api.InputEvent{{Kind: "release_all"}, kbd("press", "a")}; !reflect.DeepEqual(calls[1], want) {
		t.Errorf("retry = %+v, want %+v", calls[1], want)
	}
	f.mu.Lock()
	gap := f.times[1].Sub(f.times[0])
	f.mu.Unlock()
	if gap < 250*time.Millisecond {
		t.Errorf("retried after %v, want at least 250ms", gap)
	}
}

func TestSyncer_ErrorDropsTaps(t *testing.T) {
	f := &fakeSender{fail: 1}
	y := newSyncer(t, f)
	y.Update(State{Taps: []Tap{{Kind: "keyboard", Inputs: []string{"l"}}}})
	eventually(t, "the failed call", callCount(f))
	y.Update(State{Keys: Set{"b": true}})
	eventually(t, "the next call", func() bool { return len(f.snapshot()) >= 2 })
	time.Sleep(50 * time.Millisecond)
	calls := f.snapshot()
	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2: %+v", len(calls), calls)
	}
	if want := []api.InputEvent{{Kind: "release_all"}, kbd("press", "b")}; !reflect.DeepEqual(calls[1], want) {
		t.Errorf("second call = %+v, want %+v (the tap must not be repeated)", calls[1], want)
	}
}

func TestSyncer_StateAfterOutageIsCurrent(t *testing.T) {
	f := &fakeSender{fail: 2}
	y := newSyncer(t, f)
	y.Update(State{Keys: Set{"a": true}})
	eventually(t, "the failed call", callCount(f))
	y.Update(State{Keys: Set{"b": true}})
	eventually(t, "a successful call", func() bool { return len(f.snapshot()) >= 3 })
	time.Sleep(50 * time.Millisecond)
	calls := f.snapshot()
	last := calls[len(calls)-1]
	if want := []api.InputEvent{{Kind: "release_all"}, kbd("press", "b")}; !reflect.DeepEqual(last, want) {
		t.Errorf("after the outage got %+v, want %+v", last, want)
	}
}

func TestSyncer_404Unsupported(t *testing.T) {
	f := &fakeSender{status: 404}
	y := newSyncer(t, f)
	y.Update(State{Keys: Set{"a": true}})
	eventually(t, "Unsupported", y.Unsupported)
	y.Update(State{Keys: Set{"b": true}})
	time.Sleep(50 * time.Millisecond)
	if n := len(f.snapshot()); n != 1 {
		t.Errorf("got %d calls, want 1: nothing after the 404", n)
	}
}

func moves(calls [][]api.InputEvent) []int {
	var xs []int
	for _, batch := range calls {
		for _, ev := range batch {
			if ev.Move != nil {
				xs = append(xs, ev.Move.X)
			}
		}
	}
	return xs
}

func TestSyncer_MouseClampAndRemainder(t *testing.T) {
	f := &fakeSender{}
	y := newSyncer(t, f)
	y.Update(State{DX: 600})
	eventually(t, "three moves", func() bool { return len(moves(f.snapshot())) >= 3 })
	time.Sleep(50 * time.Millisecond)
	if got := moves(f.snapshot()); !reflect.DeepEqual(got, []int{127, 127, 127}) {
		t.Errorf("moves = %v, want [127 127 127]: one per batch, remainder capped at 254", got)
	}
}

func TestSyncer_MousePendingPausesMove(t *testing.T) {
	f := &fakeSender{pending: 3}
	y := newSyncer(t, f)
	y.Update(State{DX: 10})
	eventually(t, "the first move", callCount(f))
	y.Update(State{Keys: Set{"a": true}, DX: 5})
	eventually(t, "the second call", func() bool { return len(f.snapshot()) >= 2 })
	if got := moves(f.snapshot()[1:2]); len(got) != 0 {
		t.Errorf("second call moved %v while 3 reports were pending", got)
	}
	y.Update(State{Keys: Set{"a": true}})
	eventually(t, "the held-back move", func() bool { return len(moves(f.snapshot())) >= 2 })
	if got := moves(f.snapshot()); !reflect.DeepEqual(got, []int{10, 5}) {
		t.Errorf("moves = %v, want [10 5]", got)
	}
}

func TestSyncer_ReleaseAll(t *testing.T) {
	f := &fakeSender{}
	y := newSyncer(t, f)
	y.Update(State{Keys: Set{"a": true}})
	eventually(t, "press a", callCount(f))
	y.ReleaseAll()
	eventually(t, "release_all", func() bool { return len(f.snapshot()) >= 2 })
	if want := []api.InputEvent{{Kind: "release_all"}}; !reflect.DeepEqual(f.snapshot()[1], want) {
		t.Errorf("got %+v, want %+v", f.snapshot()[1], want)
	}
	y.Update(State{Keys: Set{"a": true}})
	eventually(t, "press a again", func() bool { return len(f.snapshot()) >= 3 })
	if want := []api.InputEvent{kbd("press", "a")}; !reflect.DeepEqual(f.snapshot()[2], want) {
		t.Errorf("got %+v, want %+v", f.snapshot()[2], want)
	}
}

func TestSyncer_CloseSendsReleaseAll(t *testing.T) {
	f := &fakeSender{}
	y := New(f)
	y.Update(State{Keys: Set{"a": true}})
	eventually(t, "press a", callCount(f))
	y.Close(500 * time.Millisecond)
	calls := f.snapshot()
	if want := []api.InputEvent{{Kind: "release_all"}}; !reflect.DeepEqual(calls[len(calls)-1], want) {
		t.Errorf("last call = %+v, want %+v", calls[len(calls)-1], want)
	}

	slow := &fakeSender{delay: 2 * time.Second}
	y = New(slow)
	start := time.Now()
	y.Close(500 * time.Millisecond)
	if took := time.Since(start); took > 900*time.Millisecond {
		t.Errorf("Close took %v with a hanging device, want about 500ms", took)
	}
}

// Review finding: a letter typed just before Space must arrive before it.
func TestSyncer_TapThenPressKeepsOrder(t *testing.T) {
	f := &fakeSender{delay: 50 * time.Millisecond}
	y := newSyncer(t, f)
	y.Update(State{Keys: Set{"x": true}}) // occupies the sender
	time.Sleep(10 * time.Millisecond)
	y.Update(State{Keys: Set{"x": true}, Taps: []Tap{{Kind: "keyboard", Inputs: []string{"o"}}}})
	y.Update(State{Keys: Set{"x": true, "space": true}})
	eventually(t, "two calls", func() bool { return len(f.snapshot()) >= 2 })
	want := []api.InputEvent{kbd("tap", "o"), kbd("press", "space")}
	if got := f.snapshot()[1]; !reflect.DeepEqual(got, want) {
		t.Errorf("second call = %+v, want %+v", got, want)
	}
}

// Review finding: a quick Left arrow is Shift plus cursor, and must stay one
// keystroke rather than becoming Shift, then cursor right.
func TestSyncer_ShortMultiKeyPressIsOneTap(t *testing.T) {
	f := &fakeSender{delay: 50 * time.Millisecond}
	y := newSyncer(t, f)
	y.Update(State{Keys: Set{"x": true}})
	time.Sleep(10 * time.Millisecond)
	y.Update(State{Keys: Set{"x": true, "left_shift": true, "cursor_left_right": true}})
	y.Update(State{Keys: Set{"x": true}})
	eventually(t, "two calls", func() bool { return len(f.snapshot()) >= 2 })
	want := []api.InputEvent{kbd("tap", "cursor_left_right", "left_shift")}
	if got := f.snapshot()[1]; !reflect.DeepEqual(got, want) {
		t.Errorf("second call = %+v, want %+v", got, want)
	}
}

// Review finding: a press the device applied although the call failed must
// not stay down when the key is released before the retry.
func TestSyncer_ReleaseDuringOutageIsNotLost(t *testing.T) {
	f := &fakeSender{fail: 1}
	y := newSyncer(t, f)
	y.Update(State{Keys: Set{"a": true}})
	eventually(t, "the failed call", callCount(f))
	y.Update(State{})
	eventually(t, "the retry", func() bool { return len(f.snapshot()) >= 2 })
	if want := []api.InputEvent{{Kind: "release_all"}}; !reflect.DeepEqual(f.snapshot()[1], want) {
		t.Errorf("retry = %+v, want %+v", f.snapshot()[1], want)
	}
}

// Review finding: Close must not let a press in flight land after release_all.
func TestSyncer_CloseWaitsForInFlightCall(t *testing.T) {
	f := &fakeSender{delay: 100 * time.Millisecond}
	y := New(f)
	y.Update(State{Keys: Set{"a": true}})
	time.Sleep(10 * time.Millisecond)
	y.Close(time.Second)
	calls := f.snapshot()
	if len(calls) != 2 || !reflect.DeepEqual(calls[1], []api.InputEvent{{Kind: "release_all"}}) {
		t.Errorf("calls = %+v, want press a, then release_all last", calls)
	}
}

// wheels sums the vertical wheel steps sent in calls.
func wheels(calls [][]api.InputEvent) (sum, most int) {
	for _, call := range calls {
		for _, e := range call {
			if e.Wheel != nil {
				sum += e.Wheel.Vertical
				if e.Wheel.Vertical > most {
					most = e.Wheel.Vertical
				}
			}
		}
	}
	return sum, most
}

// Device finding: a trackpad scroll queued 208 reports, about four seconds of
// lag for every click and move behind it. One step per call, the rest dropped.
func TestSyncer_WheelOneStepPerCall(t *testing.T) {
	f := &fakeSender{}
	y := newSyncer(t, f)
	y.Update(State{WheelV: 30})
	eventually(t, "the wheel", callCount(f))
	time.Sleep(50 * time.Millisecond)
	if sum, most := wheels(f.snapshot()); sum != 1 || most != 1 {
		t.Errorf("sent %d wheel steps, at most %d per call; want 1 and 1", sum, most)
	}
}

// Device finding: the wheel must wait while the firmware is behind, as motion does.
func TestSyncer_WheelHeldBackWhilePending(t *testing.T) {
	f := &fakeSender{pending: 10}
	y := newSyncer(t, f)
	y.Update(State{DX: 10})
	eventually(t, "the first move", callCount(f))
	y.Update(State{WheelV: 5})
	time.Sleep(50 * time.Millisecond)
	if sum, _ := wheels(f.snapshot()); sum != 0 {
		t.Errorf("sent %d wheel steps while 10 reports were pending, want 0", sum)
	}
	y.Update(State{WheelV: 3})
	eventually(t, "the next wheel", func() bool { s, _ := wheels(f.snapshot()); return s > 0 })
	if sum, _ := wheels(f.snapshot()); sum != 1 {
		t.Errorf("sent %d wheel steps after the pause, want 1", sum)
	}
}
