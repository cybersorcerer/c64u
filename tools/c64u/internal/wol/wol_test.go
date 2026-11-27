package wol

import (
	"bytes"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestMagicPacket(t *testing.T) {
	mac, _ := net.ParseMAC("9C:13:9E:ED:D5:EC")
	p := MagicPacket(mac)
	if len(p) != 102 {
		t.Fatalf("len = %d, want 102", len(p))
	}
	if !bytes.Equal(p[:6], bytes.Repeat([]byte{0xff}, 6)) {
		t.Errorf("sync = % x", p[:6])
	}
	for i := 0; i < 16; i++ {
		if !bytes.Equal(p[6+i*6:12+i*6], mac) {
			t.Fatalf("repeat %d = % x", i, p[6+i*6:12+i*6])
		}
	}
}

func TestParseMAC(t *testing.T) {
	for _, s := range []string{"9C:13:9E:ED:D5:EC", "9c-13-9e-ed-d5-ec"} {
		if _, err := ParseMAC(s); err != nil {
			t.Errorf("ParseMAC(%q): %v", s, err)
		}
	}
	// The firmware ignores a broadcast or all-zero MAC: no station owns it.
	for _, s := range []string{"", "nonsense", "FF:FF:FF:FF:FF:FF", "00:00:00:00:00:00", "00:00:5e:00:53:01:02:03"} {
		if _, err := ParseMAC(s); err == nil {
			t.Errorf("ParseMAC(%q) accepted it", s)
		}
	}
}

func TestChooseMAC(t *testing.T) {
	cases := []struct {
		flag, configured, cached string
		want, source             string
	}{
		{"11:11:11:11:11:11", "22:22:22:22:22:22", "33:33:33:33:33:33", "11:11:11:11:11:11", SourceFlag},
		{"", "22:22:22:22:22:22", "33:33:33:33:33:33", "22:22:22:22:22:22", SourceConfig},
		{"", "", "33:33:33:33:33:33", "33:33:33:33:33:33", SourceCache},
	}
	for _, c := range cases {
		mac, source, err := ChooseMAC(c.flag, c.configured, c.cached)
		if err != nil || mac.String() != c.want || source != c.source {
			t.Errorf("ChooseMAC(%q,%q,%q) = %v, %q, %v", c.flag, c.configured, c.cached, mac, source, err)
		}
	}
	if _, _, err := ChooseMAC("", "", ""); !errors.Is(err, ErrNoMAC) {
		t.Errorf("no source: err = %v, want ErrNoMAC", err)
	}
	// A bad MAC from a higher source is an error, not a reason to fall through.
	if _, _, err := ChooseMAC("bad", "", "33:33:33:33:33:33"); err == nil || errors.Is(err, ErrNoMAC) {
		t.Errorf("a bad --mac: err = %v, want a parse error", err)
	}
}

func TestCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "wifi_macs.json")
	if got := Cached(path, "c64u.local"); got != "" {
		t.Errorf("empty cache = %q", got)
	}
	mac, _ := ParseMAC("9C:13:9E:ED:D5:EC")
	if err := Remember(path, "C64U.local", mac); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	other, _ := ParseMAC("24:0A:C4:1A:2B:3C")
	if err := Remember(path, "other.local", other); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if got := Cached(path, "c64u.local"); got != "9c:13:9e:ed:d5:ec" {
		t.Errorf("Cached = %q, want the first MAC (hosts compared without case)", got)
	}
	if got := Cached(path, "other.local"); got != "24:0a:c4:1a:2b:3c" {
		t.Errorf("Cached(other) = %q", got)
	}
}

func TestSend(t *testing.T) {
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	mac, _ := ParseMAC("9C:13:9E:ED:D5:EC")
	if err := Send(mac, []string{conn.LocalAddr().String()}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	buf := make([]byte, 256)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := conn.ReadFrom(buf)
	if err != nil || !bytes.Equal(buf[:n], MagicPacket(mac)) {
		t.Errorf("received %d bytes, %v", n, err)
	}
}
