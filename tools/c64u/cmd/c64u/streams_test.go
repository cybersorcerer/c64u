package main

import (
	"testing"
	"time"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
)

// Review finding: with the general 30 s timeout an unreachable device froze
// the window's input for half a minute before the title showed an error.
func TestWindowClientHasShortTimeout(t *testing.T) {
	base := api.NewClient("c64u.example", 80, false)
	base.Password = "secret"
	c := windowClient(base)
	if c.HTTPClient.Timeout != 2*time.Second {
		t.Errorf("timeout = %v, want 2s", c.HTTPClient.Timeout)
	}
	if c.BaseURL != base.BaseURL || c.Password != base.Password {
		t.Errorf("window client talks to %q with password %q, want the same device as %q", c.BaseURL, c.Password, base.BaseURL)
	}
	if base.HTTPClient.Timeout != 30*time.Second {
		t.Errorf("the shared client's timeout changed to %v", base.HTTPClient.Timeout)
	}
}
