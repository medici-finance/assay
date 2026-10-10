package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	// cwd is the directory, relative to the fixture root, the command runs
	// from ("" = the repository top level).
	cwd string
	// realMergeBase leaves the production merge-base resolver in place (only
	// the forge's base-branch name is stubbed, as "main"), so a test exercises
	// how the base is really resolved instead of handing the lane a SHA.
	realMergeBase bool
}

// runCorroborateOn chdirs into root (or st.cwd under it), stubs the forge/git
// seams, and runs the command for PR #7, returning its exit code.
func runCorroborateOn(t *testing.T, root string, st *corroborateStub) int {
	t.Helper()
	t.Chdir(filepath.Join(root, filepath.FromSlash(st.cwd)))
	oldRepo, oldFiles, oldData, oldMB := corroborateRepoFn, corroborateFilesFn, corroborateDataFn, corroborateMergeBaseFn
	oldCount, oldBaseRef := corroborateChangedFilesFn, ghPRBaseRefFn
	t.Cleanup(func() {
		corroborateRepoFn, corroborateFilesFn, corroborateDataFn, corroborateMergeBaseFn = oldRepo, oldFiles, oldData, oldMB
		corroborateChangedFilesFn, ghPRBaseRefFn = oldCount, oldBaseRef
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
	if st.realMergeBase {
		ghPRBaseRefFn = func(string, int) string { return "main" }
	} else {
		corroborateMergeBaseFn = func(string, string, int) string { return st.mergeBase }
	}
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

// TestRegSelfParkFails / Approved: an in-horizon park named to a mapped
// human fails until that human acts, then passes; an approval COMMENT counts.
func TestRegSelfParkFails(t *testing.T) {
	pinParkNow(t, "2026-07-20")
	root, path := gutFixture(t, landedOpenFinding)
	mutateFinding(t, path, landedOpenFinding, "resolved: false",
		"resolved: false\nparked-until: \"2026-09-01\"\nparked-by: human:alex\nparked-reason: deferred")
	files := findingFiles("+parked-until: \"2026-09-01\"", "+parked-by: human:alex", "+parked-reason: deferred")
	st := &corroborateStub{files: files, mergeBase: originMain(t, root)}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("a self-park must fail; rc=%d", rc)
	}
}

func TestRegParkApprovedComment(t *testing.T) {
	pinParkNow(t, "2026-07-20")
	root, path := gutFixture(t, landedOpenFinding)
	mutateFinding(t, path, landedOpenFinding, "resolved: false",
		"resolved: false\nparked-until: \"2026-09-01\"\nparked-by: human:alex\nparked-reason: deferred")
	files := findingFiles("+parked-until: \"2026-09-01\"", "+parked-by: human:alex", "+parked-reason: deferred")
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
	// The tree carries the change the listing names, so the base is shown to
	// be the PR's base (see TestRegHeadAsBaseFails).
	if err := os.WriteFile(filepath.Join(root, "docs", "other.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitGut(t, root)
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
	pinParkNow(t, "2026-11-01")
	root, path := gutFixture(t, landedParkAnchored)
	mutateFinding(t, path, landedParkAnchored, `parked-until: "2026-12-01"`, `parked-until: "2027-01-15"`)
	st := &corroborateStub{files: findingFiles(`-parked-until: "2026-12-01"`, `+parked-until: "2027-01-15"`), mergeBase: originMain(t, root)}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("a park extension authorized only by a pre-existing parked-by must fail; rc=%d", rc)
	}
}

// TestRegReusedParkExtendOK: the same in-horizon extension passes once the
// parked-by human approves the PR — the control for the two tests below.
func TestRegReusedParkExtendOK(t *testing.T) {
	pinParkNow(t, "2026-11-01")
	root, path := gutFixture(t, landedParkAnchored)
	mutateFinding(t, path, landedParkAnchored, `parked-until: "2026-12-01"`, `parked-until: "2027-01-15"`)
	st := &corroborateStub{
		files:     findingFiles(`-parked-until: "2026-12-01"`, `+parked-until: "2027-01-15"`),
		mergeBase: originMain(t, root),
		data:      approvedBy("ada"),
	}
	if rc := runCorroborateOn(t, root, st); rc != 0 {
		t.Fatalf("an in-horizon extension the parked-by human approved must pass; rc=%d", rc)
	}
}

// TestRegParkHorizonFails: a park added past the 90-day horizon is MISSING even
// when its parked-by human approved the PR — no approval authorizes it (#2012
// ruling, item 1).
func TestRegParkHorizonFails(t *testing.T) {
	pinParkNow(t, "2026-07-20")
	root, path := gutFixture(t, landedOpenFinding)
	mutateFinding(t, path, landedOpenFinding, "resolved: false",
		"resolved: false\nparked-until: \"2099-01-01\"\nparked-by: human:alex\nparked-reason: deferred")
	st := &corroborateStub{
		files:     findingFiles("+parked-until: \"2099-01-01\"", "+parked-by: human:alex", "+parked-reason: deferred"),
		mergeBase: originMain(t, root),
		data:      approvedBy("ada"),
	}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("an approved park past the horizon must fail online; rc=%d", rc)
	}
}

// TestRegExtendHorizonFails: the planted second instance online — an approved
// EXTENSION that crosses the 90 days fails too.
func TestRegExtendHorizonFails(t *testing.T) {
	pinParkNow(t, "2026-11-01") // + 90 days = 2027-01-30
	root, path := gutFixture(t, landedParkAnchored)
	mutateFinding(t, path, landedParkAnchored, `parked-until: "2026-12-01"`, `parked-until: "2027-02-01"`)
	st := &corroborateStub{
		files:     findingFiles(`-parked-until: "2026-12-01"`, `+parked-until: "2027-02-01"`),
		mergeBase: originMain(t, root),
		data:      approvedBy("ada"),
	}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("an approved extension past the horizon must fail online; rc=%d", rc)
	}
}

// TestRegAuthByNotParkOnline: online, a park is corroborated only through
// parked-by (#2012 ruling, item 2). The landed park names an unmapped account
// under parked-by; the PR extends it and adds the mapped human under
// authorized-by, who approves the PR. The stamp lane is satisfied (the added
// stamp's human acted), so only the register lane can fail it: still MISSING.
func TestRegAuthByNotParkOnline(t *testing.T) {
	pinParkNow(t, "2026-07-20")
	root, path := gutFixture(t, landedParkedFinding) // parked-until 2026-08-01, parked-by human:bot
	mutateFinding(t, path, landedParkedFinding, `parked-until: "2026-08-01"`,
		"authorized-by: human:alex\nparked-until: \"2026-09-01\"")
	st := &corroborateStub{
		files: findingFiles("+authorized-by: human:alex",
			`-parked-until: "2026-08-01"`, `+parked-until: "2026-09-01"`),
		mergeBase: originMain(t, root),
		data:      approvedBy("ada"),
	}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("authorized-by must not corroborate a park online; rc=%d", rc)
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

// commitGut commits the fixture's working-tree change on the feature branch,
// so the gut is in HEAD rather than only on disk — the CI shape.
func commitGut(t *testing.T, root string) {
	t.Helper()
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "gut")
}

// headSHA returns the fixture's HEAD commit.
func headSHA(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// TestRegSubdirCwdFails: the lane sees the same tree whatever directory the
// command runs from. Run from a subdirectory (the `cd statusgen && …` shape),
// a committed reused-anchor resolve flip must still fail — whether or not the
// forge listing names the entry.
func TestRegSubdirCwdFails(t *testing.T) {
	for _, listing := range [][]ghPRFile{findingFiles("-resolved: false", "+resolved: true"), otherFileListing()} {
		root, path := gutFixture(t, landedAnchoredFinding)
		mutateFinding(t, path, landedAnchoredFinding, "resolved: false", "resolved: true")
		commitGut(t, root)
		mustMkdirAll(t, filepath.Join(root, "statusgen"))
		st := &corroborateStub{files: listing, mergeBase: originMain(t, root), cwd: "statusgen"}
		if rc := runCorroborateOn(t, root, st); rc != 1 {
			t.Fatalf("run from a subdirectory, an uncorroborated resolve flip (listing %s) must fail; rc=%d", listing[0].Filename, rc)
		}
	}
}

// TestRegUnreadableEntryFails: only a touched entry that does not exist is a
// deletion the tombstone guard owns. One that exists but cannot be read (here:
// a directory where the entry file was) proves nothing — fail closed.
func TestRegUnreadableEntryFails(t *testing.T) {
	root, path := gutFixture(t, landedAnchoredFinding)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	mustMkdirAll(t, path)
	st := &corroborateStub{files: findingFiles("-resolved: false", "+resolved: true"), mergeBase: originMain(t, root)}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("a touched entry that exists but cannot be read must fail closed; rc=%d", rc)
	}
}

// TestRegNoBaseCountErrFails: no merge-base, an EMPTY listing and an unreadable
// changed-file count. 0 listed == 0 counted would read as a complete listing,
// so the count error itself must fail the run.
func TestRegNoBaseCountErrFails(t *testing.T) {
	root, _ := gutFixture(t, landedOpenFinding)
	st := &corroborateStub{countErr: errors.New("count unreadable")}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("an unreadable changed-file count must fail closed even beside an empty listing; rc=%d", rc)
	}
}

// TestRegHeadAsBaseFails: a merge-base that is the PR's own commit compares
// the PR with itself, so a committed gut shows no local change. The forge says
// the PR changes files, so a tree identical to that base proves the base wrong.
func TestRegHeadAsBaseFails(t *testing.T) {
	root, path := gutFixture(t, landedAnchoredFinding)
	mutateFinding(t, path, landedAnchoredFinding, "resolved: false", "resolved: true")
	commitGut(t, root)
	st := &corroborateStub{files: findingFiles("-resolved: false", "+resolved: true"), mergeBase: headSHA(t, root)}
	if rc := runCorroborateOn(t, root, st); rc != 1 {
		t.Fatalf("a merge-base equal to the PR head, with a non-empty forge listing, must fail closed; rc=%d", rc)
	}
}

// TestMergeBaseIgnoresDecoyRefs drives the REAL resolver, not the seam. A tag
// named origin/main on the PR's own commit outranks refs/remotes/origin/main
// for the short name; so does a local branch named main for the bare base name.
// The resolver must read only the fully-qualified remote-tracking ref.
func TestMergeBaseIgnoresDecoyRefs(t *testing.T) {
	old := ghPRBaseRefFn
	t.Cleanup(func() { ghPRBaseRefFn = old })
	ghPRBaseRefFn = func(string, int) string { return "main" }

	root, path := gutFixture(t, landedAnchoredFinding)
	mutateFinding(t, path, landedAnchoredFinding, "resolved: false", "resolved: true")
	commitGut(t, root)
	gitRun(t, root, "tag", "origin/main", "HEAD")
	if got, want := prMergeBaseSHA(root, "example/repo", 7), originMain(t, root); got != want {
		t.Errorf("decoy tag origin/main: merge-base = %q, want the remote-tracking base %q", got, want)
	}

	root2, path2 := gutFixture(t, landedAnchoredFinding)
	mutateFinding(t, path2, landedAnchoredFinding, "resolved: false", "resolved: true")
	commitGut(t, root2)
	gitRun(t, root2, "branch", "main", "HEAD")
	gitRun(t, root2, "update-ref", "-d", "refs/remotes/origin/main")
	if got := prMergeBaseSHA(root2, "example/repo", 7); got != "" {
		t.Errorf("no remote-tracking base and a local branch main: merge-base = %q, want \"\" (unresolvable)", got)
	}
}

// TestRegDecoyTagBaseFails: the whole command, real resolver, a decoy tag
// origin/main on the PR's own commit — a committed reused-anchor gut must still
// fail. Without the tag (control) it fails too.
func TestRegDecoyTagBaseFails(t *testing.T) {
	for _, decoy := range []bool{false, true} {
		root, path := gutFixture(t, landedAnchoredFinding)
		mutateFinding(t, path, landedAnchoredFinding, "resolved: false", "resolved: true")
		commitGut(t, root)
		if decoy {
			gitRun(t, root, "tag", "origin/main", "HEAD")
		}
		st := &corroborateStub{files: findingFiles("-resolved: false", "+resolved: true"), realMergeBase: true}
		if rc := runCorroborateOn(t, root, st); rc != 1 {
			t.Fatalf("decoy=%v: an uncorroborated resolve flip must fail under the real base resolver; rc=%d", decoy, rc)
		}
	}
}

// TestRegReportLineOneLine: entry-controlled text (a moved affects value, an
// entry path) is printed on ONE report line; a newline in it must not start a
// forged second line.
func TestRegReportLineOneLine(t *testing.T) {
	line := registerReportLine(registerTransitionResult{
		Rel:      "docs/streams/findings/x.md",
		Moves:    "affects dropped [a\nregister docs/streams/findings/y.md [] CORROBORATED — ok]",
		Verdict:  verdictMissing,
		Evidence: "e f",
	})
	if strings.ContainsAny(line, "\n\r ") {
		t.Fatalf("report line carries a line break from entry text: %q", line)
	}
}

// shortOriginRefSites returns the "file:line" of every `"origin/…" + x`
// concatenation in src: a git ref built by its SHORT name, which a tag or a
// local branch of the same name shadows (git resolves refs/tags/ and refs/heads/
// ahead of refs/remotes/). The guard below runs it over every statusgen source.
func shortOriginRefSites(fset *token.FileSet, f *ast.File) []string {
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		be, ok := n.(*ast.BinaryExpr)
		if !ok || be.Op != token.ADD {
			return true
		}
		lit, ok := be.X.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if s, err := strconv.Unquote(lit.Value); err == nil && strings.HasPrefix(s, "origin/") {
			out = append(out, fset.Position(lit.Pos()).String())
		}
		return true
	})
	return out
}

// TestNoShortOriginRefConcat is the class guard for SEC-2013-4's shape: no
// statusgen source builds a remote ref as "origin/" + name. Spell the ref in
// full (refs/remotes/origin/<name>, as remoteMainRef does). The planted source
// is the positive control — a matcher that stops matching fails here instead of
// reporting clean.
func TestNoShortOriginRefConcat(t *testing.T) {
	fset := token.NewFileSet()
	plant, err := parser.ParseFile(fset, "plant.go", "package p\nvar b = \"main\"\nvar r = \"origin/\" + b\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := shortOriginRefSites(fset, plant); len(got) != 1 {
		t.Fatalf("positive control: the planted short-ref concatenation must be flagged once; got %v", got)
	}
	matches, err := filepath.Glob("*.go")
	if err != nil || len(matches) == 0 {
		t.Fatalf("no statusgen sources found (err %v)", err)
	}
	for _, m := range matches {
		if strings.HasSuffix(m, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, m, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", m, err)
		}
		for _, site := range shortOriginRefSites(fset, f) {
			t.Errorf("%s: a remote ref built by its short name (\"origin/\" + …) — spell it refs/remotes/origin/<name>", site)
		}
	}
}

// absentRefDecoys are the refs git's short-name rules expand the FULL name
// refs/remotes/origin/main into when that exact ref does not exist: a full
// refname is still looked up through refs/, refs/tags/, refs/heads/ and
// refs/remotes/ in turn. Any of them, placed on a PR commit, would make that
// commit the base unless the resolver requires the exact ref to exist.
var absentRefDecoys = []string{
	"refs/refs/remotes/origin/main",
	"refs/tags/refs/remotes/origin/main",
	"refs/heads/refs/remotes/origin/main",
	"refs/remotes/refs/remotes/origin/main",
}

// decoyAfterGut commits a reused-anchor gut, then a second, unrelated commit,
// deletes refs/remotes/origin/main and (when decoy != "") points decoy at the
// gut commit: an EARLIER commit of the same PR, whose tree is not the PR's.
func decoyAfterGut(t *testing.T, decoy string) string {
	t.Helper()
	root, path := gutFixture(t, landedAnchoredFinding)
	mutateFinding(t, path, landedAnchoredFinding, "resolved: false", "resolved: true")
	commitGut(t, root)
	if err := os.WriteFile(filepath.Join(root, "other.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commitGut(t, root)
	gitRun(t, root, "update-ref", "-d", "refs/remotes/origin/main")
	if decoy != "" {
		gitRun(t, root, "update-ref", decoy, "HEAD~1")
	}
	return root
}

// TestMergeBaseAbsentRefDecoys drives the real resolvers with the base's
// remote-tracking ref ABSENT. Every decoy git would read the full name as must
// leave the base unresolved — online (prMergeBaseSHA) and offline
// (registerLandedBase) alike.
func TestMergeBaseAbsentRefDecoys(t *testing.T) {
	old := ghPRBaseRefFn
	t.Cleanup(func() { ghPRBaseRefFn = old })
	ghPRBaseRefFn = func(string, int) string { return "main" }
	for _, decoy := range absentRefDecoys {
		root := decoyAfterGut(t, decoy)
		if got := prMergeBaseSHA(root, "example/repo", 7); got != "" {
			t.Errorf("%s with no remote-tracking base: merge-base = %q, want \"\" (unresolvable)", decoy, got)
		}
		if got, ok := registerLandedBase(root); ok {
			t.Errorf("%s with no remote-tracking base: offline base = %q resolved, want the unresolved fallback", decoy, got)
		}
	}
}

// TestRegAbsentRefDecoyFails: the whole command, real resolver, the base's
// remote-tracking ref absent and a decoy on an earlier commit of the PR (so the
// identical-tree refusal cannot catch it). A reused-anchor gut must still fail.
// The control (no decoy) fails too.
func TestRegAbsentRefDecoyFails(t *testing.T) {
	for _, decoy := range append([]string{""}, absentRefDecoys...) {
		root := decoyAfterGut(t, decoy)
		st := &corroborateStub{files: findingFiles("-resolved: false", "+resolved: true"), realMergeBase: true}
		if rc := runCorroborateOn(t, root, st); rc != 1 {
			t.Errorf("decoy %q: an uncorroborated resolve flip must fail with the base ref absent; rc=%d", decoy, rc)
		}
	}
}

// mergeBaseAllow names every function allowed to run `git merge-base` itself.
// The class: a FIXED base ref handed to git by name, which git expands through
// other namespaces when the exact ref is absent. A fixed base goes through
// mergeBaseExact; the rest take a revision an operator (or a resolved commit)
// supplies, and are listed with the reason. The allow-list keys on the enclosing
// function, so TestFixedBaseNotHandedToHelpers separately fails on a fixed ref
// passed as an argument into one of the listed helpers.
var mergeBaseAllow = map[string]string{
	"resolveMergeBaseExact":  "the choke point (mergeBaseExact's uncached body): resolves the fixed ref exactly, then runs merge-base on its object id",
	"gitMergeBaseOut":        "per-run memo of a by-name merge-base for an operator-supplied revision (TestFixedBaseNotHandedToHelpers fails on a fixed ref passed in)",
	"consumerEntriesAtBase":  "operator-supplied --base revision",
	"pinConsumerBase":        "operator-supplied --base revision (its only caller passes the --base value; TestFixedBaseNotHandedToHelpers fails on a fixed ref passed in)",
	"productionResolveBase":  "operator-supplied base revision",
	"runMergecheck":          "both sides already resolved to commits by resolveCommit",
	"ancestorNoOtherChanges": "--is-ancestor on two commits verified with cat-file first",
	"commitIsAncestor":       "--is-ancestor on a hex-validated witness base resolved to an object id by rev-parse first, against HEAD (verify-integrity/03 fail-first provenance)",
	"flipMergeBases":         "both sides are the %P parent object ids of a commit rev-list yielded; it refuses any operand isObjectID rejects and passes --end-of-options before them",
}

// mergeBaseCallSites returns "name file:line" for every exec.Command / gitOut
// call that runs merge-base in f, keyed by its enclosing top-level declaration.
func mergeBaseCallSites(fset *token.FileSet, f *ast.File) (names, sites []string) {
	for _, d := range f.Decls {
		name := ""
		switch d := d.(type) {
		case *ast.FuncDecl:
			name = d.Name.Name
		case *ast.GenDecl:
			for _, s := range d.Specs {
				if vs, ok := s.(*ast.ValueSpec); ok && len(vs.Names) > 0 {
					name = vs.Names[0].Name
				}
			}
		}
		ast.Inspect(d, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				if x, ok := fn.X.(*ast.Ident); !ok || x.Name != "exec" {
					return true
				}
			case *ast.Ident:
				if fn.Name != "gitOut" {
					return true
				}
			default:
				return true
			}
			for _, a := range call.Args {
				if lit, ok := a.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if s, err := strconv.Unquote(lit.Value); err == nil && s == "merge-base" {
						names = append(names, name)
						sites = append(sites, name+" "+fset.Position(call.Pos()).String())
					}
				}
			}
			return true
		})
	}
	return names, sites
}

// TestMergeBaseChokePoint is the class guard for SEC-2013-4: no statusgen
// source runs `git merge-base` outside mergeBaseAllow, so a new caller that
// hands git a fixed base by name is red here. The planted source is the
// positive control; the choke point itself must be found, so a matcher that
// stops matching fails instead of reporting clean.
func TestMergeBaseChokePoint(t *testing.T) {
	fset := token.NewFileSet()
	plant, err := parser.ParseFile(fset, "plant.go",
		"package p\nimport \"os/exec\"\nfunc planted(r string) { exec.Command(\"git\", \"merge-base\", \"HEAD\", r).Run() }\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := mergeBaseCallSites(fset, plant); len(got) != 1 || got[0] != "planted" {
		t.Fatalf("positive control: the planted merge-base call must be flagged once; got %v", got)
	}
	matches, err := filepath.Glob("*.go")
	if err != nil || len(matches) == 0 {
		t.Fatalf("no statusgen sources found (err %v)", err)
	}
	seen := map[string]bool{}
	for _, m := range matches {
		if strings.HasSuffix(m, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, m, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", m, err)
		}
		names, sites := mergeBaseCallSites(fset, f)
		for i, n := range names {
			seen[n] = true
			if _, ok := mergeBaseAllow[n]; !ok {
				t.Errorf("%s: git merge-base run outside the choke point — resolve a fixed base with mergeBaseExact", sites[i])
			}
		}
	}
	if !seen["resolveMergeBaseExact"] {
		t.Errorf("the choke point resolveMergeBaseExact runs no merge-base the guard can see")
	}
}

// byNameBaseHelpers are the allow-listed functions that hand a base revision to
// git by NAME. They are fine for an operator-supplied value and wrong for a
// fixed ref, which must go through mergeBaseExact.
var byNameBaseHelpers = map[string]bool{
	"gitMergeBaseOut":       true,
	"pinConsumerBase":       true,
	"consumerEntriesAtBase": true,
	"productionResolveBase": true,
}

// fixedBaseHandedSites returns "file:line" for every call in f to a by-name
// helper that passes the fixed ref (remoteMainRef, defaultMergecheckBase, or the
// literal refs/remotes/origin/ name) as an argument.
func fixedBaseHandedSites(fset *token.FileSet, f *ast.File) []string {
	var sites []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || !byNameBaseHelpers[id.Name] {
			return true
		}
		for _, a := range call.Args {
			switch a := a.(type) {
			case *ast.Ident:
				if a.Name == "remoteMainRef" || a.Name == "defaultMergecheckBase" {
					sites = append(sites, fset.Position(call.Pos()).String())
				}
			case *ast.BasicLit:
				if s, err := strconv.Unquote(a.Value); err == nil && strings.HasPrefix(s, "refs/remotes/origin/") {
					sites = append(sites, fset.Position(call.Pos()).String())
				}
			}
		}
		return true
	})
	return sites
}

// TestFixedBaseNotHandedToHelpers is the second half of the choke-point guard:
// TestMergeBaseChokePoint keys on the function that runs `git merge-base`, so a
// fixed base ref passed INTO an allow-listed by-name helper (the shape of the
// obligation-derivation base before it was routed through mergeBaseExact) is
// invisible to it. The planted source is the positive control.
func TestFixedBaseNotHandedToHelpers(t *testing.T) {
	fset := token.NewFileSet()
	plant, err := parser.ParseFile(fset, "plant.go",
		"package p\nfunc planted(root string) { pinConsumerBase(root, remoteMainRef); productionResolveBase(root, \"refs/remotes/origin/main\") }\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := fixedBaseHandedSites(fset, plant); len(got) != 2 {
		t.Fatalf("positive control: both planted calls must be flagged; got %v", got)
	}
	matches, err := filepath.Glob("*.go")
	if err != nil || len(matches) == 0 {
		t.Fatalf("no statusgen sources found (err %v)", err)
	}
	for _, m := range matches {
		if strings.HasSuffix(m, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, m, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", m, err)
		}
		for _, site := range fixedBaseHandedSites(fset, f) {
			t.Errorf("%s: a fixed base ref handed to a by-name helper — resolve it with mergeBaseExact", site)
		}
	}
}

// TestBranchChangedSetExactBase drives the real branchChangedSet (the
// obligation-derivation base) with the base's remote-tracking ref ABSENT and a
// decoy on an earlier PR commit: every decoy must leave the diff unavailable
// (a could-not-check), never a diff against the PR's own commit. The control,
// with the exact ref present, yields the PR's changed paths.
func TestBranchChangedSetExactBase(t *testing.T) {
	for _, decoy := range absentRefDecoys {
		root := decoyAfterGut(t, decoy)
		if set, ok := branchChangedSet(root); ok {
			t.Errorf("%s with no remote-tracking base: branchChangedSet = %v, want unavailable", decoy, set)
		}
	}
	root := decoyAfterGut(t, "")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD~2")
	set, ok := branchChangedSet(root)
	if !ok || !set["other.txt"] {
		t.Errorf("control with the exact ref present: branchChangedSet = %v, %v; want other.txt in the set", set, ok)
	}
}

// TestMergeBaseExactRunMemo pins that mergeBaseExact is memoised for ONE run's
// git read session only: inside a session a second call returns the first
// answer even after the ref is deleted, and once the session ends the next call
// re-resolves (and now finds the ref absent). The memo keeps the fixed-base
// callers of one --lint to one resolution; it must never cross a run.
func TestMergeBaseExactRunMemo(t *testing.T) {
	root := decoyAfterGut(t, "")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD~2")
	end := beginGitReadSession()
	first := mergeBaseExact(root, remoteMainRef)
	if first == "" {
		end()
		t.Fatal("control: the exact ref is present, so a merge-base must resolve")
	}
	gitRun(t, root, "update-ref", "-d", "refs/remotes/origin/main")
	if again := mergeBaseExact(root, remoteMainRef); again != first {
		t.Errorf("inside one session the answer must be memoised: got %q, want %q", again, first)
	}
	end()
	if after := mergeBaseExact(root, remoteMainRef); after != "" {
		t.Errorf("after the session ends the ref must be re-resolved (now absent): got %q", after)
	}
	// A SECOND session starts with an empty memo: it must not inherit the first
	// session's answer.
	end2 := beginGitReadSession()
	defer end2()
	if next := mergeBaseExact(root, remoteMainRef); next != "" {
		t.Errorf("a new session must re-resolve (ref now absent): got %q", next)
	}
}

// TestMergeBaseMemoPerRoot pins the root half of the memo key: inside one
// session, two trees asking for the same ref get their own answers — one whose
// exact ref is present resolves, one whose ref is absent stays unresolved.
func TestMergeBaseMemoPerRoot(t *testing.T) {
	withRef := decoyAfterGut(t, "")
	gitRun(t, withRef, "update-ref", "refs/remotes/origin/main", "HEAD~2")
	noRef := decoyAfterGut(t, "")
	end := beginGitReadSession()
	defer end()
	if mb := mergeBaseExact(withRef, remoteMainRef); mb == "" {
		t.Fatal("control: the tree with the exact ref must resolve a merge-base")
	}
	if mb := mergeBaseExact(noRef, remoteMainRef); mb != "" {
		t.Errorf("another root in the same session must not reuse the first answer: got %q", mb)
	}
}
