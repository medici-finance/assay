package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// partiallist_test.go — `list` with one unreadable claim ref among readable ones. The listing
// must still print every readable claim, name each unreadable ref as could-not-check with its
// own cause, and exit 7 (partial blindness) — distinct from a full listing (0) and from a total
// failure (6). The tool never deletes the bad ref.

// blindRefStore is the fake store with chosen refs unreadable: List still enumerates them (the
// advertisement names the ref) but Read fails for them, recording a per-ref cause the way the
// real store records the transport error of the read that failed.
type blindRefStore struct {
	*fakeStore
	bad   map[string]string // id -> cause
	cause string
}

func (b *blindRefStore) List() ([]string, deskkit.ClaimReadStatus) {
	ids, st := b.fakeStore.List()
	for id := range b.bad {
		ids = append(ids, id)
	}
	return ids, st
}

func (b *blindRefStore) Read(id string) (deskkit.ClaimStoreRecord, deskkit.ClaimReadStatus) {
	if c, ok := b.bad[id]; ok {
		b.cause = c
		return deskkit.ClaimStoreRecord{}, deskkit.ClaimReadUnverifiable
	}
	return b.fakeStore.Read(id)
}

func (b *blindRefStore) TransportCause() string { return b.cause }

func blindHarness(t *testing.T, s deskkit.ClaimStore) func() (int, string, string) {
	t.Helper()
	f := newStore()
	run, so, se := harness(t, f)
	old := buildStore
	buildStore = func(_, _ string) (deskkit.ClaimStore, error) { return s, nil }
	t.Cleanup(func() { buildStore = old })
	return func() (int, string, string) {
		rc := run("list", "--repo", "medici-finance/assay")
		return rc, so.String(), se.String()
	}
}

// regression: #2831
func TestListPartialBlindExits7(t *testing.T) {
	f := newStore()
	f.seedClaim("at--issue-11", "sess-A", "dispatched", "feat/a", 5*time.Minute)
	s := &blindRefStore{fakeStore: f, bad: map[string]string{
		"at--issue-12": "test-host: gitcore: read tag object 0123abcd: object not found",
	}}
	rc, stdout, stderr := blindHarness(t, s)()

	if rc != exitPartial {
		t.Fatalf("list rc = %d, want %d (partial blindness); stdout=%q stderr=%q", rc, exitPartial, stdout, stderr)
	}
	if !strings.Contains(stdout, "HELD at--issue-11 ") {
		t.Errorf("the readable claim was not listed:\n%s", stdout)
	}
	if strings.Contains(stdout, "at--issue-12") {
		t.Errorf("the unreadable ref leaked onto stdout as if it were a listed claim:\n%s", stdout)
	}
	want := "could-not-check: refs/dispatch/at--issue-12: test-host: gitcore: read tag object 0123abcd: object not found"
	if !strings.Contains(stderr, want) {
		t.Errorf("stderr does not name the bad ref and its cause\n want substring %q\n got %q", want, stderr)
	}
	if !strings.Contains(stderr, "partial:") || !strings.Contains(stderr, "maintainer act") {
		t.Errorf("stderr lacks the partial summary:\n%s", stderr)
	}
}

// Every advertised ref unreadable is TOTAL blindness: exit 6, never 7 and never 0.
// regression: #2831
func TestListAllBlindExits6(t *testing.T) {
	s := &blindRefStore{fakeStore: newStore(), bad: map[string]string{
		"at--issue-12": "test-host: gitcore: read tag object 0123abcd: object not found",
		"at--issue-13": "test-host: gitcore: read tag object 4567cdef: object not found",
	}}
	rc, stdout, stderr := blindHarness(t, s)()
	if rc != exitUnverifiable {
		t.Fatalf("all-unreadable list rc = %d, want 6; stdout=%q stderr=%q", rc, stdout, stderr)
	}
	for _, id := range []string{"at--issue-12", "at--issue-13"} {
		if !strings.Contains(stderr, "could-not-check: refs/dispatch/"+id+": ") {
			t.Errorf("stderr does not name %s:\n%s", id, stderr)
		}
	}
}

// End to end through the live forge-ref store: a local bare repo holds one claim minted by the
// store and one ref whose value names a tag object the server does not have — the shape the
// issue observed on a real forge. The listing still returns the good claim and exits 7, and
// the bad ref is left exactly where it was.
// regression: #2831
func TestListMissingTagObjectE2E(t *testing.T) {
	server := refusingServer(t) // a plain bare repo; no hook is installed
	g := &gogitStore{url: server, host: "local-test-server"}
	good := "at--issue-2831"
	if res := g.CreateIfAbsent(good, claimMessage(good, "sess-A", "claimed", "-", "")); res != deskkit.ClaimWriteApplied {
		t.Fatalf("seed create = %v, want Applied; cause=%q", res, g.TransportCause())
	}
	// Plant a loose ref at an object id the repository does not hold. `git update-ref` refuses a
	// missing object, which is exactly why the file is written directly.
	bad := "at--issue-2832"
	badRef := filepath.Join(server, "refs", "dispatch", bad)
	missing := "0123456789abcdef0123456789abcdef01234567"
	if err := os.WriteFile(badRef, []byte(missing+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	run, so, se := harness(t, newStore())
	old := buildStore
	buildStore = func(_, _ string) (deskkit.ClaimStore, error) { return g, nil }
	t.Cleanup(func() { buildStore = old })

	rc := run("list", "--repo", "medici-finance/assay")
	if rc != exitPartial {
		t.Fatalf("list rc = %d, want %d; stdout=%q stderr=%q", rc, exitPartial, so.String(), se.String())
	}
	if !strings.Contains(so.String(), "HELD "+good+" ") {
		t.Errorf("the good claim was not listed:\n%s", so.String())
	}
	if !strings.Contains(se.String(), "could-not-check: refs/dispatch/"+bad+": local-test-server: ") {
		t.Errorf("stderr does not name the bad ref with its cause:\n%s", se.String())
	}
	if b, err := os.ReadFile(badRef); err != nil || strings.TrimSpace(string(b)) != missing {
		t.Errorf("the bad ref was touched by list (read %q, err %v) — removing it is a maintainer act", b, err)
	}
}
