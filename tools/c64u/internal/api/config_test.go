package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The Ultimate uses ':' to introduce an action, as in "/v1/configs:save_to_flash".
// Some configuration items carry one in their name - "DMA Load Mimics ID:" is the
// one that surfaced this - and the device then answers with an empty category
// instead of the item. url.PathEscape leaves ':' alone, because it is legal in a
// path segment, so it has to be encoded on top of that.
func TestConfigItemPathEncodesColon(t *testing.T) {
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"errors": []string{}}) //nolint:errcheck
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)

	if _, err := c.GetConfigItem("C64 and Cartridge Settings", "DMA Load Mimics ID:"); err != nil {
		t.Fatalf("GetConfigItem() error: %v", err)
	}
	if strings.Contains(gotPath, ":") {
		t.Errorf("GET path still carries a raw colon: %s", gotPath)
	}
	if !strings.Contains(gotPath, "%3A") {
		t.Errorf("GET path does not encode the colon: %s", gotPath)
	}

	gotPath = ""
	if err := c.SetConfigItem("C64 and Cartridge Settings", "DMA Load Mimics ID:", "8"); err != nil {
		t.Fatalf("SetConfigItem() error: %v", err)
	}
	if strings.Contains(gotPath, ":") {
		t.Errorf("PUT path still carries a raw colon: %s", gotPath)
	}
}

// A category name with a colon has to survive the same way.
func TestConfigCategoryPathEncodesColon(t *testing.T) {
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"errors": []string{}}) //nolint:errcheck
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.GetConfigCategory("Odd: Category"); err != nil {
		t.Fatalf("GetConfigCategory() error: %v", err)
	}
	if strings.Contains(gotPath, ":") {
		t.Errorf("GET path still carries a raw colon: %s", gotPath)
	}
}
