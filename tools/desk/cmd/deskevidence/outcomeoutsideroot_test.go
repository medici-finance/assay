package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: F-outcome-record-inside-home
//
// (A verifier outcome record written inside the home refuses verifier admission.)
//
// TestOutcomeRecordOutsideRoot: with --root (the verifier home), --outcome-record mirrors the
// --brief-path fragment guard. A relative path, an absolute path inside the home, and an
// absolute path that reaches the home through a symlink alias are all refused with the
// outside-the-home rule BEFORE any mint, forge resolution, admission or forge read; an
// absolute path outside the home proceeds to admission and lands.
func TestOutcomeRecordOutsideRoot(t *testing.T) {
	const rec = `{"brief":"x/01","ts":"2026-10-06T00:00:00Z","verdict":"verify-fail","digest":"0123456789abcdef"}`

	type probe struct{ mints, forges, admissions int }
	arm := func(t *testing.T) (*fakeForge, *probe, string, *bytes.Buffer) {
		t.Helper()
		f, errBuf := setupFake(t)
		p := &probe{}
		mintTokenFn = func(string) error { p.mints++; ghToken = fakeVerifierToken; return nil }
		oldForge := forgeForFn
		forgeForFn = func(owner, name string) (deskkit.Forge, deskkit.ForgeRepo, error) {
			p.forges++
			return oldForge(owner, name)
		}
		verifierEvidenceAdmissionFn = func(_, _, target string, _ deskkit.Forge) (deskkit.VerifierReceipt, error) {
			p.admissions++
			return stubAdmittedReceipt(target), nil
		}
		home := rootWithFile(t, "docs/streams/x/brief.md", "# Brief\n")
		return f, p, home, errBuf
	}
	refusedBeforeForge := func(t *testing.T, f *fakeForge, p *probe, code int) {
		t.Helper()
		if code != deskkit.ExitRefused {
			t.Fatalf("exit = %d, want %d (refused)", code, deskkit.ExitRefused)
		}
		if p.mints != 0 || p.forges != 0 || p.admissions != 0 || len(f.hits) != 0 || f.putCalls != 0 {
			t.Fatalf("refusal came after the forge was touched: mints=%d forges=%d admissions=%d hits=%v puts=%d",
				p.mints, p.forges, p.admissions, f.hits, f.putCalls)
		}
	}

	t.Run("relative path is refused", func(t *testing.T) {
		f, p, home, errBuf := arm(t)
		writeFixtureFile(t, filepath.Join(home, "record.json"), rec)
		code := run([]string{fixtureRepo, "main", "--root", home, "--outcome-record", "record.json"})
		refusedBeforeForge(t, f, p, code)
		if !strings.Contains(errBuf.String(), "OUTSIDE --root") {
			t.Errorf("refusal does not name the outside-the-home rule: %s", errBuf)
		}
	})

	t.Run("absolute path inside the home is refused", func(t *testing.T) {
		f, p, home, errBuf := arm(t)
		record := filepath.Join(home, "docs", "streams", "verify-outcomes", "x", "01-record.json")
		writeFixtureFile(t, record, rec)
		code := run([]string{fixtureRepo, "main", "--root", home, "--outcome-record", record})
		refusedBeforeForge(t, f, p, code)
		if !strings.Contains(errBuf.String(), "OUTSIDE --root") {
			t.Errorf("refusal does not name the outside-the-home rule: %s", errBuf)
		}
	})

	t.Run("symlink alias into the home is refused", func(t *testing.T) {
		f, p, home, errBuf := arm(t)
		writeFixtureFile(t, filepath.Join(home, "record.json"), rec)
		alias := filepath.Join(t.TempDir(), "alias")
		if err := os.Symlink(home, alias); err != nil {
			if runtime.GOOS == "windows" {
				t.Skip("symlinks unavailable")
			}
			t.Fatal(err)
		}
		code := run([]string{fixtureRepo, "main", "--root", home, "--outcome-record", filepath.Join(alias, "record.json")})
		refusedBeforeForge(t, f, p, code)
		if !strings.Contains(errBuf.String(), "OUTSIDE --root") {
			t.Errorf("refusal does not name the outside-the-home rule: %s", errBuf)
		}
	})

	t.Run("absolute path outside the home proceeds to admission", func(t *testing.T) {
		f, p, home, errBuf := arm(t)
		record := filepath.Join(t.TempDir(), "outcomes", "record.json")
		writeFixtureFile(t, record, rec)
		code := run([]string{fixtureRepo, "main", "--root", home, "--outcome-record", record})
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d, want 0: %s", code, errBuf)
		}
		if p.admissions != 1 {
			t.Fatalf("admission ran %d time(s), want exactly 1", p.admissions)
		}
		if f.putCalls != 1 {
			t.Fatalf("landing made %d write(s), want 1", f.putCalls)
		}
	})
}
