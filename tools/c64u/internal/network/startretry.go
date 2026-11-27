package network

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// resolveError is how the device answers a stream start when it cannot find
// the destination's hardware address on its wired port. Firmware 1.2.1 sends
// the lookup packet out of the Wi-Fi interface when Ethernet and Wi-Fi share a
// subnet, so it fails for every destination that has not itself talked to the
// wired address (checked 2026-11-27; fixed upstream in 1541ultimate 5d42e1db).
const resolveError = "Network Host Resolve Error"

// resolveHint is what to do about resolveError.
const resolveHint = "the device could not find the destination on its wired port. " +
	"Firmware 1.2.1 looks it up over Wi-Fi when Ethernet and Wi-Fi share a subnet: " +
	"turn Wi-Fi off, or give Wi-Fi a static address with netmask 255.255.255.255"

const (
	startRetryInterval = 2 * time.Second
	// StartRetryLimit is how long StartWithRetry keeps trying.
	StartRetryLimit = 6 * time.Second
)

// ExplainStartError adds the cause and the way out to the device's resolve
// error; other errors are returned unchanged.
func ExplainStartError(err error) error {
	if err == nil || !strings.Contains(err.Error(), resolveError) {
		return err
	}
	return fmt.Errorf("%w; %s", err, resolveHint)
}

// StartWithRetry starts a stream, retrying a few times while the device cannot
// find the destination: it succeeds once the destination has talked to the
// wired address. It prints a notice once and explains the error when it gives up.
func StartWithRetry(start func(ip string) error, ip string, out io.Writer, sleep func(time.Duration)) error {
	for waited := time.Duration(0); ; waited += startRetryInterval {
		err := start(ip)
		if err == nil || !strings.Contains(err.Error(), resolveError) {
			return err
		}
		if waited >= StartRetryLimit {
			return ExplainStartError(err)
		}
		if waited == 0 {
			fmt.Fprintf(out, "The device could not find %s, retrying for up to %v...\n", ip, StartRetryLimit)
		}
		sleep(startRetryInterval)
	}
}
