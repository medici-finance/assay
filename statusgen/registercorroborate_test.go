package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The ONLINE half of the statusgen/06 register guard: `statusgen --corroborate`
// re-derives each findings-register transition against the PR merge-base and
// requires a human named in the authorizing key to have acted on the PR. These
// tests drive the whole command (runCorroborate, every lane and the exit code)
// against a git fixture, with the forge reads stubbed through the package seams.

const findingRel = "docs/streams/findings/2026-07-17-f-gut.md"

// landedAnchoredFinding is an open finding that ALREADY carries a mapped
// authorized-by anchor from some earlier, legitimate change. The offline gate
// reads the anchor off the current entry; the stamp lane reads only ADDED lines.
const landedAnchoredFinding = "---\n" +
	"id: F-gut\n" +
	"date: \"2026-07-17\"\n" +
	"title: Register guard gap\n" +
	"affects: [\"stream-y\", \"stream-z/brief-01\"]\n" +
	"authorized-by: human:alex\n" +
	"resolved: false\n" +
	"---\n\nBody.\n"

type corroborateStub struct {
	files     []ghPRFile
	data      *ghPRData
	dataErr   error  // the PR reviews/comments read fails
	mergeBase string // "" = unresolvable
	dataCalls int
	// The forge's changed_files count is len(files)+listedShort: a positive
	// listedShort is a listing the forge truncated without an error. countErr
	// makes the count itself unreadable.
	listedShort int
	countErr    error
}

// runCorroborateOn chdirs into root, stubs the forge/git seams, and runs the
// command for PR #7, returning its exit code.
func runCorroborateOn(t *testing.T, root string, st *corroborateStub) int {
	t.Helper()
	t.Chdir(root)
	oldRepo, oldFiles, oldData, oldMB := corroborateRepoFn, corroborateFilesFn, corroborateDataFn, corroborateMergeBaseFn
	oldCount := corroborateChangedFilesFn
	t.Cleanup(func() {
		corroborateRepoFn, corroborateFilesFn, corroborateDataFn, corroborateMergeBaseFn = oldRepo, oldFiles, oldData, oldMB
		corroborateChangedFilesFn = oldCount
	})
	corroborateChangedFilesFn = func(string, int) (int, error) {
		if st.countErr != nil {
			return 0, st.countErr
		}
		return len(st.files) + st.listedShort, nil
	}
	corroborateRepoFn = func() string { return "example/repo" }
	corroborateFilesFn = func(string, int) ([]ghPRFile, error) { return st.files, nil }
	corroborateDataFn = func(string, int) (*ghPRData, error) {
		st.dataCalls++
		if st.dataErr != nil {
			return nil, st.dataErr
		}
		if st.data == nil {
			return &ghPRData{}, nil
		}
		return st.data, nil
	}
	corroborateMergeBaseFn = func(string, string, int) string { return st.mergeBase }
	return runCorroborate("7")
}

// originMain returns the fixture's landed base — the PR merge-base in CI.
func originMain(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "rev-parse", "refs/remotes/origin/main").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// mutateFinding rewrites the fixture's finding with one replacement.
func mutateFinding(t *testing.T, path, landed, old, new string) string {
	t.Helper()
	cur := strings.Replace(landed, old, new, 1)
	if cur == landed {
		t.Fatalf("mutation %q -> %q matched nothing", old, new)
	}
	if err := os.WriteFile(path, []byte(cur), 0o644); err != nil {
		t.Fatal(err)
	}
	return cur
}

// findingFiles is the PR-files-API listing of a PR touching only the finding.
func findingFiles(lines ...string) []ghPRFile {
	return []ghPRFile{{Filename: findingRel, Patch: "@@ -1,8 +1,8 @@\n" + strings.Join(lines, "\n")}}
}

func approvedBy(login string) *ghPRData {
	return &ghPRData{Reviews: []ghReview{{Author: ghAuthor{Login: login}, State: "APPROVED", Id: "r1"}}}
}

// TestRegReusedAnchorFails is the hole this lane closes: an anchor
// already on the entry authorizes the offline gate, and the PR adds no human:
// line for the stamp lane — yet nobody acted on the PR. Must fail.
func TestRegReusedAnchorFails(t *testing.T) {
	root, path := gutFixture(t, landedAnchoredFinding)
	mutateFinding(t, path, landedAnchoredFinding, "resolved: false", "resolved: true")
	if p := guttedRegisterFields(root); len(p) != 0 {
		t.Fatalf("precondition: the offline gate accepts the pre-existing anchor; got %v", p)
	}
	st := &corroborateStub{files: findingFiles("-resolved: false", "+resolved: true"), mergeBase: originMain(t, root)}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("a resolve flip authorized only by a pre-existing anchor, with no human action on the PR, must fail; rc=%d", rc)
	}
}

// TestRegReusedAnchorApproved: the same flip passes once the named
// human approves the PR.
func TestRegReusedAnchorApproved(t *testing.T) {
	root, path := gutFixture(t, landedAnchoredFinding)
	mutateFinding(t, path, landedAnchoredFinding, "resolved: false", "resolved: true")
	st := &corroborateStub{
		files:     findingFiles("-resolved: false", "+resolved: true"),
		mergeBase: originMain(t, root),
		data:      approvedBy("ada"),
	}
	if rc := runCorroborateOn(t, root, st); rc != 0 {
		t.Fatalf("a resolve authorized by a human who APPROVED the PR must pass; rc=%d", rc)
	}
}

// TestRegAffectsNoAuthorityFails: emptying affects with no
// authorizing key at all is a self-gut — fails online too.
func TestRegAffectsNoAuthorityFails(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	mutateFinding(t, path, landedOpenFinding, `affects: ["stream-y", "stream-z/brief-01"]`, "affects: []")
	st := &corroborateStub{
		files:     findingFiles(`-affects: ["stream-y", "stream-z/brief-01"]`, "+affects: []"),
		mergeBase: originMain(t, root),
		data:      approvedBy("ada"), // an approval by a human the entry never names does not authorize
	}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("emptying affects with no authorized-by must fail; rc=%d", rc)
	}
}

// TestRegSelfResolveFails: writing a mapped anchor in the PR,
// with no action by that human, fails.
func TestRegSelfResolveFails(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	mutateFinding(t, path, landedOpenFinding, "resolved: false", "resolved: true\nauthorized-by: human:alex")
	st := &corroborateStub{
		files:     findingFiles("-resolved: false", "+resolved: true", "+authorized-by: human:alex"),
		mergeBase: originMain(t, root),
	}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("a self-written anchor with no human action must fail; rc=%d", rc)
	}
}

// TestRegSelfParkFails / Approved: a park to 2099 named to a mapped
// human fails until that human acts, then passes; an approval COMMENT counts.
func TestRegSelfParkFails(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	mutateFinding(t, path, landedOpenFinding, "resolved: false",
		"resolved: false\nparked-until: \"2099-01-01\"\nparked-by: human:alex\nparked-reason: deferred")
	files := findingFiles("+parked-until: \"2099-01-01\"", "+parked-by: human:alex", "+parked-reason: deferred")
	st := &corroborateStub{files: files, mergeBase: originMain(t, root)}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("a self-park must fail; rc=%d", rc)
	}
}

func TestRegParkApprovedComment(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	mutateFinding(t, path, landedOpenFinding, "resolved: false",
		"resolved: false\nparked-until: \"2099-01-01\"\nparked-by: human:alex\nparked-reason: deferred")
	files := findingFiles("+parked-until: \"2099-01-01\"", "+parked-by: human:alex", "+parked-reason: deferred")
	st := &corroborateStub{
		files:     files,
		mergeBase: originMain(t, root),
		data:      &ghPRData{Comments: []ghComment{{Author: ghAuthor{Login: "ada"}, Body: "lgtm", URL: "u"}}},
	}
	if rc := runCorroborateOn(t, root, st); rc != 0 {
		t.Fatalf("a park whose parked-by human left an approval comment must pass; rc=%d", rc)
	}
}

// TestRegNoMergeBaseFailsClosed: a register-touching
// PR whose merge-base cannot be resolved is MISSING, never a pass.
func TestRegNoMergeBaseFailsClosed(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	mutateFinding(t, path, landedOpenFinding, "title: Register guard gap", "title: Register guard gap (retitled)")
	st := &corroborateStub{files: findingFiles("-title: Register guard gap", "+title: Register guard gap (retitled)")}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("no merge-base on a register-touching PR must fail closed; rc=%d", rc)
	}
}

// TestRegUnparseableFailsClosed: a touched entry the detector
// cannot parse is invisible to it, so it fails closed.
func TestRegUnparseableFailsClosed(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	if err := os.WriteFile(path, []byte("no frontmatter here\nresolved: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := &corroborateStub{files: findingFiles("+no frontmatter here"), mergeBase: originMain(t, root)}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("an unparseable touched finding must fail closed; rc=%d", rc)
	}
}

// TestRegCautionAddingPasses: widening affects (adds caution) is not a
// transition; it needs nobody and passes.
func TestRegCautionAddingPasses(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	mutateFinding(t, path, landedOpenFinding, `"stream-z/brief-01"]`, `"stream-z/brief-01", "stream-q"]`)
	st := &corroborateStub{files: findingFiles(`+affects: ["stream-y", "stream-z/brief-01", "stream-q"]`), mergeBase: originMain(t, root)}
	if rc := runCorroborateOn(t, root, st); rc != 0 {
		t.Fatalf("widening affects is not gutting and must pass; rc=%d", rc)
	}
}

// TestRegUntouchedLaneSilent: a PR whose tree touches no findings entry
// is untouched by this lane — no data fetch, exit 0.
func TestRegUntouchedLaneSilent(t *testing.T) {
	root, _ := gutFixture(t, landedOpenFinding)
	st := &corroborateStub{files: []ghPRFile{{Filename: "docs/other.md", Patch: "@@ -1 +1 @@\n+hello"}}, mergeBase: originMain(t, root)}
	if rc := runCorroborateOn(t, root, st); rc != 0 {
		t.Fatalf("a PR touching no findings entry must not be gated by this lane; rc=%d", rc)
	}
	if st.dataCalls != 0 {
		t.Errorf("lane fetched PR data %d times for a PR touching no findings entry", st.dataCalls)
	}
}

// otherFileListing is a forge file listing that names no findings entry — what
// a listing truncated ahead of docs/streams/findings/ looks like.
func otherFileListing() []ghPRFile {
	return []ghPRFile{{Filename: "docs/aaa.md", Patch: "@@ -1 +1 @@\n+pad"}}
}

// TestRegListingOmitsGutFails: whether the PR touches the register is read
// from the local tree against the resolved merge-base, never from the forge
// listing alone. A listing that omits the findings entry (truncated with no
// error) must not silence a reused-anchor gut that is in the tree.
func TestRegListingOmitsGutFails(t *testing.T) {
	root, path := gutFixture(t, landedAnchoredFinding)
	mutateFinding(t, path, landedAnchoredFinding, "resolved: false", "resolved: true")
	st := &corroborateStub{files: otherFileListing(), mergeBase: originMain(t, root), listedShort: 3000}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("a gut in the tree that the forge listing omits must still fail; rc=%d", rc)
	}
}

// TestRegTruncatedNoBaseFails: with no merge-base the lane can only lean on
// the listing, so a listing that cannot be shown complete — shorter than the
// forge's changed_files count, or the count unreadable — fails closed.
func TestRegTruncatedNoBaseFails(t *testing.T) {
	for _, st := range []*corroborateStub{
		{files: otherFileListing(), listedShort: 3000},
		{files: otherFileListing(), countErr: errors.New("count unreadable")},
	} {
		root, _ := gutFixture(t, landedOpenFinding)
		if rc := runCorroborateOn(t, root, st); rc != 1 {
			t.Fatalf("no merge-base and an unproven listing (short %d, err %v) must fail closed; rc=%d", st.listedShort, st.countErr, rc)
		}
	}
}

// TestRegCompleteNoBaseSilent: with no merge-base, a listing proven complete
// against changed_files that names no findings entry is a real "untouched".
func TestRegCompleteNoBaseSilent(t *testing.T) {
	root, _ := gutFixture(t, landedOpenFinding)
	st := &corroborateStub{files: otherFileListing()}
	if rc := runCorroborateOn(t, root, st); rc != 0 {
		t.Fatalf("a complete listing naming no findings entry must pass; rc=%d", rc)
	}
}

// TestRegBadBaseFailsClosed: a merge-base the local tree cannot be
// compared with (an object git does not have) proves nothing about what the PR
// touched — fail closed rather than read the error as "untouched".
func TestRegBadBaseFailsClosed(t *testing.T) {
	root, path := gutFixture(t, landedAnchoredFinding)
	mutateFinding(t, path, landedAnchoredFinding, "resolved: false", "resolved: true")
	st := &corroborateStub{files: otherFileListing(), mergeBase: strings.Repeat("0", 39) + "1"}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("a merge-base git cannot diff against must fail closed; rc=%d", rc)
	}
}

// landedParkAnchored is an open finding already parked by a mapped human.
const landedParkAnchored = "---\n" +
	"id: F-gut\n" +
	"date: \"2026-07-17\"\n" +
	"title: Register guard gap\n" +
	"affects: [\"stream-y\", \"stream-z/brief-01\"]\n" +
	"resolved: false\n" +
	"parked-until: \"2026-12-01\"\n" +
	"parked-by: human:alex\n" +
	"parked-reason: deferred\n" +
	"---\n\nBody.\n"

// TestRegReusedParkExtendFails: extending a park whose parked-by anchor is
// already on the entry adds no human: line; with nobody acting on the PR the
// park category itself must fail it.
func TestRegReusedParkExtendFails(t *testing.T) {
	root, path := gutFixture(t, landedParkAnchored)
	mutateFinding(t, path, landedParkAnchored, `parked-until: "2026-12-01"`, `parked-until: "2099-01-01"`)
	st := &corroborateStub{files: findingFiles(`-parked-until: "2026-12-01"`, `+parked-until: "2099-01-01"`), mergeBase: originMain(t, root)}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("a park extension authorized only by a pre-existing parked-by must fail; rc=%d", rc)
	}
}

// TestRegParkedByNotResolveAuth: each category is judged against its own
// key. parked-by authorizes a park, never a resolve — even when that human
// approved the PR.
func TestRegParkedByNotResolveAuth(t *testing.T) {
	root, path := gutFixture(t, landedParkAnchored)
	mutateFinding(t, path, landedParkAnchored, "resolved: false", "resolved: true")
	st := &corroborateStub{
		files:     findingFiles("-resolved: false", "+resolved: true"),
		mergeBase: originMain(t, root),
		data:      approvedBy("ada"),
	}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("a resolve named only under parked-by must fail; rc=%d", rc)
	}
}

// TestRegDataFetchErrFails: a lane that cannot read the PR's reviews and
// comments cannot corroborate anything — the run exits non-zero.
func TestRegDataFetchErrFails(t *testing.T) {
	root, path := gutFixture(t, landedAnchoredFinding)
	mutateFinding(t, path, landedAnchoredFinding, "resolved: false", "resolved: true")
	st := &corroborateStub{
		files:     findingFiles("-resolved: false", "+resolved: true"),
		mergeBase: originMain(t, root),
		data:      approvedBy("ada"),
		dataErr:   errors.New("gh: HTTP 502"),
	}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("an unreadable reviews/comments fetch must fail the run; rc=%d", rc)
	}
	if st.dataCalls == 0 {
		t.Fatal("precondition: the lane must have tried to read the PR data")
	}
}

// TestRegAuthorityExcludesRelay: an on-behalf-of relay names no
// authority (attribution, never sign-off), matching the offline gate.
func TestRegAuthorityExcludesRelay(t *testing.T) {
	raw := []byte("---\nid: F-x\nauthorized-by: on-behalf-of human:alex\nparked-by: human:bob\n---\n")
	got := registerAuthorityNames(raw, "authorized-by")
	if len(got) != 0 {
		t.Errorf("relay must contribute no authority name; got %v", got)
	}
	got = registerAuthorityNames(raw, "parked-by", "authorized-by")
	if len(got) != 1 || got[0] != "bob" {
		t.Errorf("parked-by names = %v, want [bob]", got)
	}
}

// TestRegTouchedFileList: the current and the previous (rename) name
// both count, a nested file under the findings dir does not, other paths are
// ignored, and a patch-less entry (removal-only or oversize) still counts.
func TestRegTouchedFileList(t *testing.T) {
	got := touchedFindings([]ghPRFile{
		{Filename: "docs/streams/findings/renamed.md", PreviousFilename: findingRel},
		{Filename: "docs/streams/findings/sub/x.md"},
		{Filename: "README.md"},
		{Filename: "docs/streams/findings/no-patch.md"},
	})
	want := []string{"docs/streams/findings/2026-07-17-f-gut.md", "docs/streams/findings/no-patch.md", "docs/streams/findings/renamed.md"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("touchedFindings = %v, want %v", got, want)
	}
}

// TestRegReusedAnchorAckFails is the second instance of the
// defect class (an authority anchor honored without on-PR corroboration), on a
// different field and a REMOVAL-ONLY change: dropping `ack:` from an entry that
// already carries an anchor adds no diff line at all, so neither the stamp lane
// nor an added-line reader sees anything. The transition lane must still fail it.
func TestRegReusedAnchorAckFails(t *testing.T) {
	landed := strings.Replace(landedAnchoredFinding, "resolved: false", "resolved: false\nack: \"2026-07-18\"", 1)
	root, path := gutFixture(t, landed)
	mutateFinding(t, path, landed, "\nack: \"2026-07-18\"", "")
	if p := guttedRegisterFields(root); len(p) != 0 {
		t.Fatalf("precondition: the offline gate accepts the pre-existing anchor; got %v", p)
	}
	st := &corroborateStub{files: findingFiles(`-ack: "2026-07-18"`), mergeBase: originMain(t, root)}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("an ack removal authorized only by a pre-existing anchor must fail; rc=%d", rc)
	}
}
