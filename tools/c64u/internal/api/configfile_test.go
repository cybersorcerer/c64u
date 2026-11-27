package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// config export nests every category twice and adds an errors member.
func TestParseConfigFile_ExportFormat(t *testing.T) {
	data := `{"Audio Mixer": {"Audio Mixer": {"Pan Drive 1": "Left 2"}, "errors": []},
	          "Drive A Settings": {"Drive A Settings": {"Drive Bus ID": 8}, "errors": []}}`
	got, err := ParseConfigFile([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	want := ConfigSettings{
		"Audio Mixer":      {"Pan Drive 1": "Left 2"},
		"Drive A Settings": {"Drive Bus ID": float64(8)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseConfigFile_FlatFormat(t *testing.T) {
	got, err := ParseConfigFile([]byte(`{"Drive A Settings": {"Drive": "Enabled", "Drive Type": "1581"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got["Drive A Settings"]["Drive Type"] != "1581" || len(got["Drive A Settings"]) != 2 {
		t.Errorf("got %v", got)
	}
}

func TestParseConfigFile_Rejects(t *testing.T) {
	for _, data := range []string{`[]`, `{"Audio Mixer": "loud"}`, `not json`, `{}`} {
		if got, err := ParseConfigFile([]byte(data)); err == nil {
			t.Errorf("%s accepted as %v", data, got)
		}
	}
}

func TestIsIdentity(t *testing.T) {
	cases := map[[2]string]bool{
		{"Ethernet Settings", "Static IP"}:       true,
		{"WiFi settings", "Use DHCP"}:            true,
		{"Network Settings", "Host Name"}:        true,
		{"Network Settings", "Unique ID"}:        true,
		{"Network Settings", "TimeZone"}:         false,
		{"Network Settings", "Network Password"}: false,
		{"Drive A Settings", "Drive Bus ID"}:     false,
	}
	for k, want := range cases {
		if got := IsIdentity(k[0], k[1]); got != want {
			t.Errorf("IsIdentity(%q, %q) = %v, want %v", k[0], k[1], got, want)
		}
	}
}

// Only differences go out; numbers compare by value; a setting the device does
// not have is still sent, so the device can name it as unknown.
func TestDiffConfig(t *testing.T) {
	current := ConfigSettings{
		"Drive A Settings": {"Drive Bus ID": float64(8), "Drive Type": "1541"},
		"Audio Mixer":      {"Pan Drive 1": "Left 2"},
	}
	wanted := ConfigSettings{
		"Drive A Settings": {"Drive Bus ID": 8, "Drive Type": "1581"},
		"Audio Mixer":      {"Pan Drive 1": "Left 2", "Vol Ultimate": "0 dB"},
	}
	got := DiffConfig(current, wanted)
	want := []ConfigChange{
		{Category: "Audio Mixer", Item: "Vol Ultimate", Old: nil, New: "0 dB"},
		{Category: "Drive A Settings", Item: "Drive Type", Old: "1541", New: "1581"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

// The device applies what it can and names the rest in a 400.
func TestApplyConfig_ReportsFailures(t *testing.T) {
	var body map[string]map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/configs" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errors": ["'Drive Speed' is not a valid item name in category 'Drive A Settings'."]}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, HTTPClient: srv.Client()}

	changes := []ConfigChange{
		{Category: "Drive A Settings", Item: "Drive Speed", New: "fast"},
		{Category: "Drive A Settings", Item: "Drive Type", Old: "1541", New: "1581"},
	}
	failed, err := c.ApplyConfig(changes)
	if err != nil {
		t.Fatal(err)
	}
	if len(failed) != 1 || body["Drive A Settings"]["Drive Type"] != "1581" {
		t.Errorf("failed = %v, body = %v", failed, body)
	}
}
