package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// authoring_test.go — a briefs-AUTHORING PR's `Brief:` trailer must not make the brief it authored
// undispatchable, while every real delivery stays refused. Stream slugs are synthetic (example-*).

// withPRFiles wires a recorded per-PR file-list transport for one test and restores the nil default.
func withPRFiles(t *testing.T, fn func(repo string, number int) ([]deskkit.ChangedFile, error)) {
	t.Helper()
	old := listPRFiles
	listPRFiles = fn
	t.Cleanup(func() { listPRFiles = old })
}

// filesByPR serves a fixed file list per PR number and fails the test on any other number, which
// pins that only PRs naming the dispatched brief are ever read.
func filesByPR(t *testing.T, m map[int][]deskkit.ChangedFile) func(string, int) ([]deskkit.ChangedFile, error) {
	return func(_ string, n int) ([]deskkit.ChangedFile, error) {
		files, ok := m[n]
		if !ok {
			t.Fatalf("the file transport was read for #%d, which does not name the dispatched brief", n)
		}
		return files, nil
	}
}

// authoringFiles is the shape of a docs-only PR that WROTE briefs 11 and 12 of a stream.
func authoringFiles() []deskkit.ChangedFile {
	return []deskkit.ChangedFile{
		{Filename: "changelog/example-port-11-12-briefs.md", Status: "added"},
		{Filename: "docs/streams/example-port/README.md", Status: "modified"},
		{Filename: "docs/streams/example-port/brief-11-portable-pollers.md", Status: "added"},
		{Filename: "docs/streams/example-port/brief-12-deposix-prose.md", Status: "added"},
	}
}

// TestPhantomAdmitsAuthoredBrief is the fail-first proof of the issue: the only
// PR naming the brief is the MERGED docs-only PR that authored it. Before the fix the phantom check
// refused the dispatch as "already represented" by that PR, so the brief could never be dispatched.
func TestPhantomAdmitsAuthoredBrief(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 1439, State: "MERGED", Body: "Authors briefs 11-12.\n\nBrief: example-port/11"}}, nil
	})
	withPRFiles(t, filesByPR(t, map[int][]deskkit.ChangedFile{1439: authoringFiles()}))

	if _, err := phantomCheck(dispatchOpts{item: "assay--example-port--11", kit: "worker"}, allowedRepo); err != nil {
		t.Fatalf("a brief whose only PR AUTHORED it must be dispatchable, got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}

// A PR that touches docs/streams AND code is a delivery, whatever else it touches, so it still refuses.
func TestPhantomRefusesDocsPlusCode(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 1600, State: "MERGED", Body: "Delivers it.\n\nBrief: example-port/11"}}, nil
	})
	files := append(authoringFiles(), deskkit.ChangedFile{Filename: "tools/desk/cmd/example/main.go", Status: "added"})
	withPRFiles(t, filesByPR(t, map[int][]deskkit.ChangedFile{1600: files}))

	_, err := phantomCheck(dispatchOpts{item: "assay--example-port--11", kit: "worker"}, allowedRepo)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("a docs+code PR is a delivery and must REFUSE (exit 5), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "#1600") {
		t.Errorf("the refusal must name the delivering PR #1600: %v", err)
	}
}

// A brief whose deliverable is itself a doc under docs/streams is delivered by a PR that touches only
// docs/streams. It does not add the brief's own file, so it is still a delivery and still refuses.
func TestPhantomRefusesDocsOnlyDelivery(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 323, State: "MERGED", Body: "The audit.\n\nBrief: example-port/02"}}, nil
	})
	withPRFiles(t, filesByPR(t, map[int][]deskkit.ChangedFile{323: {
		{Filename: "docs/streams/example-port/README.md", Status: "modified"},
		{Filename: "docs/streams/example-port/brief-02-portability-audit.md", Status: "modified"},
		{Filename: "docs/streams/example-port/portability-audit.md", Status: "added"},
	}}))

	_, err := phantomCheck(dispatchOpts{item: "assay--example-port--02", kit: "worker"}, allowedRepo)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("a docs-only DELIVERY must still REFUSE (exit 5), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}

// A PR that authored the brief AND delivered its document under docs/streams in one change adds the
// brief's own file, but the delivered document is neither a board README nor a brief, so it refuses.
func TestPhantomRefusesAuthorAndDeliver(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 1489, State: "MERGED", Body: "Authors and delivers.\n\nBrief: example-port/02"}}, nil
	})
	withPRFiles(t, filesByPR(t, map[int][]deskkit.ChangedFile{1489: {
		{Filename: "changelog/example-port-02.md", Status: "added"},
		{Filename: "docs/streams/example-port/README.md", Status: "modified"},
		{Filename: "docs/streams/example-port/brief-02-portability-audit.md", Status: "added"},
		{Filename: "docs/streams/example-port/portability-audit.md", Status: "added"},
	}}))

	_, err := phantomCheck(dispatchOpts{item: "assay--example-port--02", kit: "worker"}, allowedRepo)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("an author-and-deliver PR must still REFUSE (exit 5), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}

// The authoring PR is set aside, but a real delivery PR for the same brief still refuses. The
// exemption removes one PR from the match; it never clears the brief.
func TestPhantomAuthoringKeepsDelivery(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{
			{Number: 1439, State: "MERGED", Body: "Brief: example-port/11"},
			{Number: 1650, State: "OPEN", Body: "Implements it.\n\nBrief: example-port/11"},
			{Number: 1700, State: "OPEN", Body: "Brief: example-other/03"}, // names another brief: never read
		}, nil
	})
	withPRFiles(t, filesByPR(t, map[int][]deskkit.ChangedFile{
		1439: authoringFiles(),
		1650: {{Filename: "tools/desk/cmd/example/main.go", Status: "added"}},
	}))

	_, err := phantomCheck(dispatchOpts{item: "assay--example-port--11", kit: "worker"}, allowedRepo)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("the OPEN delivery PR must still refuse the dispatch, got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "#1650") {
		t.Errorf("the refusal must name the DELIVERY PR #1650, not the authoring PR: %v", err)
	}
}

// A file list that cannot be read is could-not-check (exit 6), never "authoring" and never a guess.
func TestPhantomUnreadableFilesHold(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 1439, State: "MERGED", Body: "Brief: example-port/11"}}, nil
	})
	withPRFiles(t, func(string, int) ([]deskkit.ChangedFile, error) { return nil, os.ErrDeadlineExceeded })

	_, err := phantomCheck(dispatchOpts{item: "assay--example-port--11", kit: "worker"}, allowedRepo)
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("an unreadable file list is UNVERIFIABLE (exit 6), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "#1439") {
		t.Errorf("the hold must name the PR whose files could not be read: %v", err)
	}
}

// With no file transport wired (the offline reference build), every representing PR counts as a
// delivery, exactly as before the exemption existed.
func TestPhantomNoFileTransportRefuses(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 1439, State: "MERGED", Body: "Brief: example-port/11"}}, nil
	})
	withPRFiles(t, nil)

	_, err := phantomCheck(dispatchOpts{item: "assay--example-port--11", kit: "worker"}, allowedRepo)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("with no file transport the representing PR must still refuse, got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}

// fileForge answers GetPullRequest and ListChangedFiles only; any other Forge method panics on the
// embedded nil interface, pinning that the file read touches nothing else.
type fileForge struct {
	deskkit.Forge
	count int
	files []deskkit.ChangedFile
}

func (f fileForge) GetPullRequest(_ deskkit.ForgeRepo, n int) (*deskkit.PullRequest, error) {
	return &deskkit.PullRequest{Number: n, ChangedFiles: f.count}, nil
}

func (f fileForge) ListChangedFiles(deskkit.ForgeRepo, int) ([]deskkit.ChangedFile, error) {
	return f.files, nil
}

// A file list shorter than the forge's own count is not provably complete. The files it did not list
// could be the code that makes the PR a delivery, so the answer is could-not-check, not "authoring".
func TestPhantomTruncatedFilesHold(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 1439, State: "MERGED", Body: "Brief: example-port/11"}}, nil
	})
	truncated := fileForge{count: 3001, files: authoringFiles()}
	withPRFiles(t, func(repo string, n int) ([]deskkit.ChangedFile, error) {
		fr, err := forgeRepoOf(repo)
		if err != nil {
			return nil, err
		}
		return completePRFiles(truncated, fr, n)
	})

	_, err := phantomCheck(dispatchOpts{item: "assay--example-port--11", kit: "worker"}, allowedRepo)
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("a truncated file list must be UNVERIFIABLE (exit 6), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}

// completePRFiles returns the list when it matches the forge's count, and passes a read error through.
func TestCompletePRFiles(t *testing.T) {
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "example-repo"}
	files, err := completePRFiles(fileForge{count: 4, files: authoringFiles()}, fr, 1439)
	if err != nil || len(files) != 4 {
		t.Fatalf("a complete list must come back whole: %d files, %v", len(files), err)
	}
	if _, err := completePRFiles(fileForge{count: 5, files: authoringFiles()}, fr, 1439); err == nil {
		t.Fatal("a list shorter than the forge's count must be an error")
	}
	boom := errors.New("boom")
	if _, err := completePRFiles(errForge{err: boom}, fr, 1439); !errors.Is(err, boom) {
		t.Fatalf("a read error must pass through: %v", err)
	}
}

type errForge struct {
	deskkit.Forge
	err error
}

func (f errForge) GetPullRequest(deskkit.ForgeRepo, int) (*deskkit.PullRequest, error) {
	return nil, f.err
}

// TestAuthoredBriefDispatchesEndToEnd drives a full worker dispatch of a brief whose only PR authored
// it: the phantom check lets it through, and the durable claim and the worktree follow.
func TestAuthoredBriefDispatchesEndToEnd(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 1439, State: "MERGED", Body: "Brief: example-port/11"}}, nil
	})
	withPRFiles(t, filesByPR(t, map[int][]deskkit.ChangedFile{1439: authoringFiles()}))

	promptFile := filepath.Join(t.TempDir(), "prompt.md")
	rc := run([]string{"assay--example-port--11", "--root", root, "--repo", allowedRepo, "--kit", "worker",
		"--tier", "strong", "--prompt-file", promptFile})
	if rc != deskkit.ExitOK {
		t.Fatalf("a brief whose only PR authored it must dispatch (exit 0), got %d", rc)
	}
	if !s.ran("dispatch-claim.sh acquire") {
		t.Error("the durable claim was not acquired")
	}
	if !s.ran("deskwt add") {
		t.Error("no worktree was cut")
	}
}

// TestPhantomAdmitsAuthorsTrailerPR (#1339): an authoring PR that carries the `Authors:` trailer
// names the briefs it wrote without asserting delivery, so it never represents them — even with NO
// file transport wired, where a `Brief:` authoring PR would still refuse
// (TestPhantomNoFileTransportRefuses). The file transport is wired to fail the test if read: an
// `Authors:` PR is not a candidate for the exemption at all.
func TestPhantomAdmitsAuthorsTrailerPR(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 1439, State: "MERGED", Body: "Authors briefs 11-12.\n\nAuthors: example-port/11, example-port/12"}}, nil
	})
	withPRFiles(t, filesByPR(t, map[int][]deskkit.ChangedFile{}))

	if _, err := phantomCheck(dispatchOpts{item: "assay--example-port--11", kit: "worker"}, allowedRepo); err != nil {
		t.Fatalf("a brief named only by an `Authors:` PR must be dispatchable, got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
	withPRFiles(t, nil)
	if _, err := phantomCheck(dispatchOpts{item: "assay--example-port--12", kit: "worker"}, allowedRepo); err != nil {
		t.Fatalf("with no file transport an `Authors:` PR must still not represent the brief, got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}

// TestPhantomAuthoringShapeAdmitted pins the exact changed-file shape of the field instance
// (#1339): a merged PR that added one changelog fragment, edited the stream board README and added
// FOUR brief files (11–14) carried `Brief: <stream>/11`. Every brief it wrote is dispatchable.
func TestPhantomAuthoringShapeAdmitted(t *testing.T) {
	files := []deskkit.ChangedFile{
		{Filename: "changelog/example-port-11-14-briefs.md", Status: "added"},
		{Filename: "docs/streams/example-port/README.md", Status: "modified"},
		{Filename: "docs/streams/example-port/brief-11-portable-desk-pollers.md", Status: "added"},
		{Filename: "docs/streams/example-port/brief-12-deposix-skill-prose.md", Status: "added"},
		{Filename: "docs/streams/example-port/brief-13-inbox-verb-port.md", Status: "added"},
		{Filename: "docs/streams/example-port/brief-14-windows-leg.md", Status: "added"},
	}
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 1439, State: "MERGED", Body: "Four briefs.\n\nBrief: example-port/11"}}, nil
	})
	withPRFiles(t, filesByPR(t, map[int][]deskkit.ChangedFile{1439: files}))
	if _, err := phantomCheck(dispatchOpts{item: "assay--example-port--11", kit: "worker"}, allowedRepo); err != nil {
		t.Fatalf("the field authoring shape must not block example-port/11, got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}
