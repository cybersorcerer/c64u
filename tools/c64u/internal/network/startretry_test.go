package network

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeStart answers with the resolve error the first n calls, then succeeds.
func fakeStart(failing int, calls *int) func(string) error {
	return func(string) error {
		*calls++
		if *calls <= failing {
			return errors.New("Network Host Resolve Error")
		}
		return nil
	}
}

func noSleep(slept *time.Duration) func(time.Duration) {
	return func(d time.Duration) { *slept += d }
}

// The device finds the destination once it has learned it on the wired port,
// which can happen between tries; the notice is printed once.
func TestStartWithRetry_RetriesResolveError(t *testing.T) {
	var calls int
	var slept time.Duration
	var out bytes.Buffer
	if err := StartWithRetry(fakeStart(2, &calls), "192.168.1.155", &out, noSleep(&slept)); err != nil {
		t.Fatalf("err = %v, want nil once the device found the destination", err)
	}
	if calls != 3 || slept != 4*time.Second {
		t.Errorf("calls = %d, slept %v; want 3 calls and 4s", calls, slept)
	}
	if n := strings.Count(out.String(), "retrying"); n != 1 {
		t.Errorf("printed the retry notice %d times, want once:\n%s", n, out.String())
	}
}

func TestStartWithRetry_OtherErrorsFailAtOnce(t *testing.T) {
	calls := 0
	var slept time.Duration
	start := func(string) error { calls++; return errors.New("connection refused") }
	if err := StartWithRetry(start, "192.168.1.155", &bytes.Buffer{}, noSleep(&slept)); err == nil || calls != 1 || slept != 0 {
		t.Errorf("err = %v after %d calls and %v; want the error at once", err, calls, slept)
	}
}

// Firmware 1.2.1 looks the destination up over Wi-Fi when Ethernet and Wi-Fi
// share a subnet (checked 2026-11-27); waiting does not fix that, so the retry
// is short and the error names the cause and the way out.
func TestStartWithRetry_GivesUpWithCause(t *testing.T) {
	var calls int
	var slept time.Duration
	err := StartWithRetry(fakeStart(1000, &calls), "192.168.1.155", &bytes.Buffer{}, noSleep(&slept))
	if err == nil {
		t.Fatal("err = nil, want the resolve error")
	}
	for _, want := range []string{"Network Host Resolve Error", "Wi-Fi", "255.255.255.255"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q lacks %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "reboot") {
		t.Errorf("err = %q still suggests a reboot", err)
	}
	if slept != 6*time.Second || calls != 4 {
		t.Errorf("slept %v over %d calls, want 6s and 4 calls", slept, calls)
	}
}

// Commands that start a stream without retrying add the same explanation.
func TestExplainStartError(t *testing.T) {
	if got := ExplainStartError(errors.New("connection refused")); got.Error() != "connection refused" {
		t.Errorf("other error changed to %q", got)
	}
	if got := ExplainStartError(errors.New("Network Host Resolve Error")); !strings.Contains(got.Error(), "255.255.255.255") {
		t.Errorf("resolve error not explained: %q", got)
	}
	if ExplainStartError(nil) != nil {
		t.Error("nil error turned into an error")
	}
}
