//go:build !darwin

package audio

import "fmt"

const AudioPort = 11001

func Listen(localIP string, startFn func(ip string) error, stopFn func() error) error {
	return fmt.Errorf("audio stream is only supported on macOS")
}
