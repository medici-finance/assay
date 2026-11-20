package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/contexthandoff"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// pickup_test.go — the context pickup admission (statusgen/21) on the real dispatch path:
// the receipt is rechecked against the snapshot before the claim, a compatible change is
// admitted and carried into both implementer kits, and a conflict, an unavailable input or
// an unsupported schema stops the dispatch with zero child processes (so no claim exists).

// pickupJourney is statusgen's shipped context fixture, the one its recheck tests use.
const pickupJourney = "../../../../statusgen/testdata/contexthandoff/journeys/base"

// pickupFixture copies the journey snapshot and writes the receipt discovery emits for it.
func pickupFixture(t *testing.T) (snap, receipt string) {
	t.Helper()
	snap = filepath.Join(t.TempDir(), "snapshot")
	err := filepath.WalkDir(pickupJourney, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(pickupJourney, p)
		dst := filepath.Join(snap, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err := contexthandoff.LoadSnapshot(os.DirFS(snap), contexthandoff.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	h, err := contexthandoff.Discover(s)
	if err != nil {
		t.Fatal(err)
	}
	receipt = filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(receipt, h.Canonical(), 0o600); err != nil {
		t.Fatal(err)
	}
	return snap, receipt
}

func pickupWrite(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// pickupDispatch runs one dispatch with the receipt and snapshot; it returns the stub, the
// error and the emitted prompt ("" when none was written).
func pickupDispatch(t *testing.T, kit string, extra ...string) (*stub, error, string) {
	t.Helper()
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
	pf := filepath.Join(t.TempDir(), "p.md")
	args := append([]string{"example-stream--07", "--root", root, "--repo", allowedRepo,
		"--kit", kit, "--prompt-file", pf, "--quiet"}, extra...)
	err := cmdDispatch(args)
	prompt := ""
	if b, rerr := os.ReadFile(emittedPathFor(pf)); rerr == nil {
		prompt = string(b)
	}
	return s, err, prompt
}

// regression: statusgen/21 — the recheck runs before launch and gates it
func TestPickupAdmission(t *testing.T) {
	type tc struct {
		name  string
		edit  func(t *testing.T, snap, receipt string)
		admit bool
		want  []string // fragments of the prompt (admit) or the refusal (stop)
	}
	cases := []tc{
		{"current", nil, true, []string{"## Context pickup receipt", "none: the receipt is current", "scope: " + contexthandoff.ArchitectureOnly}},
		{"unrelated edit", func(t *testing.T, snap, _ string) {
			pickupWrite(t, snap, "sources/repo-a/docs/streams/alpha/brief-05-audit-log-rotation.md",
				"# Brief 05 — Audit log rotation\n\n## Task\n\nRotate the audit log daily.\n")
		}, true, []string{"## Context pickup receipt", "**preserved:** file:repo-a:docs/streams/alpha/brief-05-audit-log-rotation.md"}},
		{"material conflict", func(t *testing.T, snap, _ string) {
			pickupWrite(t, snap, "sources/guides/docs/engineering/errors.md", "# Errors\n\nWrap every error with its input.\n")
		}, false, []string{pickupHeldMarker, "must be reconciled", "guideline:guides:docs/engineering/errors.md"}},
		{"unavailable input", func(t *testing.T, snap, _ string) {
			if err := os.RemoveAll(filepath.Join(snap, "sources/guides")); err != nil {
				t.Fatal(err)
			}
		}, false, []string{pickupHeldMarker, "cannot be confirmed current", "source:guides"}},
		{"missing snapshot", func(t *testing.T, snap, _ string) {
			if err := os.RemoveAll(snap); err != nil {
				t.Fatal(err)
			}
		}, false, []string{pickupHeldMarker, "could-not-check"}},
		{"unsupported schema", func(t *testing.T, _, receipt string) {
			b, _ := os.ReadFile(receipt)
			b = []byte(strings.Replace(string(b), `"`+contexthandoff.HandoffSchema+`"`, `"context-handoff-v9"`, 1))
			if err := os.WriteFile(receipt, b, 0o600); err != nil {
				t.Fatal(err)
			}
		}, false, []string{"not admitted unchecked", "manual bootstrap", contexthandoff.HandoffSchemaV3}},
	}
	for _, kit := range []string{"worker", "worker-objective"} {
		for _, c := range cases {
			t.Run(kit+"/"+c.name, func(t *testing.T) {
				snap, receipt := pickupFixture(t)
				if c.edit != nil {
					c.edit(t, snap, receipt)
				}
				s, err, prompt := pickupDispatch(t, kit, "--context-snapshot", snap, "--context-receipt", receipt)
				if c.admit {
					if err != nil {
						t.Fatalf("a compatible receipt was not admitted: %v", err)
					}
					if !s.ran("dispatch-claim.sh acquire") {
						t.Error("the admitted dispatch took no claim")
					}
					for _, w := range c.want {
						if !strings.Contains(prompt, w) {
							t.Errorf("the %s prompt does not carry %q", kit, w)
						}
					}
					return
				}
				if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
					t.Fatalf("the dispatch was not stopped: err=%v", err)
				}
				for _, w := range c.want {
					if !strings.Contains(err.Error(), w) {
						t.Errorf("the stop does not name %q:\n%s", w, err.Error())
					}
				}
				if len(s.calls) != 0 {
					t.Fatalf("the stop came after %d child process(es); a claim may exist: %v", len(s.calls), s.calls)
				}
			})
		}
	}
}

// Without a receipt the dispatch is unchanged: the kit's manual bootstrap applies.
// regression: statusgen/21 — a dispatch with no receipt keeps the manual bootstrap
func TestPickupAbsentIsBootstrap(t *testing.T) {
	_, err, prompt := pickupDispatch(t, "worker")
	if err != nil {
		t.Fatalf("a dispatch without a receipt failed: %v", err)
	}
	if strings.Contains(prompt, "## Context pickup receipt") || !strings.Contains(prompt, "Use the\nmanual bootstrap") {
		t.Error("a dispatch without a receipt must carry the kit's manual bootstrap and no receipt section")
	}
}

// One flag without the other, or a non-implementer kit, is refused before any child runs.
// regression: statusgen/21 — half a pickup is refused, never run unchecked
func TestPickupFlagMisuse(t *testing.T) {
	snap, receipt := pickupFixture(t)
	for name, extra := range map[string][]string{
		"snapshot only": {"--context-snapshot", snap},
		"receipt only":  {"--context-receipt", receipt},
	} {
		s, err, _ := pickupDispatch(t, "worker", extra...)
		if err == nil || len(s.calls) != 0 {
			t.Errorf("%s: err=%v calls=%v, want a refusal with no child", name, err, s.calls)
		}
	}
	if _, err := pickupAdmit(dispatchOpts{kit: "review", contextSnapshot: snap, contextReceipt: receipt}); err == nil {
		t.Error("a review dispatch admitted a context receipt")
	}
}

// regression: #6189 — the pickup receipt bound is statusgen context's input bound
func TestPickupReceiptBound(t *testing.T) {
	if pickupMaxReceipt != contexthandoff.MaxInputBytes {
		t.Fatalf("pickupMaxReceipt = %d, want the shared contexthandoff.MaxInputBytes %d", pickupMaxReceipt, contexthandoff.MaxInputBytes)
	}
	for _, tc := range []struct {
		name string
		size int64
		ok   bool
	}{
		{"just over the old 8 MiB bound", 8<<20 + 1, true},
		{"exactly the shared bound", contexthandoff.MaxInputBytes, true},
		{"one past the shared bound", contexthandoff.MaxInputBytes + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "receipt.json")
			f, err := os.Create(p)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.Truncate(tc.size); err != nil {
				t.Fatal(err)
			}
			f.Close()
			b, err := readPickupFile(p)
			if tc.ok && (err != nil || int64(len(b)) != tc.size) {
				t.Fatalf("size %d: read %d bytes, err %v; want it read whole", tc.size, len(b), err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("size %d: read without error, want a larger-than refusal", tc.size)
			}
		})
	}
}
