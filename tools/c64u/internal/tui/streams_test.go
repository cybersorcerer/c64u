package tui

import (
	"errors"
	"testing"
)

// A listener that fails before it starts the stream reports its own error.
func TestRunUntilStarted_ListenFails(t *testing.T) {
	listen := func(func(string) error) error { return errors.New("listen UDP :11001: address in use") }
	err := runUntilStarted(listen, func(string) error { return nil })
	if err == nil || err.Error() != "listen UDP :11001: address in use" {
		t.Errorf("err = %v, want the listen error", err)
	}
}

// The start's error reaches the TUI instead of being dropped.
func TestRunUntilStarted_StartFails(t *testing.T) {
	listen := func(start func(string) error) error { return start("192.168.1.155") }
	err := runUntilStarted(listen, func(string) error { return errors.New("Network Host Resolve Error") })
	if err == nil || err.Error() != "Network Host Resolve Error" {
		t.Errorf("err = %v, want the start error", err)
	}
}

// A started stream returns at once while the listener keeps running.
func TestRunUntilStarted_Started(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	listen := func(start func(string) error) error {
		if err := start("192.168.1.155"); err != nil {
			return err
		}
		<-release
		return nil
	}
	if err := runUntilStarted(listen, func(string) error { return nil }); err != nil {
		t.Errorf("err = %v, want nil while the stream runs", err)
	}
}

// The video child prints its failure the way the CLI does; the TUI shows that
// part, not the hotkey help printed before it.
func TestChildError(t *testing.T) {
	out := "Mod+P swap ports\n✗ Error: Network Host Resolve Error; the device could not find it\n"
	if got := childError([]byte(out)); got != "Network Host Resolve Error; the device could not find it" {
		t.Errorf("childError = %q", got)
	}
	out = "✗ Error: Failed to start stream\n  - HTTP request failed: timeout\n"
	if got := childError([]byte(out)); got != "Failed to start stream: HTTP request failed: timeout" {
		t.Errorf("childError = %q", got)
	}
	if got := childError([]byte("panic: something\n")); got != "panic: something" {
		t.Errorf("childError without an error line = %q, want the last line", got)
	}
}
