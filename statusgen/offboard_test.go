package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Problem classification.
//
// The off-board problem class — the single NON-BLOCKING check — and its
// property-(1) tests are build-tagged out of this shipped file (they exercise a
// check that is a no-op in the default build). This file pins the BLOCKING
// properties, which stand independently:
//
//  2. a link problem DOES suppress generation — docFiles(root) includes every
//     stream README and brief file, and the dead-link lint is the only check
//     that catches a README row with no brief file behind it;
//  3. a register-ref problem likewise blocks (same docFiles superset);
//  4. a board-source problem blocks, as before.
//
// The tests pin the partition per KIND, not just the mechanism — an earlier
// revision planted the same dead link in every case, which pinned "some problem
// is non-blocking" and would have stayed green while a kind silently changed
// class.

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
		"\n| 07 | [Phantom work](brief-07.md) | 1 | done | 2026-07-20 someone | human:alex |\n"
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
	// under findings/ or intake/.
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
