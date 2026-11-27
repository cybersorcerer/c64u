package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Limits the REST API enforces on one batch. The two are independent: 64 events
// with short names stay inside both, 64 events with long ones do not, so the
// body has to be measured as well as counted.
const (
	MaxInputEvents    = 64
	MaxInputBodyBytes = 4096
	MaxKeyboardInputs = 8
	MaxJoystickInputs = 7
	MaxMouseInputs    = 3
)

// InputEvent is one entry of a machine:input batch. A release_all event carries
// nothing else, which is why every other field is omitted when empty. Mouse
// buttons use Inputs and Transition like keys do; Move, Wheel and Path are the
// other mouse reports.
type InputEvent struct {
	Kind       string      `json:"kind"`
	Port       int         `json:"port,omitempty"`
	Inputs     []string    `json:"inputs,omitempty"`
	Transition string      `json:"transition,omitempty"`
	Move       *MouseMove  `json:"move,omitempty"`
	Wheel      *MouseWheel `json:"wheel,omitempty"`
	Path       [][2]int    `json:"path,omitempty"`
	IntervalMS int         `json:"interval_ms,omitempty"`
}

// MouseMove is one relative mouse report, -127..127 on each axis; positive is
// right on x and down on y.
type MouseMove struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// MouseWheel turns the wheels by whole detents, -64..64; positive vertical
// turns away from the user, positive horizontal to the right.
type MouseWheel struct {
	Vertical   int `json:"vertical,omitempty"`
	Horizontal int `json:"horizontal,omitempty"`
}

// SplitInputBatches cuts events into batches the device accepts, keeping their
// order: at most MaxInputEvents each, and each serialising to at most
// MaxInputBodyBytes as a request body.
func SplitInputBatches(events []InputEvent) [][]InputEvent {
	// {"events":[]} plus one comma per event after the first
	const overhead = len(`{"events":[]}`)
	var batches [][]InputEvent
	var cur []InputEvent
	size := overhead
	for _, ev := range events {
		encoded, _ := json.Marshal(ev)
		add := len(encoded)
		if len(cur) > 0 {
			add++
		}
		if len(cur) == MaxInputEvents || (len(cur) > 0 && size+add > MaxInputBodyBytes) {
			batches = append(batches, cur)
			cur, size, add = nil, overhead, len(encoded)
		}
		cur = append(cur, ev)
		size += add
	}
	if len(cur) > 0 {
		batches = append(batches, cur)
	}
	return batches
}

// The names the firmware accepts. Validating here rather than letting the
// device answer 400 keeps a typo from needing a reachable machine to be found.
var keyboardInputs = map[string]bool{
	"inst_del": true, "return": true, "cursor_left_right": true, "f7": true,
	"f1": true, "f3": true, "f5": true, "cursor_up_down": true,
	"left_shift": true, "right_shift": true, "plus": true, "minus": true,
	"period": true, "colon": true, "at": true, "comma": true, "pound": true,
	"star": true, "semicolon": true, "clr_home": true, "equals": true,
	"arrow_up": true, "slash": true, "arrow_left": true, "ctrl": true,
	"space": true, "commodore": true, "run_stop": true, "restore": true,
}

var mouseInputs = map[string]bool{"left": true, "right": true, "middle": true}

var joystickInputs = map[string]bool{
	"up": true, "down": true, "left": true, "right": true,
	"fire": true, "fire2": true, "fire3": true,
}

// MaxMousePathSteps is the most steps one path event takes; with 127 counts per
// step that bounds a move to 32512 counts on either axis.
const MaxMousePathSteps = 256

func init() {
	for c := 'a'; c <= 'z'; c++ {
		keyboardInputs[string(c)] = true
	}
	for d := '0'; d <= '9'; d++ {
		keyboardInputs[string(d)] = true
	}
}

// ParseInputEvent turns one command line argument into an event.
//
//	tap:commodore+o      a keyboard event
//	press:joy2:up+fire   a joystick event on port 2
//	tap:mouse:left       a mouse button event (port 1)
//	move:300,-40         a relative mouse move
//	wheel:-2             turns the mouse wheel
//	release-all          clears every injected input
func ParseInputEvent(arg string) (InputEvent, error) {
	if arg == "release-all" || arg == "release_all" {
		return InputEvent{Kind: "release_all"}, nil
	}
	if rest, ok := strings.CutPrefix(arg, "move:"); ok {
		return mouseMoveEvent(rest)
	}
	if rest, ok := strings.CutPrefix(arg, "wheel:"); ok {
		return mouseWheelEvent(rest)
	}

	transition, rest, found := strings.Cut(arg, ":")
	if !found || rest == "" {
		return InputEvent{}, fmt.Errorf("%q is not an event: expected press:, release:, tap: or release-all", arg)
	}
	switch transition {
	case "press", "release", "tap":
	default:
		return InputEvent{}, fmt.Errorf("unknown transition %q in %q: expected press, release or tap", transition, arg)
	}

	if buttons, ok := strings.CutPrefix(rest, "mouse:"); ok {
		names := strings.Split(buttons, "+")
		if err := checkInputs(names, mouseInputs, MaxMouseInputs, "mouse"); err != nil {
			return InputEvent{}, err
		}
		return InputEvent{Kind: "mouse", Inputs: names, Transition: transition}, nil
	}
	if port, names, ok := cutJoystick(rest); ok {
		return joystickEvent(port, names, transition)
	}
	return keyboardEvent(strings.Split(rest, "+"), transition)
}

// cutJoystick recognises the "joy1:"/"joy2:" prefix that separates a joystick
// event from a keyboard one.
func cutJoystick(rest string) (port int, names []string, ok bool) {
	head, tail, found := strings.Cut(rest, ":")
	if !found || !strings.HasPrefix(head, "joy") {
		return 0, nil, false
	}
	port, err := strconv.Atoi(strings.TrimPrefix(head, "joy"))
	if err != nil {
		return 0, nil, false
	}
	return port, strings.Split(tail, "+"), true
}

func joystickEvent(port int, names []string, transition string) (InputEvent, error) {
	if port != 1 && port != 2 {
		return InputEvent{}, fmt.Errorf("joystick port %d does not exist: use joy1 or joy2", port)
	}
	if err := checkInputs(names, joystickInputs, MaxJoystickInputs, "joystick"); err != nil {
		return InputEvent{}, err
	}
	return InputEvent{Kind: "joystick", Port: port, Inputs: names, Transition: transition}, nil
}

func keyboardEvent(names []string, transition string) (InputEvent, error) {
	if err := checkInputs(names, keyboardInputs, MaxKeyboardInputs, "keyboard"); err != nil {
		return InputEvent{}, err
	}
	// restore is wired to the NMI rather than the key matrix, so the firmware
	// only accepts it on its own and only as a tap.
	for _, name := range names {
		if name == "restore" && (len(names) > 1 || transition != "tap") {
			return InputEvent{}, fmt.Errorf("restore has to stand alone and can only be tapped: use tap:restore")
		}
	}
	return InputEvent{Kind: "keyboard", Inputs: names, Transition: transition}, nil
}

func checkInputs(names []string, valid map[string]bool, limit int, kind string) error {
	if len(names) < 1 || len(names) > limit {
		return fmt.Errorf("a %s event takes 1 to %d inputs, got %d", kind, limit, len(names))
	}
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if !valid[name] {
			return fmt.Errorf("unknown %s input %q: valid names are %s", kind, name, strings.Join(sortedKeys(valid), ", "))
		}
		if seen[name] {
			return fmt.Errorf("%s input %q is listed twice", kind, name)
		}
		seen[name] = true
	}
	return nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// MachineInput applies a batch of keyboard, joystick, mouse and release-all events.
func (c *Client) MachineInput(events []InputEvent) (*Response, error) {
	if len(events) < 1 || len(events) > MaxInputEvents {
		return nil, fmt.Errorf("a batch takes 1 to %d events, got %d", MaxInputEvents, len(events))
	}

	body := map[string]interface{}{"events": events}

	// The device refuses an oversized body outright, so measure it here and say
	// which of the two limits was hit rather than passing on a bare 400.
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	if len(encoded) > MaxInputBodyBytes {
		return nil, fmt.Errorf("the batch serialises to %d bytes, over the %d byte limit: send fewer events per call",
			len(encoded), MaxInputBodyBytes)
	}

	return c.PostJSON("/v1/machine:input", body)
}

// MachineInputState returns the inputs the REST API currently holds down.
func (c *Client) MachineInputState() (*Response, error) {
	return c.Get("/v1/machine:input", nil)
}

// MachineMenuScreen returns the active firmware menu screen: a 40x25 character
// matrix followed by a 40x25 colour matrix.
func (c *Client) MachineMenuScreen() (*Response, error) {
	return c.Get("/v1/machine:menu_screen", nil)
}

// MenuScreen is the decoded form of that binary reply.
type MenuScreen struct {
	Rows   []string
	Colors []byte
}

const (
	menuScreenWidth  = 40
	menuScreenHeight = 25
	menuScreenCells  = menuScreenWidth * menuScreenHeight
)

// DecodeMenuScreen turns the binary reply into text rows. The character matrix
// is ASCII, not screen codes, so only unprintable bytes need substituting.
func DecodeMenuScreen(body []byte) (*MenuScreen, error) {
	if len(body) < menuScreenCells {
		return nil, fmt.Errorf("menu screen is %d bytes, expected at least %d", len(body), menuScreenCells)
	}

	screen := &MenuScreen{Rows: make([]string, 0, menuScreenHeight)}
	for row := 0; row < menuScreenHeight; row++ {
		line := make([]rune, menuScreenWidth)
		for col := 0; col < menuScreenWidth; col++ {
			b := body[row*menuScreenWidth+col]
			if b < 0x20 || b > 0x7e {
				line[col] = '.'
			} else {
				line[col] = rune(b)
			}
		}
		screen.Rows = append(screen.Rows, strings.TrimRight(string(line), " "))
	}

	if len(body) >= 2*menuScreenCells {
		screen.Colors = body[menuScreenCells : 2*menuScreenCells]
	}
	return screen, nil
}

// parseInts reads exactly n comma separated integers.
func parseInts(s string, n int) ([]int, error) {
	parts := strings.Split(s, ",")
	if len(parts) != n {
		return nil, fmt.Errorf("expected %d comma separated numbers, got %q", n, s)
	}
	nums := make([]int, n)
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", p)
		}
		nums[i] = v
	}
	return nums, nil
}

// mouseMoveEvent sends a move that fits one report as it is and splits a larger
// one into a path of steps within ±127 that add up to it exactly.
func mouseMoveEvent(s string) (InputEvent, error) {
	xy, err := parseInts(s, 2)
	if err != nil {
		return InputEvent{}, fmt.Errorf("move:%s: %v, use move:X,Y", s, err)
	}
	x, y := xy[0], xy[1]
	if x == 0 && y == 0 {
		return InputEvent{}, fmt.Errorf("move:%s moves nowhere", s)
	}
	limit := 127 * MaxMousePathSteps
	if abs(x) > limit || abs(y) > limit {
		return InputEvent{}, fmt.Errorf("move:%s is too far: at most %d on each axis in one event", s, limit)
	}
	steps := max((abs(x)+126)/127, (abs(y)+126)/127)
	if steps == 1 {
		return InputEvent{Kind: "mouse", Move: &MouseMove{X: x, Y: y}}, nil
	}
	path := make([][2]int, steps)
	for i := range path {
		path[i] = [2]int{x*(i+1)/steps - x*i/steps, y*(i+1)/steps - y*i/steps}
	}
	return InputEvent{Kind: "mouse", Path: path}, nil
}

func mouseWheelEvent(s string) (InputEvent, error) {
	parts := 1
	if strings.Contains(s, ",") {
		parts = 2
	}
	vh, err := parseInts(s, parts)
	if err != nil {
		return InputEvent{}, fmt.Errorf("wheel:%s: %v, use wheel:V or wheel:V,H", s, err)
	}
	vh = append(vh, 0)
	v, h := vh[0], vh[1]
	if v == 0 && h == 0 {
		return InputEvent{}, fmt.Errorf("wheel:%s turns nothing", s)
	}
	if abs(v) > 64 || abs(h) > 64 {
		return InputEvent{}, fmt.Errorf("wheel:%s is out of range: -64 to 64 detents", s)
	}
	return InputEvent{Kind: "mouse", Wheel: &MouseWheel{Vertical: v, Horizontal: h}}, nil
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
