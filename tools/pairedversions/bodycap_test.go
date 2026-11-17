package main

// bodycap_test.go — every release-home answer is read under a bound and REFUSED past it,
// never handed on as its first bytes. Each over-bound body is a well-formed answer padded
// with bytes its parser skips, so a truncating read would hand back an answer that parses.

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// withCaps lowers the read bounds for one test, so the over-bound path runs on a small body.
func withCaps(t *testing.T, asset, api int64) {
	t.Helper()
	pa, pj := maxAssetBytes, maxAPIBodyBytes
	maxAssetBytes, maxAPIBodyBytes = asset, api
	t.Cleanup(func() { maxAssetBytes, maxAPIBodyBytes = pa, pj })
}

// pad appends spaces (or newlines) to b until it is n bytes long.
func pad(b []byte, fill byte, n int) []byte {
	return append(append([]byte(nil), b...), bytes.Repeat([]byte{fill}, n-len(b))...)
}

// regression: #6755
func TestAssetRefusedOverCap(t *testing.T) {
	const limit = 4096
	withCaps(t, limit, 1<<20)
	// No digest on the asset, so only the read bound stands between a cut-short
	// checksums.txt and the pins; blank lines are skipped by parseChecksums.
	for _, tc := range []struct {
		name   string
		size   int
		refuse bool
	}{
		{"over the bound", limit + 1, true},
		{"at the bound", limit, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newTestGH(t, stubAPI{
				releaseJSON: releaseJSON(""),
				assetBody:   pad(checksumsBody, '\n', tc.size),
			})
			rel, err := g.Release("o/r", "v1.2.3")
			if !tc.refuse {
				if err != nil || rel.Checksums["statusgen-linux-amd64"] != shaB {
					t.Fatalf("an asset exactly at the bound was not read whole: rel=%v err=%v", rel, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("a %d-byte checksums.txt was read as complete (%v); want a refusal", tc.size, rel.Checksums)
			}
			if !errors.Is(err, errBodyOverCap) {
				t.Errorf("the refusal is not an over-bound refusal: %v", err)
			}
		})
	}
}

// regression: #6755
func TestReleaseJSONRefusedOverCap(t *testing.T) {
	const limit = 4096
	withCaps(t, 1<<20, limit)
	g := newTestGH(t, stubAPI{
		releaseJSON: string(pad([]byte(releaseJSON(digestOf(checksumsBody))), ' ', limit+1)),
		assetBody:   checksumsBody,
	})
	rel, err := g.Release("o/r", "v1.2.3")
	if err == nil {
		t.Fatalf("a release answer past the bound was decoded (%v); want a refusal", rel)
	}
	if !errors.Is(err, errBodyOverCap) {
		t.Errorf("the refusal is not an over-bound refusal: %v", err)
	}
}

// regression: #6755
func TestReleasePageRefusedOverCap(t *testing.T) {
	const limit = 4096
	withCaps(t, 1<<20, limit)
	page := listPage(0, 3)
	g := newTestGH(t, stubAPI{listPages: []string{string(pad([]byte(page), ' ', limit+1))}})
	got, err := g.Releases("o/r")
	if err == nil {
		t.Fatalf("a release-list page past the bound was decoded (%d releases); want a refusal", len(got))
	}
	if !errors.Is(err, errBodyOverCap) || strings.Contains(err.Error(), page) {
		t.Errorf("want an over-bound refusal that carries none of the answer, got %v", err)
	}
	// The same page within the bound still reads.
	g = newTestGH(t, stubAPI{listPages: []string{page}})
	if got, err := g.Releases("o/r"); err != nil || len(got) != 3 {
		t.Errorf("a page within the bound did not read: %d releases, err=%v", len(got), err)
	}
}
