package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type countedPendingForge struct {
	*pendingForge
	fileReads, issueReads, trustReads, labelReads int
}

func (f *countedPendingForge) ReadFile(repo deskkit.ForgeRepo, in deskkit.ReadFileInput) (*deskkit.FileContent, error) {
	f.fileReads++
	return f.pendingForge.ReadFile(repo, in)
}
func (f *countedPendingForge) GetIssueTyped(repo deskkit.ForgeRepo, n int, kind deskkit.TargetKind) (*deskkit.Issue, error) {
	f.issueReads++
	return f.pendingForge.GetIssueTyped(repo, n, kind)
}
func (f *countedPendingForge) IssueTrustEvents(repo deskkit.ForgeRepo, n int) (*deskkit.TrustPayload, error) {
	f.trustReads++
	return f.pendingForge.IssueTrustEvents(repo, n)
}
func (f *countedPendingForge) ListIssueLabelEvents(repo deskkit.ForgeRepo, n int) ([]deskkit.LabelEvent, error) {
	f.labelReads++
	return f.pendingForge.ListIssueLabelEvents(repo, n)
}

func TestPendingReuseRefreshesPassAndKeepsHeadReconciliation(t *testing.T) {
	v, base, _, brief := pendingSetup(t)
	f := &countedPendingForge{pendingForge: base}
	s := v.Issues.(*forgeIssueSource)
	s.forgeFor = func(deskkit.ForgeRepo) (deskkit.Forge, error) { return f, nil }
	items, err := v.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	item := items[0]
	if item.BriefPath != brief {
		t.Fatal("fixture did not select the attested brief first")
	}
	prReads := base.prReads[51]
	for range 3 {
		ref, err := s.PendingEvidence(v.Root, v.RootRepo, item, v.wakeNow(), nil)
		if err != nil || ref != "#51" {
			t.Fatalf("reused hold = %q, %v", ref, err)
		}
	}
	if f.fileReads != 1 || f.issueReads != 2 || f.trustReads != 1 || f.labelReads != 1 {
		t.Fatalf("file/issue/trust/labels = %d/%d/%d/%d; want 1/2/1/1", f.fileReads, f.issueReads, f.trustReads, f.labelReads)
	}
	if base.prReads[51] != prReads+3 {
		t.Fatal("a repeated hold skipped its current PR-head/state reconciliation")
	}
	base.move = true
	if ref, err := s.PendingEvidence(v.Root, v.RootRepo, item, v.wakeNow(), nil); err == nil || ref != "" {
		t.Fatal("moved head retained a cached hold")
	}
	base.move = false
	base.edited = true
	items, err = v.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Payload["could_not_check"] == "" || f.trustReads != 2 {
		t.Fatal("new selection reused an attestation after its body history changed")
	}
	base.edited, base.missing = false, true
	items, err = v.SelectQueue()
	if err != nil || items[0].Payload["could_not_check"] == "" {
		t.Fatalf("missing Evidence = %v, %v", items, err)
	}
	reads := f.fileReads
	base.missing = false
	if _, err := s.PendingEvidence(v.Root, v.RootRepo, item, v.wakeNow(), nil); err == nil || f.fileReads != reads {
		t.Fatal("failed file observation was not retained for just this pass")
	}
	items, err = v.SelectQueue()
	if err != nil || items[0].Payload["awaiting_merge"] != "#51" {
		t.Fatalf("a new pass did not recover from a file error: %v, %v", items, err)
	}
}

type pendingFileForge struct {
	deskkit.Forge
	reads int
	kind  string
	err   error
}

func (f *pendingFileForge) ReadFile(repo deskkit.ForgeRepo, in deskkit.ReadFileInput) (*deskkit.FileContent, error) {
	f.reads++
	return &deskkit.FileContent{Exists: true, Kind: f.kind, Content: []byte(repo.Slug() + ":" + in.Ref + ":" + in.File)}, f.err
}

func TestPendingFileReuseIsQualifiedImmutableAndDefensive(t *testing.T) {
	f := &pendingFileForge{kind: deskkit.FileKindRegular}
	c := &pendingChanges{}
	repo := deskkit.ForgeRepo{Owner: "example-org", Name: "one"}
	sha := strings.Repeat("a", 40)
	first, err := c.file(f, repo, "brief.md", sha)
	if err != nil {
		t.Fatal(err)
	}
	first.content[0] = '!'
	again, err := c.file(f, repo, "brief.md", sha)
	if err != nil || f.reads != 1 || again.content[0] == '!' {
		t.Fatal("file observation is not reused defensively")
	}
	for _, key := range []struct{ repo, file, sha string }{
		{"two", "brief.md", sha}, {"one", "another.md", sha}, {"one", "brief.md", strings.Repeat("b", 40)},
	} {
		got, err := c.file(f, deskkit.ForgeRepo{Owner: repo.Owner, Name: key.repo}, key.file, key.sha)
		if err != nil || string(got.content) != fmt.Sprintf("example-org/%s:%s:%s", key.repo, key.sha, key.file) {
			t.Fatalf("qualified observation collided: %q, %v", got.content, err)
		}
	}
	if f.reads != 4 {
		t.Fatal("distinct repo/commit/path observations were conflated")
	}
	if _, err := c.file(f, repo, "brief.md", "main"); err == nil || f.reads != 4 {
		t.Fatal("a mutable ref was cached or sent to the forge")
	}
	for _, failure := range []struct {
		kind string
		err  error
	}{
		{deskkit.FileKindSymlink, nil}, {deskkit.FileKindUnproven, nil}, {deskkit.FileKindRegular, errors.New("offline")},
	} {
		bad := &pendingFileForge{kind: failure.kind, err: failure.err}
		pass := &pendingChanges{}
		for range 2 {
			if _, err := pass.file(bad, repo, "brief.md", sha); err == nil {
				t.Fatal("failed or non-regular file was admitted")
			}
		}
		if bad.reads != 1 {
			t.Fatal("an unavailable observation was fetched repeatedly within the pass")
		}
	}
}

func TestPendingAttestationReuseCannotCrossRepositories(t *testing.T) {
	_, base, _, _ := pendingSetup(t)
	f := &countedPendingForge{pendingForge: base}
	c := &pendingChanges{}
	if _, err := c.attestation(f, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, 41, verifyPerformance{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.attestation(f, deskkit.ForgeRepo{Owner: "example-org", Name: "other"}, 41, verifyPerformance{}); err == nil {
		t.Fatal("same-number attestation from another repository was admitted")
	}
}

func TestPendingIndexPreservesUnknownsOrderAndLazyReads(t *testing.T) {
	verifier := "example-verifier[bot]"
	changes := []deskkit.ChangeRef{{Number: 1}, {Number: 2}, {Number: 3}, {Number: 4}}
	for i := range changes {
		changes[i].Author.Login = verifier
	}
	c := &pendingChanges{list: &deskkit.ChangeList{Changes: changes}, reads: map[int]*pendingRead{
		1: {files: []deskkit.ChangedFile{{Filename: "other.md"}}},
		2: {err: errors.New("files unknown")},
		3: {files: []deskkit.ChangedFile{{Filename: "brief.md"}, {Filename: "brief.md"}}},
		4: {files: []deskkit.ChangedFile{{Filename: "brief.md", Status: "removed"}}},
	}}
	c.indexChanges(verifier)
	got := c.changesFor("brief.md")
	if len(got) != 2 || got[0].Number != 2 || got[1].Number != 3 {
		t.Fatalf("index lost uncertainty, order or deduplication: %+v", got)
	}
	if got := c.changesFor("absent.md"); len(got) != 1 || got[0].Number != 2 {
		t.Fatal("unknown changed-file population disappeared from another brief")
	}
	delete(c.reads, 4)
	c.indexed = false
	c.indexChanges(verifier)
	if c.indexed || len(c.changesFor("brief.md")) != 4 {
		t.Fatal("a partial walk became an exhaustive index")
	}
}
