package api

import "testing"

func TestDrivesUnlink_Path(t *testing.T) {
	var method, path string
	srv := storeServer(t, 200, `{"errors":[]}`, &method, &path)
	resp, err := newTestClient(srv.URL).DrivesUnlink("a")
	if err != nil || resp.HasErrors() {
		t.Fatalf("err = %v, resp errors = %v", err, resp.Errors)
	}
	if method != "PUT" || path != "/v1/drives/a:unlink" {
		t.Errorf("request = %s %s", method, path)
	}
}

// Firmware before 1.2.1 has no unlink route; the caller must see an error.
func TestDrivesUnlink_NotFound(t *testing.T) {
	var method, path string
	srv := storeServer(t, 404, ``, &method, &path)
	resp, err := newTestClient(srv.URL).DrivesUnlink("a")
	if err == nil && !resp.HasErrors() {
		t.Errorf("404 reported no error")
	}
}
