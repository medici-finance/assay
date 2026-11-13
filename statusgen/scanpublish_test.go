package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskcore/issueflow"
)

// scanpublish_test.go — the scan publishes recorded intake judgments (intake-class-routing/02).

const (
	pubOpenedA = "2026-11-10T09:00:00Z"
	pubOpenedB = "2026-11-12T09:00:00Z"
)

// pubRoot is an empty scan root with the issue-loop stream directory present.
func pubRoot(t *testing.T) (root, dir string) {
	t.Helper()
	root = t.TempDir()
	dir = filepath.Join(root, "docs", "streams", scanStreamName)
	if err := os.MkdirAll(filepath.Join(dir, archiveDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, dir
}

func pubPlaceholder(t *testing.T, dir, repo string, issue int) string {
	t.Helper()
	path := filepath.Join(dir, placeholderFileName(repo, issue))
	if err := os.WriteFile(path, []byte(renderPlaceholder(repo, issue, "model", nil)), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// pubRepair is a direct-repair judgment on the issue itself.
func pubRepair(item, opened string, revision int, evidence string) issueflow.IntakeHandoff {
	return issueflow.IntakeHandoff{Schema: issueflow.IntakeHandoffSchema, Item: item, Opened: opened, Revision: revision,
		DecidingRole: "intake-desk", Disposition: issueflow.Repair, Route: "repair", Successor: item, Evidence: evidence}
}

// pubExternal is an external-prerequisite judgment: its action and wake condition are free text.
func pubExternal(item, opened, evidence string) issueflow.IntakeHandoff {
	return issueflow.IntakeHandoff{Schema: issueflow.IntakeHandoffSchema, Item: item, Opened: opened, Revision: 1,
		DecidingRole: "intake-desk", Disposition: issueflow.External, Route: "external", ExternalRole: "human",
		RequiredAction: "rotate the signing key", WakeCondition: "the key has been rotated", Evidence: evidence}
}

func pubLog(t *testing.T, records ...issueflow.IntakeHandoff) string {
	t.Helper()
	var b bytes.Buffer
	for _, r := range records {
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	path := filepath.Join(t.TempDir(), "intake-handoffs-v1.jsonl")
	if err := os.WriteFile(path, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func pubParse(t *testing.T, path string) *Placeholder {
	t.Helper()
	ph, ok, err := parsePlaceholderFile(path)
	if err != nil || !ok {
		t.Fatalf("parse %s: ok=%v err=%v", path, ok, err)
	}
	return ph
}

func pubEligible(ph *Placeholder) bool {
	s := &Stream{Name: scanStreamName, Status: "active"}
	return eligibleBase([]*Stream{s}, s, ph.toBrief(), nil, nil)
}

// treeDigest hashes every file under dir, by relative path, so "writes nothing" is a comparison.
func treeDigest(t *testing.T, dir string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(dir, p)
		lines = append(lines, fmt.Sprintf("%s %x", rel, sha256.Sum256(raw)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// TestScanPublishesHandoff — a recorded direct repair is published by the scan and the placeholder
// becomes eligible; a judgment for an event the log shows to be older, or one a reopen already spent,
// is not published and the placeholder stays held; a second scan with no change writes nothing.
//
// regression: F-judged-no-publisher — a recorded judgment lived only in a local log and nothing wrote the companion, so the issue stayed a placeholder the board held forever.
func TestScanPublishesHandoff(t *testing.T) {
	home := scanHomeRepo()
	item := fmt.Sprintf("%s#77", home)

	t.Run("a real scan creates the placeholder and publishes its judgment", func(t *testing.T) {
		root := t.TempDir()
		if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
			t.Fatal(err)
		}
		data := map[string][]ghIssue{home: {{Number: 77, Title: "a bug", Labels: lbl("bug")}}}
		list := fixtureLister(data, "")
		opts := scanPublishOpts{Judgments: pubLog(t, pubRepair(item, pubOpenedA, 1, item))}
		dir := filepath.Join(root, "docs", "streams", scanStreamName)
		path := filepath.Join(dir, "issue-77.md")

		if code := runScanIssuesWith(root, false, scanPublishOpts{}, list, nilCommentLister, blessAll, labelHoldClearAll); code != 0 {
			t.Fatalf("scan without the log exited %d", code)
		}
		if pubEligible(pubParse(t, path)) {
			t.Fatal("raw scanner placeholder was eligible before any judgment was published")
		}
		if code := runScanIssuesWith(root, false, opts, list, nilCommentLister, blessAll, labelHoldClearAll); code != 0 {
			t.Fatalf("publishing scan exited %d", code)
		}
		ph := pubParse(t, path)
		if ph.Handoff != "issue-77.handoff.json" || ph.HandoffOpened != pubOpenedA {
			t.Fatalf("pointer = %q / %q", ph.Handoff, ph.HandoffOpened)
		}
		if !pubEligible(ph) {
			t.Fatalf("a published direct repair did not reach the board: %+v", ph.HandoffAssessment)
		}
		// A second scan with no change writes nothing.
		before := treeDigest(t, root)
		if code := runScanIssuesWith(root, false, opts, list, nilCommentLister, blessAll, labelHoldClearAll); code != 0 {
			t.Fatalf("second scan exited %d", code)
		}
		if after := treeDigest(t, root); after != before {
			t.Fatalf("an unchanged second scan wrote:\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})

	t.Run("a later revision replaces the companion; the latest event wins; older events are ignored", func(t *testing.T) {
		root, dir := pubRoot(t)
		path := pubPlaceholder(t, dir, home, 77)
		older := pubRepair(item, pubOpenedA, 1, "F-old-finding")
		log := pubLog(t, older, pubRepair(item, pubOpenedB, 1, "F-first"), pubRepair(item, pubOpenedB, 2, "F-second"))
		res, err := publishHandoffs(root, scanPublishOpts{Judgments: log}, false, io.Discard)
		if err != nil || res.Published != 1 {
			t.Fatalf("publish: %+v %v", res, err)
		}
		cur, _, err := readCompanionIn(dir, "issue-77.handoff.json")
		if err != nil || cur.Opened != pubOpenedB || cur.Revision != 2 || cur.Evidence != "F-second" {
			t.Fatalf("companion = %+v %v", cur, err)
		}
		if ph := pubParse(t, path); ph.HandoffOpened != pubOpenedB || !pubEligible(ph) {
			t.Fatalf("pointer %q eligible=%v", ph.HandoffOpened, pubEligible(ph))
		}
		// A revision 3 recorded afterwards replaces it.
		log = pubLog(t, older, pubRepair(item, pubOpenedB, 1, "F-first"), pubRepair(item, pubOpenedB, 2, "F-second"), pubRepair(item, pubOpenedB, 3, "F-third"))
		if res, err = publishHandoffs(root, scanPublishOpts{Judgments: log}, false, io.Discard); err != nil || res.Published != 1 {
			t.Fatalf("revision 3: %+v %v", res, err)
		}
		if cur, _, _ = readCompanionIn(dir, "issue-77.handoff.json"); cur.Revision != 3 {
			t.Fatalf("revision 3 did not replace the companion: %+v", cur)
		}
	})

	t.Run("a record for an older event than the published one is not published", func(t *testing.T) {
		root, dir := pubRoot(t)
		path := pubPlaceholder(t, dir, home, 77)
		if _, err := publishHandoffs(root, scanPublishOpts{Judgments: pubLog(t, pubRepair(item, pubOpenedB, 1, "F-new"))}, false, io.Discard); err != nil {
			t.Fatal(err)
		}
		before := treeDigest(t, root)
		var res scanPublishResult
		var err error
		stderr := captureStderr(t, func() {
			res, err = publishHandoffs(root, scanPublishOpts{Judgments: pubLog(t, pubRepair(item, pubOpenedA, 1, "F-old"))}, false, io.Discard)
		})
		if err != nil || res.Published != 0 || res.Unpublished != 1 {
			t.Fatalf("older event: %+v %v", res, err)
		}
		if !strings.Contains(stderr, item) || !strings.Contains(stderr, "later event") {
			t.Fatalf("the skipped judgment was not named: %q", stderr)
		}
		if treeDigest(t, root) != before {
			t.Fatal("an older-event record changed the published judgment")
		}
		if ph := pubParse(t, path); ph.HandoffOpened != pubOpenedB {
			t.Fatalf("pointer moved to %q", ph.HandoffOpened)
		}
	})

	t.Run("a reactivated placeholder's spent judgment is not published again", func(t *testing.T) {
		root, dir := pubRoot(t)
		path := pubPlaceholder(t, dir, home, 77)
		log := pubLog(t, pubRepair(item, pubOpenedA, 1, "F-spent"))
		if _, err := publishHandoffs(root, scanPublishOpts{Judgments: log}, false, io.Discard); err != nil {
			t.Fatal(err)
		}
		// The issue was reopened: reactivation strips the pointer and a new event owes a new judgment.
		if err := applyCloseOut(closeOutPlan{Path: path, Action: "reactivate", Gate: "model"}); err != nil {
			t.Fatal(err)
		}
		if pubParse(t, path).Handoff != "" {
			t.Fatal("reactivation kept the pointer")
		}
		var res scanPublishResult
		stderr := captureStderr(t, func() {
			res, _ = publishHandoffs(root, scanPublishOpts{Judgments: log}, false, io.Discard)
		})
		ph := pubParse(t, path)
		if res.Published != 0 || res.Unpublished != 1 || ph.Handoff != "" || pubEligible(ph) {
			t.Fatalf("a spent judgment was republished: %+v handoff=%q eligible=%v", res, ph.Handoff, pubEligible(ph))
		}
		if !strings.Contains(stderr, "spent") {
			t.Fatalf("not named: %q", stderr)
		}
		// A judgment for a newer event IS published.
		log = pubLog(t, pubRepair(item, pubOpenedA, 1, "F-spent"), pubRepair(item, pubOpenedB, 1, "F-new-event"))
		if res, _ = publishHandoffs(root, scanPublishOpts{Judgments: log}, false, io.Discard); res.Published != 1 || !pubEligible(pubParse(t, path)) {
			t.Fatalf("the new event's judgment was not published: %+v", res)
		}
	})

	t.Run("a record for another issue, an undecodable line and an absent log do not publish", func(t *testing.T) {
		root, dir := pubRoot(t)
		path := pubPlaceholder(t, dir, home, 77)
		other := pubRepair(fmt.Sprintf("%s#78", home), pubOpenedA, 1, "F-other")
		log := pubLog(t, other)
		if err := os.WriteFile(log, []byte(mustRead(t, log)+"{not json\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		var res scanPublishResult
		stderr := captureStderr(t, func() { res, _ = publishHandoffs(root, scanPublishOpts{Judgments: log}, false, io.Discard) })
		if res.Published != 0 || res.Unpublished != 1 || !strings.Contains(stderr, "line 2") {
			t.Fatalf("res=%+v stderr=%q", res, stderr)
		}
		if ph := pubParse(t, path); ph.Handoff != "" || pubEligible(ph) {
			t.Fatal("placeholder was published from someone else's judgment")
		}
		before := treeDigest(t, root)
		var out bytes.Buffer
		if res, err := publishHandoffs(root, scanPublishOpts{Judgments: filepath.Join(t.TempDir(), "absent.jsonl")}, false, &out); err != nil || res != (scanPublishResult{}) {
			t.Fatalf("absent log: %+v %v", res, err)
		}
		if treeDigest(t, root) != before {
			t.Fatal("an absent log wrote")
		}
	})

	t.Run("dry-run lists and writes nothing", func(t *testing.T) {
		root, dir := pubRoot(t)
		pubPlaceholder(t, dir, home, 77)
		before := treeDigest(t, root)
		var out bytes.Buffer
		res, err := publishHandoffs(root, scanPublishOpts{Judgments: pubLog(t, pubRepair(item, pubOpenedA, 1, item))}, true, &out)
		if err != nil || res.Published != 1 || !strings.Contains(out.String(), "would publish issue-77.handoff.json") {
			t.Fatalf("dry-run: %+v %v %q", res, err, out.String())
		}
		if treeDigest(t, root) != before {
			t.Fatal("dry-run wrote")
		}
	})
}

// TestScanPublishKeepsRegister — the intake register lane's handoff and exit exports are byte-identical
// before and after a publishing scan, and the register's own files are untouched.
//
// regression: F-judged-no-publisher — publication must not change the intake register lane's exports.
func TestScanPublishKeepsRegister(t *testing.T) {
	root, dir := pubRoot(t)
	home := scanHomeRepo()
	item := fmt.Sprintf("%s#77", home)
	pubPlaceholder(t, dir, home, 77)

	entryDir := filepath.Join(root, "docs", "streams", "intake", "completed")
	if err := os.MkdirAll(entryDir, 0o700); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(entryDir, "I-test.md")
	if err := os.WriteFile(entry, []byte("---\nid: I-test\ndate: 2026-10-25\ntitle: Handoff fixture\ndisposition: scoped\nscoped-to: issue #77\ntriaged: 2026-10-25\ntriaged-by: intake-desk\ntriager-tier: strong\nhandoff: handoff.json\nhandoff-opened: \"2026-10-25T12:00:00Z\"\n---\nFixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reg := pubRepair("I-test", "2026-10-25T12:00:00Z", 1, item)
	reg.Successor = item // the register entry is routed to the issue
	raw, _ := json.Marshal(reg)
	if err := os.WriteFile(filepath.Join(entryDir, "handoff.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	export := func() (handoffs, exits string) {
		t.Helper()
		var h, e, errb bytes.Buffer
		if code := writeIntakeHandoffs(root, &h, &errb); code != 0 {
			t.Fatalf("handoff export: %d %s", code, &errb)
		}
		if code := writeIntakeExits(root, true, &e, &errb); code != 0 {
			t.Fatalf("exit export: %d %s", code, &errb)
		}
		return h.String(), e.String()
	}
	h0, e0 := export()
	if !strings.Contains(h0, "observed") {
		t.Fatalf("fixture register entry is not observed: %s", h0)
	}
	register0 := treeDigest(t, filepath.Join(root, "docs", "streams", "intake"))

	res, err := publishHandoffs(root, scanPublishOpts{Judgments: pubLog(t, pubRepair(item, pubOpenedA, 1, item))}, false, io.Discard)
	if err != nil || res.Published != 1 {
		t.Fatalf("publishing scan: %+v %v", res, err)
	}
	h1, e1 := export()
	if h1 != h0 || e1 != e0 {
		t.Fatalf("a publishing scan changed the register export:\nhandoffs before:\n%s\nafter:\n%s\nexits before:\n%s\nafter:\n%s", h0, h1, e0, e1)
	}
	if treeDigest(t, filepath.Join(root, "docs", "streams", "intake")) != register0 {
		t.Fatal("a publishing scan wrote under the intake register")
	}
}

// pubGitRoot is a scan root that is a git checkout whose origin is origin ("" = no remote).
func pubGitRoot(t *testing.T, origin string) (root, dir string) {
	t.Helper()
	root, dir = pubRoot(t)
	for _, args := range [][]string{{"init", "-q"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if origin != "" {
		if out, err := exec.Command("git", "-C", root, "remote", "add", "origin", origin).CombinedOutput(); err != nil {
			t.Fatalf("git remote add: %v %s", err, out)
		}
	}
	return root, dir
}

// TestScanPublishScrubBoundary — free text reaches a committed companion only when a commit target was
// supplied, the root's origin push destination names that same repository, and the roster marks it
// private. Otherwise only a typed-reference repair is published and the rest are named and counted.
//
// regression: F-judged-free-text-public — publishing a judgment copies free text into a committed file, which in a public repository would leak it.
func TestScanPublishScrubBoundary(t *testing.T) {
	const (
		private  = "example-org/agents"       // marked :private in the fixture roster
		public   = "example-org/ledger-k8s"   // marked :public
		unlisted = "example-org/never-listed" // not in the roster
		homeRepo = "example-org/tracker"      // the roster's home repository, marked :private
	)
	url := func(r string) string { return "https://github.com/" + r + ".git" }

	type outcome struct{ prose, typed, external bool }
	all := outcome{true, true, true}
	onlyTyped := outcome{false, true, false}

	cases := []struct {
		name   string
		target string // the commit target supplied to the scan
		origin string // the root's origin ("" = none)
		source string // the repository the scanned issues come from
		home   string // roster HOME_REPO
		want   outcome
	}{
		{"public root", public, url(public), homeRepo, homeRepo, onlyTyped},
		{"unlisted root", unlisted, url(unlisted), homeRepo, homeRepo, onlyTyped},
		{"private root", private, url(private), homeRepo, homeRepo, all},
		{"private root, scp-style origin", private, "git@github.com:" + private + ".git", homeRepo, homeRepo, all},
		{"public root scanning an issue from a private repo", public, url(public), private, homeRepo, onlyTyped},
		{"private root scanning an issue from a public repo", private, url(private), public, homeRepo, all},
		{"home marked private, target public", public, url(public), homeRepo, homeRepo, onlyTyped},
		{"home marked private, target unlisted", unlisted, url(unlisted), homeRepo, homeRepo, onlyTyped},
		{"no target supplied, private origin, private home", "", url(private), homeRepo, homeRepo, onlyTyped},
		{"private target, origin names a public repo", private, url(public), homeRepo, homeRepo, onlyTyped},
		{"private target, origin names an unlisted repo", private, url(unlisted), homeRepo, homeRepo, onlyTyped},
		{"private target, origin cannot be read", private, "", homeRepo, homeRepo, onlyTyped},
		{"private target, origin is not a repository url", private, "/some/local/path", homeRepo, homeRepo, onlyTyped},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scanWithRoster(t, rosterWith(map[string]string{scanEnvHomeRepo: tc.home}))
			root, dir := pubGitRoot(t, tc.origin)
			item := func(n int) string { return fmt.Sprintf("%s#%d", tc.source, n) }
			phs := map[string]string{
				"prose":    pubPlaceholder(t, dir, tc.source, 1),
				"typed":    pubPlaceholder(t, dir, tc.source, 2),
				"external": pubPlaceholder(t, dir, tc.source, 3),
			}
			log := pubLog(t,
				pubRepair(item(1), pubOpenedA, 1, "reproduced on the staging cluster last Tuesday"),
				pubRepair(item(2), pubOpenedA, 1, item(2)),
				pubExternal(item(3), pubOpenedA, "F-needs-key-rotation"),
			)
			var res scanPublishResult
			var err error
			stderr := captureStderr(t, func() {
				res, err = publishHandoffs(root, scanPublishOpts{Judgments: log, Target: tc.target}, false, io.Discard)
			})
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]bool{"prose": tc.want.prose, "typed": tc.want.typed, "external": tc.want.external}
			published := 0
			for name, path := range phs {
				got := pubParse(t, path).Handoff != ""
				_, statErr := os.Stat(filepath.Join(dir, handoffCompanionName(path)))
				if got != want[name] || (statErr == nil) != want[name] {
					t.Errorf("%s: pointer=%v companion=%v, want published=%v\n%s", name, got, statErr == nil, want[name], stderr)
				}
				if got {
					published++
				}
			}
			if res.Published != published || res.Unpublished != 3-published {
				t.Errorf("counts = %+v, want %d published", res, published)
			}
			if !tc.want.prose && !strings.Contains(stderr, item(1)+": evidence is not a typed reference") {
				t.Errorf("free-prose evidence not named with its field:\n%s", stderr)
			}
			if !tc.want.external && !strings.Contains(stderr, item(3)+": required_action is free text") {
				t.Errorf("external judgment's free-text action not named with its field:\n%s", stderr)
			}
			// Free text must not be on disk anywhere when it was not published.
			if !tc.want.external {
				if strings.Contains(treeText(t, root), "rotate the signing key") || strings.Contains(treeText(t, root), "the key has been rotated") {
					t.Error("a withheld free-text field reached the tree")
				}
			}
		})
	}
}

// treeText concatenates every file under root except .git, for "this text is nowhere" checks.
func treeText(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		raw, _ := os.ReadFile(p)
		b.Write(raw)
		return nil
	})
	return b.String()
}
