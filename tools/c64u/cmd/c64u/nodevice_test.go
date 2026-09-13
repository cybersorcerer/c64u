package main

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// isolate points the config lookup at an empty home directory, so no device is
// configured for the duration of the test.
func isolate(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("C64U_HOST", "")
	t.Setenv("C64U_PORT", "")
	t.Setenv("C64U_DEVICE", "")
	t.Cleanup(viper.Reset)
}

// run executes one command against the real root, the way a shell would.
func run(t *testing.T, args ...string) error {
	t.Helper()
	rootCmd.SetArgs(args)
	rootCmd.SetOut(new(strings.Builder))
	rootCmd.SetErr(new(strings.Builder))
	t.Cleanup(func() { rootCmd.SetArgs(nil) })
	return rootCmd.Execute()
}

// A fresh install has no config file. The commands that do not talk to the
// device have to keep working there - "cli-config init" above all, because it
// is what writes the file that the error message tells the user to create.
func TestCommandsThatNeedNoDeviceRunWithoutConfig(t *testing.T) {
	for _, args := range [][]string{
		{"version"},
		{"cli-config", "init"},
		{"cli-config", "show"},
		{"completion", "bash"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			isolate(t)
			if err := run(t, args...); err != nil {
				t.Fatalf("%v failed without a configured device: %v", args, err)
			}
		})
	}
}

// The counterpart: a command that does talk to the device still has to stop
// with the explanation, rather than reaching for an address nobody chose.
func TestDeviceCommandStillReportsMissingHost(t *testing.T) {
	isolate(t)

	err := run(t, "info")
	if err == nil {
		t.Fatal("info succeeded without a configured device")
	}
	if !strings.Contains(err.Error(), "no C64 Ultimate configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}
