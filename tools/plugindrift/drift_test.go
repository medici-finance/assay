package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// fakeClient is a scripted SourceClient. Anything not scripted errors, so a
// test can never accidentally pass on a silent default.
type fakeClient struct {
	branch    string
	branchErr error
	// blobs is keyed "repo\x00path\x00ref".
	blobs   map[string]string
	blobErr map[string]error
	// commits is keyed "repo\x00path"; value is (count, capped).
	commits    map[string][2]int
	commitsErr error
	// commitDate is keyed "repo\x00commit".
	commitDate    map[string]string
	commitDateErr error
	// tagCommits is keyed "repo\x00tag"; value is the peeled commit sha.
	tagCommits map[string]string
	tagErr     map[string]error
	calls      []string
}

func key(parts ...string) string { return strings.Join(parts, "\x00") }

func (f *fakeClient) DefaultBranch(repo string) (string, error) {
	f.calls = append(f.calls, "DefaultBranch:"+repo)
	if f.branchErr != nil {
		return "", f.branchErr
	}
	return f.branch, nil
}

func (f *fakeClient) BlobSHA(repo, path, ref string) (string, error) {
	f.calls = append(f.calls, "BlobSHA:"+repo+":"+path)
	k := key(repo, path, ref)
	if err, ok := f.blobErr[k]; ok {
		return "", err
	}
	if sha, ok := f.blobs[k]; ok {
		return sha, nil
	}
	return "", fmt.Errorf("fake: no scripted blob for %s@%s", path, ref)
}

func (f *fakeClient) TagCommit(repo, tag string) (string, error) {
	f.calls = append(f.calls, "TagCommit:"+repo+":"+tag)
	k := key(repo, tag)
	if err, ok := f.tagErr[k]; ok {
		return "", err
	}
	if sha, ok := f.tagCommits[k]; ok {
		return sha, nil
	}
	return "", fmt.Errorf("fake: no scripted tag %s in %s", tag, repo)
}

func (f *fakeClient) CommitDate(repo, commit string) (string, error) {
	f.calls = append(f.calls, "CommitDate:"+repo+":"+commit)
	if f.commitDateErr != nil {
		return "", f.commitDateErr
	}
	if d, ok := f.commitDate[key(repo, commit)]; ok {
		return d, nil
	}
	// Default: every scripted repo resolves the snapshot commit.
	return "2026-07-17T03:55:06Z", nil
}

// sinceSeen records the `since` value the last CommitsSince call was given.
func (f *fakeClient) CommitsSince(repo, path, ref, since string, maxPages int) (int, bool, error) {
	f.calls = append(f.calls, "CommitsSince:"+repo+":"+path+":"+since)
	if f.commitsErr != nil {
		return 0, false, f.commitsErr
	}
	if v, ok := f.commits[key(repo, path)]; ok {
		return v[0], v[1] == 1, nil
	}
	return 0, false, fmt.Errorf("fake: no scripted history for %s", path)
}

func ghSource(blob string) Source {
	return Source{
		Kind:   "github",
		Repo:   "acme/upstream",
		Path:   ".claude/skills/the-desk/SKILL.md",
		Commit: "52cf9d524e5f4cf103d1f0e24ef305646063999c",
		Blob:   blob,
		AsOf:   "2026-07-17",
	}
}

func manifestOf(files ...BundleFile) *Manifest {
	return &Manifest{Version: 1, Bundle: "plugins/assay", BundleVersion: "0.1.0", Files: files}
}

func one(t *testing.T, results []Result) Result {
	t.Helper()
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d: %+v", len(results), results)
	}
	return results[0]
}

// --- no drift ---------------------------------------------------------------

func TestCheck_NoDrift(t *testing.T) {
	src := ghSource("blob-recorded")
	c := &fakeClient{
		branch: "main",
		blobs:  map[string]string{key(src.Repo, src.Path, "main"): "blob-recorded"},
	}

	r := one(t, Check(c, manifestOf(BundleFile{Path: "skills/the-desk/SKILL.md", Source: src}), 5))
	if r.Status != StatusInSync {
		t.Fatalf("status = %q, want %q (%s)", r.Status, StatusInSync, r.Detail)
	}
	if r.Commits != 0 {
		t.Errorf("commits = %d, want 0", r.Commits)
	}
	if s := Summarize([]Result{r}); s.Drift {
		t.Error("Summarize reported drift on an in-sync source")
	}
	// An identical blob must short-circuit: no history walk needed.
	for _, call := range c.calls {
		if strings.HasPrefix(call, "CommitsSince") {
			t.Errorf("walked commit history despite an identical blob: %v", c.calls)
		}
	}
}

// A github source with no recorded blob must never be reported in-sync: with no
// baseline there is nothing to compare, so the honest answer is "unknown".
func TestCheck_GithubSourceWithoutRecordedBlobIsUnreachableNotInSync(t *testing.T) {
	src := ghSource("")
	c := &fakeClient{
		branch:  "main",
		blobs:   map[string]string{key(src.Repo, src.Path, "main"): "whatever"},
		commits: map[string][2]int{key(src.Repo, src.Path): {0, 0}},
	}

	r := one(t, Check(c, manifestOf(BundleFile{Path: "skills/the-desk/SKILL.md", Source: src}), 5))
	if r.Status != StatusUnreachable {
		t.Fatalf("status = %q, want %q (%s)", r.Status, StatusUnreachable, r.Detail)
	}
	if !strings.Contains(r.Detail, "no recorded blob") {
		t.Errorf("detail does not say why: %q", r.Detail)
	}
	if s := Summarize([]Result{r}); !s.Drift {
		t.Error("a blob-less source was not counted as drift")
	}
	// It must not spend calls pretending to measure something it cannot.
	for _, call := range c.calls {
		if strings.HasPrefix(call, "CommitsSince") {
			t.Errorf("walked commit history for a source with no baseline: %v", c.calls)
		}
	}
}

// --- drift detected ---------------------------------------------------------

func TestCheck_DriftDetected(t *testing.T) {
	src := ghSource("blob-recorded")
	c := &fakeClient{
		branch:  "main",
		blobs:   map[string]string{key(src.Repo, src.Path, "main"): "blob-moved-on"},
		commits: map[string][2]int{key(src.Repo, src.Path): {29, 0}},
	}

	r := one(t, Check(c, manifestOf(BundleFile{Path: "skills/the-desk/SKILL.md", Source: src}), 5))
	if r.Status != StatusBehind {
		t.Fatalf("status = %q, want %q (%s)", r.Status, StatusBehind, r.Detail)
	}
	if r.Commits != 29 {
		t.Errorf("commits = %d, want 29", r.Commits)
	}
	if !strings.Contains(r.Detail, "29 commit(s)") {
		t.Errorf("detail does not report the count: %q", r.Detail)
	}
	if strings.Contains(r.Detail, "at least") {
		t.Errorf("an uncapped count must not be hedged as a lower bound: %q", r.Detail)
	}
	s := Summarize([]Result{r})
	if !s.Drift {
		t.Error("Summarize did not report drift on a behind source")
	}
	if !strings.Contains(s.Verdict(), "DRIFT") {
		t.Errorf("verdict = %q, want DRIFT", s.Verdict())
	}
}

func TestCheck_DriftCountCappedIsReportedAsLowerBound(t *testing.T) {
	src := ghSource("blob-recorded")
	c := &fakeClient{
		branch:  "main",
		blobs:   map[string]string{key(src.Repo, src.Path, "main"): "blob-moved-on"},
		commits: map[string][2]int{key(src.Repo, src.Path): {500, 1}}, // hit the page ceiling
	}

	r := one(t, Check(c, manifestOf(BundleFile{Path: "skills/the-desk/SKILL.md", Source: src}), 5))
	if r.Status != StatusBehind {
		t.Fatalf("status = %q, want %q", r.Status, StatusBehind)
	}
	if !strings.Contains(r.Detail, "at least 500") {
		t.Errorf("detail should report a lower bound, got %q", r.Detail)
	}
}

// The recorded commit is the tree the copy was taken from — it need not be a
// commit that touched the path — so the count must be anchored on that commit's
// DATE, one second later so the commit itself is excluded.
func TestCheck_CommitCountIsAnchoredOnCommitDatePlusOneSecond(t *testing.T) {
	src := ghSource("blob-recorded")
	c := &fakeClient{
		branch:     "main",
		blobs:      map[string]string{key(src.Repo, src.Path, "main"): "blob-moved-on"},
		commitDate: map[string]string{key(src.Repo, src.Commit): "2026-07-17T03:55:06Z"},
		commits:    map[string][2]int{key(src.Repo, src.Path): {31, 0}},
	}

	Check(c, manifestOf(BundleFile{Path: "skills/the-desk/SKILL.md", Source: src}), 5)

	want := "CommitsSince:" + src.Repo + ":" + src.Path + ":2026-07-17T03:55:07Z"
	found := false
	for _, call := range c.calls {
		if call == want {
			found = true
		}
	}
	if !found {
		t.Errorf("want a history call anchored at %q, got calls %v", want, c.calls)
	}
}

func TestCheck_RecordedCommitDoesNotResolve(t *testing.T) {
	src := ghSource("blob-recorded")
	c := &fakeClient{
		branch:        "main",
		blobs:         map[string]string{key(src.Repo, src.Path, "main"): "blob-moved-on"},
		commitDateErr: fmt.Errorf("%w: no commit found for SHA", ErrNotFound),
	}

	r := one(t, Check(c, manifestOf(BundleFile{Path: "skills/the-desk/SKILL.md", Source: src}), 5))
	if r.Status != StatusUnreachable {
		t.Fatalf("status = %q, want %q (%s)", r.Status, StatusUnreachable, r.Detail)
	}
	if !strings.Contains(r.Detail, "SOURCES.yaml may be wrong") {
		t.Errorf("detail should point at the manifest, got %q", r.Detail)
	}
}

func TestCheck_ContentDiffersButNoCommitsIsNotInSync(t *testing.T) {
	src := ghSource("blob-recorded")
	c := &fakeClient{
		branch:  "main",
		blobs:   map[string]string{key(src.Repo, src.Path, "main"): "blob-different"},
		commits: map[string][2]int{key(src.Repo, src.Path): {0, 0}},
	}

	r := one(t, Check(c, manifestOf(BundleFile{Path: "skills/the-desk/SKILL.md", Source: src}), 5))
	if r.Status == StatusInSync {
		t.Fatalf("a differing blob was reported in-sync: %+v", r)
	}
	if r.Status != StatusBehind {
		t.Fatalf("status = %q, want %q", r.Status, StatusBehind)
	}
}

// --- moved ------------------------------------------------------------------

func TestCheck_PathMoved(t *testing.T) {
	src := ghSource("blob-recorded")
	c := &fakeClient{
		branch:  "main",
		blobErr: map[string]error{key(src.Repo, src.Path, "main"): fmt.Errorf("%w: gone", ErrNotFound)},
	}

	r := one(t, Check(c, manifestOf(BundleFile{Path: "skills/the-desk/SKILL.md", Source: src}), 5))
	if r.Status != StatusMoved {
		t.Fatalf("status = %q, want %q (%s)", r.Status, StatusMoved, r.Detail)
	}
	if !Summarize([]Result{r}).Drift {
		t.Error("a moved source must count as drift")
	}
}

// --- unreachable ------------------------------------------------------------

func TestCheck_UnreachableNetwork(t *testing.T) {
	src := ghSource("blob-recorded")
	c := &fakeClient{branchErr: errors.New("dial tcp: no route to host")}

	r := one(t, Check(c, manifestOf(BundleFile{Path: "skills/the-desk/SKILL.md", Source: src}), 5))
	if r.Status != StatusUnreachable {
		t.Fatalf("status = %q, want %q (%s)", r.Status, StatusUnreachable, r.Detail)
	}
	if r.Status == StatusInSync {
		t.Fatal("an unreachable source must never be reported in-sync")
	}
	if !strings.Contains(r.Detail, "no route to host") {
		t.Errorf("detail should carry the underlying cause, got %q", r.Detail)
	}
	if !Summarize([]Result{r}).Drift {
		t.Error("an unreachable source must count as drift so --fail-on-drift fails closed")
	}
}

func TestCheck_UnreachableMidway(t *testing.T) {
	// Branch and blob resolve; the history walk fails. Still unreachable.
	src := ghSource("blob-recorded")
	c := &fakeClient{
		branch:     "main",
		blobs:      map[string]string{key(src.Repo, src.Path, "main"): "blob-different"},
		commitsErr: errors.New("gh api: HTTP 502"),
	}

	r := one(t, Check(c, manifestOf(BundleFile{Path: "skills/the-desk/SKILL.md", Source: src}), 5))
	if r.Status != StatusUnreachable {
		t.Fatalf("status = %q, want %q (%s)", r.Status, StatusUnreachable, r.Detail)
	}
}

func TestCheck_LocalGitSourceIsUnreachableByConstruction(t *testing.T) {
	src := Source{Kind: "local-git", Repo: "~/.claude", Path: "skills/author-brief/SKILL.md", Commit: "9287ee7"}
	c := &fakeClient{} // must never be consulted

	r := one(t, Check(c, manifestOf(BundleFile{Path: "skills/author-brief/SKILL.md", Source: src}), 5))
	if r.Status != StatusUnreachable {
		t.Fatalf("status = %q, want %q", r.Status, StatusUnreachable)
	}
	if len(c.calls) != 0 {
		t.Errorf("a local-git source must not hit the network, got calls %v", c.calls)
	}
}

// --- cross-references are advisory -----------------------------------------

func TestCheck_CrossReferenceIsAdvisoryAndDoesNotDecideVerdict(t *testing.T) {
	primary := ghSource("blob-recorded")
	xref := ghSource("xref-blob")
	xref.Path = ".claude/skills/author-brief/SKILL.md"

	c := &fakeClient{
		branch: "main",
		blobs: map[string]string{
			key(primary.Repo, primary.Path, "main"): "blob-recorded", // in-sync
			key(xref.Repo, xref.Path, "main"):       "xref-moved-on", // drifted
		},
		commits: map[string][2]int{key(xref.Repo, xref.Path): {11, 0}},
	}

	results := Check(c, manifestOf(BundleFile{
		Path:           "skills/author-brief/SKILL.md",
		Source:         primary,
		CrossReference: &xref,
	}), 5)
	if len(results) != 2 {
		t.Fatalf("want 2 results (source + cross-reference), got %d", len(results))
	}
	if results[0].Origin != "source" || results[0].Advisory {
		t.Errorf("first result should be the non-advisory source, got %+v", results[0])
	}
	if results[1].Origin != "cross-reference" || !results[1].Advisory {
		t.Errorf("second result should be the advisory cross-reference, got %+v", results[1])
	}
	if results[1].Status != StatusBehind || results[1].Commits != 11 {
		t.Errorf("cross-reference drift not reported: %+v", results[1])
	}
	if s := Summarize(results); s.Drift {
		t.Error("an advisory cross-reference must not flip the verdict to DRIFT")
	}
}

// --- summary / verdict ------------------------------------------------------

func TestSummarize_CleanVerdict(t *testing.T) {
	s := Summarize([]Result{{Status: StatusInSync}, {Status: StatusInSync}})
	if s.Drift {
		t.Fatal("drift reported on all-in-sync results")
	}
	v := s.Verdict()
	if !strings.Contains(v, "CLEAN") || !strings.Contains(v, "in-sync 2") || !strings.Contains(v, "2 origin(s)") {
		t.Errorf("verdict = %q", v)
	}
}

func TestSummarize_CountsEveryStatus(t *testing.T) {
	s := Summarize([]Result{
		{Status: StatusInSync},
		{Status: StatusBehind},
		{Status: StatusMoved},
		{Status: StatusUnreachable},
	})
	for status, want := range map[string]int{StatusInSync: 1, StatusBehind: 1, StatusMoved: 1, StatusUnreachable: 1} {
		if got := s.Counts[status]; got != want {
			t.Errorf("count[%s] = %d, want %d", status, got, want)
		}
	}
	if !s.Drift {
		t.Error("want drift")
	}
}

// --- manifest validation ----------------------------------------------------

func TestManifestValidate(t *testing.T) {
	good := manifestOf(BundleFile{Path: "skills/the-desk/SKILL.md", Source: ghSource("b")})
	if err := good.Validate(); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}

	cases := map[string]*Manifest{
		"unsupported version": {Version: 2, Files: good.Files},
		// Nothing declared at all: a manifest with no files, no canonical and no
		// unported entries has nothing for coverage to check against.
		"nothing declared":    {Version: 1},
		"missing path":        manifestOf(BundleFile{Source: ghSource("b")}),
		"missing kind":        manifestOf(BundleFile{Path: "p", Source: Source{Repo: "a/b", Path: "x", Commit: "c"}}),
		"unknown kind":        manifestOf(BundleFile{Path: "p", Source: Source{Kind: "svn", Repo: "a/b", Path: "x", Commit: "c"}}),
		"bad github repo":     manifestOf(BundleFile{Path: "p", Source: Source{Kind: "github", Repo: "noslash", Path: "x", Commit: "c"}}),
		"missing commit":      manifestOf(BundleFile{Path: "p", Source: Source{Kind: "github", Repo: "a/b", Path: "x"}}),
		"bad cross-reference": manifestOf(BundleFile{Path: "p", Source: ghSource("b"), CrossReference: &Source{Kind: "github", Repo: "a/b", Path: "x"}}),
		// A github source with no blob has no drift baseline: reject it rather
		// than let it degrade to a content-free "nothing touched the path" pass.
		"github source missing blob": manifestOf(BundleFile{Path: "p", Source: Source{Kind: "github", Repo: "a/b", Path: "x", Commit: "c"}}),
		"github cross-reference missing blob": manifestOf(BundleFile{
			Path: "p", Source: ghSource("b"),
			CrossReference: &Source{Kind: "github", Repo: "a/b", Path: "x", Commit: "c"},
		}),
		"duplicate file entry": manifestOf(
			BundleFile{Path: "dup", Source: ghSource("b")},
			BundleFile{Path: "dup", Source: ghSource("b")},
		),
		"canonical without a path": {
			Version: 1, Files: good.Files,
			Canonical: []CanonicalFile{{Reason: "canonical here"}},
		},
		"canonical without a reason": {
			Version: 1, Files: good.Files,
			Canonical: []CanonicalFile{{Path: "skills/the-desk/SKILL.md"}},
		},
		"canonical also pinned": {
			Version: 1, Files: good.Files,
			Canonical: []CanonicalFile{{Path: "skills/the-desk/SKILL.md", Reason: "canonical here"}},
		},
		"canonical also unported": {
			Version:   1,
			Canonical: []CanonicalFile{{Path: "skills/adopt/SKILL.md", Reason: "canonical here"}},
			Unported:  []UnportedFile{{Path: "skills/adopt/SKILL.md", Reason: "authored here"}},
		},
		"unported without a path": {
			Version: 1, Files: good.Files,
			Unported: []UnportedFile{{Reason: "authored here"}},
		},
		"unported without a reason": {
			Version: 1, Files: good.Files,
			Unported: []UnportedFile{{Path: "skills/adopt/SKILL.md"}},
		},
		"unported also pinned": {
			Version: 1, Files: good.Files,
			Unported: []UnportedFile{{Path: "skills/the-desk/SKILL.md", Reason: "authored here"}},
		},
	}
	for name, m := range cases {
		if err := m.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want error", name)
		}
	}
}

func TestAfterInstant(t *testing.T) {
	got, err := afterInstant("2026-07-17T03:55:06Z")
	if err != nil {
		t.Fatalf("afterInstant: %v", err)
	}
	if got != "2026-07-17T03:55:07Z" {
		t.Errorf("afterInstant = %q, want 2026-07-17T03:55:07Z", got)
	}
	if _, err := afterInstant("not a date"); err == nil {
		t.Error("afterInstant accepted a non-date")
	}
}

func TestEscapePath(t *testing.T) {
	if got := escapePath(".claude/skills/the-desk/SKILL.md"); got != ".claude/skills/the-desk/SKILL.md" {
		t.Errorf("escapePath mangled a plain path: %q", got)
	}
	if got := escapePath("a b/c.md"); got != "a%20b/c.md" {
		t.Errorf("escapePath = %q, want a%%20b/c.md", got)
	}
}

// --- coverage ---------------------------------------------------------------

// fakeBundle writes a bundle tree with one skills/<name>/SKILL.md per name.
func fakeBundle(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		sub := filepath.Join(dir, "skills", n)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(sub, "SKILL.md"), []byte("# "+n+"\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return dir
}

func TestCheckCoverage_AllAccounted(t *testing.T) {
	dir := fakeBundle(t, "the-desk", "adopt")
	m := manifestOf(BundleFile{Path: "skills/the-desk/SKILL.md", Source: ghSource("b")})
	m.Unported = []UnportedFile{{Path: "skills/adopt/SKILL.md", Reason: "authored in this repo"}}

	cov, err := CheckCoverage(dir, m)
	if err != nil {
		t.Fatalf("CheckCoverage: %v", err)
	}
	if cov.OnDisk != 2 || cov.Pinned != 1 || cov.Canonical != 0 || cov.Unported != 1 {
		t.Fatalf("coverage = %+v, want 2 on disk / 1 pinned / 0 canonical / 1 unported", cov)
	}
}

// Post-flip shape: a manifest with NO files: rows but canonical: declarations
// still covers the bundle — canonical entries are accounted-for, not drift
// baselines.
func TestCheckCoverage_CanonicalOnly(t *testing.T) {
	dir := fakeBundle(t, "the-desk", "adopt")
	m := &Manifest{
		Version: 1, Bundle: "plugins/assay", BundleVersion: "0.2.0",
		Canonical: []CanonicalFile{
			{Path: "skills/the-desk/SKILL.md", Reason: "canonical home", Notes: "ported; now the home"},
		},
		Unported: []UnportedFile{{Path: "skills/adopt/SKILL.md", Reason: "authored here"}},
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	cov, err := CheckCoverage(dir, m)
	if err != nil {
		t.Fatalf("CheckCoverage: %v", err)
	}
	if cov.OnDisk != 2 || cov.Pinned != 0 || cov.Canonical != 1 || cov.Unported != 1 {
		t.Fatalf("coverage = %+v, want 2 on disk / 0 pinned / 1 canonical / 1 unported", cov)
	}
}

// The finding this exists for: a skill ported from upstream, added with no
// manifest entry, must not pass silently.
func TestCheckCoverage_UnpinnedSkillIsAnError(t *testing.T) {
	dir := fakeBundle(t, "the-desk", "newly-ported")
	m := manifestOf(BundleFile{Path: "skills/the-desk/SKILL.md", Source: ghSource("b")})

	cov, err := CheckCoverage(dir, m)
	if err == nil {
		t.Fatal("CheckCoverage accepted a bundled skill that is in neither list")
	}
	if !strings.Contains(err.Error(), "skills/newly-ported/SKILL.md") {
		t.Errorf("error does not name the unaccounted file: %v", err)
	}
	if len(cov.Unaccounted) != 1 || cov.Unaccounted[0] != "skills/newly-ported/SKILL.md" {
		t.Errorf("Unaccounted = %v, want [skills/newly-ported/SKILL.md]", cov.Unaccounted)
	}
}

func TestCheckCoverage_ManifestEntryWithNoFileIsAnError(t *testing.T) {
	dir := fakeBundle(t, "the-desk")
	m := manifestOf(
		BundleFile{Path: "skills/the-desk/SKILL.md", Source: ghSource("b")},
		BundleFile{Path: "skills/renamed-away/SKILL.md", Source: ghSource("b")},
	)

	cov, err := CheckCoverage(dir, m)
	if err == nil {
		t.Fatal("CheckCoverage accepted a manifest entry with no file on disk")
	}
	if len(cov.Missing) != 1 || cov.Missing[0] != "skills/renamed-away/SKILL.md" {
		t.Errorf("Missing = %v, want [skills/renamed-away/SKILL.md]", cov.Missing)
	}
}

// Fail closed: an empty or wrong bundle directory is not "everything accounted
// for". Without this the check would pass vacuously on a bad --manifest path.
func TestCheckCoverage_EmptyBundleIsNotAPass(t *testing.T) {
	m := manifestOf(BundleFile{Path: "skills/the-desk/SKILL.md", Source: ghSource("b")})
	_, err := CheckCoverage(t.TempDir(), m)
	if err == nil {
		t.Fatal("CheckCoverage passed on a bundle directory with no skills in it")
	}
	// Pin the REASON, not just that some error arrived. This manifest also pins a
	// path that is missing from the empty dir, so the `Missing` branch returns an
	// error too — delete the empty-glob guard entirely and this test still sees
	// err != nil. Asserting the empty-glob message is what makes the mutation red.
	if !strings.Contains(err.Error(), "nothing to check, which is never a pass") {
		t.Fatalf("wrong failure reason — the empty-glob guard did not fire.\n got: %v\nwant: an error naming that no files matched the coverage glob", err)
	}
}

// The real manifest must cover the real bundle — this is the assertion the
// review asked for, run against the shipped tree rather than a fixture.
//
// It reads `../../plugins/assay/...`, which makes this file a CROSS-MODULE
// READER under the cross-module-reader guard: it is registered in
// tools/desk/internal/deskkit/citrigger_test.go's ciCrossModuleRegistry, and
// that row is what keeps `test (tools/plugindrift)` triggered by the very files
// this test reads. Do not delete the read without deleting the row.
func TestCheckCoverage_ShippedManifestCoversShippedBundle(t *testing.T) {
	path := filepath.Join("..", "..", defaultManifest)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	cov, err := CheckCoverage(filepath.Dir(path), &m)
	if err != nil {
		t.Fatalf("%s: coverage: %v", path, err)
	}
	if cov.OnDisk == 0 {
		t.Fatal("no bundled skills found — the test is looking in the wrong place")
	}
	if cov.Pinned+cov.Canonical+cov.Unported != cov.OnDisk {
		t.Fatalf("coverage = %+v: pinned+canonical+unported must equal the files on disk", cov)
	}
}
