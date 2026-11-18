package main

// bodycap_test.go — a conversion answer over maxConversionBytes is refused, never parsed as
// its first 1 MiB. The served body is a valid conversion object padded with trailing spaces,
// so a truncating read would decode its first maxConversionBytes bytes as a complete result.

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6755
func TestConvertRefusesOverCapBody(t *testing.T) {
	obj := []byte(`{"id":1,"slug":"example-app"}`)
	body := append(obj, bytes.Repeat([]byte(" "), maxConversionBytes+1-len(obj))...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	withFakeGitHubAPI(t, srv)
	cr, err := convertCode("example-code")
	if err == nil {
		t.Fatalf("a %d-byte conversion answer parsed as complete (%+v); want a refusal", len(body), cr)
	}
	if !deskkit.IsForgeBodyOverCap(err) {
		t.Errorf("the refusal is not an over-cap refusal: %v", err)
	}
	if cr != nil {
		t.Errorf("a refused answer still returned a result: %+v", cr)
	}
}
