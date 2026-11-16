package cmd

import (
	"errors"
	"testing"

	"github.com/Mario-F/cloudflare-dyndns/internal/cloudflare"
)

type fakeClient struct {
	records map[string]cloudflare.Record
	patched []string
}

func (f *fakeClient) Verify() error { return nil }

func (f *fakeClient) Record(ref, recordType string) (cloudflare.Record, error) {
	r, ok := f.records[ref]
	if !ok {
		return cloudflare.Record{}, errors.New("not found")
	}
	return r, nil
}

func (f *fakeClient) UpdateContent(zoneID, recordID, content string) (cloudflare.Record, error) {
	f.patched = append(f.patched, zoneID+"/"+recordID+"="+content)
	return cloudflare.Record{}, nil
}

func newFake() *fakeClient {
	return &fakeClient{records: map[string]cloudflare.Record{
		"a.example.com": {ZoneID: "z1", ID: "r1", Name: "a.example.com", Content: "192.0.2.1"},
		"b.example.org": {ZoneID: "z2", ID: "r2", Name: "b.example.org", Content: "192.0.2.9"},
	}}
}

func ip(v string) func() (string, error) {
	return func() (string, error) { return v, nil }
}

func TestUpdateSameIPNoPatch(t *testing.T) {
	f := newFake()
	if err := runUpdate(f, ip("192.0.2.1"), []string{"a.example.com"}, "A"); err != nil {
		t.Fatal(err)
	}
	if len(f.patched) != 0 {
		t.Fatalf("expected no PATCH, got %v", f.patched)
	}
}

func TestUpdateMultipleRecords(t *testing.T) {
	f := newFake()
	if err := runUpdate(f, ip("192.0.2.1"), []string{"a.example.com", "b.example.org"}, "A"); err != nil {
		t.Fatal(err)
	}
	if len(f.patched) != 1 || f.patched[0] != "z2/r2=192.0.2.1" {
		t.Fatalf("unexpected patches %v", f.patched)
	}
}

func TestUpdatePartialFailure(t *testing.T) {
	f := newFake()
	err := runUpdate(f, ip("192.0.2.5"), []string{"missing.example.com", "a.example.com"}, "A")
	if err == nil {
		t.Fatal("expected error for missing record")
	}
	if len(f.patched) != 1 || f.patched[0] != "z1/r1=192.0.2.5" {
		t.Fatalf("other records should still be updated, got %v", f.patched)
	}
}

func TestUpdateIPError(t *testing.T) {
	f := newFake()
	failing := func() (string, error) { return "", errors.New("no providers") }
	if err := runUpdate(f, failing, []string{"a.example.com"}, "A"); err == nil {
		t.Fatal("expected error")
	}
	if len(f.patched) != 0 {
		t.Fatalf("expected no PATCH, got %v", f.patched)
	}
}

func TestSameIP(t *testing.T) {
	if !sameIP("2001:db8::1", "2001:0db8:0:0:0:0:0:1") {
		t.Fatal("expected equal IPv6 addresses")
	}
	if sameIP("192.0.2.1", "192.0.2.2") {
		t.Fatal("expected different addresses")
	}
}
