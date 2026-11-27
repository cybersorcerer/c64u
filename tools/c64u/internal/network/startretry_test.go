package network

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeStart answers busy the first n calls, then succeeds.
func fakeStart(busy int, calls *int) func(string) error {
	return func(string) error {
		*calls++
		if *calls <= busy {
			return errors.New("Network Host Resolve Error")
		}
		return nil
	}
}

const (
	testNotice = "busy, retrying"
	testHint   = "restart the device"
)

func noSleep(slept *time.Duration) func(time.Duration) {
	return func(d time.Duration) { *slept += d }
}

// Device finding: after the window closed, firmware 1.2.1 refused a new video
// stream with "Network Host Resolve Error" for about 50 seconds.
func TestStartWithRetry_RetriesWhileBusy(t *testing.T) {
	var calls int
	var slept time.Duration
	var out bytes.Buffer
	if err := StartWithRetry(fakeStart(2, &calls), "192.168.1.155", testNotice, testHint, &out, noSleep(&slept)); err != nil {
		t.Fatalf("err = %v, want nil after the device recovered", err)
	}
	if calls != 3 || slept != 10*time.Second {
		t.Errorf("calls = %d, slept %v; want 3 calls and 10s", calls, slept)
	}
	if n := strings.Count(out.String(), testNotice); n != 1 {
		t.Errorf("printed the retry notice %d times, want once:\n%s", n, out.String())
	}
}

func TestStartWithRetry_OtherErrorsFailAtOnce(t *testing.T) {
	calls := 0
	var slept time.Duration
	start := func(string) error { calls++; return errors.New("connection refused") }
	if err := StartWithRetry(start, "192.168.1.155", testNotice, testHint, &bytes.Buffer{}, noSleep(&slept)); err == nil || calls != 1 || slept != 0 {
		t.Errorf("err = %v after %d calls and %v; want the error at once", err, calls, slept)
	}
}

func TestStartWithRetry_GivesUpAfter90Seconds(t *testing.T) {
	var calls int
	var slept time.Duration
	err := StartWithRetry(fakeStart(1000, &calls), "192.168.1.155", testNotice, testHint, &bytes.Buffer{}, noSleep(&slept))
	if err == nil || !strings.Contains(err.Error(), testHint) || !strings.Contains(err.Error(), "Network Host Resolve Error") {
		t.Errorf("err = %v, want the device error and the hint", err)
	}
	if slept != 90*time.Second || calls != 19 {
		t.Errorf("slept %v over %d calls, want 90s and 19 calls", slept, calls)
	}
}
