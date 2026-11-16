package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEndpoints(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler(nil))
	defer ts.Close()

	r, _ := http.Get(ts.URL + "/api/city")
	var c struct{ Wards []any }
	json.NewDecoder(r.Body).Decode(&c)
	if len(c.Wards) != 243 {
		t.Fatalf("want 243 wards, got %d", len(c.Wards))
	}
	r, _ = http.Post(ts.URL+"/api/sessions", "application/json", strings.NewReader(`{"policy":"nope"}`))
	if r.StatusCode != 400 {
		t.Fatalf("unknown policy should be 400, got %d", r.StatusCode)
	}
	r, _ = http.Post(ts.URL+"/api/sessions", "application/json", strings.NewReader(`{"policy":"No dynamic fee","seed":3}`))
	var created struct{ ID string }
	json.NewDecoder(r.Body).Decode(&created)
	if r.StatusCode != 201 || created.ID == "" {
		t.Fatalf("create failed: %d", r.StatusCode)
	}
	r, _ = http.Post(ts.URL+"/api/sessions/"+created.ID+"/cmd", "application/json", strings.NewReader(`{"action":"pause"}`))
	if r.StatusCode != 202 {
		t.Fatalf("cmd: %d", r.StatusCode)
	}
	req, _ := http.NewRequest("DELETE", ts.URL+"/api/sessions/"+created.ID, nil)
	if r, _ = http.DefaultClient.Do(req); r.StatusCode != 204 {
		t.Fatalf("delete: %d", r.StatusCode)
	}
	if r, _ = http.Post(ts.URL+"/api/sessions/"+created.ID+"/cmd", "application/json", strings.NewReader(`{}`)); r.StatusCode != 404 {
		t.Fatalf("deleted session should 404, got %d", r.StatusCode)
	}
}
