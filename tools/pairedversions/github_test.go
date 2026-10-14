package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// stubAPI serves the two GitHub REST endpoints ghAPI reads, so the transport —
// including the 404-means-checked-FAILED mapping and the digest cross-check — is
// exercised without a network.
type stubAPI struct {
	releaseJSON string
	status      int
	assetBody   []byte
	assetStatus int
}

func (s stubAPI) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/releases/tags/"):
			st := s.status
			if st == 0 {
				st = http.StatusOK
			}
			w.WriteHeader(st)
			fmt.Fprint(w, s.releaseJSON)
		case strings.Contains(r.URL.Path, "/releases/assets/"):
			st := s.assetStatus
			if st == 0 {
				st = http.StatusOK
			}
			w.WriteHeader(st)
			w.Write(s.assetBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func newTestGH(t *testing.T, s stubAPI) *ghAPI {
	t.Helper()
	srv := httptest.NewServer(s.handler())
	t.Cleanup(srv.Close)
	return &ghAPI{baseURL: srv.URL, client: &http.Client{Timeout: 5 * time.Second}}
}

func releaseJSON(sumsDigest string) string {
	return fmt.Sprintf(`{
	  "tag_name": "v1.2.3", "draft": false, "prerelease": false,
	  "assets": [
	    {"id": 1, "name": "checksums.txt", "digest": "%s"},
	    {"id": 2, "name": "statusgen-linux-amd64", "digest": "sha256:%s"}
	  ]
	}`, sumsDigest, shaB)
}

var checksumsBody = []byte(shaB + "  statusgen-linux-amd64\n")

func digestOf(b []byte) string {
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}

func TestGHAPIResolvesAPublishedRelease(t *testing.T) {
	g := newTestGH(t, stubAPI{
		releaseJSON: releaseJSON(digestOf(checksumsBody)),
		assetBody:   checksumsBody,
	})
	rel, err := g.Release("o/r", "v1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.Checksums["statusgen-linux-amd64"] != shaB {
		t.Errorf("checksums.txt not parsed: %#v", rel.Checksums)
	}
	if len(rel.Assets) != 2 {
		t.Errorf("want 2 asset names, got %v", rel.Assets)
	}
}

// A 404 is the release home ANSWERING that the tag names nothing — a checked
// failure, which must not be laundered into a transport could-not-check (or the
// other way round: the two route to different messages for the reader).
func TestGHAPIMapsNotFoundToErrNoRelease(t *testing.T) {
	g := newTestGH(t, stubAPI{status: http.StatusNotFound})
	_, err := g.Release("o/r", "v9.9.9")
	if !errors.Is(err, errNoRelease) {
		t.Fatalf("want errNoRelease, got %v", err)
	}
}

// Any other non-200 is could-not-check and must NOT come back as errNoRelease —
// a rate-limited or 500-ing API would otherwise be reported to the reader as
// "that release does not exist".
func TestGHAPIServerErrorIsNotErrNoRelease(t *testing.T) {
	g := newTestGH(t, stubAPI{status: http.StatusInternalServerError})
	_, err := g.Release("o/r", "v1.2.3")
	if err == nil || errors.Is(err, errNoRelease) {
		t.Fatalf("want a transport error distinct from errNoRelease, got %v", err)
	}
}

// The checksums.txt bytes are cross-checked against the digest the API reports
// for that asset, so a truncated or substituted transfer cannot become the hash
// authority every pin is then compared against.
func TestGHAPIRejectsChecksumsThatDoNotMatchTheirDigest(t *testing.T) {
	g := newTestGH(t, stubAPI{
		releaseJSON: releaseJSON("sha256:" + strings.Repeat("f", 64)),
		assetBody:   checksumsBody,
	})
	_, err := g.Release("o/r", "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "does not match the digest") {
		t.Fatalf("want a digest-mismatch refusal, got %v", err)
	}
}

// A release with no checksums.txt has no hash authority at all. Fail-closed:
// the alternative is to trust whatever the manifest already says.
func TestGHAPIRefusesAReleaseWithNoChecksums(t *testing.T) {
	g := newTestGH(t, stubAPI{releaseJSON: `{"tag_name":"v1.2.3","assets":[{"id":2,"name":"statusgen-linux-amd64"}]}`})
	_, err := g.Release("o/r", "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "publishes no checksums.txt") {
		t.Fatalf("want a no-checksums refusal, got %v", err)
	}
}

func TestParseChecksums(t *testing.T) {
	good := []byte("# a comment\n" + shaA + "  statusgen-darwin-arm64\n" + shaB + "  *statusgen-linux-amd64\n\n")
	got, err := parseChecksums(good)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["statusgen-darwin-arm64"] != shaA || got["statusgen-linux-amd64"] != shaB {
		t.Errorf("parsed wrong: %#v", got)
	}

	for _, bad := range []string{
		"",                                  // nothing to read
		"notahash  statusgen-linux-amd64\n", // not a sha256
		shaA + "\n",                         // no artifact name
	} {
		if _, err := parseChecksums([]byte(bad)); err == nil {
			t.Errorf("parseChecksums(%q) should have failed", bad)
		}
	}
}
