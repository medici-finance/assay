package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Problem classification (oit issue #1368).
//
// EXACTLY ONE check is off-board: darSyncCheck. Everything else blocks. The
// tests below pin the partition per KIND, not just the mechanism — an earlier
// revision of this file planted the same dead link in every case, which pinned
// "some problem is non-blocking" and would have stayed green while a kind
// silently changed class.
//
// Four properties, and all four must hold together:
//
//  1. a dar-sync problem does not suppress generation, but still exits 1;
//  2. a link problem DOES suppress generation — docFiles(root) includes every
//     stream README and brief file, and the dead-link lint is the only check
//     that catches a README row with no brief file behind it;
//  3. a register-ref problem likewise blocks (same docFiles superset);
//  4. every mode still exits non-zero, including --check.

// darMismatch overlays the #465 shape onto an existing root: daml.yaml at
// 0.1.16 while both envs' DAR ConfigMaps and deploy Jobs sit at 0.1.8. This is
// the same shape that armed oit #1368 on main (prod's EXPECTED_DAR_VERSION
// trailing daml.yaml).
func darMismatch(t *testing.T, root string) {
	t.Helper()
	writeDarCore(t, root, "0.1.16", "0.1.8")
	for _, env := range []string{"dev", "prod"} {
		mustWriteFile(t, filepath.Join(root, "k8s", env, "app", "medici-deploy.yaml"), deployJobYAML("0.1.8"))
	}
	// Kind isolation: the fixture must trip dar-sync and NOTHING else, or a
	// passing test proves nothing about dar-sync's class.
	if dp, _ := darSyncCheck(root, nil); len(dp) == 0 {
		t.Fatal("fixture is stale: darSyncCheck reported no problems")
	}
	if lp := linkProblems(root, docFiles(root)); len(lp) != 0 {
		t.Fatalf("fixture does not isolate dar-sync — linkProblems also fired: %v", lp)
	}
	if rp, _ := registerRefProblems(root, docFiles(root)); len(rp) != 0 {
		t.Fatalf("fixture does not isolate dar-sync — registerRefProblems also fired: %v", rp)
	}
}

func goodrepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "STATUS.md")); !os.IsNotExist(err) {
		t.Fatal("fixture is stale: goodrepo must ship no STATUS.md")
	}
	return root
}

// --- (1) dar-sync is off-board -------------------------------------------

func TestDarSyncProblemFailsButStillWrites(t *testing.T) {
	root := goodrepo(t)
	darMismatch(t, root)

	code := 0
	stderr := captureStderr(t, func() { code = run(root, "write", nil, nil, "") })

	if code != 1 {
		t.Errorf("write with a dar-sync problem exited %d, want 1 — the run must still fail", code)
	}
	if !strings.Contains(stderr, "PROBLEM:") || !strings.Contains(stderr, "daml.yaml is 0.1.16") {
		t.Errorf("the dar-sync problem must still be reported in full; stderr was:\n%s", stderr)
	}
	// N1: the banner carries the safety argument — a written STATUS.md must not
	// read as "the run passed". Assert it rather than trusting it exists.
	if !strings.Contains(stderr, "outside the board's own sources") || !strings.Contains(stderr, "FAILS (exit 1)") {
		t.Errorf("the off-board banner must state that the run failed; stderr was:\n%s", stderr)
	}
	out, err := os.ReadFile(filepath.Join(root, "STATUS.md"))
	if err != nil {
		t.Fatalf("STATUS.md was not written despite a dar-sync-only problem: %v", err)
	}
	if !strings.Contains(string(out), "alpha") {
		t.Error("STATUS.md was written but is not a real board")
	}
}

func TestDarSyncProblemFailsButStillRecords(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/historyrepo")); err != nil {
		t.Fatal(err)
	}
	darMismatch(t, root)
	historyPath := filepath.Join(root, "docs", "streams", ".history.jsonl")

	before, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeLines := strings.Count(strings.TrimSpace(string(before)), "\n") + 1

	readme := filepath.Join(root, "docs", "streams", "hist", "README.md")
	raw, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	flipped := strings.Replace(string(raw),
		"| 01 | [First](brief-01.md) | 0 | implemented | — | — |",
		"| 01 | [First](brief-01.md) | 0 | verified | 2026-07-09 test | — |", 1)
	if flipped == string(raw) {
		t.Fatal("fixture row not found — test setup is stale")
	}
	if err := os.WriteFile(readme, []byte(flipped), 0o644); err != nil {
		t.Fatal(err)
	}
	fixedNow(t, "2026-07-09T12:00:00Z")

	if code := run(root, "record", nil, nil, ""); code != 1 {
		t.Errorf("record with a dar-sync problem exited %d, want 1 — the run must still fail", code)
	}
	after, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	afterLines := strings.Count(strings.TrimSpace(string(after)), "\n") + 1
	if afterLines != beforeLines+1 {
		t.Fatalf("history got %d lines, want %d — the append-only log must not lose the transition to a dar-sync problem", afterLines, beforeLines+1)
	}
}

func TestDarSyncProblemStillFailsLint(t *testing.T) {
	root := goodrepo(t)
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("control: clean fixture must lint clean, exited %d", code)
	}
	darMismatch(t, root)
	if code := run(root, "lint", nil, nil, ""); code != 1 {
		t.Errorf("lint with a dar-sync problem exited %d, want 1 — the PR-side gate is unchanged", code)
	}
	if _, err := os.Stat(filepath.Join(root, "STATUS.md")); !os.IsNotExist(err) {
		t.Error("lint must still never write STATUS.md")
	}
}

// TestDarSyncProblemStillFailsCheck pins the --check leg (the reviewer's
// surviving mutation 3). The board is byte-current, so there is no drift — the
// run must still exit 1 on the dar-sync problem alone, and the verdict must not
// call a dar-sync problem "drift" (N4).
func TestDarSyncProblemStillFailsCheck(t *testing.T) {
	root := goodrepo(t)
	if code := run(root, "write", nil, nil, ""); code != 0 {
		t.Fatalf("setup: clean write exited %d", code)
	}
	if code := run(root, "check", nil, nil, ""); code != 0 {
		t.Fatalf("control: check on a fresh board exited %d, want 0", code)
	}
	darMismatch(t, root)

	code := 0
	stdout := captureStdout(t, func() { code = run(root, "check", nil, nil, "") })

	if code != 1 {
		t.Errorf("check with a dar-sync problem exited %d, want 1", code)
	}
	if strings.Contains(stdout, "CHECK: DRIFT") {
		t.Errorf("STATUS.md is byte-current — a dar-sync problem must not be reported as drift (N4); stdout was:\n%s", stdout)
	}
}

// --- (2) link problems block ---------------------------------------------

// TestPhantomBriefRowStillBlocks is the falsification case. A README row can
// claim a `done` brief with a human sign-off while no brief file exists behind
// it. checkBriefFiles, verifySectionProblems and attributionProblems all
// iterate briefFilePaths(s) — a glob over files that EXIST — so the row is
// invisible to every one of them. The dead-link lint on the stream README is
// the ONLY check that catches it. If the link lint were ever reclassified
// off-board, this row would land in a committed STATUS.md and in the
// append-only history.
func TestPhantomBriefRowStillBlocks(t *testing.T) {
	root := goodrepo(t)
	readme := filepath.Join(root, "docs/streams/alpha/README.md")
	raw, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	phantom := strings.TrimRight(string(raw), "\n") +
		"\n| 07 | [Phantom work](brief-07.md) | 1 | done | 2026-07-20 someone | human:ian |\n"
	if err := os.WriteFile(readme, []byte(phantom), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs/streams/alpha/brief-07.md")); !os.IsNotExist(err) {
		t.Fatal("fixture is stale: brief-07.md must not exist")
	}

	code := 0
	stderr := captureStderr(t, func() { code = run(root, "write", nil, nil, "") })

	if code != 1 {
		t.Errorf("phantom brief row exited %d, want 1", code)
	}
	if !strings.Contains(stderr, `dead link "brief-07.md"`) {
		t.Errorf("the phantom row must be reported; stderr was:\n%s", stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "STATUS.md")); !os.IsNotExist(err) {
		t.Fatal("a phantom `done` brief must NOT reach a written STATUS.md — the link lint must stay blocking")
	}

	// The same row must not reach the append-only history either.
	histPath := filepath.Join(root, "docs", "streams", ".history.jsonl")
	if code := run(root, "record", nil, nil, ""); code != 1 {
		t.Errorf("record with a phantom brief row exited %d, want 1", code)
	}
	if _, err := os.Stat(histPath); !os.IsNotExist(err) {
		t.Error("a phantom `done` brief must NOT be appended to the append-only history")
	}
}

func TestDeadLinkAnywhereStillBlocks(t *testing.T) {
	// Both halves of docFiles(root): a board source, and outbound docs. Both
	// block — the partition is per check-input, and this check's inputs are a
	// superset of the board's sources, so it is blocking wherever it fires.
	for _, rel := range []string{
		"docs/streams/alpha/brief-01.md", // board source
		"docs/articles/outbound.md",      // not a board source
	} {
		t.Run(rel, func(t *testing.T) {
			root := goodrepo(t)
			p := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			existing, _ := os.ReadFile(p)
			body := string(existing) + "\n\nSee [nothing](./no-such-file.md).\n"
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if code := run(root, "write", nil, nil, ""); code != 1 {
				t.Errorf("dead link in %s exited %d, want 1", rel, code)
			}
			if _, err := os.Stat(filepath.Join(root, "STATUS.md")); !os.IsNotExist(err) {
				t.Errorf("a dead link in %s must still suppress the write", rel)
			}
		})
	}
}

// --- (3) register-ref problems block --------------------------------------

func TestRegisterRefProblemStillBlocks(t *testing.T) {
	root := goodrepo(t)
	// The target must EXIST, or the dead-link lint fires too and the case stops
	// isolating registerRefProblems — this is the kind-isolation the reviewer
	// flagged. brief-02.md exists, so linkProblems is satisfied; the register-ref
	// lint fires alone, because an F-NN link must point at its own per-entry file
	// under findings/ or intake/ (methodology/33).
	p := filepath.Join(root, "docs/streams/alpha/brief-01.md")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw) + "\n\nBlocked by [F-01](brief-02.md).\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	// Guard the isolation itself: exactly one problem, and it is the register-ref
	// one. If a future change makes this fixture also trip the link lint, this
	// fails loudly instead of quietly passing for the wrong reason.
	if lp := linkProblems(root, docFiles(root)); len(lp) != 0 {
		t.Fatalf("fixture no longer isolates registerRefProblems — linkProblems also fired: %v", lp)
	}
	rp, _ := registerRefProblems(root, docFiles(root))
	if len(rp) != 1 {
		t.Fatalf("want exactly 1 register-ref problem, got %d: %v", len(rp), rp)
	}

	code := 0
	stderr := captureStderr(t, func() { code = run(root, "write", nil, nil, "") })
	if code != 1 {
		t.Errorf("register-ref problem exited %d, want 1", code)
	}
	if !strings.Contains(stderr, "F-01") {
		t.Errorf("the register-ref problem must be reported; stderr was:\n%s", stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "STATUS.md")); !os.IsNotExist(err) {
		t.Error("a register-ref problem must still suppress the write — it reads the same docFiles superset")
	}
}

// --- (4) board-source problems block, as before ---------------------------

func TestBlockingProblemStillSuppressesGeneration(t *testing.T) {
	root := goodrepo(t)
	readme := filepath.Join(root, "docs/streams/alpha/README.md")
	b, _ := os.ReadFile(readme)
	if err := os.WriteFile(readme, []byte(strings.Replace(string(b), "status: active", "status: bogus", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := run(root, "write", nil, nil, ""); code != 1 {
		t.Errorf("write with a blocking problem exited %d, want 1", code)
	}
	if _, err := os.Stat(filepath.Join(root, "STATUS.md")); !os.IsNotExist(err) {
		t.Error("a blocking problem must still suppress the STATUS.md write — the board's own sources are suspect")
	}
}
