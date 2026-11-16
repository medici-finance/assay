package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// deliveredrows_test.go — #5853: `plan` must never offer IMPLEMENT for a row whose own README
// Status is already past `in-progress` (implemented, verified, done). Such a row is delivered work
// waiting on verification; it is ROUTED to the verify listing under its own heading and is absent
// from the DISPATCH list. The field symptom was the `### Awaiting implementer rework` lane: every row
// there reads `implemented`/`verified` by construction, and each was rendered as an IMPLEMENT prompt
// with a fresh target SHA, so a worker re-implemented a gate:human brief that had already shipped.

// deliveredFixture builds an offline git repo whose origin/main carries a STATUS.md with the given
// rework-section rows and a stream README whose `## Briefs` table gives each brief its live Status.
// briefs maps `<num>` → `<status>|<gate>`; every brief gets a frontmatter file so resolveBrief reads
// its gate. No network: the ref is published with update-ref, exactly what a fetch would leave.
func deliveredFixture(t *testing.T, rework []string, briefs map[string]string) string {
	t.Helper()
	root := t.TempDir()
	status := "# STATUS\n\n## Next up\n\n| Stream | Brief | Wave | Score |\n|---|---|---|---|\n\n" +
		"### Awaiting implementer rework (" + strconv.Itoa(len(rework)) + ")\n\n" +
		"| Stream | Brief | Status | Score | _Blocked_ | Age | Owner | Next act | Verified | Reviewed |\n" +
		"|---|---|---|---|---|---|---|---|---|---|\n"
	for _, r := range rework {
		status += r + "\n"
	}
	writeFile(t, root, "STATUS.md", status)
	readme := "## Briefs\n\n| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n"
	for num, sg := range briefs {
		parts := strings.SplitN(sg, "|", 2)
		readme += "| " + num + " | example brief | 1 | M | " + parts[0] + " | — | — |\n"
		writeFile(t, root, "docs/streams/example-stream/brief-"+num+"-example.md",
			"---\nbrief: example-stream/"+num+"\ngate: "+parts[1]+"\neffort: M\n---\n\n# example\n")
	}
	writeFile(t, root, "docs/streams/example-stream/README.md", readme)
	for _, args := range [][]string{
		{"init", "-q"}, {"add", "STATUS.md", "docs"}, {"commit", "-qm", "board"},
		{"update-ref", "refs/remotes/origin/main", "HEAD"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return root
}

// planFor renders `plan` over the fixture root with the real STATUS.md rework reader (Rework nil)
// and every other source stubbed empty, so the assertion is ONLY about the delivered rows.
func planFor(t *testing.T, root string) string {
	t.Helper()
	f := &FanoutLoop{
		Root:              root,
		TargetSHA:         "deadbeef",
		Board:             func() ([]BoardRow, error) { return nil, nil },
		Orphans:           func() ([]OrphanPR, error) { return nil, nil },
		RepairObligations: func() ([]deskkit.RepairObligation, error) { return nil, nil },
		InFlight:          func() ([]loopengine.Item, error) { return nil, nil },
		Emit:              io.Discard,
	}
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	return out.String()
}

// regression: #5853
func TestPlanImplementedHumanGoesVerify(t *testing.T) {
	setupDeskHome(t)
	root := deliveredFixture(t,
		[]string{"| example-stream | 15 [exec:strong] | implemented | 100 | 1 | 1h | worker | fix, cite example-org/example-repo#1 | — | — |"},
		map[string]string{"15": "implemented|human"})
	s := planFor(t, root)

	if strings.Contains(s, "=== DISPATCH example-stream/15") || strings.Contains(s, "IMPLEMENT example-stream/15") {
		t.Fatalf("an `implemented` gate:human row was offered for dispatch — delivered work must never be re-implemented (#5853):\n%s", s)
	}
	if !strings.Contains(s, "=== ROUTED TO VERIFY: 1 row(s)") || !strings.Contains(s, "example-stream/15 — README status implemented") {
		t.Fatalf("the implemented row must be listed under the ROUTED TO VERIFY heading with its live status (#5853):\n%s", s)
	}
}

// regression: #5853
func TestPlanVerifiedRowGoesVerify(t *testing.T) {
	setupDeskHome(t)
	// The planted second instance: a DIFFERENT past-in-progress status (`verified`) on a gate:model
	// row, beside a genuinely live in-progress brief (16) that the rework lane would not list.
	root := deliveredFixture(t,
		[]string{"| example-stream | 17 | verified | 90 | 1 | 1h | worker | fix, cite example-org/example-repo#2 | — | — |"},
		map[string]string{"16": "in-progress|model", "17": "verified|model"})
	s := planFor(t, root)

	if strings.Contains(s, "=== DISPATCH example-stream/17") {
		t.Fatalf("a `verified` row was offered for dispatch (#5853):\n%s", s)
	}
	if !strings.Contains(s, "example-stream/17 — README status verified") {
		t.Fatalf("the verified row must be listed under the ROUTED TO VERIFY heading (#5853):\n%s", s)
	}
}

// TestSelectQueueRoutesInjectedDone pins the choke point for ANY board source: an injected fresh row
// at `done` and a rework row at `implemented` are routed, while an `in-progress` row and a row whose
// Status could not be read ("") stay dispatchable — could-not-check is never rounded to delivered.
func TestSelectQueueRoutesInjectedDone(t *testing.T) {
	setupDeskHome(t)
	row := func(num, status string) BoardRow {
		r := briefRow("example-stream", num, "M", "", "model", false)
		r.Status = status
		return r
	}
	f := &FanoutLoop{
		Board: func() ([]BoardRow, error) {
			return []BoardRow{row("01", "done"), row("02", "in-progress"), row("03", "")}, nil
		},
		Rework: func() ([]BoardRow, error) { return []BoardRow{row("04", "implemented")}, nil },
		Emit:   io.Discard,
	}
	items, err := f.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if contains(ids, "example-stream/01") || contains(ids, "example-stream/04") {
		t.Fatalf("delivered rows offered for dispatch: %v", ids)
	}
	if !contains(ids, "example-stream/02") || !contains(ids, "example-stream/03") {
		t.Fatalf("in-progress / could-not-check rows wrongly withheld: %v", ids)
	}
	if got := len(f.awaitingVerifyRows()); got != 2 {
		t.Fatalf("want 2 rows routed to verify, got %d", got)
	}
}
