package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The password has to reach the device on every kind of request. A command
// that quietly skipped it would work on an open device and fail only on a
// protected one, which is the hardest way to find out.
func TestPassword_SentOnEveryRequestType(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+":"+r.Header.Get("X-Password"))
		w.Write([]byte(`{"errors":[]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.Password = "hunter2"

	if _, err := c.Get("/v1/version", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Put("/v1/machine:reset", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Post("/v1/runners:run_prg", strings.NewReader("x"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PostJSON("/v1/configs", map[string]string{"a": "b"}); err != nil {
		t.Fatal(err)
	}

	want := []string{"GET:hunter2", "PUT:hunter2", "POST:hunter2", "POST:hunter2"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("requests = %v, want %v", got, want)
	}
}

// Without a password no header is sent at all, so an open device sees exactly
// the requests it always did.
func TestPassword_NoHeaderWhenUnset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, present := r.Header["X-Password"]; present {
			t.Errorf("X-Password sent although no password is set")
		}
		w.Write([]byte(`{"errors":[]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	if _, err := newTestClient(srv.URL).Get("/v1/version", nil); err != nil {
		t.Fatal(err)
	}
}

// A rejected request says why, instead of a bare "HTTP 403".
func TestPassword_RejectionHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, err := c.Get("/v1/version", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.HasErrors() || !strings.Contains(strings.Join(resp.Errors, " "), "may require a network password") {
		t.Errorf("no password hint without a password: %v", resp.Errors)
	}

	c.Password = "wrong"
	resp, err = c.Get("/v1/version", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(resp.Errors, " "), "was rejected") {
		t.Errorf("no rejection message with a password: %v", resp.Errors)
	}
	if strings.Contains(strings.Join(resp.Errors, " "), "wrong") {
		t.Errorf("error text leaks the password: %v", resp.Errors)
	}
}
