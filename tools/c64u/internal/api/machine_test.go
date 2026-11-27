package api

import "testing"

func TestMachineHeap_Path(t *testing.T) {
	var method, path string
	srv := storeServer(t, 200, `{"free":1583280,"min_ever_free":1502864,"total":2097152,"errors":[]}`, &method, &path)
	resp, err := newTestClient(srv.URL).MachineHeap()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if method != "GET" || path != "/v1/machine:heap" {
		t.Errorf("request = %s %s", method, path)
	}
	if free, _ := resp.Data["free"].(float64); free != 1583280 {
		t.Errorf("free = %v", resp.Data["free"])
	}
}
