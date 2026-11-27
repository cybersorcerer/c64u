package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stdout runs fn and returns what it printed; cli-config show prints with fmt.
func stdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = saved
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

// machine poweron takes the MAC from config.toml, so cli-config show has to
// show which one it would use, for the device in use and for every entry.
func TestCliConfigShowWifiMAC(t *testing.T) {
	isolate(t)
	dir := filepath.Join(os.Getenv("HOME"), ".config", "c64u")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	conf := `default = "living-room"

[devices.living-room]
host = "192.168.1.100"
wifi_mac = "9C:13:9E:ED:D5:EC"

[devices.attic]
host = "attic.local"
`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	out := stdout(t, func() {
		if err := run(t, "cli-config", "show"); err != nil {
			t.Fatalf("cli-config show: %v", err)
		}
	})
	if !strings.Contains(out, "Wi-Fi MAC:   9C:13:9E:ED:D5:EC") {
		t.Errorf("device in use: no Wi-Fi MAC line in\n%s", out)
	}
	if !strings.Contains(out, "living-room  192.168.1.100:80  wifi_mac 9C:13:9E:ED:D5:EC") {
		t.Errorf("device list: no wifi_mac in\n%s", out)
	}
}
