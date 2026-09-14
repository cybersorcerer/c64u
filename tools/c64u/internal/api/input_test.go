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
