// Package wol wakes a C64 Ultimate that was switched off with machine poweroff.
//
// The device listens for a Wake-on-LAN magic packet on its Wi-Fi interface
// only, and only with "Wake On Wi-Fi" in "U64 Specific Settings" enabled and
// saved to flash (checked on firmware 1.2.1, 2026-11-27). The firmware matches
// the pattern anywhere in the frame, so a UDP broadcast on port 9 or 7 does.
//
// A device that is off cannot say what its MAC is, so the address comes from
// the command line, the device entry in config.toml, or a cache that
// machine poweroff fills while the device can still answer.
package wol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// Where ChooseMAC found the address.
const (
	SourceFlag   = "--mac"
	SourceConfig = "config file"
	SourceCache  = "last poweroff"
)

// ErrNoMAC means no source gave an address at all.
var ErrNoMAC = errors.New("no Wi-Fi MAC address known for this device")

// Destinations is where Send goes by default: the limited broadcast, on both
// ports Wake-on-LAN tools use.
var Destinations = []string{"255.255.255.255:9", "255.255.255.255:7"}

// MagicPacket is six 0xFF bytes followed by the MAC sixteen times.
func MagicPacket(mac net.HardwareAddr) []byte {
	p := bytes.Repeat([]byte{0xff}, 6)
	for i := 0; i < 16; i++ {
		p = append(p, mac...)
	}
	return p
}

// ParseMAC reads a 6-byte MAC. A broadcast or all-zero address is refused:
// no station owns it, and the firmware ignores a packet for it.
func ParseMAC(s string) (net.HardwareAddr, error) {
	mac, err := net.ParseMAC(strings.TrimSpace(s))
	if err != nil || len(mac) != 6 {
		return nil, fmt.Errorf("%q is not a MAC address like 9C:13:9E:ED:D5:EC", s)
	}
	if bytes.Equal(mac, bytes.Repeat([]byte{0xff}, 6)) || bytes.Equal(mac, make([]byte, 6)) {
		return nil, fmt.Errorf("%s belongs to no device", mac)
	}
	return mac, nil
}

// ChooseMAC takes the first address given: the flag, then the config file,
// then the cache. A bad address is an error rather than a reason to try the
// next source, so a typo does not wake some other device.
func ChooseMAC(flag, configured, cached string) (net.HardwareAddr, string, error) {
	for _, c := range []struct{ value, source string }{
		{flag, SourceFlag}, {configured, SourceConfig}, {cached, SourceCache},
	} {
		if c.value == "" {
			continue
		}
		mac, err := ParseMAC(c.value)
		if err != nil {
			return nil, "", fmt.Errorf("%s: %w", c.source, err)
		}
		return mac, c.source, nil
	}
	return nil, "", ErrNoMAC
}

// Send broadcasts the magic packet for mac to every destination.
func Send(mac net.HardwareAddr, destinations []string) error {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return err
	}
	defer conn.Close()
	packet := MagicPacket(mac)
	for _, d := range destinations {
		addr, err := net.ResolveUDPAddr("udp4", d)
		if err != nil {
			return err
		}
		if _, err := conn.WriteTo(packet, addr); err != nil {
			return fmt.Errorf("send to %s: %w", d, err)
		}
	}
	return nil
}

// CachePath is the cache next to config.toml; empty without a home directory.
func CachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "c64u", "wifi_macs.json")
}

// Cached returns the MAC remembered for host, or "".
func Cached(path, host string) string {
	return readCache(path)[strings.ToLower(host)]
}

// Remember stores mac for host in the cache.
func Remember(path, host string, mac net.HardwareAddr) error {
	cache := readCache(path)
	cache[strings.ToLower(host)] = mac.String()
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// readCache reads the cache; a missing or broken file is an empty cache.
func readCache(path string) map[string]string {
	cache := map[string]string{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &cache)
	}
	return cache
}
