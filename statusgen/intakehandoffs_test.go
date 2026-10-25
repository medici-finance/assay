package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskcore/issueflow"
)

func TestScanHandoffAdmission(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "issue-77.md")
	write := func(p, body string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	parse := func() *Placeholder {
		t.Helper()
		ph, ok, err := parsePlaceholderFile(path)
		if err != nil || !ok {
			t.Fatalf("parse: %v %v", ok, err)
		}
		return ph
	}
	s := &Stream{Name: "issue-loop", Status: "active"}
	eligible := func(ph *Placeholder) bool { return eligibleBase([]*Stream{s}, s, ph.toBrief(), nil, nil) }
	write(path, renderPlaceholder("example/repo", 77, "model", nil))
	ph := parse()
	if eligible(ph) || !strings.Contains(ph.toBrief().Title, "classification-needed") {
		t.Fatal("raw scanner representation became work")
	}
	// Closure must not erase an unknown source before the judgment exists.
	closedSource := *ph
	closedSource.Repo = scanHomeRepo()
	s.Placeholders = []Placeholder{closedSource}
	_, closed, notices, _ := planScan(dir, []*Stream{s}, fixtureLister(nil, ""), blessAll, labelHoldClearAll)
	if len(closed) != 0 {
		t.Fatal("closed raw intake disappeared before disposition")
	}
	if len(notices) == 0 {
		t.Fatal("closed raw intake was not visible")
	}
	h := issueflow.IntakeHandoff{Schema: issueflow.IntakeHandoffSchema, Item: "example/repo#77", Opened: "2026-10-25T12:00:00Z", Revision: 1, DecidingRole: "intake-desk", Disposition: issueflow.Repair, Route: "repair", Successor: "example/repo#77", Evidence: "example/repo#77"}
	raw, _ := json.Marshal(h)
	write(filepath.Join(dir, "handoff.json"), string(raw))
	body := strings.Replace(renderPlaceholder("example/repo", 77, "model", nil), "status: todo", "status: todo\nhandoff: handoff.json\nhandoff-opened: \"2026-10-25T12:00:00Z\"", 1)
	write(path, body)
	if !eligible(parse()) {
		t.Fatal("recorded repair disposition not considered")
	}
	for _, label := range []string{"to:worker", "to:", "needs-decision", "question", "help wanted", "review-request"} {
		write(path, strings.Replace(body, "status: todo", "status: todo\nlabels: [\""+label+"\"]", 1))
		if eligible(parse()) {
			t.Fatalf("%s entered ordinary work", label)
		}
	}
	write(path, body)
	write(filepath.Join(dir, "handoff.json"), "{broken")
	if eligible(parse()) {
		t.Fatal("unread disposition became ready")
	}
	write(filepath.Join(dir, "handoff.json"), string(raw))
	write(path, body)
	if err := applyCloseOut(closeOutPlan{Path: path, Action: "reactivate", Gate: "model"}); err != nil {
		t.Fatal(err)
	}
	if eligible(parse()) {
		t.Fatal("reactivation reused an old disposition")
	}
	// The register exports the same companion without altering v1 exits.
	root := t.TempDir()
	entryDir := filepath.Join(root, "docs", "streams", "intake", "completed")
	if err := os.MkdirAll(entryDir, 0700); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(entryDir, "I-test.md")
	write(entry, "---\nid: I-test\ndate: 2026-10-25\ntitle: Handoff fixture\ndisposition: scoped\nscoped-to: issue #77\ntriaged: 2026-10-25\ntriaged-by: intake-desk\ntriager-tier: strong\n---\nFixture\n")
	var old, next, exits, errb bytes.Buffer
	if code := writeIntakeExits(root, true, &old, &errb); code != 0 {
		t.Fatalf("old export: %d %s", code, &errb)
	}
	if code := writeIntakeHandoffs(root, &next, &errb); code != 0 || !strings.Contains(next.String(), "classification-needed") {
		t.Fatalf("legacy export: %d %s %s", code, &next, &errb)
	}
	h.Item = "I-test"
	raw, _ = json.Marshal(h)
	write(filepath.Join(entryDir, "handoff.json"), string(raw))
	data, _ := os.ReadFile(entry)
	write(entry, strings.Replace(string(data), "disposition: scoped", "disposition: scoped\nhandoff: handoff.json\nhandoff-opened: \"2026-10-25T12:00:00Z\"", 1))
	next.Reset()
	if code := writeIntakeHandoffs(root, &next, &errb); code != 0 || !strings.Contains(next.String(), "observed") {
		t.Fatalf("companion export: %d %s %s", code, &next, &errb)
	}
	if code := writeIntakeExits(root, true, &exits, &errb); code != 0 || old.String() != exits.String() {
		t.Fatalf("v1 changed: %d %s %s", code, &old, &exits)
	}
}
