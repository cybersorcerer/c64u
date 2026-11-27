package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseInputEvent(t *testing.T) {
	tests := []struct {
		arg  string
		want InputEvent
	}{
		{"tap:commodore+o", InputEvent{Kind: "keyboard", Inputs: []string{"commodore", "o"}, Transition: "tap"}},
		{"press:left_shift", InputEvent{Kind: "keyboard", Inputs: []string{"left_shift"}, Transition: "press"}},
		{"release:a", InputEvent{Kind: "keyboard", Inputs: []string{"a"}, Transition: "release"}},
		{"tap:restore", InputEvent{Kind: "keyboard", Inputs: []string{"restore"}, Transition: "tap"}},
		{"press:joy2:up+fire", InputEvent{Kind: "joystick", Port: 2, Inputs: []string{"up", "fire"}, Transition: "press"}},
		{"release:joy1:left", InputEvent{Kind: "joystick", Port: 1, Inputs: []string{"left"}, Transition: "release"}},
		{"release-all", InputEvent{Kind: "release_all"}},
		{"release_all", InputEvent{Kind: "release_all"}},
	}

	for _, tt := range tests {
		got, err := ParseInputEvent(tt.arg)
		if err != nil {
			t.Errorf("ParseInputEvent(%q) returned error: %v", tt.arg, err)
			continue
		}
		if got.Kind != tt.want.Kind || got.Port != tt.want.Port || got.Transition != tt.want.Transition {
			t.Errorf("ParseInputEvent(%q) = %+v, want %+v", tt.arg, got, tt.want)
		}
		if strings.Join(got.Inputs, ",") != strings.Join(tt.want.Inputs, ",") {
			t.Errorf("ParseInputEvent(%q) inputs = %v, want %v", tt.arg, got.Inputs, tt.want.Inputs)
		}
	}
}

func TestParseInputEvent_Rejects(t *testing.T) {
	tests := []struct {
		arg     string
		wantMsg string
	}{
		{"commodore+o", "is not an event"},
		{"hold:a", "unknown transition"},
		{"tap:", "is not an event"},
		{"tap:comodore", "unknown keyboard input"},
		{"tap:a+a", "listed twice"},
		{"press:restore", "has to stand alone"},
		{"tap:restore+a", "has to stand alone"},
		{"press:joy3:up", "does not exist"},
		{"press:joy2:north", "unknown joystick input"},
		{"tap:a+b+c+d+e+f+g+h+i", "1 to 8 inputs"},
	}

	for _, tt := range tests {
		_, err := ParseInputEvent(tt.arg)
		if err == nil {
			t.Errorf("ParseInputEvent(%q) accepted an invalid event", tt.arg)
			continue
		}
		if !strings.Contains(err.Error(), tt.wantMsg) {
			t.Errorf("ParseInputEvent(%q) error = %q, want it to mention %q", tt.arg, err, tt.wantMsg)
		}
	}
}

func TestMachineInput_SendsBatch(t *testing.T) {
	var got struct {
		Events []InputEvent `json:"events"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/machine:input" {
			t.Errorf("path = %s, want /v1/machine:input", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("body is not the expected JSON: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"errors": []string{}})
	}))
	defer srv.Close()

	events := []InputEvent{
		{Kind: "joystick", Port: 2, Inputs: []string{"up", "fire"}, Transition: "press"},
		{Kind: "release_all"},
	}
	resp, err := newTestClient(srv.URL).MachineInput(events)
	if err != nil {
		t.Fatalf("MachineInput returned error: %v", err)
	}
	if resp.HasErrors() {
		t.Errorf("response carries errors: %v", resp.Errors)
	}
	if len(got.Events) != 2 || got.Events[0].Port != 2 || got.Events[1].Kind != "release_all" {
		t.Errorf("device received %+v", got.Events)
	}
	// A release_all event must not carry the other fields.
	if got.Events[1].Transition != "" || len(got.Events[1].Inputs) != 0 {
		t.Errorf("release_all was sent with extra fields: %+v", got.Events[1])
	}
}

func TestMachineInput_StatusCodes(t *testing.T) {
	for _, code := range []int{http.StatusNotFound, http.StatusNotImplemented, http.StatusBadRequest} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))

		resp, err := newTestClient(srv.URL).MachineInput([]InputEvent{{Kind: "release_all"}})
		srv.Close()

		if err != nil {
			t.Fatalf("MachineInput returned a transport error for %d: %v", code, err)
		}
		if resp.StatusCode != code {
			t.Errorf("StatusCode = %d, want %d", resp.StatusCode, code)
		}
		if !resp.HasErrors() {
			t.Errorf("HTTP %d produced no error entry", code)
		}
	}
}

func TestMachineInput_BatchLimits(t *testing.T) {
	client := newTestClient("http://127.0.0.1:1")

	if _, err := client.MachineInput(nil); err == nil {
		t.Error("an empty batch was accepted")
	}

	tooMany := make([]InputEvent, 65)
	for i := range tooMany {
		tooMany[i] = InputEvent{Kind: "release_all"}
	}
	if _, err := client.MachineInput(tooMany); err == nil {
		t.Error("a batch of 65 events was accepted")
	}

	// 64 events is allowed by count, but long input lists blow the body limit,
	// and the two limits are checked separately.
	tooBig := make([]InputEvent, 64)
	for i := range tooBig {
		tooBig[i] = InputEvent{
			Kind:       "keyboard",
			Inputs:     []string{"left_shift", "right_shift", "commodore", "ctrl", "run_stop", "clr_home", "cursor_up_down", "cursor_left_right"},
			Transition: "press",
		}
	}
	_, err := client.MachineInput(tooBig)
	if err == nil || !strings.Contains(err.Error(), "4096") {
		t.Errorf("oversized body error = %v, want it to name the 4096 byte limit", err)
	}
}

func TestDecodeMenuScreen(t *testing.T) {
	body := make([]byte, 2*menuScreenCells)
	for i := range body[:menuScreenCells] {
		body[i] = ' '
	}
	copy(body, []byte("ULTIMATE MENU"))
	body[menuScreenWidth] = 0x00 // unprintable, has to become a placeholder

	screen, err := DecodeMenuScreen(body)
	if err != nil {
		t.Fatalf("DecodeMenuScreen returned error: %v", err)
	}
	if len(screen.Rows) != menuScreenHeight {
		t.Errorf("got %d rows, want %d", len(screen.Rows), menuScreenHeight)
	}
	if screen.Rows[0] != "ULTIMATE MENU" {
		t.Errorf("row 0 = %q, want %q", screen.Rows[0], "ULTIMATE MENU")
	}
	if screen.Rows[1] != "." {
		t.Errorf("row 1 = %q, want the unprintable byte replaced and the rest trimmed", screen.Rows[1])
	}
	if len(screen.Colors) != menuScreenCells {
		t.Errorf("got %d colour bytes, want %d", len(screen.Colors), menuScreenCells)
	}
}

func TestDecodeMenuScreen_Short(t *testing.T) {
	if _, err := DecodeMenuScreen(make([]byte, 100)); err == nil {
		t.Error("a truncated menu screen was accepted")
	}
}

func TestInputEvent_MouseJSON(t *testing.T) {
	cases := map[string]InputEvent{
		`{"kind":"mouse","move":{"x":5,"y":-3}}`:                  {Kind: "mouse", Move: &MouseMove{X: 5, Y: -3}},
		`{"kind":"mouse","move":{"x":0,"y":4}}`:                   {Kind: "mouse", Move: &MouseMove{Y: 4}},
		`{"kind":"mouse","wheel":{"vertical":-1}}`:                {Kind: "mouse", Wheel: &MouseWheel{Vertical: -1}},
		`{"kind":"mouse","inputs":["left"],"transition":"press"}`: {Kind: "mouse", Inputs: []string{"left"}, Transition: "press"},
		`{"kind":"release_all"}`:                                  {Kind: "release_all"},
	}
	for want, ev := range cases {
		got, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}
}

func TestSplitInputBatches(t *testing.T) {
	if got := SplitInputBatches(nil); got != nil {
		t.Errorf("SplitInputBatches(nil) = %v, want nil", got)
	}

	short := make([]InputEvent, 130)
	for i := range short {
		short[i] = InputEvent{Kind: "release_all"}
	}
	var lens []int
	for _, b := range SplitInputBatches(short) {
		lens = append(lens, len(b))
	}
	if len(lens) != 3 || lens[0] != 64 || lens[1] != 64 || lens[2] != 2 {
		t.Errorf("batch sizes = %v, want [64 64 2]", lens)
	}

	// Long names: 64 of these exceed the body limit, so the split has to
	// measure bytes as well as count events.
	long := make([]InputEvent, 64)
	for i := range long {
		long[i] = InputEvent{
			Kind:       "keyboard",
			Inputs:     []string{"left_shift", "right_shift", "commodore", "ctrl", "run_stop", "clr_home", "cursor_up_down", "cursor_left_right"},
			Transition: []string{"press", "release"}[i%2],
		}
	}
	batches := SplitInputBatches(long)
	if len(batches) < 2 {
		t.Fatalf("got %d batch(es), want the body limit to force at least 2", len(batches))
	}
	var flat []InputEvent
	for _, b := range batches {
		body, _ := json.Marshal(map[string]interface{}{"events": b})
		if len(body) >= MaxInputBodyBytes {
			t.Errorf("batch serialises to %d bytes, not under %d", len(body), MaxInputBodyBytes)
		}
		if len(b) > MaxInputEvents {
			t.Errorf("batch has %d events, over %d", len(b), MaxInputEvents)
		}
		flat = append(flat, b...)
	}
	if len(flat) != len(long) {
		t.Fatalf("got %d events back, want %d", len(flat), len(long))
	}
	for i := range flat {
		if flat[i].Transition != long[i].Transition {
			t.Fatalf("event %d out of order", i)
		}
	}
}

func TestParseInputEvent_Mouse(t *testing.T) {
	cases := map[string]string{
		"tap:mouse:left":         `{"kind":"mouse","inputs":["left"],"transition":"tap"}`,
		"press:mouse:left+right": `{"kind":"mouse","inputs":["left","right"],"transition":"press"}`,
		"release:mouse:middle":   `{"kind":"mouse","inputs":["middle"],"transition":"release"}`,
		"move:10,-5":             `{"kind":"mouse","move":{"x":10,"y":-5}}`,
		"move:0,127":             `{"kind":"mouse","move":{"x":0,"y":127}}`,
		"wheel:-2":               `{"kind":"mouse","wheel":{"vertical":-2}}`,
		"wheel:1,-3":             `{"kind":"mouse","wheel":{"vertical":1,"horizontal":-3}}`,
		"wheel:0,64":             `{"kind":"mouse","wheel":{"horizontal":64}}`,
	}
	for arg, want := range cases {
		ev, err := ParseInputEvent(arg)
		if err != nil {
			t.Errorf("%s: %v", arg, err)
			continue
		}
		got, _ := json.Marshal(ev)
		if string(got) != want {
			t.Errorf("%s = %s, want %s", arg, got, want)
		}
	}
}

// Moves beyond one report become a path whose steps each fit and add up exactly.
func TestParseInputEvent_MousePath(t *testing.T) {
	cases := []struct {
		arg   string
		x, y  int
		steps int
	}{
		{"move:300,-40", 300, -40, 3},
		{"move:-128,0", -128, 0, 2},
		{"move:5,-1000", 5, -1000, 8},
		{"move:32512,0", 32512, 0, 256},
	}
	for _, c := range cases {
		ev, err := ParseInputEvent(c.arg)
		if err != nil {
			t.Errorf("%s: %v", c.arg, err)
			continue
		}
		if ev.Kind != "mouse" || ev.Move != nil || len(ev.Path) != c.steps {
			t.Errorf("%s: kind %q, move %v, %d steps; want a %d step path", c.arg, ev.Kind, ev.Move, len(ev.Path), c.steps)
			continue
		}
		sx, sy := 0, 0
		for i, s := range ev.Path {
			if s[0] < -127 || s[0] > 127 || s[1] < -127 || s[1] > 127 {
				t.Errorf("%s: step %d = %v exceeds ±127", c.arg, i, s)
			}
			sx += s[0]
			sy += s[1]
		}
		if sx != c.x || sy != c.y {
			t.Errorf("%s: steps add up to %d,%d", c.arg, sx, sy)
		}
	}
}

func TestParseInputEvent_MouseRejects(t *testing.T) {
	for _, arg := range []string{
		"tap:mouse:left+left", "tap:mouse:side", "tap:mouse:",
		"move:32513,0", "move:0,-32513", "move:0,0", "move:1", "move:a,b", "move:",
		"wheel:0", "wheel:65", "wheel:-65", "wheel:0,0", "wheel:1,2,3", "wheel:x",
	} {
		if ev, err := ParseInputEvent(arg); err == nil {
			t.Errorf("%s accepted as %+v", arg, ev)
		}
	}
}

// Two long paths fit the byte limit but not the device's 1024 JSON values
// (review finding): the batch is refused locally, and the split cuts it.
func TestMachineInput_ValueLimit(t *testing.T) {
	a, _ := ParseInputEvent("move:32512,0")
	b, _ := ParseInputEvent("move:12700,0") // 356 steps in all, about 2900 bytes
	events := []InputEvent{a, b}

	c := &Client{}
	if _, err := c.MachineInput(events); err == nil || !strings.Contains(err.Error(), "1024") {
		t.Errorf("err = %v, want the 1024 JSON value limit named", err)
	}
	if got := SplitInputBatches(events); len(got) != 2 {
		t.Errorf("split into %d batch(es), want 2", len(got))
	}
}

// The device wants the body under 4096 bytes, not at most.
func TestInputBodyFits(t *testing.T) {
	if inputBodyFits(4096, 1) || !inputBodyFits(4095, 1024) || inputBodyFits(100, 1025) {
		t.Error("inputBodyFits must accept under 4096 bytes and up to 1024 values only")
	}
}

func TestJSONValues(t *testing.T) {
	ev, _ := ParseInputEvent("move:300,0") // 3 steps
	body, _ := json.Marshal(map[string]interface{}{"events": []InputEvent{ev}})
	// The device counts keys too (found on 1.2.1: two paths of 337 steps in all
	// pass, 338 do not): body object, "events", array = 3; event object, "kind",
	// "mouse", "path", array = 5; 3 steps x (array + 2 ints) = 9.
	if got := jsonValues(body); got != 3+5+9 {
		t.Errorf("jsonValues = %d, want 17", got)
	}

	// The device boundary itself: 337 steps in two events fit, 338 do not.
	for steps, fits := range map[int]bool{337: true, 338: false} {
		a, b := InputEvent{Kind: "mouse", Path: make([][2]int, steps/2)}, InputEvent{Kind: "mouse", Path: make([][2]int, steps-steps/2)}
		body, _ := json.Marshal(map[string]interface{}{"events": []InputEvent{a, b}})
		if got := jsonValues(body) <= MaxInputJSONValues; got != fits {
			t.Errorf("%d steps: fits = %v, the device says %v", steps, got, fits)
		}
	}
}

// The extremes of int must not slip past the range checks (review finding).
func TestParseInputEvent_MouseExtremes(t *testing.T) {
	for _, arg := range []string{"move:-9223372036854775808,0", "move:0,-9223372036854775808", "wheel:-9223372036854775808", "wheel:0,-9223372036854775808"} {
		if ev, err := ParseInputEvent(arg); err == nil {
			t.Errorf("%s accepted as %+v", arg, ev)
		}
	}
}
