package deskkit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #2972
func TestListBadFiles(t *testing.T) {
	for _, healthy := range []bool{false, true} {
		t.Run(map[bool]string{false: "all-blind", true: "partial"}[healthy], func(t *testing.T) {
			dir := t.TempDir()
			writeRaw(t, filepath.Join(dir, "bad-one.claim"), `{broken`)
			writeRaw(t, filepath.Join(dir, "bad-two.claim"), `{"item":false}`)
			if healthy {
				writeRaw(t, filepath.Join(dir, "good.claim"), `{"brief":"good/01","session":"reader"}`)
			}
			claims, err := List(ClaimConfig{ClaimsDir: dir})
			if !IsUnverifiable(err) {
				t.Fatalf("silently omitted bad-one.claim and planted bad-two.claim: err=%v, want incomplete/unverifiable", err)
			}
			var partial *ClaimListError
			if !errors.As(err, &partial) || strings.Join(partial.Skipped, ",") != "bad-one.claim,bad-two.claim" {
				t.Fatalf("skipped files = %+v, want bad-one.claim and planted bad-two.claim", partial)
			}
			for _, name := range []string{"bad-one.claim", "bad-two.claim"} {
				if !strings.Contains(err.Error(), name) {
					t.Errorf("missing skipped file %s: %v", name, err)
				}
			}
			want := 0
			if healthy {
				want = 1
			}
			if len(claims) != want || (healthy && claims[0].Item != "good/01") {
				t.Fatalf("readable claims lost or fabricated: %+v", claims)
			}
			data, _ := os.ReadFile(filepath.Join(dir, "bad-one.claim"))
			if string(data) != `{broken` {
				t.Fatal("listing changed malformed claim data")
			}
		})
	}
}

// regression: #2972
func TestListReadFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "absent"), filepath.Join(dir, "unreadable.claim")); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	writeRaw(t, filepath.Join(dir, "good.claim"), `{"item":"good"}`)
	claims, err := List(ClaimConfig{ClaimsDir: dir})
	if !IsUnverifiable(err) || !strings.Contains(err.Error(), "unreadable.claim") {
		t.Fatalf("unreadable entry hidden: %v", err)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("underlying read cause lost: %v", err)
	}
	if len(claims) != 1 || claims[0].Item != "good" {
		t.Fatalf("readable claims = %+v", claims)
	}
}
