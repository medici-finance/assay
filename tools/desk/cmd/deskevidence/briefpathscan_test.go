package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- #1161: added-bytes scoping on EVERY landing path, and a refusal that names its origin ---
//
// #901 scoped the --brief-path scan to the evidence file and #1004 scoped the direct-write
// flow to addedLines(remote, local). The two paths answered the same question ("what does THIS
// commit add?") with two different surfaces: the --brief-path path scanned the caller's whole
// evidence file rather than the diff of what actually lands. An Evidence file that re-quotes a
// line the brief already carries — a Verify row's own grep command, a fingerprint named in
// prose — therefore still refused on text that predates the landing by weeks, and the refusal
// gave the operator nothing to isolate it with. Both paths now scan
// addedLines(remoteContent, commitContent), and a refusal names where the offending bytes
// came from.

// fixtureFingerprint is an obviously synthetic 40-uppercase-hex OpenPGP-fingerprint SHAPE. It
// is split into two fragments for the same reason TestPGPFingerprintExemption splits its
// values: deskpr scans the branch diff, and a contiguous bare 40-hex run on an added line is
// exactly what the pre-fix scanner refuses. It is not a real key fingerprint.
const fixtureFingerprint = "0123456789ABCDEF0123" + "456789ABCDEF01234567"

// briefWithBareFingerprint is the #1161 fixture: a brief that names the fingerprint bare in
// prose (line 3, no pgp:/fp: field, no annotation word) and again inside its own Verify-row
// grep command (line 7). Both predate the landing.
func briefWithBareFingerprint() string {
	return "# Brief\n" +
		"\n" +
		"The signing key is " + fixtureFingerprint + " and must not change.\n" +
		"\n" +
		"## Verify\n" +
		"| # | Command | Expect |\n" +
		"| 1 | `grep -c '" + fixtureFingerprint + "' keys.txt` | `1` |\n" +
		"\n" +
		"## Evidence\n" +
		"| 1 | a | b |\n"
}

// TestBriefPathLandsWhenBriefCarriesBareFingerprint: with --brief-path, a brief that ALREADY
// carries a bare 40-uppercase-hex run must still receive a clean Evidence block. Two shapes of
// "clean": an Evidence file with nothing secret-shaped in it at all, and one that re-quotes the
// brief's own Verify-row line VERBATIM — that line is already on the branch, so it is not a
// byte this landing adds.
func TestBriefPathLandsWhenBriefCarriesBareFingerprint(t *testing.T) {
	cases := []struct {
		name     string
		evidence string
	}{
		{"clean evidence block", "| 2 | 2026-09-15 | PASS |\n"},
		{"evidence re-quotes the brief's own Verify row verbatim",
			"| 1 | `grep -c '" + fixtureFingerprint + "' keys.txt` | `1` |\n| 2 | 2026-09-15 | PASS |\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, errBuf := setupFake(t)
			briefPath := "docs/streams/x/brief.md"
			f.setFile(briefPath, briefWithBareFingerprint())
			evidencePath := writeRepoFile(t, "row.md", c.evidence)

			code := run([]string{"example-org/tracker", "main",
				"--evidence-file", evidencePath, "--brief-path", briefPath})
			if code != deskkit.ExitOK {
				t.Fatalf("exit = %d, want 0 (a bare fingerprint ALREADY in the brief must not block a clean Evidence landing); stderr:\n%s",
					code, errBuf.String())
			}
			if f.putCalls != 1 {
				t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
			}
			if !strings.Contains(f.putContent, "| 2 | 2026-09-15 | PASS |") {
				t.Fatalf("merged content missing the new row:\n%s", f.putContent)
			}
		})
	}
}

// TestBriefPathRefusesFingerprintAddedByEvidence: the companion negative row — the SAME bare
// run, this time typed by the Evidence file itself (line 2 of it) into a brief that does not
// carry it, must still refuse, and the refusal names the origin as the evidence file and the
// line. Proves the fix narrowed the scan's scope, not its sensitivity.
func TestBriefPathRefusesFingerprintAddedByEvidence(t *testing.T) {
	f, errBuf := setupFake(t)
	briefPath := "docs/streams/x/brief.md"
	f.setFile(briefPath, "# Brief\n\n## Evidence\n| 1 | a | b |\n")
	evidencePath := writeRepoFile(t, "row.md",
		"| 2 | 2026-09-15 | PASS |\n"+
			"| 3 | key is "+fixtureFingerprint+" | PASS |\n")

	code := run([]string{"example-org/tracker", "main",
		"--evidence-file", evidencePath, "--brief-path", briefPath})
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d (a bare 40-uppercase-hex run in the NEW evidence must still refuse)", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("a secret-scanned refusal still wrote %d time(s)", f.putCalls)
	}
	if want := "added by --evidence-file:2"; !strings.Contains(errBuf.String(), want) {
		t.Fatalf("refusal does not name the origin %q; stderr:\n%s", want, errBuf.String())
	}
	if strings.Contains(errBuf.String(), fixtureFingerprint) {
		t.Fatalf("the refusal leaked the offending span itself:\n%s", errBuf.String())
	}
}

// TestDirectWriteRefusalNamesEvidenceFileLine: the direct-write flow (no --brief-path) reports
// the same origin, with the line number of the LOCAL evidence file (the whole target file the
// caller merged), not the line's position inside the added-lines surface — the operator opens
// the file they wrote and goes to that line.
func TestDirectWriteRefusalNamesEvidenceFileLine(t *testing.T) {
	f, errBuf := setupFake(t)
	remote := "# Brief\n\n## Evidence\n| 1 | a | b |\n"
	// A repo-relative docs/streams/ target resolved via --root: deskevidence's docs/streams
	// scoping guard (assay#1078) refuses an absolute target, which would shadow the
	// secret-scan refusal this test exercises. The local file content — and so the reported
	// line number — is unchanged; the offending row is still line 6 of the file.
	evidencePath := "docs/streams/x/brief.md"
	root := rootWithFile(t, evidencePath,
		remote+"| 2 | clean | PASS |\n| 3 | key is "+fixtureFingerprint+" | PASS |\n")
	f.setFile(evidencePath, remote)

	code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.putCalls != 0 {
		t.Fatalf("a secret-scanned refusal still wrote %d time(s)", f.putCalls)
	}
	// remote is 4 lines; the clean row is line 5; the offending row is line 6 of the local file.
	if want := "added by --evidence-file:6"; !strings.Contains(errBuf.String(), want) {
		t.Fatalf("refusal does not name the origin %q; stderr:\n%s", want, errBuf.String())
	}
}

// TestPreexistingRunIsNoticedNotRefused: the other origin. When the brief already carries a
// secret-shaped run and the landing's own bytes are clean, the landing goes through and the
// tool SAYS on stderr where the pre-existing run lives (`pre-existing in <brief-path>:<line>`),
// so the operator who sees the brief's text and the scan side by side is not left to isolate
// it by hand. It is a notice, never a refusal, and it never carries the span.
func TestPreexistingRunIsNoticedNotRefused(t *testing.T) {
	f, errBuf := setupFake(t)
	briefPath := "docs/streams/x/brief.md"
	f.setFile(briefPath, briefWithBareFingerprint())
	evidencePath := writeRepoFile(t, "row.md", "| 2 | 2026-09-15 | PASS |\n")

	code := run([]string{"example-org/tracker", "main",
		"--evidence-file", evidencePath, "--brief-path", briefPath})
	if code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0; stderr:\n%s", code, errBuf.String())
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
	}
	if want := "pre-existing in " + briefPath + ":3"; !strings.Contains(errBuf.String(), want) {
		t.Fatalf("stderr does not name the pre-existing origin %q:\n%s", want, errBuf.String())
	}
	if strings.Contains(errBuf.String(), fixtureFingerprint) {
		t.Fatalf("the notice leaked the pre-existing span itself:\n%s", errBuf.String())
	}
}

// TestNoNoticeOnCleanBrief: the notice is earned, not ambient — a brief with nothing
// secret-shaped in it lands with no `pre-existing in` line on stderr.
func TestNoNoticeOnCleanBrief(t *testing.T) {
	f, errBuf := setupFake(t)
	briefPath := "docs/streams/x/brief.md"
	f.setFile(briefPath, "# Brief\n\n## Evidence\n| 1 | a | b |\n")
	evidencePath := writeRepoFile(t, "row.md", "| 2 | 2026-09-15 | PASS |\n")

	code := run([]string{"example-org/tracker", "main",
		"--evidence-file", evidencePath, "--brief-path", briefPath})
	if code != deskkit.ExitOK {
		t.Fatalf("exit = %d, want 0; stderr:\n%s", code, errBuf.String())
	}
	if f.putCalls != 1 {
		t.Fatalf("expected 1 WriteFile, got %d", f.putCalls)
	}
	if strings.Contains(errBuf.String(), "pre-existing in") {
		t.Fatalf("a clean brief produced a pre-existing notice:\n%s", errBuf.String())
	}
}

// TestScanOriginTable pins the origin classifier directly: a finding on the scanned surface
// maps back to the line of the LOCAL evidence file that carries it, and a line the scanned
// surface has but the evidence file does not is reported as unmapped rather than guessed.
func TestScanOriginTable(t *testing.T) {
	local := "a\n  b\nc\n"
	cases := []struct {
		name    string
		surface string
		line    int
		want    string
	}{
		{"first surface line maps to local line 1", "a\nc\n", 1, "added by --evidence-file:1"},
		{"whitespace-trimmed merge line still maps", "b\n", 1, "added by --evidence-file:2"},
		{"second surface line maps to local line 3", "a\nc\n", 2, "added by --evidence-file:3"},
		{"line absent from the evidence file is unmapped", "zzz\n", 1, "added by this landing (line 1 of the added bytes; not found in --evidence-file)"},
		{"zero line is unmapped", "a\n", 0, "added by this landing (line 0 of the added bytes; not found in --evidence-file)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := addedOrigin([]byte(c.surface), []byte(local), c.line)
			if got != c.want {
				t.Fatalf("addedOrigin = %q, want %q", got, c.want)
			}
		})
	}
}
