package main

import (
	"bytes"
	"encoding/json"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #4947
func TestDeskCapsPreflight(t *testing.T) {
	for name, tc := range map[string]struct {
		content string
		want    int
		why     string
	}{
		"unclosed":    {"---\nschema: brief-v1\n## Evidence\n| 1 | unresolved |\n", deskkit.ExitUnverifiable, "unterminated frontmatter"},
		"unsupported": {"---\n\"reader-\\x72equirements\": {schema: \"reader-\\x72equirements-v1\", required: [{operation: acceptance, schemas: [], semantics: [future-v1]}]}\n---\n## Evidence\n", deskkit.ExitRefused, "unsupported required operation: acceptance"},
	} {
		t.Run(name, func(t *testing.T) {
			f := setupStamp(t, true)
			mintCalls := 0
			oldMint := mintTokenFn
			mintTokenFn = func(repo string) error { mintCalls++; return oldMint(repo) }
			root := rootWithFile(t, stampBrief, tc.content)
			before := len(auditEntries(t))
			if code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root}); code != tc.want {
				t.Fatalf("preflight exit = %d, want %d", code, tc.want)
			}
			if mintCalls != 0 {
				t.Errorf("startup capability refusal contacted credentials %d time(s)", mintCalls)
			}
			rows := auditEntries(t)
			if len(rows) != before+1 {
				t.Fatalf("preflight wrote %d audit rows, want exactly one", len(rows)-before)
			}
			row := rows[len(rows)-1]
			wantResult := deskkit.ResultRefused
			if tc.want == deskkit.ExitUnverifiable {
				wantResult = deskkit.ResultUnverifiable
			}
			if row.Tool != "deskevidence" || row.Verb != "commit" || row.Result != wantResult || !strings.Contains(row.Detail, "reader capabilities:") || !strings.Contains(row.Detail, stampBrief) || !strings.Contains(row.Detail, tc.why) {
				t.Fatalf("preflight refusal audit lost context: %+v", row)
			}
			if f.putCalls != 0 {
				t.Fatalf("preflight failure wrote %d time(s)", f.putCalls)
			}
		})
	}
	t.Run("diagnostic-does-not-poison-ledger", func(t *testing.T) {
		f, output := setupFake(t)
		dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
		if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(stampRoster(true)), 0600); err != nil {
			t.Fatal(err)
		}
		deskkit.ReloadConfig()
		t.Cleanup(deskkit.ReloadConfig)
		good := readStampFixture(t, "brief-01-unstamped.md")
		operation := "unknown\x1b[31m\ninjected" + strings.Repeat("x", 5*1024*1024)
		envelope, err := json.Marshal(map[string]any{"schema": "reader-requirements-v1", "required": []any{map[string]any{"operation": operation, "schemas": []string{}, "semantics": []string{}}}})
		if err != nil {
			t.Fatal(err)
		}
		raw := "---\nreader-requirements: " + string(envelope) + "\n---\n"
		root := rootWithFile(t, stampBrief, raw)
		args := []string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root}
		if code := run(args); code != deskkit.ExitRefused {
			t.Fatalf("oversized unsupported exit=%d", code)
		}
		if output.Len() > 4096 || strings.Contains(output.String(), "\x1b") || strings.Contains(output.String(), "\ninjected") {
			t.Errorf("unsafe or unbounded terminal diagnostic (%d bytes)", output.Len())
		}
		rows, loadErr := deskkit.LoadEntries()
		if loadErr != nil {
			t.Errorf("capability refusal poisoned audit: %v", loadErr)
		} else if len(rows) != 1 || rows[0].Result != deskkit.ResultRefused || len(rows[0].Detail) > 4096 || !strings.Contains(rows[0].Detail, "[truncated]") {
			t.Error("oversized refusal did not preserve one bounded audit row")
		}
		if err := os.WriteFile(filepath.Join(root, stampBrief), []byte(good), 0644); err != nil {
			t.Fatal(err)
		}
		if code := run(args); code != deskkit.ExitOK {
			t.Errorf("following ordinary landing exit=%d, want 0", code)
		}
		if f.putCalls != 1 {
			t.Errorf("following landing writes=%d, want 1", f.putCalls)
		}
	})
	for _, place := range []string{"register", "self-declared", "symlink", "inside-symlink"} {
		t.Run("named-"+place, func(t *testing.T) {
			f := setupStamp(t, true)
			good := readStampFixture(t, "brief-01-unstamped.md")
			raw := strings.Replace(good, "---\n", "---\nreader-requirements: {schema: reader-requirements-v1, required: [{operation: acceptance, schemas: [], semantics: [future-v1]}]}\n", 1)
			name := "sample"
			if place == "register" {
				name = "intake"
			}
			target := "docs/streams/" + name + "/issue-01-example.md"
			root := rootWithFile(t, target, raw)
			if place == "self-declared" {
				if err := os.WriteFile(filepath.Join(root, "docs", "streams", name, "README.md"), []byte("---\nIt is a register, not a stream.\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			want := deskkit.ExitRefused
			if place == "symlink" || place == "inside-symlink" {
				dir := filepath.Join(root, "docs", "streams", name)
				moved := filepath.Join(t.TempDir(), "records")
				if place == "inside-symlink" {
					moved = filepath.Join(root, "target-records")
				}
				if err := os.Rename(dir, moved); err != nil {
					t.Fatal(err)
				}
				linkTarget := moved
				if place == "inside-symlink" {
					linkTarget = "../../target-records"
				}
				if err := os.Symlink(linkTarget, dir); err != nil {
					t.Skip(err)
				}
				want = deskkit.ExitUnverifiable
			}
			if code := run([]string{stampPublicRepo, "main", "--evidence-file", target, "--root", root}); code != want {
				t.Errorf("named target exit=%d, want %d", code, want)
			}
			if f.putCalls != 0 {
				t.Errorf("unsupported named target landed %d writes", f.putCalls)
			}
			if !strings.Contains(lastAudit(t).Detail, "reader capabilities:") {
				t.Error("named refusal did not pass capability admission")
			}
		})
	}
	t.Run("named-receipt-root", func(t *testing.T) {
		f := setupStamp(t, true)
		good := readStampFixture(t, "brief-01-unstamped.md")
		target := "docs/streams/sample/issue-01-example.md"
		source := rootWithFile(t, target, good)
		raw := strings.Replace(good, "---\n", "---\nreader-requirements: {schema: reader-requirements-v1, required: [{operation: acceptance, schemas: [], semantics: [future-v1]}]}\n", 1)
		admitted := rootWithFile(t, target, raw)
		verifierEvidenceAdmissionFn = func(_, _, target string, _ deskkit.Forge) (deskkit.VerifierReceipt, error) {
			r := stubAdmittedReceipt(target)
			r.Binding.Version = 2
			r.BriefRoot = admitted
			return r, nil
		}
		t.Chdir(source)
		if code := run([]string{stampPublicRepo, "main", "--evidence-file", target}); code != deskkit.ExitRefused {
			t.Errorf("admitted target exit=%d, want 5", code)
		}
		detail := lastAudit(t).Detail
		if !strings.Contains(detail, admitted) || !strings.Contains(detail, "unsupported required operation: acceptance") {
			t.Error("the admitted read root did not supply capability admission")
		}
		if f.putCalls != 0 {
			t.Error("unsupported admitted target landed")
		}
	})

	t.Run("remote-record-cannot-lose-requirements", func(t *testing.T) {
		f := setupStamp(t, true)
		good := readStampFixture(t, "brief-01-unstamped.md")
		bad := strings.Replace(good, "---\n", "---\nreader-requirements: {schema: reader-requirements-v1, required: [{operation: acceptance, schemas: [], semantics: [future-v1]}]}\n", 1)
		root := rootWithFile(t, stampBrief, good)
		f.setFile(stampBrief, bad)
		if code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root}); code != deskkit.ExitRefused {
			t.Errorf("remote mandatory record exit=%d", code)
		}
		if f.putCalls != 0 {
			t.Error("landing removed an unsupported remote requirement")
		}
		if !strings.Contains(lastAudit(t).Detail, "unsupported required operation: acceptance") {
			t.Error("remote record was not admitted before interpretation")
		}
	})
	t.Run("consumer-after-upper-check", func(t *testing.T) {
		f := setupStamp(t, true)
		good := readStampFixture(t, "brief-01-unstamped.md")
		raw := strings.Replace(good, "---\n", "---\nreader-requirements: {schema: reader-requirements-v1, required: [{operation: syntax, schemas: [], semantics: []}]}\n", 1)
		root := rootWithFile(t, stampBrief, raw)
		changed := false
		lintDiffFn = func(_, _ string, content []byte) ([]string, error) {
			pos := bytes.Index(content, []byte("operation: syntax"))
			if pos >= 0 {
				copy(content[pos:], []byte("operation: future"))
				changed = true
			}
			return nil, nil
		}
		if code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root}); code != deskkit.ExitRefused {
			t.Errorf("consuming boundary exit=%d", code)
		}
		if !changed {
			t.Fatal("upper admission did not reach the deliberate late mutation")
		}
		if f.putCalls != 0 {
			t.Error("unsupported final content reached a write")
		}
		if !strings.Contains(lastAudit(t).Detail, "unsupported required operation: future") {
			t.Error("final consuming boundary did not recheck its actual bytes")
		}
	})
	t.Run("missing-admitted-root", func(t *testing.T) {
		f := setupStamp(t, true)
		good := readStampFixture(t, "brief-01-unstamped.md")
		root := rootWithFile(t, stampBrief, good)
		verifierEvidenceAdmissionFn = func(_, _, target string, _ deskkit.Forge) (deskkit.VerifierReceipt, error) {
			r := stubAdmittedReceipt(target)
			r.Binding.Version = 2
			return r, nil
		}
		t.Chdir(root)
		if code := run([]string{stampPublicRepo, "main", "--evidence-file", stampBrief, "--root", root}); code != deskkit.ExitUnverifiable {
			t.Errorf("missing admitted root exit=%d", code)
		}
		if f.putCalls != 0 {
			t.Error("unknown admitted root landed")
		}
	})
	t.Run("audit-append-failure-is-visible", func(t *testing.T) {
		f, output := setupFake(t)
		root := rootWithFile(t, stampBrief, "---\nreader-requirements: {schema: reader-requirements-v1, required: [{operation: future, schemas: [], semantics: []}]}\n---\n")
		dir, err := deskkit.StateDir()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "audit.jsonl"), 0700); err != nil {
			t.Fatal(err)
		}
		if code := run([]string{"example-org/tracker", "main", "--evidence-file", stampBrief, "--root", root}); code != deskkit.ExitRefused {
			t.Errorf("unwritable audit refusal exit=%d", code)
		}
		if f.putCalls != 0 {
			t.Error("unwritable audit admitted a write")
		}
		if !strings.Contains(output.String(), "could-not-check audit append") {
			t.Error("failed audit append was silent")
		}
	})

}
