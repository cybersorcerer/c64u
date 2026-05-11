//go:build !darwin

package video

import "fmt"

const VideoPort = 11000

func Listen(localIP string, startFn func(ip string) error, stopFn func() error, sendFn func([]byte) error, resetFn func() error) error {
	return fmt.Errorf("video stream is only supported on macOS")
}
