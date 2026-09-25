package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// authoringexempt_test.go — #1339: a MERGED briefs-AUTHORING PR must not make the brief it wrote read
// as landed-unreconciled in `plan`, while a real delivery still does. Stream slugs are synthetic.

// withPlanTransports wires recorded PR-list and file transports for one test and restores the nil
// defaults. files may be nil (no file transport — the pre-exemption behaviour).
func withPlanTransports(t *testing.T, prs []deskkit.PRRef, files func(repo string, n int) ([]deskkit.ChangedFile, error)) {
	t.Helper()
	oldPRs, oldFiles := representedPRs, representedPRFiles
	representedPRs = func(string) ([]deskkit.PRRef, error) { return prs, nil }
	representedPRFiles = files
	t.Cleanup(func() { representedPRs, representedPRFiles = oldPRs, oldFiles })
}

// planFiles serves a fixed file list per PR number and fails the test on any other number, pinning
// that only PRs naming a queued brief are read.
func planFiles(t *testing.T, m map[int][]deskkit.ChangedFile) func(string, int) ([]deskkit.ChangedFile, error) {
	return func(_ string, n int) ([]deskkit.ChangedFile, error) {
		files, ok := m[n]
		if !ok {
			t.Fatalf("the file transport was read for #%d, which names no queued brief", n)
		}
		return files, nil
	}
}

// planAuthoringFiles is the changed-file shape of a docs-only PR that WROTE briefs 11 and 12.
func planAuthoringFiles() []deskkit.ChangedFile {
	return []deskkit.ChangedFile{
		{Filename: "changelog/example-port-11-12-briefs.md", Status: "added"},
		{Filename: "docs/streams/example-port/README.md", Status: "modified"},
		{Filename: "docs/streams/example-port/brief-11-portable-pollers.md", Status: "added"},
		{Filename: "docs/streams/example-port/brief-12-deposix-prose.md", Status: "added"},
	}
}

func planWithRows(rows []BoardRow) *FanoutLoop {
	f := &FanoutLoop{
		Board:  func() ([]BoardRow, error) { return rows, nil },
		Rework: noRework,
		Emit:   io.Discard,
	}
	f.Represented = representedSourceFor("example-org/example-repo", f.candidateBriefIDs)
	return f
}

// TestPlan_AuthoringPRDoesNotLandBrief is the fail-first proof of the planner half of #1339: the only
// PR naming the queued `todo` brief is the MERGED docs-only PR that authored it (it carried
// `Brief: example-port/11`). Before the fix, plan listed the brief as LANDED-UNRECONCILED and never
// offered it; now it is a fresh DISPATCH.
func TestPlan_AuthoringPRDoesNotLandBrief(t *testing.T) {
	setupDeskHome(t)
	withPlanTransports(t,
		[]deskkit.PRRef{{Number: 1439, State: "MERGED", Body: "Authors briefs 11-12.\n\nBrief: example-port/11"}},
		planFiles(t, map[int][]deskkit.ChangedFile{1439: planAuthoringFiles()}))

	f := planWithRows([]BoardRow{briefRow("example-port", "11", "M", "", "model", false)})
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	s := out.String()
	if strings.Contains(s, "merged PR #1439") {
		t.Errorf("the authoring PR made example-port/11 read as landed-unreconciled (#1339):\n%s", s)
	}
	if !strings.Contains(s, "=== DISPATCH example-port/11") {
		t.Errorf("a brief whose only PR AUTHORED it must be offered for dispatch (#1339):\n%s", s)
	}
}

// TestPlan_DeliveryStillLands: the real phantom stays caught — a MERGED PR that touched code is a
// delivery, and the row stays landed-unreconciled.
func TestPlan_DeliveryStillLands(t *testing.T) {
	setupDeskHome(t)
	files := append(planAuthoringFiles(), deskkit.ChangedFile{Filename: "tools/x/main.go", Status: "added"})
	withPlanTransports(t,
		[]deskkit.PRRef{{Number: 1600, State: "MERGED", Body: "Delivers it.\n\nBrief: example-port/11"}},
		planFiles(t, map[int][]deskkit.ChangedFile{1600: files}))

	f := planWithRows([]BoardRow{briefRow("example-port", "11", "M", "", "model", false)})
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	s := out.String()
	if strings.Contains(s, "=== DISPATCH example-port/11") || !strings.Contains(s, "example-port/11 — merged PR #1600") {
		t.Errorf("a merged DELIVERY must keep the row landed-unreconciled:\n%s", s)
	}
}

// TestRepresentedSource_AuthoringDoesNotMaskDelivery: when the authoring PR is FIRST in the list and a
// real delivery follows, the delivery represents the brief — setting the authoring PR aside never
// hides the PR behind it (a first-wins reduction would have).
func TestRepresentedSource_AuthoringDoesNotMaskDelivery(t *testing.T) {
	code := append(planAuthoringFiles()[:0:0], deskkit.ChangedFile{Filename: "tools/x/main.go", Status: "modified"})
	withPlanTransports(t,
		[]deskkit.PRRef{
			{Number: 1439, State: "MERGED", Body: "Brief: example-port/11"},
			{Number: 1700, State: "OPEN", Body: "Brief: example-port/11"},
		},
		planFiles(t, map[int][]deskkit.ChangedFile{1439: planAuthoringFiles(), 1700: code}))

	f := planWithRows([]BoardRow{briefRow("example-port", "11", "M", "", "model", false)})
	m, err := f.Represented()
	if err != nil {
		t.Fatalf("Represented: %v", err)
	}
	if rp, ok := m["example-port/11"]; !ok || rp.Number != 1700 || rp.Merged {
		t.Fatalf("the OPEN delivery #1700 must represent example-port/11 once the authoring PR is set aside; got %+v (present=%v)", rp, ok)
	}
}

// TestRepresentedSource_UnreadableFilesKeepPR: a file list that cannot be read is could-not-check —
// the PR keeps representing the brief (the pre-exemption answer, row not dispatched), never rounded
// to "authoring".
func TestRepresentedSource_UnreadableFilesKeepPR(t *testing.T) {
	withPlanTransports(t,
		[]deskkit.PRRef{{Number: 1439, State: "MERGED", Body: "Brief: example-port/11"}},
		func(string, int) ([]deskkit.ChangedFile, error) { return nil, errors.New("HTTP 502") })

	f := planWithRows([]BoardRow{briefRow("example-port", "11", "M", "", "model", false)})
	m, err := f.Represented()
	if err != nil {
		t.Fatalf("Represented: %v", err)
	}
	if rp, ok := m["example-port/11"]; !ok || rp.Number != 1439 {
		t.Fatalf("an unreadable file list must keep #1439 representing the brief; got %+v (present=%v)", rp, ok)
	}
}

// TestRepresentedSource_OnlyQueuedBriefsRead: a PR naming a brief that is not queued this run is not
// file-read (planFiles fails the test on any read of #900) and keeps its first-wins entry.
func TestRepresentedSource_OnlyQueuedBriefsRead(t *testing.T) {
	withPlanTransports(t,
		[]deskkit.PRRef{
			{Number: 900, State: "MERGED", Body: "Brief: other/02"},
			{Number: 1439, State: "MERGED", Body: "Brief: example-port/11"},
		},
		planFiles(t, map[int][]deskkit.ChangedFile{1439: planAuthoringFiles()}))

	f := planWithRows([]BoardRow{briefRow("example-port", "11", "M", "", "model", false)})
	m, err := f.Represented()
	if err != nil {
		t.Fatalf("Represented: %v", err)
	}
	if rp, ok := m["other/02"]; !ok || rp.Number != 900 {
		t.Fatalf("a non-queued brief keeps its first-wins entry unread; got %+v (present=%v)", rp, ok)
	}
	if _, ok := m["example-port/11"]; ok {
		t.Fatalf("the queued brief's only PR is authoring and must be set aside: %+v", m)
	}
}

// TestRepresentedSource_AuthorsTrailerNeverRepresents: a PR carrying the `Authors:` trailer names the
// briefs it wrote without asserting delivery, so it represents nothing even with no file transport.
func TestRepresentedSource_AuthorsTrailerNeverRepresents(t *testing.T) {
	withPlanTransports(t,
		[]deskkit.PRRef{{Number: 1439, State: "MERGED", Body: "Authors: example-port/11, example-port/12"}},
		nil)
	m, err := representedSourceFor("example-org/example-repo", nil)()
	if err != nil {
		t.Fatalf("Represented: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("an `Authors:` PR must represent no brief; got %+v", m)
	}
}
