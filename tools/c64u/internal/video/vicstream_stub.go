//go:build !darwin && !windows && !(linux && cgo)

package video

import (
	"fmt"
	"runtime"
)

const VideoPort = 11000

func Listen(localIP string, startFn func(ip string) error, stopFn func() error, dev Device) error {
	return fmt.Errorf("video stream is not available on %s builds", runtime.GOOS)
}
