package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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

// sv and iv render a nullable record field for a failure message: the value, or "null".
func sv(p *string) string {
	if p == nil {
		return "null"
	}
	return strconv.Quote(*p)
}

// dump renders record lines as their JSON, so a failure message shows values, not pointers.
func dump(recs []deskkit.DispatchRecord) string {
	b, _ := json.Marshal(recs)
	return string(b)
}

func iv(p *int) string {
	if p == nil {
		return "null"
	}
	return strconv.Itoa(*p)
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
			t.Fatalf("got %d record lines, want exactly 1: %s", len(recs), dump(recs))
		}
		r := recs[0]
		key := s.acquiredKey()
		if key == "" || r.ClaimKey != key {
			t.Errorf("claim_key %q, want the key passed to acquire %q", r.ClaimKey, key)
		}
		if r.Event != deskkit.DispatchEventDispatched || r.DispatchRef == nil || !strings.HasPrefix(*r.DispatchRef, key+"@") {
			t.Errorf("event %q dispatch_ref %s: want a dispatched line whose ref starts with %q", r.Event, sv(r.DispatchRef), key+"@")
		}
		if r.Brief == nil || *r.Brief != "example-stream/28" {
			t.Errorf("brief = %s, want the frontmatter id example-stream/28", sv(r.Brief))
		}
		if r.BriefExec == nil || *r.BriefExec != "strong" || r.BriefEffort == nil || *r.BriefEffort != "M" {
			t.Errorf("brief_exec_tier %s / brief_effort %s, want strong / M", sv(r.BriefExec), sv(r.BriefEffort))
		}
		if r.Tier == nil || *r.Tier != "strong" || r.ModelStamp == nil || *r.ModelStamp != deskkit.ModelStampSkipped {
			t.Errorf("tier %s / model_stamp %s, want strong / skipped (no --model)", sv(r.Tier), sv(r.ModelStamp))
		}
		if r.Repo != allowedRepo || r.AttemptLocal == nil || *r.AttemptLocal != 1 || r.SessionTag != "deskdispatch-test" {
			t.Errorf("repo %q attempt_local %s session_tag %q", r.Repo, iv(r.AttemptLocal), r.SessionTag)
		}
	})
	// A tier spelling --tier accepts (validTier is case-insensitive and trimmed) records the
	// canonical token; it never makes the validator refuse the whole line.
	for _, tc := range []struct{ in, want string }{{"Strong", "strong"}, {"ANY", "any"}, {" strong", "strong"}} {
		t.Run("tier "+strings.TrimSpace(tc.in)+" is canonical", func(t *testing.T) {
			s := &stub{}
			home, root := s.install(t)
			plantScripts(t, root)
			fixEntropy(t)
			s.replies = happyReplies(t.TempDir())
			var rc int
			pf := filepath.Join(t.TempDir(), "p.md")
			errOut := captureStderr(t, func() {
				rc = run([]string{"tier-case--03", "--root", root, "--tier", tc.in, "--prompt-file", pf})
			})
			if rc != deskkit.ExitOK {
				t.Fatalf("dispatch rc = %d, want 0\n%s", rc, errOut)
			}
			recs := recordLines(t, home)
			if len(recs) != 1 {
				t.Fatalf("--tier %q: got %d record lines, want exactly 1\n%s", tc.in, len(recs), errOut)
			}
			if r := recs[0]; r.Tier == nil || *r.Tier != tc.want {
				t.Errorf("--tier %q recorded tier %s, want %q", tc.in, sv(r.Tier), tc.want)
			}
			// The prompt reads the same canonical tier: a strong spelling carries the hand-back clause.
			prompt, err := os.ReadFile(pf)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(string(prompt), tierClause); got != (tc.want == "strong") {
				t.Errorf("--tier %q: strong-tier clause in prompt = %v, want %v", tc.in, got, tc.want == "strong")
			}
		})
	}
	t.Run("invalid UTF-8 brief id is null", func(t *testing.T) {
		s := &stub{}
		home, root := s.install(t)
		plantScripts(t, root)
		fixEntropy(t)
		s.replies = happyReplies(t.TempDir())
		body := "---\nbrief: bad-\xff-id/28\ngate: model\n---\n\n# fixture\n"
		if err := os.WriteFile(filepath.Join(root, "bad.md"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		var rc int
		errOut := captureStderr(t, func() {
			rc = run([]string{"utf8--03", "--root", root, "--brief", "bad.md",
				"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
		})
		if rc != deskkit.ExitOK {
			t.Fatalf("dispatch rc = %d, want 0\n%s", rc, errOut)
		}
		recs := recordLines(t, home)
		if len(recs) != 1 || recs[0].Brief != nil {
			t.Fatalf("want one line with a null brief, got %d lines\n%s", len(recs), errOut)
		}
	})
	// Free text reaching the record from the two sources the dispatcher does not itself narrow —
	// the brief's own `brief:` line and the DESK_SESSION environment value — is dropped (brief to
	// null, session to "unknown"); the line is still written, and the text never lands in it.
	t.Run("free text never recorded", func(t *testing.T) {
		s := &stub{}
		home, root := s.install(t)
		plantScripts(t, root)
		fixEntropy(t)
		s.replies = happyReplies(t.TempDir())
		prose := "vendor-model-9 ran this; see [notes](https://example.invalid/x) @someone ‮desrever <b>&"
		body := "---\nbrief: " + prose + "\ngate: model\n---\n\n# fixture\n"
		if err := os.WriteFile(filepath.Join(root, "prose.md"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("DESK_SESSION", "any words at all, a name included")
		var rc int
		errOut := captureStderr(t, func() {
			rc = run([]string{"prose--03", "--root", root, "--brief", "prose.md", "--kit", "Worker",
				"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
		})
		if rc != deskkit.ExitOK {
			t.Fatalf("dispatch rc = %d, want 0\n%s", rc, errOut)
		}
		recs := recordLines(t, home)
		if len(recs) != 1 {
			t.Fatalf("want exactly one line, got %d\n%s", len(recs), errOut)
		}
		r := recs[0]
		if r.Brief != nil || r.SessionTag != "unknown" {
			t.Errorf("brief %s session_tag %q: want null / \"unknown\"", sv(r.Brief), r.SessionTag)
		}
		if r.Kit == nil || *r.Kit != "worker" {
			t.Errorf("kit %s: want the canonical \"worker\" for --kit Worker", sv(r.Kit))
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
			t.Fatalf("--dry-run wrote %d record lines: %s", len(recs), dump(recs))
		}
		if s.ran("assay.dispatchRef") {
			t.Error("--dry-run recorded a dispatch ref in a worktree")
		}
	})
}

// TestTierReadersAgree: the --tier gate, the stamp label and the record's tier all come from one
// normalisation, so a spelling one accepts is never one another refuses. "\u017ftrong" (long s)
// is the case-fold trap: strings.EqualFold matches it to "strong", strings.ToLower does not.
func TestTierReadersAgree(t *testing.T) {
	for _, in := range []string{"strong", "Strong", " ANY ", "\u017ftrong", "\u212a", "cheap", "", "s trong"} {
		_, labelErr := deskkit.DispatchedTierLabel(in)
		c, ok := deskkit.CanonicalDispatchTier(in)
		if validTier(in) != (labelErr == nil) || ok != (labelErr == nil) {
			t.Errorf("tier %q: validTier=%v, label error=%v, canonical ok=%v — the readers disagree",
				in, validTier(in), labelErr, ok)
		}
		if ok && !deskkit.ValidDispatchRecordField("tier", c) {
			t.Errorf("tier %q canonicalises to %q, which the record refuses", in, c)
		}
	}
}

// TestBriefFieldsStayOnLine: an empty `brief:`, `exec-tier:` or `effort:` line reads as no value,
// never as the NEXT frontmatter line.
func TestBriefFieldsStayOnLine(t *testing.T) {
	root := t.TempDir()
	body := "---\nbrief:\nexample-stream/29\nexec-tier:\nstrong\neffort:\nM\n---\n"
	if err := os.WriteFile(filepath.Join(root, "b.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	f := readBriefRecordFields(root, "b.md")
	if f.id != nil || f.execTier != nil || f.effort != nil {
		t.Errorf("empty fields read across a line: brief %s exec-tier %s effort %s",
			sv(f.id), sv(f.execTier), sv(f.effort))
	}
}

// TestModelStampOutcome: only an OK report records `applied`; an unrecognised one never does.
func TestModelStampOutcome(t *testing.T) {
	for in, want := range map[string]string{
		"OK: applied x": deskkit.ModelStampApplied, "PENDING: apply x": deskkit.ModelStampPending,
		"SKIPPED: no --model": deskkit.ModelStampSkipped, "": deskkit.ModelStampSkipped,
		"something else": deskkit.ModelStampSkipped,
	} {
		if got := modelStampOutcome(in); got != want {
			t.Errorf("modelStampOutcome(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRecordKitsAgree: the record's closed kit set is exactly the kits this binary carries.
func TestRecordKitsAgree(t *testing.T) {
	got := append([]string(nil), kitNames()...)
	want := deskkit.DispatchKits()
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("kit vocabularies disagree: deskdispatch carries %v, the record accepts %v", got, want)
	}
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
				t.Errorf("dispatch %d: claim_key %q attempt_local %s, want %q / %d", i+1, r.ClaimKey, iv(r.AttemptLocal), key, i+1)
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
			t.Fatalf("want one record with a null dispatch_ref, got %s", dump(recs))
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
		t.Fatalf("records: %s", dump(recs))
	}
	raw, err := os.ReadFile(filepath.Join(home, ".config", "assay", "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("dispatch_ref="+*recs[0].DispatchRef)) {
		t.Errorf("the audit line does not carry dispatch_ref=%s:\n%s", *recs[0].DispatchRef, raw)
	}
}

// rosterRepos re-plants the fixture roster under home with extra allowed-repo entries, so a test
// can dispatch into a repo the shared fixture does not name.
func rosterRepos(t *testing.T, home string, extra ...string) {
	t.Helper()
	const key = "ASSAY_ALLOWED_REPOS="
	if !strings.Contains(fixtureRoster, key) {
		t.Fatal("the fixture roster has no allowed-repo line")
	}
	body := strings.Replace(fixtureRoster, key, key+strings.Join(extra, ",")+",", 1)
	if err := os.WriteFile(filepath.Join(home, ".config", "assay", "roster.env"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
}

// regression: F2-repo-grammar-drops-record
// TestRecordRepoBranchAgree holds the record's repo and branch grammars to the dispatcher's own
// acceptance, the way TestTierReadersAgree and TestRecordKitsAgree hold tier and kit: every repo
// the roster admits (explicitly or by an owner/* pattern) and every branch --branch accepts up to
// the record's 256-byte cap goes through a stubbed dispatch and must write exactly one line
// carrying that value. A branch past the cap is recorded null and the line is still written.
func TestRecordRepoBranchAgree(t *testing.T) {
	long := "example-org/" + strings.Repeat("n", 101)
	explicit := []string{"example-org/.github", "example-org/_template", "example-org/-x", "_owner/x", long,
		"Example-Org/Mixed.Case_1"}
	patterned := []string{"pattern-org/.dotfile", "pattern-org/_x", "pattern-org/-y"}
	roster := append(append([]string(nil), explicit...), "pattern-org/*")
	for _, repo := range append(append([]string{allowedRepo}, explicit...), patterned...) {
		name := repo
		if len(name) > 30 {
			name = name[:30]
		}
		t.Run("repo "+name, func(t *testing.T) {
			s := &stub{}
			home, root := s.install(t)
			rosterRepos(t, home, roster...)
			plantScripts(t, root)
			fixEntropy(t)
			s.replies = happyReplies(t.TempDir())
			var rc int
			errOut := captureStderr(t, func() {
				rc = run([]string{"agree--03", "--root", root, "--repo", repo,
					"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
			})
			if rc != deskkit.ExitOK {
				t.Fatalf("--repo %q: dispatch rc = %d, want 0\n%s", repo, rc, errOut)
			}
			recs := recordLines(t, home)
			if len(recs) != 1 || recs[0].Repo != repo {
				t.Fatalf("--repo %q: want one line with that repo, got %s\n%s", repo, dump(recs), errOut)
			}
		})
	}
	b200, b255, b300 := "f"+strings.Repeat("b", 199), "f"+strings.Repeat("b", 254), "f"+strings.Repeat("b", 299)
	for _, tc := range []struct{ name, branch, want string }{
		{"plain", "feat/x", "feat/x"},
		{"200 chars", b200, b200},
		{"255 chars", b255, b255},
		{"300 chars is null", b300, ""},
	} {
		t.Run("branch "+tc.name, func(t *testing.T) {
			s := &stub{}
			home, root := s.install(t)
			plantScripts(t, root)
			fixEntropy(t)
			s.replies = happyReplies(t.TempDir())
			var rc int
			errOut := captureStderr(t, func() {
				rc = run([]string{"agree--04", "--root", root, "--repo", allowedRepo, "--branch", tc.branch,
					"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
			})
			if rc != deskkit.ExitOK {
				t.Fatalf("--branch of %d chars: dispatch rc = %d, want 0\n%s", len(tc.branch), rc, errOut)
			}
			recs := recordLines(t, home)
			if len(recs) != 1 {
				t.Fatalf("--branch of %d chars: want one line, got %d\n%s", len(tc.branch), len(recs), errOut)
			}
			got := ""
			if recs[0].Branch != nil {
				got = *recs[0].Branch
			}
			if got != tc.want {
				t.Errorf("--branch of %d chars recorded %s, want %q", len(tc.branch), sv(recs[0].Branch), tc.want)
			}
		})
	}
}

// regression: F2-repo-grammar-drops-record (second instance of the class: frontmatter fields)
// TestOutOfSetBriefFieldsNull: an exec-tier or effort outside the record's closed set is
// recorded null and the line is still written with the brief id, never refused whole.
func TestOutOfSetBriefFieldsNull(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantScripts(t, root)
	fixEntropy(t)
	s.replies = happyReplies(t.TempDir())
	body := "---\nbrief: example-stream/28\ngate: model\nexec-tier: fast\neffort: XL\n---\n\n# fixture\n"
	if err := os.WriteFile(filepath.Join(root, "oos.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var rc int
	errOut := captureStderr(t, func() {
		rc = run([]string{"oos--03", "--root", root, "--brief", "oos.md",
			"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0\n%s", rc, errOut)
	}
	recs := recordLines(t, home)
	if len(recs) != 1 {
		t.Fatalf("want exactly one line, got %d\n%s", len(recs), errOut)
	}
	r := recs[0]
	if r.BriefExec != nil || r.BriefEffort != nil || r.Brief == nil || *r.Brief != "example-stream/28" {
		t.Errorf("brief %s exec %s effort %s: want example-stream/28 / null / null", sv(r.Brief), sv(r.BriefExec), sv(r.BriefEffort))
	}
}
