package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// #1195: a `--workpad` body that was rebuilt by appending the old workpad to itself in a
// shell loop grew to 115 GB in $TMPDIR before anything refused it. Two properties keep that
// from reaching the forge, or memory, again: a body over GitHub's own 65,536-character
// comment limit is refused before any read or write, and a body carrying the workpad marker
// more than once — the signature of a body that re-embedded its predecessor — is a caller
// bug and is refused on both the workpad and the plain-reply path.

// twoMarkerBody is a rendered workpad with a second copy of itself appended: exactly the
// shape the runaway loop produced (old body + new section, marker and all).
func twoMarkerBody() string {
	first := workpadBody("w@abc1234", "- step one")
	return first + "\n\n" + workpadBody("w@abc1234", "- step two")
}

// refusedCode reports whether err is a deskkit refusal (exit 5).
func refusedCode(err error) bool {
	var de *deskkit.DeskError
	return errors.As(err, &de) && de.Code == deskkit.ExitRefused
}

// TestWorkpadUpsertRefusesOversizedBody drives cmdWorkpadUpsert DIRECTLY — the seam a
// caller that has already read a body hits — with a marked body one character over the
// forge's limit. It must refuse before it lists a single comment: the finder failing the
// test is what pins "before doing anything else".
func TestWorkpadUpsertRefusesOversizedBody(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	swapWorkpadSeams(t,
		func(fg deskkit.Forge, fr deskkit.ForgeRepo, pr int, workerLogin string) ([]workpadCandidate, error) {
			t.Fatalf("the workpad finder ran on an oversized body — the size refusal must come first")
			return nil, nil
		},
		func(fg deskkit.Forge, fr deskkit.ForgeRepo, commentID, body string) error {
			t.Fatalf("the workpad editor ran on an oversized body")
			return nil
		},
	)
	pad := workpadBody("w@abc1234", "- step one")
	body := pad + "\n" + strings.Repeat("x", 65536+1-len(pad)-1)
	if len(body) != 65537 {
		t.Fatalf("test body is %d chars, want 65537", len(body))
	}
	err := cmdWorkpadUpsert(&auditCtx{}, nil, deskkit.ForgeRepo{}, work, "example-org/tracker", 7, []byte(body), false)
	if !refusedCode(err) {
		t.Fatalf("cmdWorkpadUpsert on a 65,537-char body: err = %v, want a refusal (exit 5)", err)
	}
	if !strings.Contains(err.Error(), "65536") {
		t.Fatalf("the refusal does not name the limit: %v", err)
	}
	if forgeRec(t).posted() {
		t.Fatalf("the oversized body reached the forge: %v", forgeRec(t).writes())
	}
}

// TestWorkpadUpsertRefusesDuplicateMarker: the same direct seam, a body carrying the
// marker twice. Refused before the finder runs; the message names the marker.
func TestWorkpadUpsertRefusesDuplicateMarker(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	swapWorkpadSeams(t,
		func(fg deskkit.Forge, fr deskkit.ForgeRepo, pr int, workerLogin string) ([]workpadCandidate, error) {
			t.Fatalf("the workpad finder ran on a double-marker body — the marker refusal must come first")
			return nil, nil
		},
		nil,
	)
	err := cmdWorkpadUpsert(&auditCtx{}, nil, deskkit.ForgeRepo{}, work, "example-org/tracker", 7, []byte(twoMarkerBody()), false)
	if !refusedCode(err) {
		t.Fatalf("cmdWorkpadUpsert on a body with two markers: err = %v, want a refusal (exit 5)", err)
	}
	if !strings.Contains(err.Error(), deskkit.WorkpadMarker) {
		t.Fatalf("the refusal does not name the marker: %v", err)
	}
	if forgeRec(t).posted() {
		t.Fatalf("the double-marker body reached the forge: %v", forgeRec(t).writes())
	}
}

// TestWorkpadRunRefusesDuplicateMarker is the same refusal through the verb's own entry
// point with --workpad: exit 5, no forge request.
func TestWorkpadRunRefusesDuplicateMarker(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	swapWorkpadSeams(t,
		func(fg deskkit.Forge, fr deskkit.ForgeRepo, pr int, workerLogin string) ([]workpadCandidate, error) {
			t.Fatalf("the workpad finder ran on a double-marker body")
			return nil, nil
		},
		nil,
	)
	bf := bodyFileWith(t, twoMarkerBody())
	rc := run([]string{"example-org/tracker", "7", "--workpad", "--body-file", bf})
	if rc != deskkit.ExitRefused {
		t.Fatalf("--workpad body carrying the marker twice rc = %d, want 5 (refused)", rc)
	}
	if rec := forgeRec(t); len(rec.requests) != 0 {
		t.Fatalf("the verb reached the forge despite the duplicate marker: %v", rec.requests)
	}
}

// TestPlainReplyRefusesDuplicateMarker: the plain-reply path (no --workpad) is the other
// way a runaway body reaches the forge, so it refuses the same shape. A single marker on the
// plain path is not this test's concern; two is the caller bug.
func TestPlainReplyRefusesDuplicateMarker(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	bf := bodyFileWith(t, twoMarkerBody())
	rc := run([]string{"example-org/tracker", "7", "--body-file", bf})
	if rc != deskkit.ExitRefused {
		t.Fatalf("plain reply body carrying the marker twice rc = %d, want 5 (refused)", rc)
	}
	if rec := forgeRec(t); len(rec.requests) != 0 {
		t.Fatalf("the verb reached the forge despite the duplicate marker: %v", rec.requests)
	}
}

// TestOversizedBodyFileIsRefusedWithoutBeingRead pins the property that actually matters
// for a 115 GB file: the refusal is decided from the file's SIZE, before its bytes are read
// into memory. A 64 MiB sparse body file must be refused (exit 5) while the process
// allocates nowhere near 64 MiB doing it.
func TestOversizedBodyFileIsRefusedWithoutBeingRead(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)

	const size = 64 << 20
	bf := filepath.Join(t.TempDir(), "huge.md")
	f, err := os.Create(bf)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatalf("truncate to %d: %v", size, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	rc := run([]string{"example-org/tracker", "7", "--body-file", bf})
	runtime.ReadMemStats(&after)

	if rc != deskkit.ExitRefused {
		t.Fatalf("64 MiB body file rc = %d, want 5 (refused)", rc)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > size/4 {
		t.Fatalf("refusing a %d-byte body allocated %d bytes — the size check must run on os.Stat, "+
			"before the file is read into memory", size, grew)
	}
	if rec := forgeRec(t); len(rec.requests) != 0 {
		t.Fatalf("the verb reached the forge with an oversized body: %v", rec.requests)
	}
}
