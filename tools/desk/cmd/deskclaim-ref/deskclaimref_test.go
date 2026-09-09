package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeStore is an in-memory model of the forge git-data surface the claim tool drives via the
// go-git transport (gogit.go). It stands in for the live remote so a test exercises the verbs'
// decision logic — the CAS create/update/delete semantics, the two-phase TTL, the holder-only
// progress, the output grammar — with no network and no external process. The REAL go-git
// mechanics (the empty-blob tag mint, the explicit-old receive-pack CAS, the filtered
// upload-pack read, and the tag-object interop with a commit-target REST tag) are proven
// end-to-end, offline, against a local git server in internal/gitcore/claimref_test.go; this
// file drives the seam above them, exactly as the old ghRun seam was driven.
type fakeStore struct {
	claims   map[string]fakeClaim
	branches map[string]bool
	now      time.Time
	n        int

	// failure injection: the fail-closed (exit 6) paths.
	readFails  bool // read/list return claimUnverifiable
	writeFails bool // create/update return writeUnverifiable

	// cause is the "<host>: <error>" attribution the real gogitStore records on a transport
	// failure; the fake returns it from transportCause() so a verb-level test can assert the
	// operator-facing message carries it (assay#727).
	cause string
}

type fakeClaim struct{ sha, msg, date string }

func newStore() *fakeStore {
	return &fakeStore{
		claims:   map[string]fakeClaim{},
		branches: map[string]bool{},
		now:      time.Now().UTC(),
	}
}

func (f *fakeStore) nextSHA() string { f.n++; return fmt.Sprintf("tagsha%d", f.n) }

// seedClaim installs a held claim with a chosen holder/state/branch and age, so a test controls
// exactly what acquire/show read back without minting through the store.
func (f *fakeStore) seedClaim(id, owner, state, branch string, age time.Duration) {
	f.claims[id] = fakeClaim{
		sha:  f.nextSHA(),
		msg:  claimMessage(id, owner, state, branch, ""),
		date: f.now.Add(-age).Format(time.RFC3339),
	}
}

func (f *fakeStore) read(id string) (claimRef, claimStatus) {
	if f.readFails {
		return claimRef{}, claimUnverifiable
	}
	if c, ok := f.claims[id]; ok {
		return claimRef{sha: c.sha, msg: c.msg, date: c.date}, claimHeld
	}
	return claimRef{}, claimFree
}

func (f *fakeStore) createIfAbsent(id, msg string) writeOutcome {
	if f.writeFails {
		return writeUnverifiable
	}
	if _, ok := f.claims[id]; ok {
		return writeRejected // the server-side CAS: a create loses against an existing ref
	}
	f.claims[id] = fakeClaim{sha: f.nextSHA(), msg: msg, date: f.now.Format(time.RFC3339)}
	return writeApplied
}

func (f *fakeStore) updateFrom(id, oldSHA, msg string) writeOutcome {
	if f.writeFails {
		return writeUnverifiable
	}
	c, ok := f.claims[id]
	if !ok || c.sha != oldSHA {
		return writeRejected // the CAS: the ref moved (or vanished) under the caller
	}
	f.claims[id] = fakeClaim{sha: f.nextSHA(), msg: msg, date: f.now.Format(time.RFC3339)}
	return writeApplied
}

func (f *fakeStore) remove(id string) (writeOutcome, bool) {
	_, existed := f.claims[id]
	delete(f.claims, id)
	return writeApplied, existed
}

func (f *fakeStore) list() ([]string, claimStatus) {
	if f.readFails {
		return nil, claimUnverifiable
	}
	var ids []string
	for id := range f.claims {
		ids = append(ids, id)
	}
	return ids, claimHeld
}

func (f *fakeStore) branchExists(branch string) (bool, bool) {
	if branch == "" || branch == "-" {
		return false, true
	}
	if f.readFails {
		return false, false
	}
	return f.branches[branch], true
}

func (f *fakeStore) transportCause() string { return f.cause }

// harness wires the fake into the tool's store seam and captures its output.
func harness(t *testing.T, f *fakeStore) (rc func(args ...string) int, stdout, stderr *bytes.Buffer) {
	t.Helper()
	var so, se bytes.Buffer
	oldBuild, oldOut, oldErr := buildStore, out, errOut
	buildStore = func(_, _ string) (claimStore, error) { return f, nil }
	out = &so
	errOut = &se
	t.Cleanup(func() { buildStore, out, errOut = oldBuild, oldOut, oldErr })
	return func(args ...string) int {
		so.Reset()
		se.Reset()
		return run(args)
	}, &so, &se
}

// --- acquire ----------------------------------------------------------------

func TestAcquireFreeCreatesTheRefAndEncodesTheHolder(t *testing.T) {
	f := newStore()
	run, so, _ := harness(t, f)

	rc := run("acquire", "at--stream--07", "--repo", "medici-finance/assay", "--owner", "sess-A", "--branch", "feat/x")
	if rc != exitOK {
		t.Fatalf("acquire rc = %d, want 0; out=%s", rc, so.String())
	}
	c, held := f.claims["at--stream--07"]
	if !held {
		t.Fatal("no refs/dispatch/at--stream--07 was created")
	}
	// The holder is encoded in the tag message exactly as the wire contract requires.
	for _, want := range []string{"owner=sess-A", "state=claimed", "branch=feat/x", "dispatch-claim at--stream--07"} {
		if !strings.Contains(c.msg, want) {
			t.Errorf("claim tag message %q missing %q", c.msg, want)
		}
	}
	if !strings.Contains(so.String(), "acquired at--stream--07") {
		t.Errorf("acquire did not log the acquisition: %s", so.String())
	}
}

// A second acquire of a LIVE claim (within TTL) refuses (exit 5), logs the DEDUP holder, and
// never steals.
func TestAcquireLiveHolderRefusesAndNeverSteals(t *testing.T) {
	f := newStore()
	f.seedClaim("at--stream--07", "other-sess", "dispatched", "", 42*time.Minute)
	run, so, _ := harness(t, f)

	rc := run("acquire", "at--stream--07", "--repo", "medici-finance/assay", "--owner", "sess-B")
	if rc != exitRefused {
		t.Fatalf("live-holder acquire rc = %d, want 5; out=%s", rc, so.String())
	}
	if !strings.Contains(so.String(), "DEDUP at--stream--07") {
		t.Errorf("no DEDUP line for the live holder: %s", so.String())
	}
	if strings.Contains(so.String(), "stole") {
		t.Error("a live claim was stolen inline")
	}
}

// A claim past its state's TTL is DEAD and reclaimable: acquire reclaims it (exit 0) via an
// auditable steal, mirroring the two-phase-TTL contract (claimed 20m, dispatched 120m).
func TestAcquireStaleClaimIsReclaimed(t *testing.T) {
	cases := []struct {
		state string
		age   time.Duration
		stale bool
	}{
		{"dispatched", 121 * time.Minute, true},
		{"dispatched", 42 * time.Minute, false},
		{"claimed", 25 * time.Minute, true},
		{"claimed", 5 * time.Minute, false},
	}
	for _, c := range cases {
		t.Run(c.state+"-"+c.age.String(), func(t *testing.T) {
			f := newStore()
			f.seedClaim("at--stream--07", "old-sess", c.state, "", c.age)
			run, so, _ := harness(t, f)
			rc := run("acquire", "at--stream--07", "--repo", "medici-finance/assay", "--owner", "sess-N")
			if c.stale {
				if rc != exitOK {
					t.Fatalf("stale reclaim rc = %d, want 0; out=%s", rc, so.String())
				}
				if !strings.Contains(so.String(), "stole at--stream--07") {
					t.Errorf("stale claim was not reclaimed via steal: %s", so.String())
				}
			} else {
				if rc != exitRefused {
					t.Fatalf("live claim rc = %d, want 5; out=%s", rc, so.String())
				}
			}
		})
	}
}

// A held claim whose recorded branch already exists on the remote is a branch-as-claim: the
// work is in flight, not stalled — refuse (exit 5), regardless of age.
func TestAcquireBranchAsClaimRefuses(t *testing.T) {
	f := newStore()
	f.seedClaim("at--stream--07", "old-sess", "dispatched", "feat/live", 9999*time.Minute)
	f.branches["feat/live"] = true
	run, so, _ := harness(t, f)
	rc := run("acquire", "at--stream--07", "--repo", "medici-finance/assay", "--owner", "sess-N")
	if rc != exitRefused {
		t.Fatalf("branch-as-claim rc = %d, want 5; out=%s", rc, so.String())
	}
	if !strings.Contains(so.String(), "branch-as-claim") {
		t.Errorf("no branch-as-claim note: %s", so.String())
	}
}

// A transport failure on the create is the fail-closed path: exit 6, never a claim assumed
// placed or free.
func TestAcquireTransportFailureIsUnverifiable(t *testing.T) {
	f := newStore()
	f.writeFails = true
	run, _, se := harness(t, f)
	rc := run("acquire", "at--stream--07", "--repo", "medici-finance/assay")
	if rc != exitUnverifiable {
		t.Fatalf("transport-fail acquire rc = %d, want 6; err=%s", rc, se.String())
	}
}

// --- show / list output parity ---------------------------------------------

// The `show` output is a wire contract: desksupervise/live.go and deskdispatch/dispatch.go
// parse state=/age=/owner=/branch= and the "FREE <key>" marker out of it. This pins the exact
// line shape and proves those very regexes extract the right values.
func TestShowOutputIsTheParsedWireContract(t *testing.T) {
	f := newStore()
	f.seedClaim("at--stream--07", "sess-Z", "dispatched", "feat/y", 42*time.Minute)
	run, so, _ := harness(t, f)

	if rc := run("show", "at--stream--07", "--repo", "medici-finance/assay"); rc != exitOK {
		t.Fatalf("show rc = %d, want 0", rc)
	}
	line := strings.TrimSpace(so.String())
	if !strings.HasPrefix(line, "dispatch-claim: HELD at--stream--07 — ") {
		t.Fatalf("HELD line prefix wrong: %q", line)
	}
	for re, want := range map[*regexp.Regexp]string{
		regexp.MustCompile(`state=([A-Za-z]+)`): "dispatched",
		regexp.MustCompile(`age=(\d+)m`):        "42",
		regexp.MustCompile(`owner=(\S+)`):       "sess-Z",
		regexp.MustCompile(`branch=(\S+)`):      "feat/y",
	} {
		m := re.FindStringSubmatch(line)
		if m == nil || m[1] != want {
			t.Errorf("regex %s on %q -> %v, want %q", re, line, m, want)
		}
	}
}

// TestShowOutputByteStable pins the FULL show line byte-for-byte (composed from the seeded
// values), so an edit to the format is caught, not only a field regex.
func TestShowOutputByteStable(t *testing.T) {
	f := newStore()
	f.seedClaim("at--stream--07", "sess-Z", "dispatched", "feat/y", 42*time.Minute)
	c := f.claims["at--stream--07"]
	run, so, _ := harness(t, f)
	if rc := run("show", "at--stream--07", "--repo", "medici-finance/assay"); rc != exitOK {
		t.Fatalf("show rc = %d, want 0", rc)
	}
	want := fmt.Sprintf("dispatch-claim: HELD at--stream--07 — %s at=%s age=42m\n", c.msg, c.date)
	if so.String() != want {
		t.Fatalf("show line drifted:\n got %q\nwant %q", so.String(), want)
	}
}

func TestShowFreeCarriesTheFreeMarker(t *testing.T) {
	f := newStore()
	run, so, _ := harness(t, f)
	if rc := run("show", "at--stream--07", "--repo", "medici-finance/assay"); rc != exitOK {
		t.Fatalf("show FREE rc = %d, want 0", rc)
	}
	if !strings.Contains(so.String(), "FREE at--stream--07") {
		t.Errorf("FREE marker absent: %s", so.String())
	}
}

// An unreadable claim (the transport read itself failed) is exit 6, never FREE.
func TestShowUnreadableIsUnverifiableNotFree(t *testing.T) {
	f := newStore()
	f.readFails = true
	run, _, se := harness(t, f)
	if rc := run("show", "at--stream--07", "--repo", "medici-finance/assay"); rc != exitUnverifiable {
		t.Fatalf("unreadable show rc = %d, want 6; err=%s", rc, se.String())
	}
}

func TestListShowsEachClaim(t *testing.T) {
	f := newStore()
	f.seedClaim("at--stream--07", "s1", "dispatched", "", 10*time.Minute)
	f.seedClaim("at--issue-5", "s2", "claimed", "", 3*time.Minute)
	run, so, _ := harness(t, f)
	if rc := run("list", "--repo", "medici-finance/assay"); rc != exitOK {
		t.Fatalf("list rc = %d, want 0", rc)
	}
	for _, want := range []string{"HELD at--stream--07", "HELD at--issue-5"} {
		if !strings.Contains(so.String(), want) {
			t.Errorf("list missing %q:\n%s", want, so.String())
		}
	}
}

// --- release / steal / progress ---------------------------------------------

func TestReleaseDeletesTheRefAndIsNoopWhenMissing(t *testing.T) {
	f := newStore()
	f.seedClaim("at--stream--07", "s1", "dispatched", "", time.Minute)
	run, so, _ := harness(t, f)

	if rc := run("release", "at--stream--07", "--repo", "medici-finance/assay"); rc != exitOK {
		t.Fatalf("release rc = %d, want 0", rc)
	}
	if _, held := f.claims["at--stream--07"]; held {
		t.Error("release did not delete the ref")
	}
	if !strings.Contains(so.String(), "released at--stream--07") {
		t.Errorf("release did not log: %s", so.String())
	}
	// A second release of a now-missing claim is a no-op, not a failure.
	if rc := run("release", "at--stream--07", "--repo", "medici-finance/assay"); rc != exitOK {
		t.Fatalf("release-missing rc = %d, want 0; out=%s", rc, so.String())
	}
	if !strings.Contains(so.String(), "no claim — no-op") {
		t.Errorf("release-missing did not log the no-op line: %s", so.String())
	}
}

func TestStealRequiresAReasonThenSucceeds(t *testing.T) {
	f := newStore()
	f.seedClaim("at--stream--07", "old", "dispatched", "", 5*time.Minute)
	run, _, se := harness(t, f)

	if rc := run("steal", "at--stream--07", "--repo", "medici-finance/assay"); rc != exitRefused {
		t.Fatalf("reasonless steal rc = %d, want 5; err=%s", rc, se.String())
	}
	run2, so, _ := harness(t, f)
	if rc := run2("steal", "at--stream--07", "--repo", "medici-finance/assay", "--reason", "TTL dead", "--owner", "new"); rc != exitOK {
		t.Fatalf("steal-with-reason rc = %d, want 0; out=%s", rc, so.String())
	}
	c := f.claims["at--stream--07"]
	if !strings.Contains(c.msg, "note=TTL_dead") {
		t.Errorf("steal did not record the reason in the replacement: %q", c.msg)
	}
	if !strings.Contains(c.msg, "owner=new") {
		t.Errorf("steal did not record the new owner: %q", c.msg)
	}
}

// A steal of a FREE key collapses to a create (the takeover still records its reason).
func TestStealOfFreeKeyCreates(t *testing.T) {
	f := newStore()
	run, so, _ := harness(t, f)
	if rc := run("steal", "at--stream--07", "--repo", "medici-finance/assay", "--reason", "cold take", "--owner", "new"); rc != exitOK {
		t.Fatalf("steal-of-free rc = %d, want 0; out=%s", rc, so.String())
	}
	if _, held := f.claims["at--stream--07"]; !held {
		t.Fatal("steal of a free key did not create the ref")
	}
}

func TestProgressRequiresHolderAndBranch(t *testing.T) {
	f := newStore()
	f.seedClaim("at--stream--07", "owner-1", "claimed", "", time.Minute)
	run, _, se := harness(t, f)

	// A free claim cannot be advanced.
	if rc := run("progress", "at--issue-9", "--repo", "medici-finance/assay", "--owner", "owner-1", "--branch", "feat/x"); rc != exitRefused {
		t.Fatalf("progress-on-free rc = %d, want 5; err=%s", rc, se.String())
	}
	// A non-holder cannot advance someone else's claim.
	if rc := run("progress", "at--stream--07", "--repo", "medici-finance/assay", "--owner", "intruder", "--branch", "feat/x"); rc != exitRefused {
		t.Fatalf("progress-by-nonholder rc = %d, want 5; err=%s", rc, se.String())
	}
	// progress with no --branch is refused.
	if rc := run("progress", "at--stream--07", "--repo", "medici-finance/assay", "--owner", "owner-1"); rc != exitRefused {
		t.Fatalf("progress-no-branch rc = %d, want 5", rc)
	}
	// The holder advances its own claim to dispatched.
	if rc := run("progress", "at--stream--07", "--repo", "medici-finance/assay", "--owner", "owner-1", "--branch", "feat/x"); rc != exitOK {
		t.Fatalf("progress-by-holder rc = %d, want 0; err=%s", rc, se.String())
	}
	c := f.claims["at--stream--07"]
	if !strings.Contains(c.msg, "state=dispatched") {
		t.Errorf("progress did not advance state: %q", c.msg)
	}
}

// The compare-and-swap that the gh-CLI port's PATCH force=true lacked: if the claim is stolen
// out from under the holder between the read and the advance, progress is REFUSED (the CAS
// loses), never clobbered back into existence.
func TestProgressRefusedWhenClaimMovedUnderHolder(t *testing.T) {
	f := newStore()
	f.seedClaim("at--stream--07", "owner-1", "claimed", "", time.Minute)
	// Simulate the claim being stolen after acquire: its sha changes (a new tag), so the
	// holder's explicit-old CAS no longer matches. The tool re-reads a stale sha via the seam
	// by driving updateFrom with an sha that no longer matches — emulated by mutating the
	// stored sha out from under the read the tool just did.
	run, _, se := harness(t, f)
	// Wrap read so that after the tool reads the current sha, the store's sha is rotated,
	// forcing updateFrom's CAS to reject.
	moving := &movingStore{fakeStore: f}
	buildStore = func(_, _ string) (claimStore, error) { return moving, nil }
	if rc := run("progress", "at--stream--07", "--repo", "medici-finance/assay", "--owner", "owner-1", "--branch", "feat/x"); rc != exitRefused {
		t.Fatalf("raced progress rc = %d, want 5 (refused); err=%s", rc, se.String())
	}
}

// movingStore rotates the stored sha right after a read, so the subsequent CAS update sees a
// stale old — the exact race the server-side compare-and-swap rejects.
type movingStore struct{ *fakeStore }

func (m *movingStore) read(id string) (claimRef, claimStatus) {
	ref, st := m.fakeStore.read(id)
	if st == claimHeld {
		if c, ok := m.fakeStore.claims[id]; ok {
			c.sha = m.fakeStore.nextSHA() // someone else advanced/stole it
			m.fakeStore.claims[id] = c
		}
	}
	return ref, st
}

// --- interop: the message grammar round-trips both ways ---------------------

// The claim payload is a wire contract shared with tools/dispatch-claim.sh: a bash
// `dispatch-claim.sh show` parses a Go-minted tag's message, and this tool parses a
// bash/REST-minted tag's message. Both readers split the SAME space-separated `key=value`
// grammar. This asserts the grammar this tool BUILDS is parsed back field-for-field, and that a
// message built in the bash shape is parsed identically by this tool's field reader. (The
// tag-OBJECT round-trip — Go reads a commit-target REST tag; a git reader reads a Go blob-target
// tag — is proven over a real git server in internal/gitcore/claimref_test.go.)
func TestClaimMessageGrammarRoundTrips(t *testing.T) {
	msg := claimMessage("at--stream--07", "sess-A", "dispatched", "feat/x", "TTL dead reason")
	// The Go builder's exact grammar.
	want := "dispatch-claim at--stream--07 owner=sess-A state=dispatched branch=feat/x note=TTL_dead_reason"
	if msg != want {
		t.Fatalf("claimMessage grammar drifted:\n got %q\nwant %q", msg, want)
	}
	// This tool's field reader (the bash `field_of` equivalent) extracts every field back.
	for key, val := range map[string]string{
		"owner": "sess-A", "state": "dispatched", "branch": "feat/x", "note": "TTL_dead_reason",
	} {
		if got := fieldOf(msg, key); got != val {
			t.Errorf("fieldOf(%q) = %q, want %q", key, got, val)
		}
	}
	// A message minted in the bash/REST shape (assembled independently, as the script does) is
	// parsed identically — the two readers agree on the grammar.
	bashShaped := "dispatch-claim at--issue-5 owner=bash-sess state=claimed branch=-"
	if fieldOf(bashShaped, "owner") != "bash-sess" || fieldOf(bashShaped, "state") != "claimed" || fieldOf(bashShaped, "branch") != "-" {
		t.Errorf("this tool did not parse a bash-shaped message: %q", bashShaped)
	}
}

// --- argument-level refusals (no forge write) -------------------------------

func TestInvalidKeyRefusedBeforeAnyForgeWrite(t *testing.T) {
	for _, key := range []string{"noprefix", "at stream", "at--..--1", ".at--x--1", "at--x--1.lock", "at~x--1"} {
		f := newStore()
		run, _, se := harness(t, f)
		rc := run("acquire", key, "--repo", "medici-finance/assay")
		if rc != exitRefused {
			t.Errorf("key %q rc = %d, want 5; err=%s", key, rc, se.String())
		}
		// A malformed key must not reach the forge (no claim written).
		if len(f.claims) != 0 {
			t.Errorf("key %q wrote a claim: %v", key, f.claims)
		}
	}
}

func TestUnknownVerbAndFlagRefused(t *testing.T) {
	f := newStore()
	run, _, _ := harness(t, f)
	if rc := run("frobnicate", "at--x--1", "--repo", "medici-finance/assay"); rc != exitRefused {
		t.Errorf("unknown verb rc = %d, want 5", rc)
	}
	if rc := run("acquire", "at--x--1", "--repo", "medici-finance/assay", "--bogus", "v"); rc != exitRefused {
		t.Errorf("unknown flag rc = %d, want 5", rc)
	}
}

// --- assay#727: attributable fail-closed + worktreeConfig-aware origin read ---------------

// A fail-closed (exit 6) transport failure must name the host it dialed and the underlying
// cause in the operator-facing message — not the bare "unverifiable" that sent an operator
// debugging the claim namespace, token scopes and ref permissions while the real fault was the
// host (assay#727). Drives the verb layer over the store seam's attribution.
func TestTransportFailureMessageCarriesHostAndCause(t *testing.T) {
	f := newStore()
	f.writeFails = true
	f.cause = "gitlab.example.com: authentication required: HTTP Basic: Access denied"
	run, _, se := harness(t, f)

	rc := run("acquire", "at--issue-727", "--repo", "group/repo", "--owner", "sess-A")
	if rc != exitUnverifiable {
		t.Fatalf("transport-fail acquire rc = %d, want 6; err=%s", rc, se.String())
	}
	got := se.String()
	for _, want := range []string{
		"could not create the claim refs/dispatch/at--issue-727", // still names the ref
		"gitlab.example.com",        // AND the host dialed
		"HTTP Basic: Access denied", // AND the underlying cause
	} {
		if !strings.Contains(got, want) {
			t.Errorf("fail-closed message missing %q — an operator cannot see WHERE/WHY:\n%s", want, got)
		}
	}
}

// The store-level formatter: a recorded transport error renders as "<host>: <error>", and a
// store with no failure attributes nothing (so a non-transport unverifiable stays bare).
func TestGogitStoreTransportCauseFormatsHostAndError(t *testing.T) {
	g := &gogitStore{host: "gitlab.example.com"}
	if c := g.transportCause(); c != "" {
		t.Fatalf("a store with no transport failure attributes %q, want empty", c)
	}
	g.fail(errors.New("authentication required: HTTP Basic: Access denied"))
	want := "gitlab.example.com: authentication required: HTTP Basic: Access denied"
	if got := g.transportCause(); got != want {
		t.Fatalf("transportCause = %q, want %q", got, want)
	}
}

// The origin read must fall back to a direct parse of the common .git/config when go-git cannot
// read the checkout — the worktreeConfig case that made every claim verb exit 6 on a
// self-hosted GitLab (assay#727). This drives the config-file parser directly (the no-git
// fallback), which needs neither go-git's extension support nor a git binary.
func TestConfigFileOriginURLReadsCommonConfig(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A worktreeConfig-enabled config — exactly the shape go-git refuses to read.
	cfg := "[core]\n\trepositoryformatversion = 1\n" +
		"[extensions]\n\tworktreeConfig = true\n" +
		"[remote \"origin\"]\n\turl = https://gitlab.example.com/group/repo.git\n" +
		"\tfetch = +refs/heads/*:refs/remotes/origin/*\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	want := "https://gitlab.example.com/group/repo.git"
	if got := configFileOriginURL(dir); got != want {
		t.Fatalf("configFileOriginURL = %q, want %q", got, want)
	}
}

// The linked-worktree case: the checkout's `.git` is a FILE pointing at
// `<main>/.git/worktrees/<name>`, and the remotes live in the COMMON config one level up (where
// the `commondir` file points) — not in the per-worktree config go-git reads (which is why
// go-git returns "remote not found" with zero remotes here). The parser must follow the pointer.
func TestConfigFileOriginURLResolvesLinkedWorktreeCommondir(t *testing.T) {
	root := t.TempDir()
	mainGit := filepath.Join(root, "main", ".git")
	wtGitDir := filepath.Join(mainGit, "worktrees", "wt1")
	if err := os.MkdirAll(wtGitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[remote \"origin\"]\n\turl = git@gitlab.example.com:group/repo.git\n"
	if err := os.WriteFile(filepath.Join(mainGit, "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	wtCheckout := filepath.Join(root, "wt1")
	if err := os.MkdirAll(wtCheckout, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtCheckout, ".git"), []byte("gitdir: "+wtGitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// git writes a `commondir` in the worktree gitdir pointing back to the main .git.
	if err := os.WriteFile(filepath.Join(wtGitDir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := "git@gitlab.example.com:group/repo.git"
	if got := configFileOriginURL(wtCheckout); got != want {
		t.Fatalf("linked-worktree origin = %q, want %q", got, want)
	}
}

// The exit codes this port emits ARE the deskkit contract deskdispatch passes through
// untouched — pin the mapping so a future edit cannot silently repoint one.
func TestExitCodesAreTheDeskkitContract(t *testing.T) {
	if exitOK != deskkit.ExitOK || exitRefused != deskkit.ExitRefused || exitUnverifiable != deskkit.ExitUnverifiable {
		t.Fatalf("exit codes drifted from the deskkit contract: ok=%d refused=%d unverifiable=%d",
			exitOK, exitRefused, exitUnverifiable)
	}
}
