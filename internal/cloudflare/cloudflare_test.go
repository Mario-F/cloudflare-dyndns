package cloudflare

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

const testToken = "test-token-placeholder"

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
			t.Errorf("unexpected Authorization header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	c := New(testToken)
	c.BaseURL = srv.URL
	c.RetryDelay = 0
	return c
}

func writeOK(w http.ResponseWriter, result any, page, totalPages int) {
	body := map[string]any{"success": true, "errors": []any{}, "result": result}
	if totalPages > 0 {
		body["result_info"] = map[string]int{"page": page, "total_pages": totalPages}
	}
	_ = json.NewEncoder(w).Encode(body)
}

func page(r *http.Request) int {
	p, _ := strconv.Atoi(r.URL.Query().Get("page"))
	return p
}

func TestZonesPagination(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/zones" || r.URL.Query().Get("per_page") != "50" {
			t.Errorf("unexpected request %s", r.URL)
		}
		p := page(r)
		writeOK(w, []Zone{{ID: fmt.Sprintf("z%d", p), Name: fmt.Sprintf("zone%d.example", p)}}, p, 3)
	})

	zones, err := c.Zones()
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 3 || zones[0].ID != "z1" || zones[2].ID != "z3" {
		t.Fatalf("unexpected zones %+v", zones)
	}
	if _, err := c.Zones(); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Fatalf("expected 3 calls (cached afterwards), got %d", calls.Load())
	}
}

func TestRecordsPagination(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") != "AAAA" || r.URL.Query().Has("name") {
			t.Errorf("unexpected query %s", r.URL.RawQuery)
		}
		p := page(r)
		writeOK(w, []Record{{ID: fmt.Sprintf("r%d", p), Name: "a.example.com", Type: "AAAA"}}, p, 2)
	})

	records, err := c.Records(Zone{ID: "z1", Name: "example.com"}, "AAAA", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1].Ref() != "z1/r2" {
		t.Fatalf("unexpected records %+v", records)
	}
}

func TestFindZoneLongestSuffix(t *testing.T) {
	zones := []Zone{
		{ID: "1", Name: "example.com"},
		{ID: "2", Name: "sub.example.com"},
		{ID: "3", Name: "ample.com"},
	}
	cases := map[string]string{
		"host.example.com":      "1",
		"example.com":           "1",
		"x.sub.example.com":     "2",
		"sub.example.com":       "2",
		"foo.ample.com":         "3",
		"host.otherexample.com": "",
	}
	for fqdn, want := range cases {
		z, ok := FindZone(zones, fqdn)
		if want == "" {
			if ok {
				t.Errorf("%s: expected no zone, got %s", fqdn, z.ID)
			}
			continue
		}
		if !ok || z.ID != want {
			t.Errorf("%s: expected zone %s, got %s (%t)", fqdn, want, z.ID, ok)
		}
	}
}

func zoneAndRecordHandler(t *testing.T, records []Record) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/zones":
			writeOK(w, []Zone{{ID: "za", Name: "example.com"}, {ID: "zs", Name: "sub.example.com"}}, 1, 1)
		case "/zones/zs/dns_records":
			q := r.URL.Query()
			if q.Get("type") != "A" || q.Get("name") != "host.sub.example.com" {
				t.Errorf("unexpected query %s", r.URL.RawQuery)
			}
			writeOK(w, records, 1, 1)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestRecordByFQDN(t *testing.T) {
	c := newTestClient(t, zoneAndRecordHandler(t, []Record{{ID: "r1", Name: "host.sub.example.com", Type: "A", Content: "192.0.2.1"}}))
	r, err := c.Record("Host.Sub.Example.com.", "A")
	if err != nil {
		t.Fatal(err)
	}
	if r.Ref() != "zs/r1" || r.Content != "192.0.2.1" {
		t.Fatalf("unexpected record %+v", r)
	}
}

func TestRecordNotFound(t *testing.T) {
	c := newTestClient(t, zoneAndRecordHandler(t, nil))
	_, err := c.Record("host.sub.example.com", "A")
	if err == nil || !strings.Contains(err.Error(), "no A record") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestRecordAmbiguous(t *testing.T) {
	c := newTestClient(t, zoneAndRecordHandler(t, []Record{
		{ID: "r1", Name: "host.sub.example.com", Type: "A"},
		{ID: "r2", Name: "host.sub.example.com", Type: "A"},
	}))
	_, err := c.Record("host.sub.example.com", "A")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "zs/r2") {
		t.Fatalf("expected ambiguous error, got %v", err)
	}
}

func TestRecordNoZone(t *testing.T) {
	c := newTestClient(t, zoneAndRecordHandler(t, nil))
	if _, err := c.Record("host.example.org", "A"); err == nil || !strings.Contains(err.Error(), "no zone") {
		t.Fatalf("expected no zone error, got %v", err)
	}
}

func TestRecordByID(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zones/z1/dns_records/r1" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		writeOK(w, Record{ID: "r1", Name: "a.example.com", Type: "A", Content: "192.0.2.1"}, 0, 0)
	})
	r, err := c.Record("z1/r1", "A")
	if err != nil || r.Ref() != "z1/r1" {
		t.Fatalf("unexpected %+v, %v", r, err)
	}
	if _, err := c.Record("z1/r1", "AAAA"); err == nil {
		t.Fatal("expected type mismatch error")
	}
}

func TestUpdateContentSendsOnlyContent(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/zones/z1/dns_records/r1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got["content"] != "198.51.100.7" {
			t.Errorf("PATCH body must contain only content, got %s", body)
		}
		writeOK(w, Record{ID: "r1", Type: "A", Content: "198.51.100.7", Proxied: true}, 0, 0)
	})
	r, err := c.UpdateContent("z1", "r1", "198.51.100.7")
	if err != nil {
		t.Fatal(err)
	}
	if r.Content != "198.51.100.7" || !r.Proxied {
		t.Fatalf("unexpected record %+v", r)
	}
}

func TestErrorResponse(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"success":false,"errors":[{"code":9109,"message":"Invalid access token"}],"result":null}`)
	})
	err := c.Verify()
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusForbidden {
		t.Fatalf("expected APIError, got %v", err)
	}
	if !strings.Contains(err.Error(), "9109: Invalid access token") {
		t.Fatalf("error should contain code and message: %v", err)
	}
	if strings.Contains(err.Error(), testToken) {
		t.Fatal("error must not contain the token")
	}
}

func TestRetryOn429(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"success":false,"errors":[{"code":971,"message":"rate limited"}]}`)
			return
		}
		writeOK(w, []Zone{}, 1, 1)
	})
	if err := c.Verify(); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls, got %d", calls.Load())
	}
}

func TestRetryOnlyOnce(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, `{"success":false,"errors":[]}`)
	})
	if err := c.Verify(); err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls, got %d", calls.Load())
	}
}
