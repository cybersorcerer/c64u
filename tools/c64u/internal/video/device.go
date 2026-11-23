package video

import (
	"time"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/inputsync"
)

// Device is the part of the C64 Ultimate API the video window drives; *api.Client
// is one. It lives outside the build-tagged files so the stub shares it.
type Device interface {
	inputsync.Sender
	SendKeyBytes(data []byte, delay time.Duration) error
	MachineMenuButton() (*api.Response, error)
	MachineReset() (*api.Response, error)
}
