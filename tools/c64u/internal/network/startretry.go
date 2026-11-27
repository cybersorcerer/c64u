package network

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// streamBusy is how firmware 1.2.1 answers a stream start while it is busy: for
// about 50 seconds after the video window closed, and now and then for debug. The text is misleading: the device resolves nothing.
const streamBusy = "Network Host Resolve Error"

const (
	startRetryInterval = 5 * time.Second
	startRetryLimit    = 90 * time.Second
)

// StartWithRetry starts a stream, retrying while the device is busy. It prints
// notice once when the first try was busy and adds giveUpHint to the error when
// it gives up. It does not stop the stream between tries, which kept the device
// from recovering.
func StartWithRetry(start func(ip string) error, ip, notice, giveUpHint string, out io.Writer, sleep func(time.Duration)) error {
	for waited := time.Duration(0); ; waited += startRetryInterval {
		err := start(ip)
		if err == nil || !strings.Contains(err.Error(), streamBusy) {
			return err
		}
		if waited >= startRetryLimit {
			return fmt.Errorf("%w; %s", err, giveUpHint)
		}
		if waited == 0 {
			fmt.Fprintln(out, notice)
		}
		sleep(startRetryInterval)
	}
}
