package main

// pushrotate.go — `deskfleet push-token rotate|check` (forge-gitlab/18; #2910 part B): the
// board-push credential STATUSGEN_PUSH_TOKEN rotated in ONE process. The new value lives only
// in the forge and in this process's memory: it is never printed, logged, put in argv, the
// environment or a file. pushtoken.go provisions the token the first time; this file replaces
// it.
//
// The design choices are the recorded ruling on #2912 (DR-forge-gitlab-18): a Go subcommand
// (1A); a PROJECT access token (2); a person-held rotator credential in an owner-only file
// (3A); 30-day tokens by default, at most 90, proof needs >= --min-days (default 7) left and an
// expiry no earlier than one day before the requested date (4); hidden through the create path
// where the instance sets it, else masked and protected (5); a 403 insufficient-scope answer
// from self-introspection is accepted beside the rotator-side by-id read and the live push
// probe (6A). Rotate-first (7A) is the briefed order.
//
// rotate: refusals (exit 5, zero requests) → preflight reads (exit 5 on a precondition, 6 on a
// read that fails) → rotate or create → install (retried twice) → read back → contain on a
// mismatch → three proof probes (by-id, self, push). check: reads only, under a separate
// reader credential, by-id probe only.
//
// CUSTODY. The value is decoded straight into pushSecret, whose every rendering is
// [redacted]; reveal() is the one way to its bytes, and it feeds exactly three consumers: the
// variable-write body, the two probes' credential, and the read-back's constant-time compare.
// Every response that can carry a value decodes into a type with no plaintext field, and a
// non-2xx answer on those endpoints is reported by HTTP status alone (a forge error can quote
// the submitted value). This file calls no os.Setenv/WriteFile/Create/OpenFile and imports no
// os/exec (TestPushTokenSourceGuard).

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	pushRotateDefaultDays = 30 // ruling choice 4
	pushRotateMaxDays     = 90 // ruling choice 4
	pushDefaultMinDays    = 7  // ruling choice 4
	pushRetries           = 2  // install retries after the first attempt (Task 4.2)
	pushAPISuffix         = "/api/v4"
	// pushProbeUser is the HTTP Basic user name of the push probe; GitLab accepts any
	// non-blank name beside an access token, and the credential is the password.
	pushProbeUser = "push-probe"
	// pushSessionMarker is the desk loop's self-declared session marker. The refusal on it is
	// ADVISORY: a session that unsets it passes (Threat model; ruling choice 3).
	pushSessionMarker = "DESK_LOOP"
	redactedText      = "[redacted]"
)

// pushSecret holds a token value. Every rendering is [redacted]; reveal is the only reader.
type pushSecret struct{ b []byte }

func (s pushSecret) String() string                { return redactedText }
func (s pushSecret) GoString() string              { return redactedText }
func (s pushSecret) Format(f fmt.State, _ rune)    { _, _ = io.WriteString(f, redactedText) }
func (s pushSecret) MarshalJSON() ([]byte, error)  { return []byte(`"` + redactedText + `"`), nil }
func (s pushSecret) MarshalText() ([]byte, error)  { return []byte(redactedText), nil }
func (s pushSecret) present() bool                 { return len(s.b) > 0 }
func (s *pushSecret) UnmarshalJSON(b []byte) error { return s.decode(b) }

func (s *pushSecret) decode(b []byte) error {
	var v *string
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if v != nil {
		s.b = []byte(*v)
	}
	return nil
}

// reveal is the one way to the bytes (see the file comment for its three consumers).
func (s pushSecret) reveal() string { return string(s.b) }

// sameAs compares two values in constant time without rendering either.
func (s pushSecret) sameAs(o pushSecret) bool {
	return subtle.ConstantTimeCompare([]byte(s.reveal()), []byte(o.reveal())) == 1
}

// wipe zeroes the bytes: best-effort hygiene, not a control (Threat model).
func (s *pushSecret) wipe() {
	for i := range s.b {
		s.b[i] = 0
	}
	s.b = nil
}

// pushVariable is a CI/CD variable response. Its value field is a pushSecret, never a string.
type pushVariable struct {
	Key       string     `json:"key"`
	Value     pushSecret `json:"value"`
	Masked    bool       `json:"masked"`
	Protected bool       `json:"protected"`
	Hidden    bool       `json:"hidden"`
	Scope     string     `json:"environment_scope"`
}

// pushTokenRecord is an access-token record (list, by-id, self). It has no value field.
type pushTokenRecord struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Active      bool     `json:"active"`
	Revoked     bool     `json:"revoked"`
	Scopes      []string `json:"scopes"`
	AccessLevel int      `json:"access_level"`
	ExpiresAt   string   `json:"expires_at"`
}

// pushIssued is a rotate or create response: the new id and the value, once.
type pushIssued struct {
	ID        int64      `json:"id"`
	Token     pushSecret `json:"token"`
	ExpiresAt string     `json:"expires_at"`
}

type pushOpts struct {
	mode       string // "rotate" | "check"
	project    string
	credFile   string // --rotator-token-file (rotate) or --reader-token-file (check)
	credFlag   string
	tokenName  string
	days       int
	minDays    int
	create     bool
	dryRun     bool
	apiBase    string
	sessionTag string // the session marker's value, as the command tree resolved it
}

// pushRun is one invocation's state.
type pushRun struct {
	o       pushOpts
	e       *env
	base    string // the API base, no trailing slash, ending in /api/v4
	web     *url.URL
	gl      *apiClient
	pp      string // /projects/<id>
	path    string // path_with_namespace, checked by pushPathRE
	today   time.Time
	want    time.Time // the requested expiry
	hidden  bool      // the hidden state the read-back must show
	varSeen bool      // the project-level variable exists
	oldID   int64
	newID   int64
	newExp  string
	secret  pushSecret
}

var pushPathRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*$`)

func (r *pushRun) outf(format string, a ...any) { fmt.Fprintf(r.e.stdout, format+"\n", a...) }
func (r *pushRun) errf(format string, a ...any) { fmt.Fprintf(r.e.stderr, format+"\n", a...) }

// runPushToken is the handler for both modes: the command tree has parsed the flags.
func runPushToken(o pushOpts, e *env) int {
	r := &pushRun{o: o, e: e}
	defer r.secret.wipe()
	if code := r.refusals(); code != exitOK {
		return code
	}
	r.outf("deskfleet push-token %s — target %s project=%s variable=%s token-name=%s", o.mode, r.target(),
		o.project, pushTokenVariable, o.tokenName)
	if o.mode == "check" {
		return r.check()
	}
	if o.dryRun {
		r.dryRunSteps()
		return exitOK
	}
	return r.rotate()
}

func (r *pushRun) target() string {
	if r.o.dryRun {
		return "(dry-run: no network)"
	}
	return r.base
}

// refusals are every precondition checked before any network contact: exit 5, zero requests.
func (r *pushRun) refusals() int {
	refuse := func(format string, a ...any) int {
		r.errf("deskfleet push-token %s: refused: "+format, append([]any{r.o.mode}, a...)...)
		return exitRefused
	}
	if strings.TrimSpace(r.o.project) == "" || strings.TrimSpace(r.o.tokenName) == "" {
		return refuse("--project and --token-name are required")
	}
	if r.o.mode == "rotate" && (r.o.days < 1 || r.o.days > pushRotateMaxDays) {
		return refuse("--days must be 1-%d (got %d)", pushRotateMaxDays, r.o.days)
	}
	if r.o.minDays < 1 || r.o.minDays > pushRotateMaxDays {
		return refuse("--min-days must be 1-%d (got %d)", pushRotateMaxDays, r.o.minDays)
	}
	if r.o.mode == "rotate" && r.o.minDays >= r.o.days {
		return refuse("--min-days (%d) must be below --days (%d)", r.o.minDays, r.o.days)
	}
	r.today = r.e.now().UTC().Truncate(24 * time.Hour)
	r.want = r.today.AddDate(0, 0, r.o.days)
	if r.o.dryRun {
		return exitOK
	}
	if r.o.mode == "rotate" && strings.TrimSpace(r.o.sessionTag) != "" {
		return refuse("the desk loop's session marker %s is set — rotate is run by the person who holds the "+
			"rotator credential, not by an automated session (an ADVISORY check: the marker is self-declared)",
			pushSessionMarker)
	}
	base, err := gitlabAPIBase(r.o.apiBase)
	if err != nil {
		return refuse("%v", err)
	}
	if !strings.HasSuffix(base, pushAPISuffix) {
		return refuse("GITLAB_API_BASE must end in %s (the push probe's web root is derived from it)", pushAPISuffix)
	}
	web, err := url.Parse(strings.TrimSuffix(base, pushAPISuffix))
	if err != nil || web.Host == "" || web.User != nil || web.RawQuery != "" || web.Fragment != "" {
		return refuse("GITLAB_API_BASE does not yield a plain web root for the push probe")
	}
	tok, err := readCredentialFile(r.o.credFlag, r.o.credFile, operatorNamedFile)
	if err != nil {
		return refuse("%v", err)
	}
	r.base, r.web = base, web
	r.gl = newGitLabClient(base, r.e.http, tok)
	return exitOK
}

func (r *pushRun) dryRunSteps() {
	steps := []string{
		"refusals: GITLAB_API_BASE (https or loopback, ending /api/v4), --rotator-token-file owner-only, --days 1-90, --min-days below --days, session marker",
		"preflight: read project " + r.o.project + " (default branch, path); the default branch must be protected",
		"preflight: read project-level variable " + pushTokenVariable + " — scope *, masked, protected; hidden state recorded" + orCreate(r.o.create, "; missing = create it"),
		"preflight: list access tokens — exactly one active named " + r.o.tokenName + ", scopes [" + pushTokenScope + "], level 40" + orCreate(r.o.create, "; none = create one"),
		"rotate (or create): new token, expires_at " + r.want.Format("2006-01-02"),
		"install: write " + pushTokenVariable + " with masked, protected and scope * sent explicitly (retry a transport error or 5xx twice)",
		"read-back: masked, protected, scope *, the recorded hidden state, and the value (constant-time) — a mismatch revokes the new token and confirms the revoke",
		fmt.Sprintf("proof by-id: active, not revoked, scopes [%s], expires_at >= requested-1d and >= %d days away", pushTokenScope, r.o.minDays),
		"proof self: GET /personal_access_tokens/self under the new token; id equals the new id",
		"proof push: GET <web root>/<project path>.git/info/refs?service=git-receive-pack under the new token answers 200",
	}
	for _, s := range steps {
		r.outf("dry-run: %s", s)
	}
	r.outf("dry-run: zero requests made")
}

func orCreate(create bool, s string) string {
	if create {
		return s
	}
	return ""
}

// rotate is the write mode.
func (r *pushRun) rotate() int {
	if code := r.preflight(); code != exitOK {
		return code
	}
	if code := r.issue(); code != exitOK {
		return code
	}
	r.outf("token: %s id %d expires_at %s", r.o.tokenName, r.newID, r.newExp)
	if code := r.install(); code != exitOK {
		return code
	}
	if code := r.readBack(); code != exitOK {
		return code
	}
	if code := r.prove(); code != exitOK {
		return code
	}
	r.outf("outcome: rotated — %s holds token %s id %d (expires_at %s), proven by-id, self and push",
		pushTokenVariable, r.o.tokenName, r.newID, r.newExp)
	return exitOK
}

// read is one preflight or check read: a transport failure or a 5xx is could-not-check.
func (r *pushRun) read(what, path string) (apiResponse, int) {
	resp, err := r.gl.do("GET", path, nil)
	if err != nil {
		r.errf("could-not-check: %s read failed (%v) — nothing written", what, err)
		return resp, exitUnverifiable
	}
	if resp.Status >= 500 || (resp.Status >= 300 && resp.Status < 400) {
		r.errf("could-not-check: %s read answered HTTP %d — nothing written", what, resp.Status)
		return resp, exitUnverifiable
	}
	return resp, exitOK
}

func (r *pushRun) refuse(format string, a ...any) int {
	r.errf("refused: "+format+" — nothing written", a...)
	return exitRefused
}

func (r *pushRun) preflight() int {
	resp, code := r.read("project", "/projects/"+url.PathEscape(r.o.project))
	if code != exitOK {
		return code
	}
	if resp.Status != 200 {
		return r.refuse("project %s: HTTP %d", r.o.project, resp.Status)
	}
	var pr struct {
		ID            int64  `json:"id"`
		Path          string `json:"path_with_namespace"`
		DefaultBranch string `json:"default_branch"`
	}
	if json.Unmarshal(resp.Body, &pr) != nil || pr.ID == 0 {
		return r.refuse("project %s: the response carries no id", r.o.project)
	}
	if !pushPathRE.MatchString(pr.Path) || strings.Contains("/"+pr.Path+"/", "/../") || strings.Contains(pr.Path, "/./") {
		return r.refuse("project %s: path_with_namespace is not a plain project path", r.o.project)
	}
	if pr.DefaultBranch == "" {
		return r.refuse("project %s: no default branch", r.o.project)
	}
	r.pp, r.path = "/projects/"+strconv.FormatInt(pr.ID, 10), pr.Path
	r.outf("project: %s (id %d) default branch %s", pr.Path, pr.ID, pr.DefaultBranch)

	resp, code = r.read("protected branch", r.pp+"/protected_branches/"+url.PathEscape(pr.DefaultBranch))
	if code != exitOK {
		return code
	}
	if resp.Status != 200 {
		return r.refuse("branch: default branch %s is not protected (HTTP %d) — a protected variable never reaches "+
			"the regen job there", pr.DefaultBranch, resp.Status)
	}

	if code := r.preflightVariable(); code != exitOK {
		return code
	}
	return r.preflightToken()
}

func (r *pushRun) varPath() string { return r.pp + "/variables/" + url.PathEscape(pushTokenVariable) }

func (r *pushRun) preflightVariable() int {
	resp, code := r.read("variable "+pushTokenVariable, r.varPath())
	if code != exitOK {
		return code
	}
	switch resp.Status {
	case 200:
	case 404:
		if !r.o.create {
			return r.refuse("variable: project-level %s does not exist (a group-level variable of that key does "+
				"not count); re-run with --create to create it", pushTokenVariable)
		}
		r.outf("variable: project-level %s absent — --create will create it masked, protected and hidden "+
			"where the instance supports hidden", pushTokenVariable)
		return exitOK
	default:
		return r.refuse("variable: %s read answered HTTP %d", pushTokenVariable, resp.Status)
	}
	// The predecessor's value decodes into a pushSecret and is never read; it is wiped on return.
	var v pushVariable
	if json.Unmarshal(resp.Body, &v) != nil {
		return r.refuse("variable: %s response could not be decoded", pushTokenVariable)
	}
	defer v.Value.wipe()
	if !v.Masked {
		return r.refuse("variable: %s is not masked (field masked)", pushTokenVariable)
	}
	if !v.Protected {
		return r.refuse("variable: %s is not protected (field protected)", pushTokenVariable)
	}
	if v.Scope != "*" {
		return r.refuse("variable: %s environment_scope is not * (field environment_scope)", pushTokenVariable)
	}
	r.varSeen, r.hidden = true, v.Hidden
	r.outf("variable: %s scope=* masked=true protected=true hidden=%t", pushTokenVariable, v.Hidden)
	return exitOK
}

// activeByName lists the project's access tokens and keeps the active, unrevoked ones named
// --token-name.
func (r *pushRun) activeByName() ([]pushTokenRecord, int) {
	resp, code := r.read("access-token list", r.pp+"/access_tokens?per_page=100")
	if code != exitOK {
		return nil, code
	}
	var all []pushTokenRecord
	if resp.Status != 200 || json.Unmarshal(resp.Body, &all) != nil {
		r.errf("could-not-check: access-token list answered HTTP %d or could not be decoded — nothing written", resp.Status)
		return nil, exitUnverifiable
	}
	var hits []pushTokenRecord
	for _, t := range all {
		if t.Name == r.o.tokenName && t.Active && !t.Revoked {
			hits = append(hits, t)
		}
	}
	return hits, exitOK
}

// scopesExact is true only for a scope list that EQUALS [write_repository].
func scopesExact(s []string) bool {
	return len(s) == 1 && s[0] == pushTokenScope
}

func (r *pushRun) preflightToken() int {
	hits, code := r.activeByName()
	if code != exitOK {
		return code
	}
	switch {
	case len(hits) == 0 && r.o.create:
		r.outf("token: no active %s — --create will create one", r.o.tokenName)
		return exitOK
	case len(hits) == 0:
		return r.refuse("token: zero active access tokens named %s (an expired token is no longer active); "+
			"re-run with --create", r.o.tokenName)
	case len(hits) > 1:
		return r.refuse("token: the forge holds %d active tokens named %s — more than one valid push credential; "+
			"revoke the extras by hand", len(hits), r.o.tokenName)
	}
	t := hits[0]
	if !scopesExact(t.Scopes) {
		return r.refuse("token: %s id %d scopes are not exactly [%s] (field scopes)", t.Name, t.ID, pushTokenScope)
	}
	if t.AccessLevel != pushTokenAccessLevel {
		return r.refuse("token: %s id %d access_level is %d, not %d (field access_level)", t.Name, t.ID,
			t.AccessLevel, pushTokenAccessLevel)
	}
	r.oldID = t.ID
	r.outf("token: %s id %d scopes=[%s] access_level=%d — rotating it", t.Name, t.ID, pushTokenScope, pushTokenAccessLevel)
	return exitOK
}

// issue rotates the one active token, or (with --create and none) creates one.
func (r *pushRun) issue() int {
	expires := r.want.Format("2006-01-02")
	var (
		resp apiResponse
		err  error
		verb = "rotate"
		want = 200
	)
	if r.oldID == 0 {
		verb, want = "create", 201
		resp, err = r.gl.do("POST", r.pp+"/access_tokens", map[string]any{"name": r.o.tokenName,
			"scopes": []string{pushTokenScope}, "access_level": pushTokenAccessLevel, "expires_at": expires})
	} else {
		resp, err = r.gl.do("POST", r.pp+"/access_tokens/"+strconv.FormatInt(r.oldID, 10)+"/rotate",
			map[string]any{"expires_at": expires})
	}
	if err == nil && resp.Status >= 400 && resp.Status < 500 {
		r.errf("rotate-refused: the forge refused the %s (HTTP %d) — no variable write followed; a concurrent "+
			"run may already have rotated this token, and `push-token check` confirms its install", verb, resp.Status)
		return exitFailed
	}
	var got pushIssued
	if err != nil || resp.Status != want || json.Unmarshal(resp.Body, &got) != nil || got.ID == 0 || !got.Token.present() {
		r.errf("rotate-response-lost: the %s was sent but its answer was lost or unreadable (%s) — the predecessor "+
			"may already be revoked and the new value is unrecoverable; re-run `push-token rotate` (it finds the one "+
			"active token by name and rotates it again)", verb, lostCause(resp, err))
		return exitFailed
	}
	r.newID, r.secret = got.ID, got.Token
	r.newExp = got.ExpiresAt
	if r.newExp == "" {
		r.newExp = expires
	}
	return exitOK
}

func lostCause(resp apiResponse, err error) string {
	if err != nil {
		return "transport error"
	}
	return fmt.Sprintf("HTTP %d", resp.Status)
}

// install writes the new value, sending masked, protected and scope * on every write.
func (r *pushRun) install() int {
	body := map[string]any{"value": r.secret.reveal(), "masked": true, "protected": true, "environment_scope": "*"}
	method, path, want := "PUT", r.varPath(), 200
	if !r.varSeen {
		// ruling choice 5: hidden through the create path where the instance supports it.
		body["key"], body["masked_and_hidden"] = pushTokenVariable, true
		method, path, want = "POST", r.pp+"/variables", 201
	}
	var (
		resp apiResponse
		err  error
	)
	for attempt := 0; attempt <= pushRetries; attempt++ {
		resp, err = r.gl.do(method, path, body)
		if err == nil && resp.Status < 500 {
			break
		}
	}
	delete(body, "value")
	if err != nil || resp.Status != want {
		r.errf("rotated-not-installed: the variable write failed (%s) — %s still holds the revoked predecessor "+
			"(or does not exist); token %s id %d is active and found by name, so re-run `push-token rotate`",
			statusOnly(resp, err), pushTokenVariable, r.o.tokenName, r.newID)
		return exitFailed
	}
	if !r.varSeen {
		var v pushVariable
		_ = json.Unmarshal(resp.Body, &v)
		v.Value.wipe()
		r.hidden = v.Hidden
	}
	r.outf("install: %s written (%s) masked=true protected=true scope=*", pushTokenVariable, method)
	return exitOK
}

// readBack re-reads the variable; any disagreement is contained.
func (r *pushRun) readBack() int {
	resp, err := r.gl.do("GET", r.varPath(), nil)
	var v pushVariable
	if err != nil || resp.Status != 200 || json.Unmarshal(resp.Body, &v) != nil {
		return r.contain("read-back (" + statusOnly(resp, err) + ")")
	}
	defer v.Value.wipe()
	field := ""
	switch {
	case !v.Masked:
		field = "masked"
	case !v.Protected:
		field = "protected"
	case v.Scope != "*":
		field = "environment_scope"
	case v.Hidden != r.hidden:
		field = "hidden"
	case v.Value.present() && !v.Value.sameAs(r.secret):
		field = "value"
	}
	if field != "" {
		return r.contain(field)
	}
	r.outf("read-back: %s masked=true protected=true scope=* hidden=%t value=%s", pushTokenVariable, r.hidden,
		valueCheck(v.Value.present()))
	return exitOK
}

func valueCheck(returned bool) string {
	if returned {
		return "matches (constant-time)"
	}
	return "not returned (hidden)"
}

// contain revokes the new token at once and confirms the revoke by a by-id read.
func (r *pushRun) contain(field string) int {
	id := strconv.FormatInt(r.newID, 10)
	resp, err := r.gl.do("DELETE", r.pp+"/access_tokens/"+id, nil)
	if err != nil || resp.Status < 200 || resp.Status >= 300 {
		return r.containFailed(field, "revoke "+statusOnly(resp, err))
	}
	resp, err = r.gl.do("GET", r.pp+"/access_tokens/"+id, nil)
	var rec pushTokenRecord
	switch {
	case err != nil:
		return r.containFailed(field, "confirming read "+statusOnly(resp, err))
	case resp.Status == 404:
	case resp.Status == 200 && json.Unmarshal(resp.Body, &rec) == nil && (rec.Revoked || !rec.Active):
	default:
		return r.containFailed(field, "confirming read did not show the token dead ("+statusOnly(resp, err)+")")
	}
	r.errf("contained: revoked %d — the read-back disagreed on %s; %s now holds a dead value: fix the variable's "+
		"flags and re-run `push-token rotate --create`", r.newID, field, pushTokenVariable)
	return exitFailed
}

func (r *pushRun) containFailed(field, why string) int {
	r.errf("containment-failed: revoke %d by hand — the read-back disagreed on %s and the token's death could not "+
		"be verified (%s); %s may hold a LIVE token with wrong flags", r.newID, field, why, pushTokenVariable)
	return exitUnverifiable
}

// byIDVerdict is the comparator rotate's by-id probe and check share. want is the requested
// expiry (zero in check). It returns the first field that fails, or "".
func byIDVerdict(t pushTokenRecord, today, want time.Time, minDays int) (string, int) {
	if !t.Active {
		return "active", 0
	}
	if t.Revoked {
		return "revoked", 0
	}
	if !scopesExact(t.Scopes) {
		return "scopes", 0
	}
	exp, err := time.Parse("2006-01-02", t.ExpiresAt)
	if err != nil {
		return "expires_at", 0
	}
	left := int(exp.Sub(today).Hours() / 24)
	if !want.IsZero() && exp.Before(want.AddDate(0, 0, -1)) {
		return "expires_at", left
	}
	if left < minDays {
		return "expires_at", left
	}
	return "", left
}

func (r *pushRun) byID(gl *apiClient, id int64, want time.Time) int {
	resp, err := gl.do("GET", r.pp+"/access_tokens/"+strconv.FormatInt(id, 10), nil)
	var t pushTokenRecord
	if err != nil || resp.Status != 200 || json.Unmarshal(resp.Body, &t) != nil {
		r.errf("by-id: could-not-check — the token record read failed (%s)", statusOnly(resp, err))
		return exitUnverifiable
	}
	field, left := byIDVerdict(t, r.today, want, r.o.minDays)
	if field != "" {
		r.errf("by-id: FAIL field %s — id %d active=%t revoked=%t scopes ok=%t expires_at=%s (%d days left, "+
			"--min-days %d)", field, id, t.Active, t.Revoked, scopesExact(t.Scopes), t.ExpiresAt, left, r.o.minDays)
		return exitFailed
	}
	r.outf("by-id: ok — id %d active=true revoked=false scopes=[%s] expires_at=%s (%d days left, --min-days %d)",
		id, pushTokenScope, t.ExpiresAt, left, r.o.minDays)
	return exitOK
}

// prove runs the three probes; rotate exits 0 only if all pass.
func (r *pushRun) prove() int {
	if code := r.byID(r.gl, r.newID, r.want); code != exitOK {
		if code == exitUnverifiable {
			return exitFailed
		}
		return code
	}
	if code := r.proveSelf(); code != exitOK {
		return code
	}
	return r.provePush()
}

func (r *pushRun) proveSelf() int {
	self := newGitLabClient(r.base, r.e.http, r.secret.reveal())
	resp, err := self.do("GET", "/personal_access_tokens/self", nil)
	if err == nil && resp.Status == 403 && insufficientScope(resp.Body) {
		// ruling choice 6A: the by-id read above and the push probe below stand as the proof.
		r.outf("self: not-permitted-by-scope — accepted (ruling choice 6A) beside the by-id read and the push probe")
		return exitOK
	}
	var t pushTokenRecord
	if err != nil || resp.Status != 200 || json.Unmarshal(resp.Body, &t) != nil {
		r.errf("self: FAIL — the new token's self-introspection failed (%s)", statusOnly(resp, err))
		return exitFailed
	}
	if t.ID != r.newID {
		r.errf("self: FAIL field id — the record answered is not the new token's (id %d, want %d)", t.ID, r.newID)
		return exitFailed
	}
	if field, left := byIDVerdict(t, r.today, r.want, r.o.minDays); field != "" {
		r.errf("self: FAIL field %s — id %d (%d days left)", field, t.ID, left)
		return exitFailed
	}
	r.outf("self: ok — id %d equals the new id; active=true revoked=false scopes=[%s] expires_at=%s",
		t.ID, pushTokenScope, t.ExpiresAt)
	return exitOK
}

// insufficientScope is true for GitLab's 403 that names insufficient scope.
func insufficientScope(body []byte) bool {
	var m struct {
		Error string `json:"error"`
	}
	return json.Unmarshal(body, &m) == nil && m.Error == "insufficient_scope"
}

// pushProbeURL is the receive-pack advertisement on the configured host: the web root keeps
// the base's scheme, host and path prefix; nothing from a response but the checked project
// path enters it.
func (r *pushRun) pushProbeURL() string {
	u := *r.web
	u.Path = strings.TrimRight(u.Path, "/") + "/" + r.path + ".git/info/refs"
	u.RawQuery = "service=git-receive-pack"
	return u.String()
}

func (r *pushRun) provePush() int {
	req, err := http.NewRequest("GET", r.pushProbeURL(), nil)
	if err != nil {
		r.errf("push: FAIL — the probe request could not be built")
		return exitFailed
	}
	if req.URL.Host != r.web.Host {
		r.errf("push: FAIL — the probe URL left the configured host")
		return exitFailed
	}
	req.SetBasicAuth(pushProbeUser, r.secret.reveal())
	resp, err := noRedirects(r.e.http).Do(req)
	if err != nil {
		r.errf("push: FAIL — the receive-pack probe failed in transport")
		return exitFailed
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		r.errf("push: FAIL field status — the receive-pack advertisement answered HTTP %d (want 200; a redirect "+
			"is never followed)", resp.StatusCode)
		return exitFailed
	}
	r.outf("push: ok — receive-pack advertisement on the API host answered 200 under the new token")
	return exitOK
}

// check is the read-only mode: by-id probe only, under the reader credential.
func (r *pushRun) check() int {
	resp, code := r.read("project", "/projects/"+url.PathEscape(r.o.project))
	if code != exitOK {
		return code
	}
	var pr struct {
		ID int64 `json:"id"`
	}
	if resp.Status != 200 || json.Unmarshal(resp.Body, &pr) != nil || pr.ID == 0 {
		r.errf("could-not-check: project %s answered HTTP %d or carries no id", r.o.project, resp.Status)
		return exitUnverifiable
	}
	r.pp = "/projects/" + strconv.FormatInt(pr.ID, 10)
	hits, code := r.activeByName()
	if code != exitOK {
		return code
	}
	switch {
	case len(hits) == 0:
		r.errf("no-active-token: zero active access tokens named %s — rotate with --create", r.o.tokenName)
		return exitFailed
	case len(hits) > 1:
		r.errf("check: %d active tokens named %s — more than one valid push credential; revoke the extras by hand",
			len(hits), r.o.tokenName)
		return exitFailed
	}
	if code := r.byID(r.gl, hits[0].ID, time.Time{}); code != exitOK {
		return code
	}
	r.outf("outcome: healthy — %s id %d", r.o.tokenName, hits[0].ID)
	return exitOK
}
