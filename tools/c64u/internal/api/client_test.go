package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient creates a Client pointing at the given test server URL.
func newTestClient(serverURL string) *Client {
	c := NewClient("unused", 80, false)
	c.BaseURL = serverURL
	return c
}

func TestNewClient_BaseURL(t *testing.T) {
	c := NewClient("192.168.1.100", 80, false)
	if c.BaseURL != "http://192.168.1.100:80" {
		t.Errorf("BaseURL = %q, want %q", c.BaseURL, "http://192.168.1.100:80")
	}
}

func TestNewClient_CustomPort(t *testing.T) {
	c := NewClient("myhost", 8080, false)
	if c.BaseURL != "http://myhost:8080" {
		t.Errorf("BaseURL = %q, want %q", c.BaseURL, "http://myhost:8080")
	}
}

func TestGet_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"version": "3.14",
			"errors":  []string{},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.Get("/v1/version", nil)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if resp.HasErrors() {
		t.Errorf("unexpected errors: %v", resp.Errors)
	}
	if resp.GetString("version") != "3.14" {
		t.Errorf("version = %q, want %q", resp.GetString("version"), "3.14")
	}
}

func TestGet_WithParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ip") != "192.168.1.1" {
			t.Errorf("ip param = %q, want %q", r.URL.Query().Get("ip"), "192.168.1.1")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"errors": []string{}})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Get("/v1/test", map[string]string{"ip": "192.168.1.1"})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
}

func TestPut_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"errors": []string{}})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.Put("/v1/streams/debug:start", map[string]string{"ip": "192.168.1.1"})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}
	if resp.HasErrors() {
		t.Errorf("unexpected errors: %v", resp.Errors)
	}
}

func TestResponse_HasErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"Network Host Resolve Error"},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.Get("/v1/test", nil)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if !resp.HasErrors() {
		t.Error("expected HasErrors()=true")
	}
	if resp.Errors[0] != "Network Host Resolve Error" {
		t.Errorf("error = %q, want %q", resp.Errors[0], "Network Host Resolve Error")
	}
}

func TestResponse_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.Get("/v1/notfound", nil)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if !resp.HasErrors() {
		t.Error("expected HasErrors()=true for 404")
	}
	if resp.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", resp.StatusCode)
	}
}

func TestResponse_GetString_Missing(t *testing.T) {
	r := &Response{Data: map[string]interface{}{}}
	if r.GetString("missing") != "" {
		t.Error("GetString on missing key should return empty string")
	}
}

func TestResponse_GetInt(t *testing.T) {
	r := &Response{Data: map[string]interface{}{"count": float64(42)}}
	if r.GetInt("count") != 42 {
		t.Errorf("GetInt = %d, want 42", r.GetInt("count"))
	}
}

func TestResponse_GetInt_Missing(t *testing.T) {
	r := &Response{Data: map[string]interface{}{}}
	if r.GetInt("missing") != 0 {
		t.Errorf("GetInt on missing key = %d, want 0", r.GetInt("missing"))
	}
}

func TestNonJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.Get("/v1/test", nil)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	// Non-JSON should not cause an error, just empty data
	if resp == nil {
		t.Fatal("resp should not be nil")
	}
}
