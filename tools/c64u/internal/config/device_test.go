package config

import (
	"errors"
	"testing"

	"github.com/spf13/viper"
)

// resolveDevice decides which machine a command talks to, and getting the order
// wrong sends it to the wrong one silently. Each case here pins one rung of
// that ladder.
func TestResolveDevice(t *testing.T) {
	devices := map[string]Device{
		"attic":  {Host: "attic.local", Port: 80},
		"cellar": {Host: "cellar.local", Port: 8080},
	}

	cases := []struct {
		name       string
		cfg        Config
		flagDevice string
		envDevice  string
		flagHost   string
		wantHost   string
		wantPort   int
		wantDevice string
	}{
		{
			name:       "named device wins over the default entry",
			cfg:        Config{Host: "localhost", Port: 80, Devices: devices, Default: "cellar"},
			flagDevice: "attic",
			wantHost:   "attic.local", wantPort: 80, wantDevice: "attic",
		},
		{
			name:      "environment names a device when the flag does not",
			cfg:       Config{Host: "localhost", Port: 80, Devices: devices, Default: "cellar"},
			envDevice: "attic",
			wantHost:  "attic.local", wantPort: 80, wantDevice: "attic",
		},
		{
			name:     "the default entry decides when nothing is given",
			cfg:      Config{Host: "localhost", Port: 80, Devices: devices, Default: "cellar"},
			wantHost: "cellar.local", wantPort: 8080, wantDevice: "cellar",
		},
		{
			name:     "a single device needs no default entry",
			cfg:      Config{Host: "localhost", Port: 80, Devices: map[string]Device{"only": {Host: "only.local"}}},
			wantHost: "only.local", wantPort: 80, wantDevice: "only",
		},
		{
			name:     "a flat config file keeps working untouched",
			cfg:      Config{Host: "old.local", Port: 81},
			wantHost: "old.local", wantPort: 81, wantDevice: "",
		},
		{
			name:       "an explicit host outranks the device it would have used",
			cfg:        Config{Host: "one-off.local", Port: 80, Devices: devices, Default: "cellar"},
			flagHost:   "one-off.local",
			wantHost:   "one-off.local", wantPort: 8080, wantDevice: "cellar",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			viper.Reset()
			t.Setenv("C64U_DEVICE", c.envDevice)
			if c.flagDevice != "" {
				viper.Set("device", c.flagDevice)
			}
			ExplicitHost = c.flagHost != ""
			ExplicitPort = false
			t.Cleanup(func() { ExplicitHost, ExplicitPort = false, false })
			if c.flagHost != "" {
				viper.Set("host", c.flagHost)
			}

			cfg := c.cfg
			if err := cfg.resolveDevice(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Host != c.wantHost {
				t.Errorf("host = %q, want %q", cfg.Host, c.wantHost)
			}
			if cfg.Port != c.wantPort {
				t.Errorf("port = %d, want %d", cfg.Port, c.wantPort)
			}
			if cfg.Device != c.wantDevice {
				t.Errorf("device = %q, want %q", cfg.Device, c.wantDevice)
			}
		})
	}
}

// A typo must not fall back to some other machine.
func TestResolveDeviceUnknownName(t *testing.T) {
	viper.Reset()
	t.Setenv("C64U_DEVICE", "")
	viper.Set("device", "attik")

	cfg := Config{Devices: map[string]Device{
		"attic":  {Host: "attic.local"},
		"cellar": {Host: "cellar.local"},
	}}

	err := cfg.resolveDevice()
	if err == nil {
		t.Fatal("expected an error for an unknown device")
	}

	var unknown *UnknownDeviceError
	if !errors.As(err, &unknown) {
		t.Fatalf("error is %T, want *UnknownDeviceError", err)
	}
	if unknown.Name != "attik" {
		t.Errorf("name = %q, want attik", unknown.Name)
	}
	// The message has to name what does exist, in a stable order.
	want := `unknown device "attik": defined devices are attic, cellar`
	if err.Error() != want {
		t.Errorf("message = %q, want %q", err.Error(), want)
	}
}

func TestDeviceNamesAreSorted(t *testing.T) {
	cfg := Config{Devices: map[string]Device{"zulu": {}, "alpha": {}, "mike": {}}}
	got := cfg.DeviceNames()
	want := []string{"alpha", "mike", "zulu"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("names = %v, want %v", got, want)
		}
	}
}
