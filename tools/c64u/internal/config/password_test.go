package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// A device carries its own password, but anything given explicitly outranks
// it, exactly as with host and port.
func TestResolveDevice_Password(t *testing.T) {
	devices := map[string]Device{
		"attic":  {Host: "attic.local", Password: "attic-pw"},
		"cellar": {Host: "cellar.local"},
	}

	cases := []struct {
		name         string
		cfg          Config
		device       string
		envPassword  string
		flagPassword bool
		want         string
	}{
		{"device password is used", Config{Port: 80, Devices: devices}, "attic", "", false, "attic-pw"},
		{"a device without one leaves it empty", Config{Port: 80, Devices: devices}, "cellar", "", false, ""},
		{"environment outranks the device", Config{Port: 80, Password: "from-env", Devices: devices}, "attic", "from-env", false, "from-env"},
		{"flag outranks the device", Config{Port: 80, Password: "from-flag", Devices: devices}, "attic", "", true, "from-flag"},
		{"flat top-level password keeps working", Config{Host: "h", Port: 80, Password: "flat"}, "", "", false, "flat"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			viper.Reset()
			t.Setenv("C64U_DEVICE", c.device)
			t.Setenv("C64U_PASSWORD", c.envPassword)
			ExplicitPassword = c.flagPassword
			t.Cleanup(func() { ExplicitPassword = false })

			cfg := c.cfg
			if err := cfg.resolveDevice(); err != nil {
				t.Fatal(err)
			}
			if cfg.Password != c.want {
				t.Errorf("password = %q, want %q", cfg.Password, c.want)
			}
		})
	}
}

// The debug log is a plain file; a dump of the config into it must not carry
// the password along.
func TestRedactSecrets(t *testing.T) {
	in := "host = \"h\"\npassword = \"s3cret\"\n[devices.a]\n  password=\"also-secret\"\nport = 80\n"
	out := redactSecrets(in)
	if strings.Contains(out, "s3cret") || strings.Contains(out, "also-secret") {
		t.Errorf("secret survived redaction:\n%s", out)
	}
	if !strings.Contains(out, "host = \"h\"") || !strings.Contains(out, "port = 80") {
		t.Errorf("redaction removed unrelated lines:\n%s", out)
	}
}
