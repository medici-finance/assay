package deskkit

// outbound_test.go — the conformance table of desktools-v2/10, the completeness layer over
// the Forge interface, and the override audit row.
//
// Every personal-data fixture below is assembled by concatenation, so no line of this file
// is itself an e-mail address or a telephone number: the push-path check this file tests
// reads the added lines of the change that carries it.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Invented targets: one public, one private, one the roster does not list at all.
const (
	obPublic   = "example-org/example-public"
	obPrivate  = "example-org/example-restricted"
	obUnknown  = "example-org/example-unlisted"
	obInternal = "example-org/example-internal" // private per the roster; a public body must not name it
	obWithheld = "example-withheld-slug"
)

var (
	obEmailOutside  = "person" + "@" + "corp-mail.dev"
	obEmailReserved = "person" + "@" + "corp-mail.test"
	obEmailNoReply  = "1234567+example-bot[bot]" + "@" + "users.noreply.github.com"
	obPhoneIntl     = "+1 415 " + "555 0100"
	obPhoneAmbig    = "415-555" + "-0100"
	obRulingClaim   = "I (" + "Alex" + ") have decided to ship it."
)

// obRoster plants the configured roster every row reads: the three listed targets and the
// one configured human whose voice the ruling-claim guard protects.
func obRoster(t *testing.T) {
	t.Helper()
	r := goldenRoster()
	r[EnvAllowedRepos] = obPublic + ":ci:public," + obPrivate + ":ci:private," + obInternal + ":ci:private"
	r[EnvHumanLoginMap] = "alex:ada"
	withRoster(t, r)
	t.Setenv(EnvWithheldIdentifiers, obWithheld)
	t.Setenv(EnvOutboundCalloutRequired, "") // an ambient requirement would refuse the compiled table's passes
}

func obRepo(slug string) ForgeRepo {
	o, n, _ := strings.Cut(slug, "/")
	return ForgeRepo{Owner: o, Name: n}
}

// outboundRecordingForge is the fake behind the decorator: it implements every text-carrying write
// (recording the call and the text it would have published) and ReadFile (the file does
// not exist yet). Any other method reaches the nil embedded Forge and panics.
type outboundRecordingForge struct {
	Forge
	calls  []string
	bodies []string
}

func (r *outboundRecordingForge) rec(m string, text ...string) {
	r.calls = append(r.calls, m)
	r.bodies = append(r.bodies, strings.Join(text, "\n"))
}

func (r *outboundRecordingForge) FileIssue(_ ForgeRepo, in IssueInput) (*IssueRef, error) {
	r.rec("FileIssue", in.Title, in.Body)
	return &IssueRef{}, nil
}

func (r *outboundRecordingForge) PostComment(_ ForgeRepo, _ int, body string) (*CommentRef, error) {
	r.rec("PostComment", body)
	return &CommentRef{}, nil
}

func (r *outboundRecordingForge) PostCommentTyped(_ ForgeRepo, _ int, _ TargetKind, body string) (*CommentRef, error) {
	r.rec("PostCommentTyped", body)
	return &CommentRef{}, nil
}

func (r *outboundRecordingForge) EditComment(_ ForgeRepo, _, body string) error {
	r.rec("EditComment", body)
	return nil
}

func (r *outboundRecordingForge) CreateDraftChange(_ ForgeRepo, in DraftChangeInput) (*PullRef, error) {
	r.rec("CreateDraftChange", in.Title, in.Body, in.Head)
	return &PullRef{}, nil
}

func (r *outboundRecordingForge) EditChange(_ ForgeRepo, _ int, in EditChangeInput) error {
	r.rec("EditChange", in.Title, in.Body)
	return nil
}

func (r *outboundRecordingForge) PostReview(_ ForgeRepo, _ int, in ReviewInput) error {
	r.rec("PostReview", in.Body)
	return nil
}

func (r *outboundRecordingForge) ApplyLabels(_ ForgeRepo, _ int, c LabelChange) (*LabelOutcome, error) {
	var parts []string
	for _, l := range c.Add {
		parts = append(parts, l.Name, l.Description)
	}
	r.rec("ApplyLabels", parts...)
	return &LabelOutcome{}, nil
}

func (r *outboundRecordingForge) WriteFile(_ ForgeRepo, in WriteFileInput) (*WriteFileResult, error) {
	r.rec("WriteFile", in.File, string(in.Content), in.Message)
	return &WriteFileResult{}, nil
}

func (r *outboundRecordingForge) ReadFile(ForgeRepo, ReadFileInput) (*FileContent, error) {
	return &FileContent{Exists: false}, nil
}

// obTextCall is one text-carrying write method, called with text in its author-text field.
type obTextCall struct {
	kind string
	call func(f Forge, repo ForgeRepo, text string) error
}

// obTextMethods is every text-carrying write of the decorator. The completeness test pins
// that this set equals the "text" rows of its classification table.
var obTextMethods = map[string]obTextCall{
	"FileIssue": {OutboundKindIssue, func(f Forge, r ForgeRepo, s string) error {
		_, err := f.FileIssue(r, IssueInput{Title: "a neutral title", Body: s})
		return err
	}},
	"PostComment": {OutboundKindComment, func(f Forge, r ForgeRepo, s string) error {
		_, err := f.PostComment(r, obNumber, s)
		return err
	}},
	"PostCommentTyped": {OutboundKindComment, func(f Forge, r ForgeRepo, s string) error {
		_, err := f.PostCommentTyped(r, obNumber, TargetIssue, s)
		return err
	}},
	"EditComment": {OutboundKindComment, func(f Forge, r ForgeRepo, s string) error {
		return f.EditComment(r, "c1", s)
	}},
	"CreateDraftChange": {OutboundKindChange, func(f Forge, r ForgeRepo, s string) error {
		_, err := f.CreateDraftChange(r, DraftChangeInput{Title: "a neutral title", Body: s, Head: "feature", Base: "main"})
		return err
	}},
	"EditChange": {OutboundKindChange, func(f Forge, r ForgeRepo, s string) error {
		return f.EditChange(r, obNumber, EditChangeInput{Title: "a neutral title", Body: s})
	}},
	"PostReview": {OutboundKindReview, func(f Forge, r ForgeRepo, s string) error {
		return f.PostReview(r, obNumber, ReviewInput{HeadSHA: "abc123", Event: "COMMENT", Body: s})
	}},
	"ApplyLabels": {OutboundKindLabel, func(f Forge, r ForgeRepo, s string) error {
		_, err := f.ApplyLabels(r, obNumber, LabelChange{Target: TargetIssue, Add: []LabelSpec{{Name: s}}})
		return err
	}},
	"WriteFile": {OutboundKindFile, func(f Forge, r ForgeRepo, s string) error {
		_, err := f.WriteFile(r, WriteFileInput{File: "docs/note.md", Branch: "feature", Content: []byte(s + "\n"), Message: "add a note"})
		return err
	}},
}

func obMethodNames() []string {
	var out []string
	for m := range obTextMethods {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// obRow is one conformance row. surfaces selects where it runs: every decorator text method
// ("forge"), only the label method ("label"), the push path's added lines ("push-file") and
// the push path's commit message ("push-commit").
type obRow struct {
	id       string
	text     string
	target   string
	refuse   bool
	rule     string // an exact rule id, or a prefix ending in "."
	notice   string // a NOTICE the passing row must print
	span     string // the sensitive span: never in an audit row
	surfaces []string
}

var obAll = []string{"forge", "push-file"}

func obRows() []obRow {
	withheldBody := "context lives in the " + obWithheld + " stream"
	return []obRow{
		{id: "C1", text: withheldBody, target: obPublic, refuse: true, rule: RuleWithheldIdentifier, span: obWithheld, surfaces: obAll},
		{id: "C2", text: withheldBody, target: obPrivate, surfaces: obAll},
		{id: "C3", text: withheldBody, target: obUnknown, refuse: true, rule: RuleWithheldIdentifier, span: obWithheld, surfaces: obAll},
		{id: "C4", text: "// mirrors the fixture in " + obInternal, target: obPublic, refuse: true, rule: RuleSelfContainPrefix, span: obInternal, surfaces: obAll},
		{id: "C5", text: "reach " + obEmailOutside + " for access", target: obPrivate, refuse: true, rule: RulePIIEmail, span: obEmailOutside, surfaces: obAll},
		{id: "C5b", text: "reach " + obEmailReserved + " for access", target: obPrivate, surfaces: obAll},
		{id: "C6", text: "co-authored by " + obEmailNoReply, target: obPublic, surfaces: obAll},
		{id: "C7", text: "call " + obPhoneIntl + " after hours", target: obPrivate, refuse: true, rule: RulePIIPhone, span: obPhoneIntl, surfaces: obAll},
		{id: "C8", text: "ticket " + obPhoneAmbig + " is closed", target: obPublic, notice: RulePIIPhoneAmbiguous, surfaces: obAll},
		{id: "C9", text: obWithheld, target: obPublic, refuse: true, rule: RuleWithheldIdentifier, span: obWithheld, surfaces: []string{"label"}},
		{id: "C10", text: "fix the " + obWithheld + " build", target: obPublic, refuse: true, rule: RuleWithheldIdentifier, span: obWithheld, surfaces: []string{"push-commit"}},
		// The impersonation guard's row: refused on any target, never overridable (C11).
		{id: "C11v", text: obRulingClaim, target: obPrivate, refuse: true, rule: RuleVoiceRulingClaim, surfaces: obAll},
	}
}

// obPushRepo is a scratch checkout holding one branch per push row: main, plus a commit
// carrying the row's text on an added line of a test file or in its message.
type obPushRepo struct {
	t   *testing.T
	dir string
	got map[string]bool
}

func newObPushRepo(t *testing.T) *obPushRepo {
	t.Helper()
	p := &obPushRepo{t: t, dir: t.TempDir(), got: map[string]bool{}}
	p.git("init", "-q", "-b", "main")
	p.write("README.md", "seed\n")
	p.git("add", "README.md")
	p.git("commit", "-q", "-m", "seed")
	return p
}

func (p *obPushRepo) git(args ...string) {
	p.t.Helper()
	full := append([]string{
		"-c", "user.name=Row Fixture", "-c", "user.email=row" + "@" + "example.com",
		"-c", "commit.gpgsign=false", "-c", "core.hooksPath=" + os.DevNull,
		// No post-commit auto-maintenance: a detached gc/maintenance child outliving the
		// command can still be writing .git/objects when t.TempDir's cleanup runs, which
		// fails the test with "directory not empty" under CI load.
		"-c", "gc.auto=0", "-c", "maintenance.auto=false",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = p.dir
	if out, err := cmd.CombinedOutput(); err != nil {
		p.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func (p *obPushRepo) write(rel, body string) {
	p.t.Helper()
	full := filepath.Join(p.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		p.t.Fatal(err)
	}
}

// branch returns the branch carrying row r on surface s, creating it once.
func (p *obPushRepo) branch(r obRow, s string) string {
	p.t.Helper()
	b := "row-" + strings.ToLower(r.id) + "-" + s
	if p.got[b] {
		return b
	}
	p.got[b] = true
	p.git("checkout", "-q", "-b", b, "main")
	file, msg := "pkg/row_test.go", "add the row fixture"
	body := "package pkg\n\n" + r.text + "\n"
	if s == "push-commit" {
		body, msg = "package pkg\n", r.text
	}
	p.write(file, body)
	p.git("add", file)
	p.git("commit", "-q", "-m", msg)
	p.git("checkout", "-q", "main")
	return b
}

// obTarget is one place a row's text leaves by: a decorator method or the push path.
type obTarget struct {
	name string
	kind string
	run  func(*testing.T) (calls int, fake *outboundRecordingForge, err error)
}

func obTargets(t *testing.T, r obRow, push *obPushRepo) []obTarget {
	var out []obTarget
	for _, s := range r.surfaces {
		switch s {
		case "forge", "label":
			for _, m := range obMethodNames() {
				if s == "label" && m != "ApplyLabels" {
					continue
				}
				m, tc := m, obTextMethods[m]
				out = append(out, obTarget{name: m, kind: tc.kind, run: func(t *testing.T) (int, *outboundRecordingForge, error) {
					fake := &outboundRecordingForge{}
					err := tc.call(OutboundChecked(fake, "worker"), obRepo(r.target), r.text)
					return len(fake.calls), fake, err
				}})
			}
		case "push-file", "push-commit":
			s := s
			kind := OutboundKindFile
			if s == "push-commit" {
				kind = OutboundKindCommit
			}
			out = append(out, obTarget{name: s, kind: kind, run: func(t *testing.T) (int, *outboundRecordingForge, error) {
				b := push.branch(r, s)
				err := OutboundCheckPush(OutboundPush{Dir: push.dir, Repo: r.target, Base: "main", Head: b, Branch: b, Role: "worker"})
				calls := 1 // the push path has no forge; "passed" stands for "the push may proceed"
				if err != nil {
					calls = 0
				}
				return calls, nil, err
			}})
		}
	}
	return out
}

func obRuleMatches(msg, rule string) bool {
	return strings.Contains(msg, "refused: "+rule)
}

// TestOutboundConformance is the brief's conformance table, run against every text-carrying
// write of the decorator and against the push path. Rows C1-C10 without an override; C11
// the same refused rows with one; C12 is asserted on every refused row of both passes.
func TestOutboundConformance(t *testing.T) {
	obRoster(t)
	push := newObPushRepo(t)
	var composed []string // every body a delegate was handed, across the whole table

	for _, r := range obRows() {
		for _, tg := range obTargets(t, r, push) {
			r, tg := r, tg
			t.Run(r.id+"/"+tg.name, func(t *testing.T) {
				setup(t)
				var notices bytes.Buffer
				defer SetOutboundNoticeWriter(&notices)()
				SetOutboundContext(OutboundContext{Tool: "conformance", Verb: r.id})
				calls, fake, err := tg.run(t)
				if fake != nil {
					composed = append(composed, fake.bodies...)
				}
				if !r.refuse {
					if err != nil {
						t.Fatalf("%s to %s: want pass, got %v", r.id, r.target, err)
					}
					if calls != 1 {
						t.Fatalf("%s: delegate called %d times, want exactly once", r.id, calls)
					}
					if r.notice != "" && !strings.Contains(notices.String(), "NOTICE: "+r.notice) {
						t.Fatalf("%s: want a NOTICE %s, got %q", r.id, r.notice, notices.String())
					}
					return
				}
				if err == nil || !IsRefused(err) {
					t.Fatalf("%s to %s: want a refusal (exit 5) on %s, got %v", r.id, r.target, r.rule, err)
				}
				if !obRuleMatches(err.Error(), r.rule) {
					t.Fatalf("%s: refusal does not name rule %s: %v", r.id, r.rule, err)
				}
				if want := "(" + tg.kind + " write to " + r.target; !strings.Contains(err.Error(), want) {
					t.Fatalf("%s: refusal does not name %q: %v", r.id, want, err)
				}
				// C12: nothing reached the forge.
				if calls != 0 {
					t.Fatalf("C12 (%s): the fake forge saw %d calls after a refusal, want ZERO", r.id, calls)
				}
			})
		}
	}

	// C11 — every refused row again, with the override and a reason of at least 12 characters.
	for _, r := range obRows() {
		if !r.refuse {
			continue
		}
		// The ruling (#1319, option 1), stated here and NOT read back from the code under test.
		overridable := r.rule != RuleWithheldIdentifier && r.rule != RuleVoiceRulingClaim
		for _, tg := range obTargets(t, r, push) {
			r, tg := r, tg
			t.Run("C11/"+r.id+"/"+tg.name, func(t *testing.T) {
				dir := setup(t)
				defer SetOutboundNoticeWriter(&bytes.Buffer{})()
				SetOutboundContext(OutboundContext{Tool: "conformance", Verb: r.id, OverrideReason: "a reviewed false positive"})
				calls, fake, err := tg.run(t)
				if fake != nil {
					composed = append(composed, fake.bodies...)
				}
				rows := obAuditRows(t, dir)
				if !overridable {
					if err == nil || !IsRefused(err) {
						t.Fatalf("%s: %s is NOT overridable, but the override let it through (err %v)", r.id, r.rule, err)
					}
					if !strings.Contains(err.Error(), "does not apply to "+r.rule) {
						t.Fatalf("%s: refusal does not say the override does not apply: %v", r.id, err)
					}
					if calls != 0 {
						t.Fatalf("C12 (%s): the fake forge saw %d calls after a refusal, want ZERO", r.id, calls)
					}
					if len(rows) != 0 {
						t.Fatalf("%s: a refused write left %d override rows", r.id, len(rows))
					}
					return
				}
				if err != nil {
					t.Fatalf("%s: %s is overridable, but the override was refused: %v", r.id, r.rule, err)
				}
				if calls != 1 {
					t.Fatalf("%s: delegate called %d times after the override, want once", r.id, calls)
				}
				if len(rows) != 1 {
					t.Fatalf("%s: want exactly one audit row, got %d: %+v", r.id, len(rows), rows)
				}
				obAssertOverrideRow(t, rows[0], r.rule, r.span)
			})
		}
	}

	// C12 — no refusal or notice text was ever composed into a body the forge received.
	for _, b := range composed {
		for _, bad := range []string{"refused:", "NOTICE:", "scan-override RECORDED"} {
			if strings.Contains(b, bad) {
				t.Fatalf("C12: a composed body carries %q: %q", bad, b)
			}
		}
	}
}

// obAuditRows reads the audit rows written under dir. A missing file is no rows.
func obAuditRows(t *testing.T, dir string) []map[string]any {
	t.Helper()
	f, err := os.Open(filepath.Join(dir, "audit.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("audit row is not JSON: %v", err)
		}
		if m["verb"] == ScanOverrideVerb {
			out = append(out, m)
		}
	}
	return out
}

var obHex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// obAssertOverrideRow pins the override row's shape: the rule id and a digest, never the text.
func obAssertOverrideRow(t *testing.T, row map[string]any, rule, span string) {
	t.Helper()
	detail, _ := row["detail"].(string)
	if !strings.Contains(detail, "rule="+rule) {
		t.Fatalf("audit row does not name the rule %s: %q", rule, detail)
	}
	if d, _ := row["bodyDigest"].(string); !obHex64.MatchString(d) {
		t.Fatalf("audit row bodyDigest %q is not a sha256 digest", d)
	}
	raw, _ := json.Marshal(row)
	if span != "" && strings.Contains(string(raw), span) {
		t.Fatalf("audit row carries the refused text %q: %s", span, raw)
	}
}

// TestOverrideAuditRowHoldsDigestNotText — the override row holds the rule id and the
// SHA-256 of the refused field, and no byte of the field: not the span, not the text around
// it. The same write checked twice in one invocation (a verb's pre-flight, then the seam)
// records one row.
func TestOverrideAuditRowHoldsDigestNotText(t *testing.T) {
	obRoster(t)
	dir := setup(t)
	var out bytes.Buffer
	defer SetOutboundNoticeWriter(&out)()
	SetOutboundContext(OutboundContext{Tool: "conformance", Verb: "comment", OverrideReason: "a reviewed false positive"})

	body := "the vendor contact is " + obEmailOutside + " per the thread"
	fake := &outboundRecordingForge{}
	f := OutboundChecked(fake, "worker")
	for i := 0; i < 2; i++ {
		if _, err := f.PostComment(obRepo(obPrivate), 7, body); err != nil {
			t.Fatalf("overridden write %d refused: %v", i, err)
		}
	}
	if len(fake.calls) != 2 {
		t.Fatalf("delegate calls = %d, want 2", len(fake.calls))
	}
	rows := obAuditRows(t, dir)
	if len(rows) != 1 {
		t.Fatalf("want ONE override row for one field checked twice, got %d", len(rows))
	}
	row := rows[0]
	if got, want := row["bodyDigest"], Sha256Hex([]byte(body)); got != want {
		t.Fatalf("bodyDigest = %v, want the sha256 of the field %s", got, want)
	}
	obAssertOverrideRow(t, row, RulePIIEmail, obEmailOutside)
	raw, err := os.ReadFile(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{obEmailOutside, "vendor contact", body} {
		if bytes.Contains(raw, []byte(leak)) {
			t.Fatalf("the audit log carries field text %q", leak)
		}
	}
	for _, want := range []string{"kind=comment", "field=body", "visibility=private"} {
		if d, _ := row["detail"].(string); !strings.Contains(d, want) {
			t.Fatalf("audit detail lacks %q: %q", want, d)
		}
	}
	if !strings.Contains(out.String(), "scan-override RECORDED") || strings.Contains(out.String(), obEmailOutside) {
		t.Fatalf("the RECORDED line is missing or quotes the text: %q", out.String())
	}
}

// obClass is the completeness table: every method of the Forge interface, classified.
// "text" methods carry author text and are checked by the decorator; "notext" writes carry
// none (or only backend-composed fixed strings) and pass straight through; "read" methods
// write nothing. A method added to the interface without a row here fails the test.
var obClass = map[string]string{
	"FileIssue": "text", "PostComment": "text", "PostCommentTyped": "text", "EditComment": "text",
	"CreateDraftChange": "text", "EditChange": "text", "PostReview": "text", "ApplyLabels": "text",
	"WriteFile": "text",

	"CloseIssue": "notext", "CloseIssueTyped": "notext", "ReopenIssue": "notext",
	"MarkReadyForReview": "notext", "SetMergeHold": "notext", "DeleteRef": "notext",
	"OpenMergeHold": "notext", "RunWorkflow": "notext", "ApproveGate": "notext",
	"RetryRun": "notext",

	"GetPullRequest": "read", "GetIssue": "read", "GetIssueTyped": "read",
	"OpenChangeForBranch": "read", "SearchIssues": "read", "ListLabels": "read",
	"ListOpenChanges": "read", "ListChanges": "read", "ListOpenIssues": "read",
	"PRTrustEvents": "read", "IssueTrustEvents": "read", "IssueContentEvents": "read",
	"ReviewsAtHead": "read", "ReviewQueueSnapshot": "read", "ListChangedFiles": "read",
	"ChecksAtHead": "read", "RequiredStatusChecks": "read", "IssueReactions": "read",
	"ListIssueLabelEvents": "read", "ListLabelEvents": "read", "ListComments": "read", "ListCommentsTyped": "read",
	"RepoVisibility": "read", "ReadFile": "read", "ListRecentCommits": "read",
	"GetCommit": "read", "ListFileCommits": "read", "ListCommitChanges": "read",
	"CompareRefs": "read", "SearchOpenChanges": "read", "ListWorkflowFiles": "read",
	"ChangeDiff": "read", "RefExists": "read", "MatchingRefs": "read",
	"RepoHardeningRead": "read", "ReadMergeHold": "read", "RunStatus": "read", "RunLog": "read",
	"PushTransportHint": "read",
	"ListIssues":        "read", "IssueStateEvents": "read", "ListChangeCommits": "read",
	"RepoDefaultBranch": "read",
}

// nilBackend is a Forge whose every WRITE panics (a nil embedded interface): a text method
// that reaches it has delegated BEFORE the check refused — or was never overridden. ReadFile
// answers "no such file", the one read the decorator itself makes (WriteFile's added lines).
type nilBackend struct{ Forge }

func (nilBackend) ReadFile(ForgeRepo, ReadFileInput) (*FileContent, error) {
	return &FileContent{Exists: false}, nil
}

// TestOutboundForgeWrapsEveryWriteMethod is the completeness layer, independent of the
// conformance fixtures: (1) every Forge method is classified, and every classified name is
// still a method; (2) every "text" method REFUSES a withheld identifier on a public target
// without reaching the backend; (3) both backends leave ForgeFor wrapped.
func TestOutboundForgeWrapsEveryWriteMethod(t *testing.T) {
	obRoster(t)
	it := reflect.TypeOf((*Forge)(nil)).Elem()
	methods := map[string]bool{}
	for i := 0; i < it.NumMethod(); i++ {
		m := it.Method(i).Name
		methods[m] = true
		if _, ok := obClass[m]; !ok {
			t.Errorf("Forge.%s has no classification — decide whether it carries author text and, "+
				"if it does, override it in outboundforge.go before adding a row here", m)
		}
	}
	for m, c := range obClass {
		if !methods[m] {
			t.Errorf("classification row %s (%s) names no Forge method — remove the stale row", m, c)
		}
		_, called := obTextMethods[m]
		if (c == "text") != called {
			t.Errorf("%s is classified %q but the text-method call table disagrees", m, c)
		}
	}

	for _, m := range obMethodNames() {
		m := m
		t.Run("refuses-before-delegate/"+m, func(t *testing.T) {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("%s reached the backend with a withheld identifier on a public target "+
						"(the decorator does not check it): %v", m, p)
				}
			}()
			restore := SetOutboundNoticeWriter(&bytes.Buffer{})
			defer restore()
			SetOutboundContext(OutboundContext{Tool: "completeness", Verb: m})
			err := obTextMethods[m].call(OutboundChecked(nilBackend{}, "worker"), obRepo(obPublic),
				"see "+obWithheld)
			if err == nil || !IsRefused(err) || !strings.Contains(err.Error(), RuleWithheldIdentifier) {
				t.Fatalf("%s: want a %s refusal, got %v", m, RuleWithheldIdentifier, err)
			}
		})
	}

	t.Run("ForgeFor-wraps-github", func(t *testing.T) {
		repo := ForgeRepo{Owner: "example-org", Name: "wrap-github"}
		r := goldenRoster()
		r[EnvRepoForges] = repo.Slug() + "=github"
		withRoster(t, r)
		SetGitHubCustodyMinter(func(string, ForgeRepo) (string, string, error) {
			return "stub-token", "https://forge.example.invalid", nil
		})
		t.Cleanup(func() { SetGitHubCustodyMinter(nil) })
		f, err := ForgeFor(repo, "worker")
		if err != nil {
			t.Fatalf("ForgeFor: %v", err)
		}
		if !IsOutboundChecked(f) {
			t.Fatalf("ForgeFor returned %T for a GitHub repo, not the outbound-checked decorator", f)
		}
	})
	t.Run("ForgeFor-wraps-gitlab", func(t *testing.T) {
		repo := ForgeRepo{Owner: "example-org", Name: "wrap-gitlab"}
		r := goldenRoster()
		r[EnvRepoForges] = repo.Slug() + "=gitlab"
		withRoster(t, r)
		credDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(credDir, "gitlab-worker.token"), []byte("gitlab-pat-stub"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(EnvConfigHome, credDir)
		f, err := ForgeFor(repo, "worker")
		if err != nil {
			t.Fatalf("ForgeFor: %v", err)
		}
		if !IsOutboundChecked(f) {
			t.Fatalf("ForgeFor returned %T for a GitLab repo, not the outbound-checked decorator", f)
		}
	})
}

// TestOutboundWriteFileChecksBranch pins that the file-write API checks the branch it lands
// on as a ref: with StartBranch set the write CREATES that branch, publishing its name the
// way a push does, and the push path already checks the pushed ref.
func TestOutboundWriteFileChecksBranch(t *testing.T) {
	obRoster(t)
	defer SetOutboundNoticeWriter(&bytes.Buffer{})()
	SetOutboundContext(OutboundContext{Tool: "branch", Verb: "writefile"})
	fake := &outboundRecordingForge{}
	_, err := OutboundChecked(fake, "worker").WriteFile(obRepo(obPublic), WriteFileInput{
		File: "docs/note.md", Branch: "fix/" + obWithheld, StartBranch: "main",
		Content: []byte("a neutral line\n"), Message: "add a note",
	})
	if err == nil || !IsRefused(err) || !obRuleMatches(err.Error(), RuleWithheldIdentifier) {
		t.Fatalf("a withheld identifier in the created branch name: want a %s refusal, got %v",
			RuleWithheldIdentifier, err)
	}
	if !strings.Contains(err.Error(), "at branch:1") || !strings.Contains(err.Error(), "(ref write to") {
		t.Fatalf("refusal does not name the branch as a ref write: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("the backend saw %d calls after a refusal, want ZERO", len(fake.calls))
	}
}
