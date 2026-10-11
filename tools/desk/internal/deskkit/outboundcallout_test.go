package deskkit

// outboundcallout_test.go — the house callout of the outbound-write check (desktools-v2/11).
//
// Every stub, word and path is invented (testdata/outbound-callout/*.sh). The stubs record
// beside themselves (`ran.log`, `request.json`, `env.dump`) because the callout's environment
// is scrubbed: a stub cannot be told where to log through a variable.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	ocReason     = "example-house-rule"
	ocDiagnostic = "example-house-diagnostic"
	ocCleanBody  = "A plain note about the build, with nothing in it a sweep would object to."
)

// ocStub installs testdata/outbound-callout/<name>.sh into a fresh 0755 directory (the
// checkout's own modes depend on the umask, and the callout plumbing refuses a writable
// directory) and returns the executable's path and its directory.
func ocStub(t *testing.T, name string) (path, dir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the callout stubs are POSIX shell scripts")
	}
	src, err := os.ReadFile(filepath.Join("testdata", "outbound-callout", name+".sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir = t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(dir, name+".sh")
	if err := os.WriteFile(path, src, 0o755); err != nil {
		t.Fatal(err)
	}
	return path, dir
}

func ocRan(dir string) int {
	b, err := os.ReadFile(filepath.Join(dir, "ran.log"))
	if err != nil {
		return 0
	}
	return strings.Count(string(b), "run\n")
}

// ocConfigure plants the outbound roster of obRoster plus extra keys, an isolated audit
// directory, and a clean REQUIRED environment. It returns the audit directory.
func ocConfigure(t *testing.T, extra map[string]string) string {
	t.Helper()
	r := goldenRoster()
	r[EnvAllowedRepos] = obPublic + ":ci:public," + obPrivate + ":ci:private," + obInternal + ":ci:private"
	r[EnvHumanLoginMap] = "alex:ada"
	for k, v := range extra {
		r[k] = v
	}
	withRoster(t, r)
	t.Setenv(EnvWithheldIdentifiers, obWithheld)
	t.Setenv(EnvOutboundCalloutRequired, "")
	return setup(t)
}

// ocFileIssue files an issue through the decorator over a recording forge.
func ocFileIssue(repo, body string) (calls int, err error) {
	fake := &outboundRecordingForge{}
	_, err = OutboundChecked(fake, "worker").FileIssue(obRepo(repo), IssueInput{Title: "a neutral title", Body: body})
	return len(fake.calls), err
}

// ocStderr captures the check's stderr (refusal reasons and notices).
func ocStderr(t *testing.T) *bytes.Buffer {
	t.Helper()
	var b bytes.Buffer
	t.Cleanup(SetOutboundNoticeWriter(&b))
	SetOutboundContext(OutboundContext{Tool: "deskfile", Verb: "new"})
	return &b
}

func ocAuditText(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "audit.jsonl"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestOutboundCalloutConformance(t *testing.T) {
	t.Run("H1-key-unset-is-the-compiled-table", ocCheckKeyUnset)
	t.Run("H2-block-refuses-reason-on-stderr-only", ocCheckBlockRefuses)
	t.Run("H3-allow-cannot-clear-a-compiled-block", ocCheckAllowCannotClear)
	t.Run("H4-broken-callout-refuses", ocCheckBrokenCalloutBlocks)
	t.Run("H5-required-with-no-callout", ocCheckRequiredAbsent)
	t.Run("H6-environment-carries-no-token", ocCheckEnvironment)
	t.Run("H7-commit-message-on-the-push-path", ocCheckPushPathCommitMessage)
}

// H1 — key unset: desktools-v2/10's table, unchanged. Every refuse/pass row of that table
// is replayed through the decorator and must give the same verdict.
func ocCheckKeyUnset(t *testing.T) {
	ocConfigure(t, nil)
	for _, r := range obRows() {
		if r.id == "C9" || r.id == "C10" { // label-only and commit-only rows
			continue
		}
		stderr := ocStderr(t)
		calls, err := ocFileIssue(r.target, r.text)
		if r.refuse {
			if err == nil || !obRuleMatches(err.Error(), r.rule) {
				t.Fatalf("%s: want the compiled refusal %s with no callout configured, got %v", r.id, r.rule, err)
			}
			if strings.Contains(err.Error(), RuleHouseCallout) {
				t.Fatalf("%s: a house.callout refusal with the key unset: %v", r.id, err)
			}
			continue
		}
		if err != nil || calls != 1 {
			t.Fatalf("%s: want pass with no callout configured, got calls=%d err=%v\n%s", r.id, calls, err, stderr)
		}
	}
}

// H2 — a block refuses on every text-carrying write, with zero forge calls; the callout's
// reason is on stderr and in NO audit field, no error text and no body the forge received.
func ocCheckBlockRefuses(t *testing.T) {
	path, dir := ocStub(t, "block")
	audit := ocConfigure(t, map[string]string{EnvOutboundCallout: path})
	for _, m := range obMethodNames() {
		tc := obTextMethods[m]
		stderr := ocStderr(t)
		fake := &outboundRecordingForge{}
		err := tc.call(OutboundChecked(fake, "worker"), obRepo(obPublic), ocCleanBody)
		if err == nil || !IsRefused(err) {
			t.Fatalf("%s: want a refusal (exit 5) from the house callout, got %v", m, err)
		}
		if !strings.Contains(err.Error(), "refused: "+RuleHouseCallout+" at ") {
			t.Fatalf("%s: refusal does not name %s: %v", m, RuleHouseCallout, err)
		}
		if len(fake.calls) != 0 {
			t.Fatalf("%s: the fake forge saw %d calls after a callout block, want ZERO", m, len(fake.calls))
		}
		if !strings.Contains(stderr.String(), "refused: "+RuleHouseCallout+" at ") ||
			!strings.Contains(stderr.String(), ocReason) {
			t.Fatalf("%s: the callout's reason is not on stderr:\n%s", m, stderr)
		}
		if strings.Contains(err.Error(), ocReason) {
			t.Fatalf("%s: the reason is in the returned error, which verbs log to the audit trail: %v", m, err)
		}
	}
	if got := ocRan(dir); got < len(obMethodNames()) {
		t.Fatalf("the callout ran %d times for %d writes", got, len(obMethodNames()))
	}
	if a := ocAuditText(t, audit); strings.Contains(a, ocReason) || strings.Contains(a, ocDiagnostic) {
		t.Fatalf("the callout's words reached the audit log:\n%s", a)
	}
}

// H3 — the callout says allow; the body names the configured withheld slug on a public
// target. The compiled verdict stands and the callout was NEVER executed: only-widens is
// structural, not a property of what the callout prints.
func ocCheckAllowCannotClear(t *testing.T) {
	path, dir := ocStub(t, "allow")
	ocConfigure(t, map[string]string{EnvOutboundCallout: path})
	ocStderr(t)
	calls, err := ocFileIssue(obPublic, "context lives in the "+obWithheld+" stream")
	if err == nil || !obRuleMatches(err.Error(), RuleWithheldIdentifier) {
		t.Fatalf("want the compiled %s refusal, got %v", RuleWithheldIdentifier, err)
	}
	if calls != 0 {
		t.Fatalf("the forge saw %d calls", calls)
	}
	if n := ocRan(dir); n != 0 {
		t.Fatalf("the callout was executed %d times for a write the compiled layer had already refused", n)
	}
}

func TestCalloutAllowCannotClearCompiledBlock(t *testing.T) { ocCheckAllowCannotClear(t) }

// H4 — key set and anything wrong: refused, each failure NAMED.
func ocCheckBrokenCalloutBlocks(t *testing.T) {
	type tc struct {
		name  string
		setup func(t *testing.T) (path string, extra map[string]string)
		want  string // a substring that names WHICH failure
	}
	stub := func(name string) func(*testing.T) (string, map[string]string) {
		return func(t *testing.T) (string, map[string]string) { p, _ := ocStub(t, name); return p, nil }
	}
	cases := []tc{
		{"missing file", func(t *testing.T) (string, map[string]string) {
			return filepath.Join(t.TempDir(), "nope"), nil
		}, "cannot be read"},
		{"mode 0775", func(t *testing.T) (string, map[string]string) {
			p, _ := ocStub(t, "allow")
			if err := os.Chmod(p, 0o775); err != nil {
				t.Fatal(err)
			}
			return p, nil
		}, "group- or world-writable"},
		{"exit 3", stub("exit3"), "exit status 3"},
		{"sleeps past the deadline", func(t *testing.T) (string, map[string]string) {
			p, _ := ocStub(t, "sleep")
			return p, map[string]string{EnvOutboundCalloutTimeout: "1s"}
		}, "did not answer within 1s"},
		{"prints nothing", stub("empty"), "printed nothing"},
		{"prints 100 KiB", stub("big"), "more than 65536 bytes"},
		// The failure is NAMED, but the callout's own words are not in it: they go to stderr
		// only (TestCalloutReasonNeverReachesForgeOrAudit pins their absence).
		{"prints maybe", stub("maybe"), "neither `allow` nor `block`"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			path, extra := c.setup(t)
			if extra == nil {
				extra = map[string]string{}
			}
			extra[EnvOutboundCallout] = path
			audit := ocConfigure(t, extra)
			ocStderr(t)
			start := time.Now()
			calls, err := ocFileIssue(obPublic, ocCleanBody)
			if err == nil || !IsRefused(err) {
				t.Fatalf("a broken callout (%s) did not refuse: calls=%d err=%v", c.name, calls, err)
			}
			if calls != 0 {
				t.Fatalf("the forge saw %d calls for a write the callout could not judge", calls)
			}
			if !strings.Contains(err.Error(), RuleHouseCallout) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("the refusal does not name the failure %q: %v", c.want, err)
			}
			if d := time.Since(start); d > 10*time.Second {
				t.Fatalf("the refusal took %s — the deadline is not enforced", d)
			}
			// A broken callout is never overridable.
			SetOutboundContext(OutboundContext{Tool: "deskfile", Verb: "new", OverrideReason: "a reviewed false positive"})
			if _, err := ocFileIssue(obPublic, ocCleanBody); err == nil || !strings.Contains(err.Error(), RuleHouseCallout) {
				t.Fatalf("the override took a broken-callout refusal through: %v", err)
			}
			if rows := obAuditRows(t, audit); len(rows) != 0 {
				t.Fatalf("a refused write left %d override rows", len(rows))
			}
		})
	}
}

func TestBrokenCalloutBlocks(t *testing.T) { ocCheckBrokenCalloutBlocks(t) }

// H5 — REQUIRED=public with no callout: public and unknown targets refuse, private passes.
func ocCheckRequiredAbsent(t *testing.T) {
	for name, set := range map[string]func(t *testing.T){
		"from the roster": func(t *testing.T) { ocConfigure(t, map[string]string{EnvOutboundCalloutRequired: "public"}) },
		"from the environment": func(t *testing.T) {
			ocConfigure(t, nil)
			t.Setenv(EnvOutboundCalloutRequired, "public")
		},
		// A roster that fails validation collapses to unconfigured and takes the REQUIRED key
		// with it; the environment still carries the requirement.
		"environment survives a collapsed roster": func(t *testing.T) {
			ocConfigure(t, map[string]string{"ASSAY_NOT_A_REAL_KEY": "x"})
			t.Setenv(EnvOutboundCalloutRequired, "public")
		},
	} {
		set := set
		t.Run(name, func(t *testing.T) {
			set(t)
			ocStderr(t)
			for _, target := range []string{obPublic, obUnknown} {
				calls, err := ocFileIssue(target, ocCleanBody)
				if err == nil || !IsRefused(err) || !strings.Contains(err.Error(), RuleHouseCallout) ||
					!strings.Contains(err.Error(), EnvOutboundCalloutRequired) {
					t.Fatalf("REQUIRED with no callout let a write to %s through (calls=%d, err=%v)", target, calls, err)
				}
				if calls != 0 {
					t.Fatalf("the forge saw %d calls", calls)
				}
			}
			if name == "environment survives a collapsed roster" {
				return // the roster is gone: every target reads as unknown
			}
			if calls, err := ocFileIssue(obPrivate, ocCleanBody); err != nil || calls != 1 {
				t.Fatalf("REQUIRED refused a PRIVATE target: calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestRequiredCalloutAbsentRefusesPublicOnly(t *testing.T) { ocCheckRequiredAbsent(t) }

// H6 — the callout's environment is exactly PATH, HOME, TMPDIR, LANG while the caller holds
// a token variable.
func ocCheckEnvironment(t *testing.T) {
	path, dir := ocStub(t, "env")
	ocConfigure(t, map[string]string{EnvOutboundCallout: path})
	const token = "example-minted-token-value"
	t.Setenv("GH_TOKEN", token)
	t.Setenv("GITHUB_TOKEN", token)
	t.Setenv("ASSAY_EXAMPLE_SECRET", token)
	t.Setenv("LANG", "C")
	ocStderr(t)
	if _, err := ocFileIssue(obPublic, ocCleanBody); err != nil {
		t.Fatalf("the env stub allows: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "env.dump"))
	if err != nil {
		t.Fatalf("the stub dumped no environment: %v", err)
	}
	dump := string(b)
	if strings.Contains(dump, token) || strings.Contains(dump, "TOKEN") || strings.Contains(dump, "ASSAY_") {
		t.Fatalf("the callout's environment carries a caller variable:\n%s", dump)
	}
	allowed := map[string]bool{"PATH": true, "HOME": true, "TMPDIR": true, "LANG": true,
		// set by the shell the stub runs under, not inherited
		"PWD": true, "SHLVL": true, "_": true, "OLDPWD": true}
	for _, ln := range strings.Split(strings.TrimSpace(dump), "\n") {
		k, _, _ := strings.Cut(ln, "=")
		if !allowed[k] {
			t.Fatalf("unexpected variable %q in the callout's environment:\n%s", k, dump)
		}
	}
	if !strings.Contains(dump, "LANG=C") {
		t.Fatalf("LANG was not passed through:\n%s", dump)
	}
}

func TestOutboundCalloutEnvironmentCarriesNoToken(t *testing.T) { ocCheckEnvironment(t) }

// H7 — the push path puts EVERY surface to the callout: the branch name (kind `ref`), each
// commit message (kind `commit`) and the added lines (kind `file`). The stub blocks ONE kind
// and allows the rest, so a refusal naming that kind proves that kind was asked: a stub that
// blocked unconditionally would be refused at the ref write, before any commit message or
// file was reached, and would prove nothing about either. The same range with no callout
// configured passes, so each refusal is the callout's.
func ocCheckPushPathCommitMessage(t *testing.T) {
	push := newObPushRepo(t)
	row := obRow{id: "H7", text: "tidy the build script", target: obPublic}
	ocConfigure(t, nil)
	ocStderr(t)
	b := push.branch(row, "push-commit")
	run := func() error {
		return OutboundCheckPush(OutboundPush{Dir: push.dir, Repo: obPublic, Base: "main", Head: b, Branch: b, Role: "worker"})
	}
	if err := run(); err != nil {
		t.Fatalf("control: a clean commit message with no callout must pass: %v", err)
	}
	for _, kind := range []string{OutboundKindRef, OutboundKindCommit, OutboundKindFile} {
		kind := kind
		t.Run(kind, func(t *testing.T) {
			path, dir := ocStub(t, "block-kind")
			if err := os.WriteFile(filepath.Join(dir, "kind"), []byte(kind+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			ocConfigure(t, map[string]string{EnvOutboundCallout: path})
			stderr := ocStderr(t)
			err := run()
			if err == nil || !IsRefused(err) || !strings.Contains(err.Error(), "refused: "+RuleHouseCallout+" at ") {
				t.Fatalf("a callout blocking only kind %q did not refuse the push: %v", kind, err)
			}
			// The refusal names the kind of write it refused: the one the stub blocks, and no
			// other — a refusal at an earlier surface would name that surface's kind.
			if !strings.Contains(err.Error(), "("+kind+" write to ") {
				t.Fatalf("the push was refused, but not on the %s write the callout blocked: %v", kind, err)
			}
			if !strings.Contains(stderr.String(), ocReason) {
				t.Fatalf("the reason is not on stderr:\n%s", stderr)
			}
			asked, _ := os.ReadFile(filepath.Join(dir, "asked.log"))
			if got := strings.Fields(string(asked)); len(got) == 0 || got[len(got)-1] != kind {
				t.Fatalf("the callout was asked about %q; want the last request to be the blocked %q", got, kind)
			}
		})
	}
}

// ocSay installs the replay stub with the given stdout, stderr and exit status.
func ocSay(t *testing.T, stdout, stderr string, code int) (path, dir string) {
	t.Helper()
	path, dir = ocStub(t, "say")
	for name, body := range map[string]string{"say.out": stdout, "say.err": stderr} {
		if body == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if code != 0 {
		if err := os.WriteFile(filepath.Join(dir, "say.code"), []byte(strconv.Itoa(code)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path, dir
}

// ocLeakCases is every way a callout can answer, each carrying the callout's words in what
// it prints: a well-formed block, answers outside the vocabulary (a first word that only
// LOOKS like `block`, an upper-case verdict, another verb), a non-zero exit, empty output and
// an oversized answer. Every one refuses, and in NONE may the callout's words reach anything
// but stderr. onStderr says whether the words must be ON stderr (the oversized answer is
// never printed back at all).
func ocLeakCases() []struct {
	name, out, err string
	code           int
	onStderr       bool
} {
	big := strings.Repeat(ocReason+" ", (70<<10)/len(ocReason+" "))
	return []struct {
		name, out, err string
		code           int
		onStderr       bool
	}{
		{"well-formed block", "block " + ocReason, ocDiagnostic, 0, true},
		{"block with a colon", "block: " + ocReason, ocDiagnostic, 0, true},
		{"BLOCKED", "BLOCKED " + ocReason, ocDiagnostic, 0, true},
		{"upper-case ALLOW", "ALLOW " + ocReason, ocDiagnostic, 0, true},
		{"deny", "deny " + ocReason, ocDiagnostic, 0, true},
		{"words on a second line", "maybe\n" + ocReason, ocDiagnostic, 0, true},
		{"terminal escape in the answer", "neither \x1b[31m" + ocReason + "\x1b[0m", ocDiagnostic, 0, true},
		{"non-zero exit", "allow " + ocReason, ocDiagnostic, 3, false},
		{"empty answer", "", ocDiagnostic + " " + ocReason, 0, true},
		{"oversized answer", big, ocDiagnostic, 0, false},
	}
}

// TestCalloutReasonNeverReachesForgeOrAudit — the CLASS guard for the callout's own words.
// For every answer a callout can give (ocLeakCases), the recording forge, the audit log and
// the returned error (which every verb writes into its own audit row) are searched for the
// words the callout printed; stderr is the one place they may appear.
func TestCalloutReasonNeverReachesForgeOrAudit(t *testing.T) {
	for _, c := range ocLeakCases() {
		c := c
		t.Run(c.name, func(t *testing.T) {
			path, _ := ocSay(t, c.out, c.err, c.code)
			audit := ocConfigure(t, map[string]string{EnvOutboundCallout: path})
			stderr := ocStderr(t)
			fake := &outboundRecordingForge{}
			_, err := OutboundChecked(fake, "worker").FileIssue(obRepo(obPublic), IssueInput{Title: "a neutral title", Body: ocCleanBody})
			if err == nil || !IsRefused(err) || !strings.Contains(err.Error(), RuleHouseCallout) {
				t.Fatalf("the callout's answer did not refuse: %v", err)
			}
			if c.onStderr && !strings.Contains(stderr.String(), ocReason) {
				t.Errorf("the callout's words are not on stderr (the one place they belong):\n%s", stderr)
			}
			if !strings.Contains(stderr.String(), ocDiagnostic) {
				t.Errorf("the callout's own stderr diagnostic was not passed to its owner:\n%s", stderr)
			}
			if strings.ContainsRune(stderr.String(), 0x1b) {
				t.Fatalf("a terminal escape the callout printed reached the terminal: %q", stderr)
			}
			for _, where := range []struct{ name, text string }{
				{"the forge", strings.Join(fake.bodies, "\n") + strings.Join(fake.calls, "\n")},
				{"the audit log", ocAuditText(t, audit)},
				{"the returned error", err.Error()},
			} {
				for _, leak := range []string{ocReason, ocDiagnostic} {
					if strings.Contains(where.text, leak) {
						t.Fatalf("%q reached %s:\n%s", leak, where.name, where.text)
					}
				}
			}
		})
	}

	path, _ := ocStub(t, "block")
	audit := ocConfigure(t, map[string]string{EnvOutboundCallout: path})
	ocStderr(t)
	if _, err := ocFileIssue(obPublic, ocCleanBody); err == nil {
		t.Fatal("the block did not refuse")
	}
	// The audit row DOES exist, and holds exactly the rule id, the field name and a digest.
	var row map[string]any
	for _, ln := range strings.Split(strings.TrimSpace(ocAuditText(t, audit)), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(ln), &m) == nil && m["verb"] == "outbound_callout" {
			row = m
		}
	}
	if row == nil {
		t.Fatalf("no outbound_callout audit row:\n%s", ocAuditText(t, audit))
	}
	if d, _ := row["detail"].(string); !strings.Contains(d, "rule="+RuleHouseCallout) || !strings.Contains(d, "outcome=block") || !strings.Contains(d, "field=") {
		t.Fatalf("audit detail = %q, want the rule id, outcome and a field name", d)
	}
	// The digest is OF THE WRITE: each non-empty field's name and text, NUL-terminated, in
	// order — computed here independently, so a constant or a digest of something else fails.
	want := sha256.Sum256([]byte("title\x00a neutral title\x00body\x00" + ocCleanBody + "\x00"))
	if d, _ := row["bodyDigest"].(string); d != hex.EncodeToString(want[:]) {
		t.Fatalf("audit bodyDigest = %q, want the SHA-256 of the write's fields %x", d, want)
	}
	if strings.Contains(ocAuditText(t, audit), ocCleanBody) {
		t.Fatal("the audit log holds the write's text")
	}
}

// A block is not overridable (it is the withheld-identifier class), and an overridable
// compiled finding that the operator overrode is STILL put to the callout.
func TestHouseCalloutIsNotOverridable(t *testing.T) {
	path, dir := ocStub(t, "block")
	audit := ocConfigure(t, map[string]string{EnvOutboundCallout: path})
	ocStderr(t)
	SetOutboundContext(OutboundContext{Tool: "deskfile", Verb: "new", OverrideReason: "a reviewed false positive"})
	if _, err := ocFileIssue(obPublic, ocCleanBody); err == nil || !strings.Contains(err.Error(), RuleHouseCallout) ||
		!strings.Contains(err.Error(), "not overridable") {
		t.Fatalf("an override took a house.callout block through: %v", err)
	}
	// A compiled, overridable finding (an e-mail address) overridden: the override row is
	// written, and then the callout is asked and still blocks.
	before := ocRan(dir)
	SetOutboundContext(OutboundContext{Tool: "deskfile", Verb: "new", OverrideReason: "a reviewed false positive"})
	_, err := ocFileIssue(obPrivate, "reach "+obEmailOutside+" for access")
	if err == nil || !strings.Contains(err.Error(), RuleHouseCallout) {
		t.Fatalf("an overridden compiled finding skipped the callout: %v", err)
	}
	if ocRan(dir) != before+1 {
		t.Fatalf("the callout ran %d times, want one more than %d", ocRan(dir), before)
	}
	if rows := obAuditRows(t, audit); len(rows) != 1 {
		t.Fatalf("want exactly the compiled override row, got %d", len(rows))
	}
}

// The request is ONE JSON object on stdin in the documented shape, and carries the text.
func TestOutboundCalloutRequestShape(t *testing.T) {
	path, dir := ocStub(t, "allow")
	ocConfigure(t, map[string]string{EnvOutboundCallout: path})
	ocStderr(t)
	if _, err := ocFileIssue(obPublic, ocCleanBody); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "request.json"))
	if err != nil {
		t.Fatal(err)
	}
	var req struct {
		Version    int    `json:"version"`
		Verb       string `json:"verb"`
		Role       string `json:"role"`
		Repo       string `json:"repo"`
		Visibility string `json:"visibility"`
		Kind       string `json:"kind"`
		Fields     []struct{ Name, Text string }
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		t.Fatalf("the request is not the documented object: %v\n%s", err, b)
	}
	if req.Version != 1 || req.Verb != "deskfile new" || req.Role != "worker" || req.Repo != obPublic ||
		req.Visibility != "public" || req.Kind != OutboundKindIssue {
		t.Fatalf("request = %+v", req)
	}
	got := map[string]string{}
	for _, f := range req.Fields {
		got[f.Name] = f.Text
	}
	if got["body"] != ocCleanBody || got["title"] != "a neutral title" {
		t.Fatalf("fields = %v", got)
	}
	// An unlisted target is reported as `unknown`, never as `private`.
	if _, err := ocFileIssue(obUnknown, ocCleanBody); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(dir, "request.json"))
	if !strings.Contains(string(b), `"visibility":"unknown"`) {
		t.Fatalf("visibility for an unlisted target = %s", b)
	}
	// The text is a JSON string WITHOUT HTML escaping: `&`, `<` and `>` reach the callout as
	// themselves (Go's default would send `&`, `<`, `>`, and a callout matching
	// the words it was given would never see them). A field with no text is not sent.
	const marked = `a & b <c> "d" \e`
	fake := &outboundRecordingForge{}
	if _, err := OutboundChecked(fake, "worker").FileIssue(obRepo(obPublic), IssueInput{Title: "", Body: marked}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(dir, "request.json"))
	if !strings.Contains(string(b), `"text":"a & b <c> \"d\" \\e"`) || strings.Contains(string(b), `\u00`) {
		t.Fatalf("the request's text is not the write's text as a plain JSON string: %s", b)
	}
	req.Fields = nil
	if err := json.Unmarshal(b, &req); err != nil {
		t.Fatal(err)
	}
	if len(req.Fields) != 1 || req.Fields[0].Name != "body" || req.Fields[0].Text != marked {
		t.Fatalf("fields = %+v, want only the non-empty body, decoding to the write's exact text", req.Fields)
	}
}

// TestCalloutAnswerParsing — the answer's FIRST WORD, separated by any whitespace, is the
// verdict, and it is exactly `allow` or `block`. Anything after `block` is the reason.
func TestCalloutAnswerParsing(t *testing.T) {
	for _, c := range []struct {
		name, out string
		pass      bool
	}{
		{"allow", "allow", true},
		{"allow tab trailing text", "allow\tall clear", true},
		{"allow then a second line", "allow\nblock this is not the first word", true},
		{"block newline reason", "block\n" + ocReason, false},
		{"block tab reason", "block\t" + ocReason, false},
		{"Allow", "Allow", false},
		{"allowed", "allowed", false},
	} {
		c := c
		t.Run(c.name, func(t *testing.T) {
			path, _ := ocSay(t, c.out, "", 0)
			ocConfigure(t, map[string]string{EnvOutboundCallout: path})
			stderr := ocStderr(t)
			calls, err := ocFileIssue(obPublic, ocCleanBody)
			if c.pass {
				if err != nil || calls != 1 {
					t.Fatalf("answer %q: want the write to proceed, got calls=%d err=%v", c.out, calls, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), RuleHouseCallout) || calls != 0 {
				t.Fatalf("answer %q: want a house.callout refusal, got calls=%d err=%v", c.out, calls, err)
			}
			// A block whose reason follows any whitespace is a BLOCK, reported as one.
			if strings.HasPrefix(c.out, "block") {
				if !strings.Contains(err.Error(), "blocked by the configured house callout") ||
					!strings.Contains(stderr.String(), "— "+ocReason) {
					t.Fatalf("answer %q: want a block with its reason on stderr, got %v\n%s", c.out, err, stderr)
				}
			}
		})
	}
}

// TestRosterRecognisesOutboundCalloutKeys — a roster carrying the three keys loads (without
// the parser change it refuses the WHOLE roster), they land on the config, and the P3 echo
// prints both the path and the required mode.
func TestRosterRecognisesOutboundCalloutKeys(t *testing.T) {
	withRoster(t, map[string]string{
		EnvBlessLogin:              "ada:2001",
		EnvTrustedLogins:           "ada:2001",
		EnvTrustedBotSlugs:         "worker=assay-worker-app:300000006",
		EnvOutboundCallout:         "/opt/example-house/callout",
		EnvOutboundCalloutRequired: "public",
		EnvOutboundCalloutTimeout:  "30s",
	})
	t.Setenv(EnvOutboundCalloutRequired, "")
	c := EffectiveConfig()
	if len(c.Problems) != 0 {
		t.Fatalf("a roster carrying the three keys was refused: %v", c.Problems)
	}
	if !c.Configured() {
		t.Fatal("the roster is not Configured")
	}
	if c.OutboundCallout != "/opt/example-house/callout" || !c.OutboundCalloutRequired ||
		c.OutboundCalloutTimeout != 30*time.Second || c.OutboundCalloutProblem != "" {
		t.Fatalf("the keys did not land on the config: %+v", c)
	}
	echo := strings.Join(c.EffectiveConfigLines(), "\n")
	for _, want := range []string{
		"assay-config: " + EnvOutboundCallout + "=/opt/example-house/callout",
		"assay-config: " + EnvOutboundCalloutRequired + "=public (roster)",
	} {
		if !strings.Contains(echo, want) {
			t.Fatalf("the startup echo lacks %q:\n%s", want, echo)
		}
	}
}

// A SET key that is malformed is not "unconfigured": the roster still loads and every
// outward write refuses, naming the key. Only an UNSET key means compiled checks alone.
func TestOutboundCalloutMalformedKeyRefusesWrites(t *testing.T) {
	for name, extra := range map[string]map[string]string{
		"relative path":        {EnvOutboundCallout: "relative/callout"},
		"a list":               {EnvOutboundCallout: "/opt/a,/opt/b"},
		"timeout below 1s":     {EnvOutboundCallout: "/opt/example-house/callout", EnvOutboundCalloutTimeout: "500ms"},
		"timeout above 60s":    {EnvOutboundCallout: "/opt/example-house/callout", EnvOutboundCalloutTimeout: "61s"},
		"timeout not duration": {EnvOutboundCallout: "/opt/example-house/callout", EnvOutboundCalloutTimeout: "soon"},
		"required not public":  {EnvOutboundCalloutRequired: "everything"},
	} {
		extra := extra
		t.Run(name, func(t *testing.T) {
			ocConfigure(t, extra)
			if c := EffectiveConfig(); len(c.Problems) != 0 || c.OutboundCalloutProblem == "" {
				t.Fatalf("want a loaded roster carrying a callout problem, got problems=%v problem=%q", c.Problems, c.OutboundCalloutProblem)
			}
			ocStderr(t)
			calls, err := ocFileIssue(obPrivate, ocCleanBody)
			if err == nil || !strings.Contains(err.Error(), RuleHouseCallout) || calls != 0 {
				t.Fatalf("a malformed callout key let a write through: calls=%d err=%v", calls, err)
			}
		})
	}
}

// ocExampleWords is the invented house list the example executable is tested against: words
// carrying every character JSON or HTML escaping rewrites, and a two-word phrase.
const ocExampleWords = "example-other-word\nhouse&word\nhouse<word>\nhouse\"word\nhouse\\word\nexample house phrase\n"

// ocExampleSweep installs the shipped example with the invented list beside it (the
// fixture's list path is `words.txt` in its own directory; the README's is the deployment's).
func ocExampleSweep(t *testing.T) (path, dir string) {
	t.Helper()
	path, dir = ocStub(t, "example-sweep")
	if err := os.WriteFile(filepath.Join(dir, "words.txt"), []byte(ocExampleWords), 0o644); err != nil {
		t.Fatal(err)
	}
	return path, dir
}

// The documented example executable answers as documented on hand-written requests.
func TestExampleSweepCallout(t *testing.T) {
	path, dir := ocExampleSweep(t)
	for body, want := range map[string]string{
		`{"version":1,"verb":"example-other-word","fields":[{"name":"body","text":"all good"}]}`:                        "allow",
		`{"version":1,"fields":[{"name":"body","text":"mentions the Example-Other-Word here"}]}`:                        "block",
		`{"version":1,"fields":[{"name":"title","text":"fine"},{"name":"body","text":"has example-other-word in it"}]}`: "block",
	} {
		res, err := Callout{Path: path, Env: outboundCalloutEnv()}.Run(body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(res.Stdout, want) {
			t.Fatalf("request %s: example printed %q, want %q first", body, res.Stdout, want)
		}
	}
	if err := os.Remove(filepath.Join(dir, "words.txt")); err != nil {
		t.Fatal(err)
	}
	if res, _ := (Callout{Path: path, Env: outboundCalloutEnv()}).Run(`{"fields":[]}`); !strings.HasPrefix(res.Stdout, "block") {
		t.Fatalf("an unreadable word list must block, got %q", res.Stdout)
	}
}

// TestExampleSweepThroughCheck — the documented example, configured as THE callout and driven
// through the real request encoding (OutboundChecked → buildOutboundCalloutRequest → the
// executable), matches a listed word exactly: one containing `&`, `<`, `>`, `"` or `\`, and a
// listed phrase whose words a line break or a run of whitespace separates in the write. A
// literal backslash-n in the text is NOT a line break and does not join a phrase.
func TestExampleSweepThroughCheck(t *testing.T) {
	path, _ := ocExampleSweep(t)
	ocConfigure(t, map[string]string{EnvOutboundCallout: path})
	for _, c := range []struct {
		body  string
		block bool
	}{
		{ocCleanBody, false},
		{"see house&word for the rule", true},
		{"see house<word> for the rule", true},
		{`see house"word for the rule`, true},
		{`see house\word for the rule`, true},
		{"the example house\nphrase, split over a line break", true},
		{"the example\r\n   house phrase, split by CRLF and spaces", true},
		{"the example\thouse  phrase, split by a tab", true},
		{`the example\nhouse phrase, a literal backslash-n`, false},
		{"house and word, apart", false},
	} {
		ocStderr(t)
		calls, err := ocFileIssue(obPublic, c.body)
		if c.block {
			if err == nil || !strings.Contains(err.Error(), RuleHouseCallout) || calls != 0 {
				t.Errorf("body %q names a listed word, but the example let it through: calls=%d err=%v", c.body, calls, err)
			}
			continue
		}
		if err != nil || calls != 1 {
			t.Errorf("body %q names no listed word, but was refused: calls=%d err=%v", c.body, calls, err)
		}
	}
}

// TestExampleSweepMatchesReadme — the fixture IS the README's example: line for line, except
// the one `words=` line that says where the list lives.
func TestExampleSweepMatchesReadme(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	const open = "```sh\n#!/bin/sh\n# EXAMPLE house callout"
	i := strings.Index(string(readme), open)
	if i < 0 {
		t.Fatal("the README carries no example house callout block")
	}
	block := string(readme)[i+len("```sh\n"):]
	block = block[:strings.Index(block, "```")]
	fixture, err := os.ReadFile(filepath.Join("testdata", "outbound-callout", "example-sweep.sh"))
	if err != nil {
		t.Fatal(err)
	}
	strip := func(s string) string {
		var keep []string
		for _, ln := range strings.Split(strings.TrimSpace(s), "\n") {
			if !strings.HasPrefix(ln, "words=") {
				keep = append(keep, ln)
			}
		}
		return strings.Join(keep, "\n")
	}
	if strip(block) != strip(string(fixture)) {
		t.Fatalf("the README example and the tested fixture differ:\n--- README\n%s\n--- fixture\n%s", block, fixture)
	}
}

// ocListedBody names a word on ocExampleWords; ocCleanBody names none.
const ocListedBody = "the rollout notes mention example-other-word twice"

// TestExampleCallerPathCannotDecide — the callout is handed the CALLING process's PATH value,
// so the example must not use it to find its tools. With that PATH emptied, or led by a
// substitute `grep` that reports "no match", a write naming a listed word is still refused
// with zero forge calls.
func TestExampleCallerPathCannotDecide(t *testing.T) {
	path, _ := ocExampleSweep(t)
	ocConfigure(t, map[string]string{EnvOutboundCallout: path})
	fake := t.TempDir()
	if err := os.WriteFile(filepath.Join(fake, "grep"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, callerPath := range map[string]string{
		"no tools on the caller's PATH":       t.TempDir(),
		"a substitute grep first on the PATH": fake + string(os.PathListSeparator) + os.Getenv("PATH"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("PATH", callerPath)
			ocStderr(t)
			calls, err := ocFileIssue(obPublic, ocListedBody)
			if err == nil || !strings.Contains(err.Error(), RuleHouseCallout) || calls != 0 {
				t.Fatalf("the caller's PATH decided the example's answer: a listed word reached the forge: calls=%d err=%v", calls, err)
			}
		})
	}
}

// ocMultibyteLocales are non-UTF-8 multibyte locales in which a UTF-8 write's bytes are not
// all characters: a text tool running in one of them skips such a field without an error.
var ocMultibyteLocales = []string{
	"ja_JP.SJIS", "ja_JP.eucJP", "zh_TW.Big5", "zh_CN.GBK", "zh_CN.GB18030", "ko_KR.eucKR",
}

// ocInstalledLocales returns the members of want this machine has installed, matched as
// `locale -a` lists them (case and the charset's punctuation vary by system); nil when
// `locale -a` cannot be run.
func ocInstalledLocales(t *testing.T, want []string) []string {
	t.Helper()
	out, err := exec.Command("locale", "-a").Output()
	if err != nil {
		return nil
	}
	norm := func(s string) string {
		return strings.NewReplacer("-", "", "_", "", ".", "").Replace(strings.ToLower(strings.TrimSpace(s)))
	}
	have := map[string]string{}
	for _, ln := range strings.Split(string(out), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			have[norm(ln)] = ln
		}
	}
	var got []string
	for _, w := range want {
		if name, ok := have[norm(w)]; ok {
			got = append(got, name)
		}
	}
	return got
}

// TestExampleLocaleCannotDecide — the callout is handed the CALLING process's LANG value, so
// the example must not match in that locale. With LANG set to a multibyte locale in which the
// write's UTF-8 bytes are not text, a write naming a listed word beside a non-ASCII character
// is still refused with zero forge calls, and a clean write with the same characters is still
// allowed. Where the machine has none of those locales the behaviour proves nothing and
// skips; the fixture's own locale line is pinned first, so that skip never hides its removal.
func TestExampleLocaleCannotDecide(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "outbound-callout", "example-sweep.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "\nLC_ALL=C\nexport PATH LC_ALL\n") {
		t.Fatal("the example does not set and export its own locale (want `LC_ALL=C` exported beside PATH)")
	}
	locales := ocInstalledLocales(t, ocMultibyteLocales)
	if len(locales) == 0 {
		t.Skip("could-not-check: none of the multibyte locales is installed on this machine")
	}
	path, _ := ocExampleSweep(t)
	ocConfigure(t, map[string]string{EnvOutboundCallout: path})
	for _, loc := range locales {
		t.Run(loc, func(t *testing.T) {
			t.Setenv("LANG", loc)
			for _, body := range []string{
				"the rollout notes — mention example-other-word twice",
				"café 日本 the rollout notes mention example-other-word twice",
			} {
				ocStderr(t)
				calls, err := ocFileIssue(obPublic, body)
				if err == nil || !strings.Contains(err.Error(), RuleHouseCallout) || calls != 0 {
					t.Errorf("LANG=%s decided the example's answer: a listed word reached the forge: calls=%d err=%v", loc, calls, err)
				}
			}
			ocStderr(t)
			if calls, err := ocFileIssue(obPublic, "the rollout notes — café 日本, nothing listed"); err != nil || calls != 1 {
				t.Errorf("LANG=%s: a clean non-ASCII write was refused: calls=%d err=%v", loc, calls, err)
			}
		})
	}
}

// ocExampleTools installs the example with its own PATH line pointed at a directory holding
// exactly the given tools (name → script body), so a test can take each tool away or make
// it fail. The fixture must fix its own PATH: a fixture without that line fails here.
func ocExampleTools(t *testing.T, tools map[string]string) string {
	t.Helper()
	path, dir := ocExampleSweep(t)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const own = "\nPATH=/usr/bin:/bin\n"
	if strings.Count(string(src), own) != 1 {
		t.Fatalf("the example does not set its own PATH (want one line %q)", strings.TrimSpace(own))
	}
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range tools {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	out := strings.Replace(string(src), own, "\nPATH="+bin+"\n", 1)
	if err := os.WriteFile(path, []byte(out), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestExampleToolFailureBlocks — the example answers `allow` only when its match ran and found
// nothing. With any of its tools missing, failing or killed at any step, a CLEAN write (one a
// working example allows) is refused with zero forge calls. The control row, every tool
// present and working, allows the same write, so a refusal below is the failure's doing.
func TestExampleToolFailureBlocks(t *testing.T) {
	grep, err1 := exec.LookPath("grep")
	sed, err2 := exec.LookPath("sed")
	if err1 != nil || err2 != nil {
		t.Skip("no grep or sed on this test machine's PATH")
	}
	realGrep, realSed := `exec "`+grep+`" "$@"`, `exec "`+sed+`" "$@"`
	// failOn makes grep fail (status 2), or be killed, only for the call carrying flag.
	failOn := func(flag, how string) string {
		return `case " $* " in *" ` + flag + ` "*) ` + how + ` ;; esac; ` + realGrep
	}
	for _, c := range []struct {
		name  string
		tools map[string]string
		allow bool
	}{
		{"control: every tool works", map[string]string{"grep": realGrep, "sed": realSed}, true},
		{"no tools at all", map[string]string{}, false},
		{"sed missing", map[string]string{"grep": realGrep}, false},
		{"sed fails", map[string]string{"grep": realGrep, "sed": "exit 2"}, false},
		{"the list check fails", map[string]string{"grep": failOn("-q", "exit 2"), "sed": realSed}, false},
		{"the extract fails", map[string]string{"grep": failOn("-oE", "exit 2"), "sed": realSed}, false},
		{"the match fails", map[string]string{"grep": failOn("-qiF", "exit 2"), "sed": realSed}, false},
		{"the match is killed", map[string]string{"grep": failOn("-qiF", "kill -9 $$"), "sed": realSed}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := ocExampleTools(t, c.tools)
			ocConfigure(t, map[string]string{EnvOutboundCallout: path})
			ocStderr(t)
			calls, err := ocFileIssue(obPublic, ocCleanBody)
			if c.allow {
				if err != nil || calls != 1 {
					t.Fatalf("with every tool working the example refused a clean write: calls=%d err=%v", calls, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), RuleHouseCallout) || calls != 0 {
				t.Fatalf("the example's match could not run, yet the write reached the forge: calls=%d err=%v", calls, err)
			}
		})
	}
}

// TestCalloutAllowShowsStderr — an `allow` that comes with a diagnostic on the callout's
// stderr prints that diagnostic to stderr (one line), so a callout whose own tools failed is
// not silent; the write goes through and the diagnostic reaches no audit row.
func TestCalloutAllowShowsStderr(t *testing.T) {
	path, _ := ocSay(t, "allow\n", ocDiagnostic+"\nsecond line\n", 0)
	audit := ocConfigure(t, map[string]string{EnvOutboundCallout: path})
	stderr := ocStderr(t)
	calls, err := ocFileIssue(obPublic, ocCleanBody)
	if err != nil || calls != 1 {
		t.Fatalf("an allow did not let the write through: calls=%d err=%v", calls, err)
	}
	if want := "house callout stderr: " + ocDiagnostic + " second line\n"; !strings.Contains(stderr.String(), want) {
		t.Fatalf("the diagnostic beside an allow is not on stderr as one line %q:\n%s", want, stderr)
	}
	if a := ocAuditText(t, audit); strings.Contains(a, ocDiagnostic) {
		t.Fatalf("the diagnostic reached the audit log:\n%s", a)
	}
}

// TestCalloutReasonClipped — what the callout prints reaches the terminal as ONE line (a
// second line could pose as another tool's output) and bounded in length.
func TestCalloutReasonClipped(t *testing.T) {
	long := strings.Repeat("x", maxCalloutReasonRunes+50)
	for _, c := range []struct{ name, answer, want string }{
		{"two lines", "block first-part\nsecond-part\n", "refused: " + RuleHouseCallout + " at (write) — first-part second-part\n"},
		{"over-long", "block " + long + "\n", "refused: " + RuleHouseCallout + " at (write) — " + long[:maxCalloutReasonRunes] + "…\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			path, _ := ocSay(t, c.answer, "", 0)
			ocConfigure(t, map[string]string{EnvOutboundCallout: path})
			stderr := ocStderr(t)
			if calls, err := ocFileIssue(obPublic, ocCleanBody); err == nil || calls != 0 {
				t.Fatalf("a block let the write through: calls=%d err=%v", calls, err)
			}
			if got := stderr.String(); got != c.want {
				t.Fatalf("stderr = %q, want exactly %q", got, c.want)
			}
		})
	}
}

// TestOutboundCalloutEcho — the startup echo never presents a callout that will refuse every
// write as healthy, and it shows the timeout in force.
func TestOutboundCalloutEcho(t *testing.T) {
	for _, c := range []struct {
		name  string
		extra map[string]string
		want  []string
	}{
		{"unset", nil, []string{
			EnvOutboundCallout + "=(unset — compiled outbound checks only)",
			EnvOutboundCalloutRequired + "=(unset",
			EnvOutboundCalloutTimeout + "=(unset — 5s)",
		}},
		{"valid path, timeout out of range", map[string]string{EnvOutboundCallout: "/opt/example-house/callout", EnvOutboundCalloutTimeout: "61s"}, []string{
			EnvOutboundCallout + "=/opt/example-house/callout (INVALID — outward writes REFUSE: ",
		}},
		{"required malformed", map[string]string{EnvOutboundCalloutRequired: "everything"}, []string{
			EnvOutboundCallout + "=(INVALID — outward writes REFUSE: ",
			EnvOutboundCalloutRequired + "=(INVALID — read as public; outward writes REFUSE until it is fixed)",
		}},
		{"timeout set", map[string]string{EnvOutboundCallout: "/opt/example-house/callout", EnvOutboundCalloutTimeout: "30s"}, []string{
			EnvOutboundCalloutTimeout + "=30s",
		}},
	} {
		c := c
		t.Run(c.name, func(t *testing.T) {
			ocConfigure(t, c.extra)
			echo := strings.Join(EffectiveConfig().EffectiveConfigLines(), "\n")
			for _, w := range c.want {
				if !strings.Contains(echo, "assay-config: "+w) {
					t.Fatalf("the startup echo lacks %q:\n%s", w, echo)
				}
			}
		})
	}
}

// houseDetailAllowed is the allow-list of identifiers a houseRefuse `detail` argument may be
// built from: our own constants, the resolved configuration's problem text, and the callout
// plumbing's error (a path and an exit status, never the callout's output). Any other
// identifier — the result, the answer, a word cut from it — is the callout's own output,
// which belongs in the stderr-only argument and nowhere else.
var houseDetailAllowed = map[string]bool{
	"fmt": true, "Sprintf": true, "s": true, "Problem": true, "err": true, "Error": true,
	"maxCalloutOutput": true, "EnvOutboundCallout": true, "EnvOutboundCalloutRequired": true,
	"outboundCalloutRequiredPublic": true,
}

// houseLeakViolations is the structural half of the class guard over outboundcallout.go:
// (1) every houseRefuse call's `detail` argument uses allow-listed identifiers only, and
// (2) no other function in the file builds a refusal, an error or an audit row — houseRefuse
// is the ONE exit, so the rule in (1) covers every path the callout's words could take to
// the returned error or the audit log.
func houseLeakViolations(src string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "outboundcallout.go", src, 0)
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				name = fun.Name
			case *ast.SelectorExpr:
				name = fun.Sel.Name
			}
			switch name {
			case "houseRefuse":
				if len(call.Args) < 5 {
					bad = append(bad, fmt.Sprintf("%s: houseRefuse call without a detail argument", fset.Position(call.Pos())))
					return true
				}
				ast.Inspect(call.Args[4], func(m ast.Node) bool {
					if id, ok := m.(*ast.Ident); ok && !houseDetailAllowed[id.Name] {
						bad = append(bad, fmt.Sprintf("%s: houseRefuse detail is built from %q", fset.Position(id.Pos()), id.Name))
					}
					return true
				})
			case "Refused", "Unverifiable", "Errorf", "New", "Log":
				if fn.Name.Name != "houseRefuse" {
					bad = append(bad, fmt.Sprintf("%s: %s() outside houseRefuse — every refusal and audit row leaves through houseRefuse", fset.Position(call.Pos()), name))
				}
			}
			return true
		})
	}
	return bad, nil
}

// TestHouseRefuseDetailNeverCarriesCalloutOutput — the class guard, run over the shipped
// file, with a positive control: a planted source repeating the defect at two NEW sites (a
// detail built from the result, and a refusal composed outside houseRefuse) must be flagged
// at both, so a guard whose matcher stopped matching fails instead of reporting clean.
func TestHouseRefuseDetailNeverCarriesCalloutOutput(t *testing.T) {
	src, err := os.ReadFile("outboundcallout.go")
	if err != nil {
		t.Fatal(err)
	}
	bad, err := houseLeakViolations(string(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != 0 {
		t.Fatalf("the callout's output can reach the returned error or the audit log:\n%s", strings.Join(bad, "\n"))
	}
	const planted = `package deskkit
func outboundHouseCheck(w OutboundWrite) error {
	res, _ := Callout{}.Run("")
	return houseRefuse(w, "", "", "broken", "the callout printed "+res.Stdout, calloutSaid{})
}
func plantedSecondExit(out string) error { return Refused("the callout said " + out) }
`
	bad, err = houseLeakViolations(planted)
	if err != nil {
		t.Fatal(err)
	}
	flagged := strings.Join(bad, "\n")
	if !strings.Contains(flagged, `houseRefuse detail is built from "res"`) || !strings.Contains(flagged, "Refused() outside houseRefuse") {
		t.Fatalf("the guard missed a planted leak; flagged:\n%s", strings.Join(bad, "\n"))
	}
}
