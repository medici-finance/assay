package deskkit

import (
	"fmt"
	"strings"
	"testing"
)

// scanDiffFixture is a realistic `git diff -U0 -M` over the issue-loop stream: two new
// placeholders, one retired by a status flip, one retired by an archive rename, one
// README edit that must not count, and one out-of-scope file.
const scanDiffFixture = `diff --git a/docs/streams/issue-loop/issue-901-thing.md b/docs/streams/issue-loop/issue-901-thing.md
new file mode 100644
index 0000000..1111111
--- /dev/null
+++ b/docs/streams/issue-loop/issue-901-thing.md
@@ -0,0 +1,5 @@
+---
+status: todo
+---
diff --git a/docs/streams/issue-loop/issue-902-other.md b/docs/streams/issue-loop/issue-902-other.md
new file mode 100644
index 0000000..2222222
--- /dev/null
+++ b/docs/streams/issue-loop/issue-902-other.md
@@ -0,0 +1,5 @@
+---
+status: todo
+---
diff --git a/docs/streams/issue-loop/issue-800-old.md b/docs/streams/issue-loop/issue-800-old.md
index 3333333..4444444 100644
--- a/docs/streams/issue-loop/issue-800-old.md
+++ b/docs/streams/issue-loop/issue-800-old.md
@@ -2 +2 @@
-status: todo
+status: done
diff --git a/docs/streams/issue-loop/issue-700-archived.md b/docs/streams/issue-loop/done/issue-700-archived.md
similarity index 100%
rename from docs/streams/issue-loop/issue-700-archived.md
rename to docs/streams/issue-loop/done/issue-700-archived.md
diff --git a/docs/streams/issue-loop/README.md b/docs/streams/issue-loop/README.md
index 5555555..6666666 100644
--- a/docs/streams/issue-loop/README.md
+++ b/docs/streams/issue-loop/README.md
@@ -30 +30 @@
-| 800 | old | todo |
+| 800 | old | done |
diff --git a/docs/streams/ground-truth/brief-05-x.md b/docs/streams/ground-truth/brief-05-x.md
new file mode 100644
index 0000000..7777777
--- /dev/null
+++ b/docs/streams/ground-truth/brief-05-x.md
@@ -0,0 +1,2 @@
+status: done
`

func TestParseScanDiff(t *testing.T) {
	c := ParseScanDiff(scanDiffFixture, ScanDir)

	if len(c.Created) != 2 {
		t.Errorf("created = %d, want 2: %+v", len(c.Created), c.Created)
	}
	if len(c.Retired) != 2 {
		t.Errorf("retired = %d, want 2: %+v", len(c.Retired), c.Retired)
	}

	// The README row edit is the stream's own board, not a placeholder — counting it
	// would inflate every scan by one and reintroduce a wrong number.
	for _, e := range append(append([]ScanEntry{}, c.Created...), c.Retired...) {
		if strings.HasSuffix(e.Path, "README.md") {
			t.Errorf("the stream README must never be counted; got %+v", e)
		}
		if !strings.HasPrefix(e.Path, ScanDir+"/") {
			t.Errorf("out-of-scope path counted: %+v", e)
		}
	}

	// The archived file is the rename destination, and the flipped file is counted
	// once each — the two retirement shapes must not double-count.
	want := map[string]bool{
		ScanDir + "/issue-800-old.md":           true,
		ScanDir + "/done/issue-700-archived.md": true,
	}
	for _, e := range c.Retired {
		if !want[e.Path] {
			t.Errorf("unexpected retirement %+v", e)
		}
	}
}

// TestParseScanDiffFlipAndArchiveCountOnce guards the union: a scan that flips a
// placeholder to done AND archives it in the same pass is one retirement.
func TestParseScanDiffFlipAndArchiveCountOnce(t *testing.T) {
	diff := `diff --git a/docs/streams/issue-loop/issue-500-x.md b/docs/streams/issue-loop/done/issue-500-x.md
similarity index 80%
rename from docs/streams/issue-loop/issue-500-x.md
rename to docs/streams/issue-loop/done/issue-500-x.md
--- a/docs/streams/issue-loop/issue-500-x.md
+++ b/docs/streams/issue-loop/done/issue-500-x.md
@@ -2 +2 @@
-status: todo
+status: done
`
	c := ParseScanDiff(diff, ScanDir)
	if len(c.Retired) != 1 {
		t.Fatalf("a flip-and-archive in one pass is ONE retirement; got %d: %+v", len(c.Retired), c.Retired)
	}
	if len(c.Created) != 0 {
		t.Fatalf("an archive move is not a creation; got %+v", c.Created)
	}
}

// TestParseScanDiffBornDoneCountsOnce guards the other double-count shape the reviewer
// of #875 named: a placeholder whose FIRST version already carries `status: done` (the
// scanner met an issue that was closed before it ever saw it). It is a new file AND its
// added lines match the status-flip retirement, so it would land in both columns and
// inflate both counts. It resolves to a retirement, matching the rule a placeholder born
// inside done/ already had — a record that never was live work is not a creation.
func TestParseScanDiffBornDoneCountsOnce(t *testing.T) {
	diff := `diff --git a/docs/streams/issue-loop/issue-950-born-done.md b/docs/streams/issue-loop/issue-950-born-done.md
new file mode 100644
index 0000000..8888888
--- /dev/null
+++ b/docs/streams/issue-loop/issue-950-born-done.md
@@ -0,0 +1,3 @@
+---
+status: done
+---
`
	c := ParseScanDiff(diff, ScanDir)
	if len(c.Created) != 0 {
		t.Errorf("a placeholder born done is not a creation; got %+v", c.Created)
	}
	if len(c.Retired) != 1 {
		t.Errorf("a placeholder born done is one retirement; got %d: %+v", len(c.Retired), c.Retired)
	}
	if len(c.Created)+len(c.Retired) != 1 {
		t.Errorf("the columns must be disjoint — one path, one count; got %+v", c)
	}
}

func TestParseScanDiffEmpty(t *testing.T) {
	c := ParseScanDiff("", ScanDir)
	if len(c.Created) != 0 || len(c.Retired) != 0 {
		t.Fatalf("an empty diff derives 0/0; got %+v", c)
	}
}

func TestScanPRTitleAndBodyAreDerived(t *testing.T) {
	c := ParseScanDiff(scanDiffFixture, ScanDir)
	title := ScanPRTitle("2026-08-13", c)
	if title != "chore(issue-loop): scan 2026-08-13 — 2 created, 2 retired" {
		t.Fatalf("unexpected title: %q", title)
	}
	body := ScanPRBody("2026-08-13", c, "abc1234")
	for _, want := range []string{ScanBodyMarker, "DERIVED", "**created:** 2", "**retired:** 2",
		"abc1234..HEAD", "issue-901-thing.md", "#685"} {
		if !strings.Contains(body, want) {
			t.Errorf("body must contain %q; got:\n%s", want, body)
		}
	}
	// The title and the body must agree by construction — they are the same numbers.
	if err := ScanBodyDrift(title, c); err != nil {
		t.Errorf("the derived title must never trip the drift lint: %v", err)
	}
	if err := ScanBodyDrift(body, c); err != nil {
		t.Errorf("the derived body must never trip the drift lint: %v", err)
	}
}

// TestScanBodyDriftCanFail is the required proof-it-can-fail: the positive control is
// #627's own numbers — a body still claiming 29/29 over a diff that grew to 48/32.
func TestScanBodyDriftCanFail(t *testing.T) {
	// Build a diff with 48 created and 32 retired, the way #627's branch actually grew.
	var b strings.Builder
	for i := 0; i < 48; i++ {
		fmt.Fprintf(&b, "diff --git a/%s/issue-%03d-a.md b/%s/issue-%03d-a.md\nnew file mode 100644\n", ScanDir, i, ScanDir, i)
	}
	for i := 0; i < 32; i++ {
		fmt.Fprintf(&b, "diff --git a/%s/issue-%03d-r.md b/%s/issue-%03d-r.md\n@@ -2 +2 @@\n-status: todo\n+status: done\n",
			ScanDir, 500+i, ScanDir, 500+i)
	}
	c := ParseScanDiff(b.String(), ScanDir)
	if len(c.Created) != 48 || len(c.Retired) != 32 {
		t.Fatalf("fixture derives %d/%d, want 48/32", len(c.Created), len(c.Retired))
	}

	stale := "chore(issue-loop): scan 2026-08-11 — 29 created, 29 retired"
	err := ScanBodyDrift(stale, c)
	if err == nil {
		t.Fatal("the stale #627 title must FAIL the drift lint — a check that cannot fire is not a check")
	}
	if ExitCodeOf(err) != ExitRefused {
		t.Errorf("drift must be exit 5 refused, got %d", ExitCodeOf(err))
	}
	for _, want := range []string{"29 created", "48 created", "32 retired", "#685"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must state both numbers and the class; missing %q in:\n%s", want, err.Error())
		}
	}
}

func TestStatedScanCounts(t *testing.T) {
	cases := []struct {
		text             string
		created, retired int
		ok               bool
	}{
		{"chore(issue-loop): scan 2026-08-11 — 29 created, 29 retired", 29, 29, true},
		{"Automated inbound scan: 48 created and 32 retired placeholders.", 48, 32, true},
		{"- **created:** 7\n- **retired:** 3\n", 7, 3, true},
		{"Automated inbound scan. Review created/retired placeholders.", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, tc := range cases {
		c, r, ok := StatedScanCounts(tc.text)
		if ok != tc.ok || c != tc.created || r != tc.retired {
			t.Errorf("StatedScanCounts(%q) = %d,%d,%t; want %d,%d,%t", tc.text, c, r, ok, tc.created, tc.retired, tc.ok)
		}
	}
}

// A body stating no counts is not drift: absence is the emitter's business.
func TestScanBodyDriftIgnoresCountlessText(t *testing.T) {
	c := ParseScanDiff(scanDiffFixture, ScanDir)
	if err := ScanBodyDrift("Automated inbound scan. Review the placeholders.", c); err != nil {
		t.Fatalf("text stating no counts must pass; got %v", err)
	}
}

func TestScanBodyCapsTheListNotTheCount(t *testing.T) {
	var b strings.Builder
	for i := 0; i < scanListCap+5; i++ {
		fmt.Fprintf(&b, "diff --git a/%s/issue-%03d-a.md b/%s/issue-%03d-a.md\nnew file mode 100644\n", ScanDir, i, ScanDir, i)
	}
	c := ParseScanDiff(b.String(), ScanDir)
	body := ScanPRBody("2026-08-13", c, "abc")
	if !strings.Contains(body, fmt.Sprintf("**created:** %d", scanListCap+5)) {
		t.Errorf("the COUNT must never be truncated; got:\n%s", body)
	}
	if !strings.Contains(body, "and 5 more") {
		t.Errorf("the LIST is capped and must say so; got:\n%s", body)
	}
}
