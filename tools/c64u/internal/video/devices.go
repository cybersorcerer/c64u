//go:build darwin || windows || linux

package video

import (
	"math"
	"sort"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/inputsync"
	"github.com/hajimehoshi/ebiten/v2"
)

// padDeadZone keeps a resting stick from drifting the joystick.
const padDeadZone = 0.5

// padFrame is one gamepad's state in Ebiten's standard layout.
type padFrame struct {
	Buttons      map[ebiten.StandardGamepadButton]bool
	AxisX, AxisY float64 // left stick, -1..1; negative Y is up
}

var padDirections = map[ebiten.StandardGamepadButton]string{
	ebiten.StandardGamepadButtonLeftTop:     "up",
	ebiten.StandardGamepadButtonLeftBottom:  "down",
	ebiten.StandardGamepadButtonLeftLeft:    "left",
	ebiten.StandardGamepadButtonLeftRight:   "right",
	ebiten.StandardGamepadButtonRightBottom: "fire",  // A
	ebiten.StandardGamepadButtonRightRight:  "fire2", // B
}

// padKeyboard are pad buttons that press C64 keys, for games that start or
// pause with them.
var padKeyboard = map[ebiten.StandardGamepadButton]string{
	ebiten.StandardGamepadButtonRightLeft:   "space",    // X
	ebiten.StandardGamepadButtonCenterRight: "run_stop", // Menu / Start
}

// padJoystick turns a gamepad into joystick inputs and the C64 keys its extra
// buttons hold.
func padJoystick(p padFrame) (joy, keys inputsync.Set) {
	joy, keys = inputsync.Set{}, inputsync.Set{}
	for button, down := range p.Buttons {
		if !down {
			continue
		}
		if name, ok := padDirections[button]; ok {
			joy[name] = true
		}
		if name, ok := padKeyboard[button]; ok {
			keys[name] = true
		}
	}
	switch {
	case p.AxisX > padDeadZone:
		joy["right"] = true
	case p.AxisX < -padDeadZone:
		joy["left"] = true
	}
	switch {
	case p.AxisY > padDeadZone:
		joy["down"] = true
	case p.AxisY < -padDeadZone:
		joy["up"] = true
	}
	return joy, keys
}

// assignPorts puts the first gamepad on port 2, where most games read, and the
// second on port 1. Further pads are ignored.
func assignPorts(ids []ebiten.GamepadID, swapped bool) map[ebiten.GamepadID]int {
	sorted := append([]ebiten.GamepadID(nil), ids...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	ports := [2]int{2, 1}
	if swapped {
		ports = [2]int{1, 2}
	}
	assigned := map[ebiten.GamepadID]int{}
	for i, id := range sorted {
		if i >= len(ports) {
			break
		}
		assigned[id] = ports[i]
	}
	return assigned
}

// keyJoystick reads the arrows and left Ctrl as a joystick.
func keyJoystick(f hostFrame) inputsync.Set {
	keys := map[ebiten.Key]string{
		ebiten.KeyArrowUp:     "up",
		ebiten.KeyArrowDown:   "down",
		ebiten.KeyArrowLeft:   "left",
		ebiten.KeyArrowRight:  "right",
		ebiten.KeyControlLeft: "fire",
	}
	joy := inputsync.Set{}
	for key, name := range keys {
		if f.Pressed[key] {
			joy[name] = true
		}
	}
	return joy
}

// mouseTracker turns cursor positions into mouse counts. The window is drawn
// at twice the C64's size, so two host pixels make one count; the odd pixel
// is carried rather than lost. Wheel fractions from touchpads are carried too.
type mouseTracker struct {
	started      bool
	lastX, lastY int
	restX, restY int
	wheelX       float64
	wheelY       float64
}

func (m *mouseTracker) reset() { *m = mouseTracker{} }

// step takes the cursor position and this frame's wheel offsets and returns
// the motion and whole wheel detents to send. Ebiten's positive wheel Y turns
// away from the user, like the API's positive vertical.
func (m *mouseTracker) step(x, y int, wheelX, wheelY float64) (dx, dy, wheelV, wheelH int) {
	if !m.started {
		m.started, m.lastX, m.lastY = true, x, y
	}
	rawX := x - m.lastX + m.restX
	rawY := y - m.lastY + m.restY
	m.lastX, m.lastY = x, y
	dx, dy = rawX/windowScale, rawY/windowScale
	m.restX, m.restY = rawX-dx*windowScale, rawY-dy*windowScale

	m.wheelX += wheelX
	m.wheelY += wheelY
	wheelH, wheelV = int(math.Trunc(m.wheelX)), int(math.Trunc(m.wheelY))
	m.wheelX -= float64(wheelH)
	m.wheelY -= float64(wheelV)
	return dx, dy, wheelV, wheelH
}
