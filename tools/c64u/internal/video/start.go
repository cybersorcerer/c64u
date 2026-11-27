package video

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// streamBusy is how firmware 1.2.1 answers a video start while it still
// releases the previous stream; seen for about 50 seconds after the window
// closed. The text is misleading: the device resolves nothing.
const streamBusy = "Network Host Resolve Error"

const (
	startRetryInterval = 5 * time.Second
	startRetryLimit    = 90 * time.Second
)

// startStream starts the stream, retrying while the device is busy with the
// previous one. It does not stop the stream between tries, which kept the
// device from recovering.
func startStream(start func(ip string) error, ip string, out io.Writer, sleep func(time.Duration)) error {
	for waited := time.Duration(0); ; waited += startRetryInterval {
		err := start(ip)
		if err == nil || !strings.Contains(err.Error(), streamBusy) {
			return err
		}
		if waited >= startRetryLimit {
			return fmt.Errorf("%w; the device did not release the previous video stream, restart it with: c64u machine reboot", err)
		}
		if waited == 0 {
			fmt.Fprintf(out, "The device is still releasing the previous video stream, retrying for up to %v...\n", startRetryLimit)
		}
		sleep(startRetryInterval)
	}
}
