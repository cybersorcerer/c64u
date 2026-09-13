//go:build !darwin && !windows

package video

import (
	"fmt"
	"runtime"
)

const VideoPort = 11000

func Listen(localIP string, startFn func(ip string) error, stopFn func() error, sendFn func([]byte) error, resetFn func() error) error {
	return fmt.Errorf("video stream is not available on %s builds", runtime.GOOS)
}
