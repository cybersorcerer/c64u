package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/output"
)

// fakeDevice serves a configuration and records every write, with the
// password each request carried.
type fakeDevice struct {
	config   map[string]map[string]interface{}
	password string
	writes   []string // "POST <body>" or "PUT <path>", in order
	auth     []string // X-Password of each write
}

func (d *fakeDevice) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Password") != d.password {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"errors": ["Forbidden."]}`))
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/configs":
		var cats []string
		for c := range d.config {
			cats = append(cats, c)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"categories": cats, "errors": []string{}})
	case r.Method == http.MethodGet:
		cat := strings.TrimPrefix(r.URL.Path, "/v1/configs/")
		json.NewEncoder(w).Encode(map[string]interface{}{cat: d.config[cat], "errors": []string{}})
	case r.Method == http.MethodPost:
		var body map[string]map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		enc, _ := json.Marshal(body)
		d.writes = append(d.writes, "POST "+string(enc))
		d.auth = append(d.auth, r.Header.Get("X-Password"))
		for cat, items := range body {
			for k, v := range items {
				d.config[cat][k] = v
				if cat == "Network Settings" && k == "Network Password" {
					d.password = v.(string) // wirkt sofort, wie am Gerät
				}
			}
		}
		w.Write([]byte(`{"errors": []}`))
	default:
		d.writes = append(d.writes, "PUT "+r.URL.Path)
		d.auth = append(d.auth, r.Header.Get("X-Password"))
		w.Write([]byte(`{"errors": []}`))
	}
}

func newFakeDevice(t *testing.T) *fakeDevice {
	d := &fakeDevice{config: map[string]map[string]interface{}{
		"Drive A Settings":  {"Drive Type": "1541"},
		"Ethernet Settings": {"Use DHCP": "Enabled", "Static IP": "192.168.2.64"},
		"Network Settings":  {"Host Name": "C64U", "Network Password": "", "TimeZone": "Western Europe"},
	}}
	srv := httptest.NewServer(d)
	t.Cleanup(srv.Close)
	apiClient = &api.Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	formatter = output.NewFormatter(false)
	jsonOut = false
	return d
}

func writeFile(t *testing.T, settings map[string]map[string]interface{}) string {
	path := filepath.Join(t.TempDir(), "cfg.json")
	data, _ := json.Marshal(settings)
	os.WriteFile(path, data, 0o644)
	return path
}

var changedFile = map[string]map[string]interface{}{
	"Drive A Settings":  {"Drive Type": "1581"},
	"Ethernet Settings": {"Use DHCP": "Enabled", "Static IP": "192.168.2.99"},
	"Network Settings":  {"Host Name": "OTHER", "Network Password": "secret", "TimeZone": "Western Europe"},
}

func TestApplyConfigFile_DryRunWritesNothing(t *testing.T) {
	d := newFakeDevice(t)
	applyConfigFile(writeFile(t, changedFile), true, true)
	if len(d.writes) != 0 {
		t.Errorf("dry run wrote %v", d.writes)
	}
}

func TestApplyConfigFile_ImportSkipsIdentity(t *testing.T) {
	d := newFakeDevice(t)
	applyConfigFile(writeFile(t, changedFile), false, false)
	for _, w := range d.writes {
		if strings.Contains(w, "Static IP") || strings.Contains(w, "Host Name") || strings.HasPrefix(w, "PUT") {
			t.Errorf("import wrote %s", w)
		}
	}
	if d.config["Drive A Settings"]["Drive Type"] != "1581" || d.config["Network Settings"]["Network Password"] != "secret" {
		t.Errorf("import did not apply the settings: %v", d.config)
	}
}

// restore: everything else, then the identity, then the password, then the
// save - which has to carry the new password, since it takes effect at once.
func TestApplyConfigFile_RestoreOrder(t *testing.T) {
	d := newFakeDevice(t)
	applyConfigFile(writeFile(t, changedFile), true, false)
	if len(d.writes) != 4 {
		t.Fatalf("writes = %v, want rest, identity, password, save", d.writes)
	}
	if !strings.Contains(d.writes[0], "Drive Type") || strings.Contains(d.writes[0], "Password") ||
		!strings.Contains(d.writes[1], "Static IP") || !strings.Contains(d.writes[1], "Host Name") ||
		!strings.Contains(d.writes[2], "Network Password") || d.writes[3] != "PUT /v1/configs:save_to_flash" {
		t.Errorf("writes in wrong order: %v", d.writes)
	}
	if d.auth[3] != "secret" {
		t.Errorf("save carried password %q, want the new one", d.auth[3])
	}
}

// A restore with nothing to change still saves, so flash matches the backup.
func TestApplyConfigFile_RestoreUnchangedStillSaves(t *testing.T) {
	d := newFakeDevice(t)
	applyConfigFile(writeFile(t, d.config), true, false)
	if len(d.writes) != 1 || d.writes[0] != "PUT /v1/configs:save_to_flash" {
		t.Errorf("writes = %v, want only the save", d.writes)
	}
}

// Host name changes under DHCP do not move the device; the hint must not send
// the user looking for a new address.
func TestNetworkHints(t *testing.T) {
	err := errors.New("timeout")
	hostOnly := []api.ConfigChange{{Category: "Network Settings", Item: "Host Name", New: "OTHER"}}
	if h := strings.Join(networkHints(err, api.ConfigSettings{}, hostOnly), " "); strings.Contains(h, "new DHCP address") || !strings.Contains(h, "config save-to-flash") {
		t.Errorf("host name only: %q", h)
	}
	moved := []api.ConfigChange{{Category: "Ethernet Settings", Item: "Static IP", New: "192.168.2.99"}}
	static := api.ConfigSettings{"Ethernet Settings": {"Use DHCP": "Disabled", "Static IP": "192.168.2.99"}}
	if h := strings.Join(networkHints(err, static, moved), " "); !strings.Contains(h, "--host 192.168.2.99") {
		t.Errorf("static: %q", h)
	}
}

// The password never shows in the output.
func TestChangeValue_HidesPassword(t *testing.T) {
	ch := api.ConfigChange{Category: "Network Settings", Item: "Network Password", Old: "old", New: "secret"}
	if got := changeValue(ch); strings.Contains(got, "secret") || strings.Contains(got, "old") {
		t.Errorf("changeValue = %q shows the password", got)
	}
}
