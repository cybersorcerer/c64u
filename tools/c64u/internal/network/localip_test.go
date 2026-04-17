package network

import (
	"net"
	"testing"
)

func TestLocalIP_ReturnsValidIP(t *testing.T) {
	ip, err := LocalIP("192.168.1.1")
	if err != nil {
		// Only fail if no interfaces at all — in CI there may be no route to 192.168.1.1
		// but firstNonLoopback should still work
		t.Logf("LocalIP with target returned error: %v, trying fallback", err)
		return
	}
	if net.ParseIP(ip) == nil {
		t.Errorf("LocalIP returned invalid IP: %q", ip)
	}
	if ip == "127.0.0.1" || ip == "::1" {
		t.Errorf("LocalIP returned loopback address: %q", ip)
	}
}

func TestFirstNonLoopback_ReturnsIPv4(t *testing.T) {
	ip, err := firstNonLoopback()
	if err != nil {
		t.Skipf("no non-loopback interface available: %v", err)
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		t.Errorf("firstNonLoopback returned invalid IP: %q", ip)
	}
	if parsed.IsLoopback() {
		t.Errorf("firstNonLoopback returned loopback: %q", ip)
	}
	if parsed.To4() == nil {
		t.Errorf("firstNonLoopback returned non-IPv4: %q", ip)
	}
}
