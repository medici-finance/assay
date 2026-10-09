package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// recordLines returns the dispatch-record lines under the fixture HOME, parsed.
func recordLines(t *testing.T, home string) []deskkit.DispatchRecord {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(home, ".config", "assay", deskkit.DispatchRecordsFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("reading the dispatch record store: %v", err)
	}
	var out []deskkit.DispatchRecord
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if l == "" {
			continue
		}
		var r deskkit.DispatchRecord
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("record line does not parse: %v\n%s", err, l)
		}
		out = append(out, r)
	}
	return out
}

// plantRecordBrief writes a brief whose frontmatter carries the three fields the record reads.
func plantRecordBrief(t *testing.T, root string) string {
	t.Helper()
	body := "---\nbrief: example-stream/28\ntitle: fixture\neffort: M\ngate: model\nexec-tier: strong\n---\n\n# fixture brief\n"
	if err := os.WriteFile(filepath.Join(root, "b28.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return "b28.md"
}

// fixEntropy pins the dispatch_ref clock to one instant and the entropy to a reader that yields
// different bytes on every read (a counter), so two mints in the SAME second still differ and no
// assertion depends on wall-clock timing.
func fixEntropy(t *testing.T) {
	t.Helper()
	oldClock, oldEnt := dispatchClock, dispatchEntropy
	at := time.Date(2026, 10, 6, 14, 15, 2, 0, time.UTC)
	dispatchClock = func() time.Time { return at }
	dispatchEntropy = &countingReader{}
	t.Cleanup(func() { dispatchClock, dispatchEntropy = oldClock, oldEnt })
}

type countingReader struct{ n byte }

func (c *countingReader) Read(p []byte) (int, error) {
	for i := range p {
		c.n++
		p[i] = c.n
	}
	return len(p), nil
}

type brokenEntropy struct{}

func (brokenEntropy) Read([]byte) (int, error) { return 0, errors.New("entropy unavailable") }

// acquiredKey returns the key of the claim tool's `acquire` call — the key actually acquired.
func (s *stub) acquiredKey() string {
	for _, c := range s.calls {
		if len(c) > 2 && strings.HasSuffix(c[0], "dispatch-claim.sh") && c[1] == "acquire" {
			return c[2]
		}
	}
	return ""
}

// TestDispatchRecordWrittenAtModelStamp is Verify row 3.
func TestDispatchRecordWrittenAtModelStamp(t *testing.T) {
	t.Run("real dispatch writes one line", func(t *testing.T) {
		s := &stub{}
		home, root := s.install(t)
		plantScripts(t, root)
		fixEntropy(t)
		s.replies = happyReplies(t.TempDir())
		brief := plantRecordBrief(t, root)
		pf := filepath.Join(t.TempDir(), "p.md")
		if rc := run([]string{"example-stream/28", "--root", root, "--brief", brief, "--tier", "strong",
			"--prompt-file", pf}); rc != deskkit.ExitOK {
			t.Fatalf("dispatch rc = %d, want 0", rc)
		}
		recs := recordLines(t, home)
		if len(recs) != 1 {
			t.Fatalf("got %d record lines, want exactly 1: %+v", len(recs), recs)
		}
		r := recs[0]
		key := s.acquiredKey()
		if key == "" || r.ClaimKey != key {
			t.Errorf("claim_key %q, want the key passed to acquire %q", r.ClaimKey, key)
		}
		if r.Event != deskkit.DispatchEventDispatched || r.DispatchRef == nil || !strings.HasPrefix(*r.DispatchRef, key+"@") {
			t.Errorf("event %q dispatch_ref %v: want a dispatched line whose ref starts with %q", r.Event, r.DispatchRef, key+"@")
		}
		if r.Brief == nil || *r.Brief != "example-stream/28" {
			t.Errorf("brief = %v, want the frontmatter id example-stream/28", r.Brief)
		}
		if r.BriefExec == nil || *r.BriefExec != "strong" || r.BriefEffort == nil || *r.BriefEffort != "M" {
			t.Errorf("brief_exec_tier %v / brief_effort %v, want strong / M", r.BriefExec, r.BriefEffort)
		}
		if r.Tier == nil || *r.Tier != "strong" || r.ModelStamp == nil || *r.ModelStamp != deskkit.ModelStampSkipped {
			t.Errorf("tier %v / model_stamp %v, want strong / skipped (no --model)", r.Tier, r.ModelStamp)
		}
		if r.Repo != allowedRepo || r.AttemptLocal == nil || *r.AttemptLocal != 1 || r.SessionTag != "deskdispatch-test" {
			t.Errorf("repo %q attempt_local %v session_tag %q", r.Repo, r.AttemptLocal, r.SessionTag)
		}
	})
	t.Run("dry run writes nothing", func(t *testing.T) {
		s := &stub{}
		home, root := s.install(t)
		plantScripts(t, root)
		brief := plantRecordBrief(t, root)
		_ = captureStdout(t, func() {
			if rc := run([]string{"example-stream/28", "--root", root, "--repo", allowedRepo, "--brief", brief,
				"--dry-run", "--prompt-file", filepath.Join(t.TempDir(), "p.md")}); rc != deskkit.ExitOK {
				t.Fatalf("dry-run rc = %d, want 0", rc)
			}
		})
		if recs := recordLines(t, home); len(recs) != 0 {
			t.Fatalf("--dry-run wrote %d record lines: %+v", len(recs), recs)
		}
		if s.ran("assay.dispatchRef") {
			t.Error("--dry-run recorded a dispatch ref in a worktree")
		}
	})
}

// realGitConfig routes the worktree-config writes to REAL git, so the value the agent's worktree
// carries is read back from the worktree itself rather than inferred from an argv. Every other
// child stays stubbed. It must be installed AFTER stub.install.
func realGitConfig(t *testing.T, s *stub) {
	t.Helper()
	inner := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		j := strings.Join(args, " ")
		if name == "git" && len(args) > 0 && args[0] == "config" &&
			(strings.Contains(j, "--worktree") || strings.Contains(j, "extensions.worktreeConfig")) {
			s.calls = append(s.calls, append([]string{name}, args...))
			return exec.Command("git", args...)
		}
		return inner(name, args...)
	}
	t.Cleanup(func() { execCommand = inner })
}

// gitHome makes a real git repository for the stubbed `deskwt add` to name as the agent's home.
func gitHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	return dir
}

func worktreeRef(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "config", "--worktree", "assay.dispatchRef").Output()
	if err != nil {
		t.Fatalf("reading assay.dispatchRef from %s: %v", dir, err)
	}
	return strings.TrimSpace(string(out))
}

// claimLedger makes the stubbed claim tool STATEFUL: an acquire of a held key exits 5 (the
// claim tool's live-holder refusal), so a second dispatch of the item can only succeed after
// the first claim was released.
type claimLedger struct{ held map[string]bool }

func installClaimLedger(t *testing.T, s *stub) *claimLedger {
	t.Helper()
	l := &claimLedger{held: map[string]bool{}}
	inner := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if strings.HasSuffix(name, "dispatch-claim.sh") && len(args) > 1 && args[0] == "acquire" {
			s.calls = append(s.calls, append([]string{name}, args...))
			if l.held[args[1]] {
				return exec.Command("/bin/sh", "-c", "echo 'DEDUP: held' 1>&2; exit 5")
			}
			l.held[args[1]] = true
			return exec.Command("/bin/sh", "-c", "exit 0")
		}
		return inner(name, args...)
	}
	t.Cleanup(func() { execCommand = inner })
	return l
}

// TestDispatchRefRecordedInWorktree is Verify row 4.
func TestDispatchRefRecordedInWorktree(t *testing.T) {
	t.Run("two dispatches two refs", func(t *testing.T) {
		s := &stub{}
		home, root := s.install(t)
		plantScripts(t, root)
		fixEntropy(t) // ONE instant for both dispatches: the same-second case
		realGitConfig(t, s)
		ledger := installClaimLedger(t, s)

		const key = "ref-key--04"
		var refs []string
		for i := 0; i < 2; i++ {
			wt := gitHome(t)
			s.replies = happyReplies(wt)
			if rc := run([]string{key, "--root", root, "--prompt-file", filepath.Join(t.TempDir(), "p.md")}); rc != deskkit.ExitOK {
				t.Fatalf("dispatch %d rc = %d, want 0", i+1, rc)
			}
			recs := recordLines(t, home)
			if len(recs) != i+1 {
				t.Fatalf("after dispatch %d the store holds %d lines", i+1, len(recs))
			}
			r := recs[i]
			if r.DispatchRef == nil {
				t.Fatalf("dispatch %d recorded a null dispatch_ref", i+1)
			}
			if got := worktreeRef(t, wt); got != *r.DispatchRef {
				t.Errorf("dispatch %d: worktree assay.dispatchRef %q != record dispatch_ref %q", i+1, got, *r.DispatchRef)
			}
			if r.ClaimKey != key || r.AttemptLocal == nil || *r.AttemptLocal != i+1 {
				t.Errorf("dispatch %d: claim_key %q attempt_local %v, want %q / %d", i+1, r.ClaimKey, r.AttemptLocal, key, i+1)
			}
			refs = append(refs, *r.DispatchRef)
			// The agent finishes and releases the claim before the item is dispatched again.
			delete(ledger.held, key)
		}
		if refs[0] == refs[1] {
			t.Fatalf("two dispatches of one item in the same second share dispatch_ref %q", refs[0])
		}
		if !strings.HasPrefix(refs[0], key+"@20261006T141502Z.") || !strings.HasPrefix(refs[1], key+"@20261006T141502Z.") {
			t.Errorf("refs %q / %q are not <key>@<the fixed instant>.<nonce>", refs[0], refs[1])
		}
	})
	t.Run("failing entropy leaves null", func(t *testing.T) {
		s := &stub{}
		home, root := s.install(t)
		plantScripts(t, root)
		oldEnt := dispatchEntropy
		dispatchEntropy = brokenEntropy{}
		t.Cleanup(func() { dispatchEntropy = oldEnt })
		s.replies = happyReplies(t.TempDir())
		var rc int
		errOut := captureStderr(t, func() {
			rc = run([]string{"ref-key--05", "--root", root, "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
		})
		if rc != deskkit.ExitOK {
			t.Fatalf("a failed mint failed the dispatch: rc = %d\n%s", rc, errOut)
		}
		if !strings.Contains(errOut, "deskdispatch: WARNING: could not mint dispatch_ref") {
			t.Errorf("no mint WARNING on stderr:\n%s", errOut)
		}
		recs := recordLines(t, home)
		if len(recs) != 1 || recs[0].DispatchRef != nil {
			t.Fatalf("want one record with a null dispatch_ref, got %+v", recs)
		}
		if s.ran("assay.dispatchRef") {
			t.Error("a dispatch with no minted ref still wrote assay.dispatchRef")
		}
	})
}

// TestDispatchRecordFailureNeverFailsDispatch is Verify row 6: an unwritable record store prints
// the WARNING, the dispatch still exits 0 and emits its prompt, and the claim is NOT released.
func TestDispatchRecordFailureNeverFailsDispatch(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantScripts(t, root)
	fixEntropy(t)
	// The store path is a DIRECTORY, so the append cannot happen.
	if err := os.MkdirAll(filepath.Join(home, ".config", "assay", deskkit.DispatchRecordsFile), 0o700); err != nil {
		t.Fatal(err)
	}
	s.replies = happyReplies(t.TempDir())
	pf := filepath.Join(t.TempDir(), "p.md")
	var rc int
	errOut := captureStderr(t, func() {
		rc = run([]string{"nonfatal--06", "--root", root, "--prompt-file", pf})
	})
	if rc != deskkit.ExitOK {
		t.Fatalf("an unwritable record store failed the dispatch: rc = %d\n%s", rc, errOut)
	}
	if !strings.Contains(errOut, "deskdispatch: WARNING: could not write dispatch record: ") {
		t.Errorf("no record-write WARNING on stderr:\n%s", errOut)
	}
	if _, err := os.Stat(pf); err != nil {
		t.Errorf("the prompt was not emitted: %v", err)
	}
	if s.ran("dispatch-claim.sh release") {
		t.Error("a record-write failure released the claim of a dispatch that stands")
	}
}

// TestDispatchAuditCarriesRef: the prepared-dispatch audit line names the dispatch_ref (the audit
// log is local 0600 state, so the visibility rule allows it in clear).
func TestDispatchAuditCarriesRef(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantScripts(t, root)
	fixEntropy(t)
	s.replies = happyReplies(t.TempDir())
	if rc := run([]string{"audit--07", "--root", root, "--prompt-file", filepath.Join(t.TempDir(), "p.md")}); rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d", rc)
	}
	recs := recordLines(t, home)
	if len(recs) != 1 || recs[0].DispatchRef == nil {
		t.Fatalf("records: %+v", recs)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".config", "assay", "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("dispatch_ref="+*recs[0].DispatchRef)) {
		t.Errorf("the audit line does not carry dispatch_ref=%s:\n%s", *recs[0].DispatchRef, raw)
	}
}
