//go:build !darwin && !windows && !(linux && cgo)

package audio

import (
	"fmt"
	"runtime"
)

const AudioPort = 11001

func Listen(localIP string, startFn func(ip string) error, stopFn func() error) error {
	return fmt.Errorf("audio stream is not available on %s builds", runtime.GOOS)
}
