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

// storeServer answers every request with status and body and records the
// method and escaped path it was called with.
func storeServer(t *testing.T, status int, body string, method, path *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*method, *path = r.Method, r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body)) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestConfigCategoryStore_PathAndResult(t *testing.T) {
	var method, path string
	srv := storeServer(t, 200, `{"written":["Drive A Settings"],"errors":[]}`, &method, &path)
	got, err := newTestClient(srv.URL).ConfigCategoryStore("save_to_flash", "Drive A Settings")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if method != "PUT" || path != "/v1/configs/Drive%20A%20Settings:save_to_flash" {
		t.Errorf("request = %s %s", method, path)
	}
	if strings.Join(got, "|") != "Drive A Settings" {
		t.Errorf("written = %q", got)
	}
}

// Firmware 1.2.1 answers an unchanged category with an empty list.
func TestConfigCategoryStore_EmptyResult(t *testing.T) {
	var method, path string
	srv := storeServer(t, 200, `{"loaded":[],"errors":[]}`, &method, &path)
	got, err := newTestClient(srv.URL).ConfigCategoryStore("load_from_flash", "Drive A Settings")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("got %#v, %v; want an empty, non-nil list", got, err)
	}
}

func TestConfigCategoryStore_EncodesColon(t *testing.T) {
	var method, path string
	srv := storeServer(t, 200, `{"reset":[],"errors":[]}`, &method, &path)
	if _, err := newTestClient(srv.URL).ConfigCategoryStore("reset_to_default", "Odd: Cat"); err != nil {
		t.Fatalf("err = %v", err)
	}
	if path != "/v1/configs/Odd%3A%20Cat:reset_to_default" {
		t.Errorf("path = %s", path)
	}
}

func TestConfigCategoryStore_Errors(t *testing.T) {
	var method, path string
	srv := storeServer(t, 400, `{"errors":["Path depth exceeds 1."]}`, &method, &path)
	_, err := newTestClient(srv.URL).ConfigCategoryStore("save_to_flash", "a/b")
	if err == nil || !strings.Contains(err.Error(), "Path depth exceeds 1.") {
		t.Errorf("err = %v", err)
	}
}
