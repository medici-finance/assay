package deskkit

// outboundcallout_test.go — the house callout of the outbound-write check (desktools-v2/11).
//
// Every stub, word and path is invented (testdata/outbound-callout/*.sh). The stubs record
// beside themselves (`ran.log`, `request.json`, `env.dump`) because the callout's environment
// is scrubbed: a stub cannot be told where to log through a variable.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
		{"prints maybe", stub("maybe"), `printed "maybe"`},
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

// H7 — a block on a commit message, on the push path: refused before any push. The same
// range with no callout configured passes, so the refusal is the callout's.
func ocCheckPushPathCommitMessage(t *testing.T) {
	path, dir := ocStub(t, "block")
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
	ocConfigure(t, map[string]string{EnvOutboundCallout: path})
	stderr := ocStderr(t)
	err := run()
	if err == nil || !IsRefused(err) || !strings.Contains(err.Error(), RuleHouseCallout) {
		t.Fatalf("the callout's block on a commit message did not refuse the push: %v", err)
	}
	if !strings.Contains(stderr.String(), ocReason) {
		t.Fatalf("the reason is not on stderr:\n%s", stderr)
	}
	if ocRan(dir) == 0 {
		t.Fatal("the callout was never asked about the push")
	}
}

// TestCalloutReasonNeverReachesForgeOrAudit — the recording forge, the audit log, the
// returned error and the audit row of a refusal are all searched for the stub's reason.
func TestCalloutReasonNeverReachesForgeOrAudit(t *testing.T) {
	path, _ := ocStub(t, "block")
	audit := ocConfigure(t, map[string]string{EnvOutboundCallout: path})
	stderr := ocStderr(t)
	fake := &outboundRecordingForge{}
	_, err := OutboundChecked(fake, "worker").FileIssue(obRepo(obPublic), IssueInput{Title: "a neutral title", Body: ocCleanBody})
	if err == nil {
		t.Fatal("the block did not refuse")
	}
	if !strings.Contains(stderr.String(), ocReason) {
		t.Fatalf("the reason is not on stderr (the one place it belongs):\n%s", stderr)
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
	if d, _ := row["bodyDigest"].(string); !obHex64.MatchString(d) {
		t.Fatalf("audit bodyDigest = %q, want a SHA-256", d)
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

// The documented example executable works as documented.
func TestExampleSweepCallout(t *testing.T) {
	path, dir := ocStub(t, "example-sweep")
	words := filepath.Join(dir, "words.txt")
	if err := os.WriteFile(words, []byte("example-withheld-slug\nexample-other-word\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EXAMPLE_WORDS_FILE", words)
	// Stub environments are scrubbed, so the example reads its list from its default
	// location relative to the invoker only through the variable; run it directly.
	for body, want := range map[string]string{
		`{"version":1,"verb":"example-withheld-slug","fields":[{"name":"body","text":"all good"}]}`:                     "allow",
		`{"version":1,"fields":[{"name":"body","text":"mentions the Example-Withheld-Slug here"}]}`:                     "block",
		`{"version":1,"fields":[{"name":"title","text":"fine"},{"name":"body","text":"has example-other-word in it"}]}`: "block",
	} {
		res, err := Callout{Path: path}.Run(body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(res.Stdout, want) {
			t.Fatalf("request %s: example printed %q, want %q first", body, res.Stdout, want)
		}
	}
	t.Setenv("EXAMPLE_WORDS_FILE", filepath.Join(dir, "missing.txt"))
	if res, _ := (Callout{Path: path}).Run(`{"fields":[]}`); !strings.HasPrefix(res.Stdout, "block") {
		t.Fatalf("an unreadable word list must block, got %q", res.Stdout)
	}
}
