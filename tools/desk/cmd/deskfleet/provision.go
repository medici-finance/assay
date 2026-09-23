package main

// provision.go — `deskfleet provision`: the account / membership / token loop, the ruled
// custody handling of each token file, the partial-run report, and the closing summary.
// Project settings live in project.go; labels in labels.go.

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type provisionOpts struct {
	group, prefix, project string
	ownerTokenFile         string
	outDir                 string
	patExpiryDays          int
	dryRun                 bool
	// avatarsDir: where <role>.png lives. Unset = the avatar step is SKIPPED, with a NOTICE
	// and a summary line. There is deliberately no default remote fetch (the script's
	// default downloads public role icons): the port adds no network default.
	avatarsDir  string
	avatarsOnly bool
}

// banner / ruleLine frame the closing summary and the partial-run report.
var (
	banner   = strings.Repeat("=", 60)
	ruleLine = strings.Repeat("-", 60)
)

var prefixRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func parseProvisionFlags(args []string, e *env) (provisionOpts, error) {
	var o provisionOpts
	fs := flag.NewFlagSet("deskfleet provision", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	fs.StringVar(&o.group, "group", "", "top-level GitLab group (path or id) that owns the fleet")
	fs.StringVar(&o.prefix, "prefix", "", "username prefix: <prefix>-<role>-bot")
	fs.StringVar(&o.project, "project", "", "fleet project (path or id) whose settings and labels to configure")
	fs.StringVar(&o.ownerTokenFile, "owner-token-file", "", "FILE holding a group-owner PAT (owner-only; never a flag value)")
	fs.StringVar(&o.outDir, "out-dir", "", "where gitlab-<role>.token files are written (default: the config home)")
	fs.IntVar(&o.patExpiryDays, "pat-expiry-days", 7, "PAT lifetime in days (1-365)")
	fs.BoolVar(&o.dryRun, "dry-run", false, "enumerate every action; make zero network calls")
	fs.StringVar(&o.avatarsDir, "avatars-dir", "", "upload <dir>/<role>.png as each account's own avatar "+
		"(PUT /user/avatar, as that role's own PAT); omitted = the avatar step is skipped")
	fs.BoolVar(&o.avatarsOnly, "avatars-only", false, "set the avatars of accounts that already exist, as each "+
		"role, from the gitlab-<role>.token files under --out-dir; creates, mints and configures nothing")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() > 0 {
		return o, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if o.avatarsOnly {
		if o.avatarsDir == "" {
			return o, errors.New("--avatars-only requires --avatars-dir <dir holding <role>.png>")
		}
		if o.prefix == "" {
			return o, errors.New("--avatars-only requires --prefix (it names the accounts)")
		}
		if o.project != "" || o.ownerTokenFile != "" {
			return o, errors.New("--avatars-only authenticates as each role from its own token file and touches " +
				"no project setting — it takes no --project and no --owner-token-file")
		}
	} else if o.group == "" || o.prefix == "" {
		return o, errors.New("--group and --prefix are required")
	}
	if !prefixRE.MatchString(o.prefix) {
		return o, fmt.Errorf("--prefix %q must be letters, digits, '_', '.' or '-', starting with a letter or digit", o.prefix)
	}
	if o.patExpiryDays < 1 || o.patExpiryDays > 365 {
		return o, fmt.Errorf("--pat-expiry-days must be 1-365 (got %d)", o.patExpiryDays)
	}
	if o.outDir == "" {
		o.outDir = deskkit.ConfigHomeDirs()[0]
	}
	return o, nil
}

// mintedToken is one PAT this run minted. It deliberately has NO field for the token value:
// the report is built from this struct, so it cannot print a credential by construction.
type mintedToken struct {
	Role, Username string
	UserID         int64
	TokenName      string
	TokenID        int64
	ExpiresAt      string
	Path           string // "" when the token never reached disk
	Custody        string // verified | WARNING-inconclusive | REFUSED | NOT WRITTEN
}

// accountNote is a service account the partial-run report must name although no minted
// token row covers it. Like mintedToken it has no field for a credential.
type accountNote struct {
	Role, Username string
	UserID         int64 // 0 when the forge never told us
	Detail         string
}

// patResponse is the mint endpoint's reply. Token is the credential: it is copied to the
// custody file and cleared, and never stored anywhere else.
type patResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type provisioner struct {
	o        provisionOpts
	e        *env
	gl       *apiClient
	base     string
	groupID  int64
	tier     string // premium | free | unknown
	minted   []mintedToken
	warnings []string
	failures []string
	// tokenless: service accounts this run CREATED that DEFINITELY hold no token from it (the
	// stop hit between account creation and the mint, or the forge refused the mint). A
	// re-run will not mint for them, so the report names each.
	tokenless []accountNote
	// unknown: requests whose OUTCOME this run could not check — a mint or an account create
	// that was sent but whose reply was lost, unreadable, or a server error. The forge may
	// have acted, so the report names each as could-not-check, never as "nothing to revoke".
	unknown []accountNote
	// boardWriterID is the board-writer service account's user id, for the Premium
	// protected-branch push allowlist; 0 when unknown.
	boardWriterID int64
}

func (p *provisioner) outf(format string, a ...any) { fmt.Fprintf(p.e.stdout, format+"\n", a...) }
func (p *provisioner) errf(format string, a ...any) { fmt.Fprintf(p.e.stderr, format+"\n", a...) }

// fail records a SETTINGS-step failure: every settings step still runs, and the run exits
// non-zero at the end with each failure named.
func (p *provisioner) fail(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	p.failures = append(p.failures, msg)
	p.errf("error: %s", msg)
}

// stopError ends the account/token loop. code is the process exit status.
type stopError struct {
	code int
	msg  string
}

func (s *stopError) Error() string { return s.msg }

func stop(format string, a ...any) *stopError {
	return &stopError{code: exitFailed, msg: fmt.Sprintf(format, a...)}
}

func cmdProvision(args []string, e *env) int {
	o, err := parseProvisionFlags(args, e)
	if err != nil {
		fmt.Fprintf(e.stderr, "deskfleet provision: %v\n", err)
		return exitUsage
	}
	p := &provisioner{o: o, e: e}
	p.outf("Assay fleet provisioning — group=%s prefix=%s project=%s dry-run=%t avatars-only=%t", orNone(o.group),
		o.prefix, orNone(o.project), o.dryRun, o.avatarsOnly)
	if o.avatarsOnly {
		return p.runAvatarsOnly()
	}
	if o.dryRun {
		p.dryRun()
		return exitOK
	}

	// Preconditions — every one is checked BEFORE the first network contact.
	base, err := gitlabAPIBase(e)
	if err != nil {
		p.errf("refused: %v", err)
		return exitRefused
	}
	owner, err := readCredentialFile("--owner-token-file", o.ownerTokenFile)
	if err != nil {
		p.errf("refused: %v", err)
		return exitRefused
	}
	if err := os.MkdirAll(o.outDir, 0o700); err != nil {
		p.errf("refused: cannot create --out-dir %s: %v", o.outDir, err)
		return exitRefused
	}
	p.base = base
	p.gl = newGitLabClient(base, e.http, owner)
	p.outf("target: %s (GITLAB_API_BASE) — token files go to %s", base, o.outDir)
	if o.avatarsDir == "" {
		p.outf("NOTICE: %s", avatarsSkippedNotice(o))
	}

	if serr := p.resolveGroup(); serr != nil {
		return p.stopRun(serr)
	}
	for _, r := range fleetRoles {
		if serr := p.provisionRole(r); serr != nil {
			return p.stopRun(serr)
		}
	}

	if o.project != "" {
		p.configureProject()
	} else {
		p.outf("NOTICE: --project not given — protected-branch, approval, tag, merge-check and label " +
			"settings skipped (could-not-check, not a pass); pass --project to configure them")
	}
	return p.summary()
}

func orNone(s string) string {
	if s == "" {
		return "<none>"
	}
	return s
}

func (p *provisioner) dryRun() {
	if strings.TrimSpace(p.e.getenv("GITLAB_API_BASE")) == "" {
		p.outf("[dry-run] NOTICE: GITLAB_API_BASE is not set — a real run would refuse before any network contact")
	}
	for _, r := range fleetRoles {
		user := serviceAccountUsername(p.o.prefix, r.Role)
		scopes := strings.Join(r.Scopes, ",")
		p.outf("[dry-run] would create service account %s (role=%s, access=%s (%d), scopes=%s)",
			user, r.Role, r.AccessName, r.AccessLevel, scopes)
		p.outf("[dry-run]   would ensure group membership at access_level=%d", r.AccessLevel)
		p.outf("[dry-run]   would mint a PAT %s (scopes=%s, expires in %dd) if the account is new, "+
			"written owner-only to %s and verified before its path is reported",
			patName(r.Role), scopes, p.o.patExpiryDays, filepath.Join(p.o.outDir, tokenFileName(r.Role)))
		if p.o.avatarsDir != "" {
			p.outf("[dry-run]   would upload avatar for %s <- %s (PUT /user/avatar, as that account's own PAT)",
				user, filepath.Join(p.o.avatarsDir, r.Role+".png"))
		}
	}
	if p.o.avatarsDir == "" {
		p.outf("[dry-run] NOTICE: %s", avatarsSkippedNotice(p.o))
	}
	if p.o.project == "" {
		p.outf("[dry-run] NOTICE: --project not given — project settings and labels would be skipped")
		return
	}
	p.outf("[dry-run] would protect branch 'main' on project %s: push = board-writer only (Premium) or no one "+
		"(Free), merge = Maintainers (%d), force-push off — never leaving main unprotected across a failure",
		p.o.project, mergeAccessLevel)
	p.outf("[dry-run] would set approvals: merge_requests_author_approval=false, merge_requests_disable_committers_approval=true")
	p.outf("[dry-run] would protect tags '%s' with create_access_level=%d (Maintainers)", protectedTagGlob, protectedTagCreateLevel)
	p.outf("[dry-run] would set only_allow_merge_if_pipeline_succeeds=true and only_allow_merge_if_all_discussions_are_resolved=true")
	for _, l := range fleetLabels {
		p.outf("[dry-run] would create project label '%s' (#%s, idempotent)", l.Name, l.Color)
	}
}

func (p *provisioner) resolveGroup() *stopError {
	path := "/groups/" + url.PathEscape(p.o.group)
	r, err := p.gl.do("GET", path, nil)
	if err != nil {
		return stop("could not resolve group %s: %v", p.o.group, err)
	}
	if r.Status != 200 {
		return stop("could not resolve group %s (%s)", p.o.group, statusText(r))
	}
	var g struct {
		ID   int64  `json:"id"`
		Plan string `json:"plan"`
	}
	if err := json.Unmarshal(r.Body, &g); err != nil || g.ID == 0 {
		return stop("group %s: response carries no id", p.o.group)
	}
	p.groupID = g.ID
	if g.Plan != "" {
		p.tier = classifyPlan(g.Plan)
		p.outf("tier: group plan='%s' -> %s", g.Plan, p.tier)
	} else {
		p.tier = "unknown"
		p.outf("tier: group response carries no 'plan' field — will probe (Premium form first, free-tier form on a 400 naming allowed_to_)")
	}
	return nil
}

func classifyPlan(plan string) string {
	switch plan {
	case "premium", "ultimate", "gold", "silver":
		return "premium"
	case "free", "default", "bronze", "":
		return "free"
	}
	return "unknown"
}

// provisionRole runs one role: service account, membership, and — for an account created
// by this run — the PAT, its custody write and its read-back. Any failure STOPS the loop.
func (p *provisioner) provisionRole(r fleetRole) *stopError {
	user := serviceAccountUsername(p.o.prefix, r.Role)
	gpath := "/groups/" + strconv.FormatInt(p.groupID, 10)

	// Service account: idempotent on username.
	resp, err := p.gl.do("GET", gpath+"/service_accounts?per_page=100", nil)
	if err != nil {
		return stop("listing service accounts (role %s): %v", r.Role, err)
	}
	if resp.Status != 200 {
		return stop("listing service accounts (role %s): %s", r.Role, statusText(resp))
	}
	var accounts []struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(resp.Body, &accounts); err != nil {
		return stop("listing service accounts (role %s): unreadable response", r.Role)
	}
	var userID int64
	for _, a := range accounts {
		if a.Username == user {
			userID = a.ID
			break
		}
	}
	createdNow := false
	if userID != 0 {
		p.outf("no-op: service account %s already exists (id=%d)", user, userID)
	} else {
		body := map[string]any{"name": "Assay " + r.Role + " (fleet bot)", "username": user}
		resp, err := p.gl.do("POST", gpath+"/service_accounts", body)
		if outcomeUnknown(resp, err, 201) {
			p.unknown = append(p.unknown, accountNote{Role: r.Role, Username: user,
				Detail: "the service-account create was sent but its outcome is unknown (" + respOrErr(resp, err) +
					") — the account may EXIST with no token; check the group's service accounts"})
			return stop("creating service account %s: %s", user, respOrErr(resp, err))
		}
		if resp.Status != 201 {
			return stop("creating service account %s: %s", user, statusText(resp))
		}
		var a struct {
			ID int64 `json:"id"`
		}
		if json.Unmarshal(resp.Body, &a) != nil || a.ID == 0 {
			p.tokenless = append(p.tokenless, accountNote{Role: r.Role, Username: user,
				Detail: "the forge answered 201 to its create but its reply carried no id"})
			return stop("creating service account %s: response carries no id", user)
		}
		userID, createdNow = a.ID, true
		p.outf("created: service account %s (id=%d)", user, userID)
	}
	if r.Role == "board-writer" {
		p.boardWriterID = userID
	}
	// stopCreated ends the loop for an account this run created: it is recorded as holding
	// no token, because a re-run will find it existing and mint nothing for it.
	stopCreated := func(s *stopError) *stopError {
		if createdNow {
			p.tokenless = append(p.tokenless, accountNote{Role: r.Role, Username: user, UserID: userID,
				Detail: "the run stopped at its group-membership step, before its token was minted"})
		}
		return s
	}

	// Group membership: idempotent existence check.
	mpath := gpath + "/members/" + strconv.FormatInt(userID, 10)
	resp, err = p.gl.do("GET", mpath, nil)
	if err != nil {
		return stopCreated(stop("reading group membership of %s: %v", user, err))
	}
	if resp.Status == 200 {
		var m struct {
			AccessLevel int `json:"access_level"`
		}
		_ = json.Unmarshal(resp.Body, &m)
		if m.AccessLevel != r.AccessLevel {
			p.outf("NOTICE: %s is already a group member at access_level=%d, expected %d — not changed "+
				"automatically, check by hand", user, m.AccessLevel, r.AccessLevel)
		} else {
			p.outf("no-op: %s already a group member at access_level=%d", user, r.AccessLevel)
		}
	} else {
		body := map[string]any{"user_id": userID, "access_level": r.AccessLevel}
		resp, err := p.gl.do("POST", gpath+"/members", body)
		if err != nil {
			return stopCreated(stop("adding %s to the group: %v", user, err))
		}
		if resp.Status != 201 {
			return stopCreated(stop("adding %s to the group: %s", user, statusText(resp)))
		}
		p.outf("added: %s to group at access_level=%d", user, r.AccessLevel)
	}

	// PAT: minted ONLY for an account created by this run. A fresh credential for an
	// existing account is a rotation (desktoken --forge gitlab <role>), not a re-provision.
	if !createdNow {
		p.outf("NOTICE: PAT minting skipped for %s (account pre-existing) — rotate via "+
			"`desktoken --forge gitlab %s` for a fresh credential", user, r.Role)
		if p.o.avatarsDir != "" {
			p.uploadAvatar(r.Role, user)
		}
		return nil
	}
	serr := p.mintAndStore(r, user, userID)
	if serr != nil && !p.accountedFor(user) {
		// Created by this run, and the forge DEFINITELY minted no token for it: a re-run will
		// NOT mint one (it only mints for accounts it creates), so the report names it.
		p.tokenless = append(p.tokenless, accountNote{Role: r.Role, Username: user, UserID: userID,
			Detail: "the forge refused its token mint"})
	}
	if serr == nil && p.o.avatarsDir != "" {
		// The avatar goes on now, as the account itself (only it can set its own avatar).
		p.uploadAvatar(r.Role, user)
	}
	return serr
}

// accountedFor reports whether the partial-run report already names user — as a minted
// token (of any custody state) or as a could-not-check outcome.
func (p *provisioner) accountedFor(user string) bool {
	for _, m := range p.minted {
		if m.Username == user {
			return true
		}
	}
	for _, u := range p.unknown {
		if u.Username == user {
			return true
		}
	}
	return false
}

// outcomeUnknown reports whether a WRITE's result cannot be known from its reply: a transport
// error (the request may have been sent), a reply that could not be read, or any status that
// is neither the expected success nor a 4xx — a server error, an unexpected 2xx, or a
// redirect this client never follows. The forge may have acted on such a request. Only the
// expected status is a success, and only a 4xx is a definite refusal.
func outcomeUnknown(r apiResponse, err error, want int) bool {
	if err != nil {
		return true
	}
	return r.Status != want && (r.Status < 400 || r.Status >= 500)
}

func (p *provisioner) mintAndStore(r fleetRole, user string, userID int64) *stopError {
	expires := p.e.now().UTC().AddDate(0, 0, p.o.patExpiryDays).Format("2006-01-02")
	body := map[string]any{"name": patName(r.Role), "scopes": r.Scopes, "expires_at": expires}
	path := "/groups/" + strconv.FormatInt(p.groupID, 10) + "/service_accounts/" +
		strconv.FormatInt(userID, 10) + "/personal_access_tokens"
	resp, err := p.gl.do("POST", path, body)
	if outcomeUnknown(resp, err, 201) {
		// The mint may have happened: the token could EXIST on the forge although this run
		// never saw it. That is could-not-check, never "nothing to revoke". Only the status
		// or the transport cause is named — never this endpoint's body.
		cause := fmt.Sprintf("HTTP %d", resp.Status)
		if err != nil {
			cause = err.Error()
		}
		p.unknown = append(p.unknown, accountNote{Role: r.Role, Username: user, UserID: userID,
			Detail: "its token mint (token name " + patName(r.Role) + ") was sent but its outcome is unknown (" + cause +
				") — a token may EXIST; list this account's personal access tokens and revoke " + patName(r.Role) +
				" if it is there"})
		return stop("minting a PAT for %s: outcome unknown (%s)", user, cause)
	}
	if resp.Status != 201 {
		// The forge's message is deliberately NOT echoed for this endpoint.
		return stop("minting a PAT for %s: HTTP %d", user, resp.Status)
	}
	var pat patResponse
	if json.Unmarshal(resp.Body, &pat) != nil {
		pat = patResponse{}
	}
	m := mintedToken{Role: r.Role, Username: user, UserID: userID, TokenName: patName(r.Role),
		TokenID: pat.ID, ExpiresAt: expires}
	if pat.Name != "" {
		m.TokenName = pat.Name
	}
	if pat.ExpiresAt != "" {
		m.ExpiresAt = pat.ExpiresAt
	}
	if pat.Token == "" {
		m.Custody = "NOT WRITTEN (the mint response carried no token value)"
		p.minted = append(p.minted, m)
		return stop("minting a PAT for %s: HTTP 201 but the response carried no token value", user)
	}

	written, werr := writeTokenFile(p.e.createRestricted, p.o.outDir, r.Role, pat.Token)
	pat.Token = ""
	if werr != nil {
		m.Custody = "NOT WRITTEN (" + werr.Error() + ")"
		p.minted = append(p.minted, m)
		return stop("writing the PAT for %s to disk: %v", user, werr)
	}
	m.Path = written

	// Layer 2: read the file back through the deskkit owner-only evaluation BEFORE the
	// path is reported as usable.
	v := p.e.classifyCustody(written)
	switch v.State {
	case deskkit.CustodyVerified:
		m.Custody = "verified owner-only"
		p.minted = append(p.minted, m)
		p.outf("minted: PAT %s for %s -> %s (owner-only, verified; expires %s) — path printed, value never echoed",
			m.TokenName, user, written, m.ExpiresAt)
		return nil
	case deskkit.CustodyInconclusive:
		m.Custody = "WARNING: owner-only access could not be verified"
		p.minted = append(p.minted, m)
		w := fmt.Sprintf("the owner-only access of %s (role %s) could NOT be verified: %v. The file was created "+
			"owner-only, but this filesystem did not confirm it. Check its access list by hand; the desk verbs' "+
			"own read-time custody check may refuse it later", written, r.Role, v.Err)
		p.warnings = append(p.warnings, w)
		p.errf("")
		p.errf("%s", strings.Repeat("!", 60))
		p.errf("WARNING: %s", w)
		p.errf("%s", strings.Repeat("!", 60))
		p.outf("minted: PAT %s for %s -> %s (WARNING: owner-only NOT verified; expires %s) — path printed, value never echoed",
			m.TokenName, user, written, m.ExpiresAt)
		return nil
	default:
		m.Custody = "REFUSED: the file is not owner-only"
		p.minted = append(p.minted, m)
		reason := "unknown custody state"
		if v.Err != nil {
			reason = v.Err.Error()
		}
		return &stopError{code: exitRefused, msg: fmt.Sprintf("the token file for %s is NOT owner-only (%s) — this "+
			"credential must be treated as exposed and REVOKED; it is not reported as usable", user, reason)}
	}
}

// stopRun ends a run whose account/token loop failed. Per the partial-run ruling it REPORTS
// every token this run minted — and every account it created, and every write whose outcome
// it could not check — and revokes nothing.
func (p *provisioner) stopRun(s *stopError) int {
	p.errf("error: provisioning stopped: %s", s.msg)
	if len(p.minted) == 0 && len(p.tokenless) == 0 && len(p.unknown) == 0 {
		// Only true when this run created no account and SENT no mint: every mint attempt
		// lands in exactly one of minted / tokenless / unknown.
		p.errf("This run created no service account and sent no token mint, so there is nothing to revoke.")
		return s.code
	}
	report := p.partialReport(s.msg)
	fmt.Fprint(p.e.stdout, report)
	name := "deskfleet-partial-run-" + p.e.now().UTC().Format("20060102T150405Z") + ".txt"
	rpath := filepath.Join(p.o.outDir, name)
	if err := p.writeReport(rpath, report); err != nil {
		p.errf("NOTICE: could not write the partial-run report to %s (%v) — the report above is the only copy", rpath, err)
	} else {
		p.outf("partial-run report written to %s", rpath)
	}
	return s.code
}

// writeReport writes the partial-run report through the same restricted create as the token
// files: a NEW file (never through an existing file or link at that name), owner-only from
// the moment it exists. The report carries no credential; this only keeps it from being
// written through something planted at its path.
func (p *provisioner) writeReport(path, report string) error {
	f, err := p.e.createRestricted(path)
	if err != nil {
		return err
	}
	if _, err := f.Write([]byte(report)); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (p *provisioner) partialReport(reason string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s\n", banner)
	fmt.Fprintf(&b, "PARTIAL RUN — provisioning STOPPED: %s\n", reason)
	fmt.Fprintf(&b, "%s\n", banner)
	fmt.Fprintf(&b, "This run minted %d personal access token(s) that EXIST NOW and were NOT revoked.\n", len(p.minted))
	fmt.Fprintf(&b, "deskfleet never revokes on a partial run (the recorded partial-run ruling: report). Revoke each one\n")
	fmt.Fprintf(&b, "by hand — by its token id, on the service account named — then delete its file:\n")
	for _, m := range p.minted {
		file := m.Path
		if file == "" {
			file = "<not on disk>"
		}
		fmt.Fprintf(&b, "  - role=%s account=%s (user id %d) token=%s (token id %d) expires=%s file=%s [%s]\n",
			m.Role, m.Username, m.UserID, m.TokenName, m.TokenID, m.ExpiresAt, file, m.Custody)
	}
	if len(p.unknown) > 0 {
		fmt.Fprintf(&b, "COULD-NOT-CHECK — %d request(s) were sent whose outcome this run could not read. The forge may\n", len(p.unknown))
		fmt.Fprintf(&b, "have acted on each; this is NOT a clean state. Check each by hand:\n")
		for _, u := range p.unknown {
			fmt.Fprintf(&b, "  ? role=%s account=%s (user id %s): %s\n", u.Role, u.Username, idOrUnknown(u.UserID), u.Detail)
		}
	}
	for _, u := range p.tokenless {
		fmt.Fprintf(&b, "Account %s was CREATED by this run but holds NO token (role %s, user id %s: %s); a re-run\n",
			u.Username, u.Role, idOrUnknown(u.UserID), u.Detail)
		fmt.Fprintf(&b, "will not mint one for an existing account — mint its token by hand, or remove the account and re-run.\n")
	}
	fmt.Fprintf(&b, "Token values are never printed. Re-running after revocation re-uses the existing accounts\n")
	fmt.Fprintf(&b, "and mints no new token for them; use `desktoken --forge gitlab <role>` to rotate one.\n")
	fmt.Fprintf(&b, "%s\n", banner)
	return b.String()
}

func (p *provisioner) summary() int {
	p.outf("")
	p.outf("%s", banner)
	p.outf("GITLAB_API_BASE — add to the shell that runs the desk verbs (it is NOT a roster.env key):")
	p.outf("  export GITLAB_API_BASE='%s'        # POSIX shells", p.base)
	p.outf("  $env:GITLAB_API_BASE = '%s'        # PowerShell (setx GITLAB_API_BASE to persist)", p.base)
	if len(p.warnings) > 0 {
		p.outf("%s", ruleLine)
		p.outf("CUSTODY WARNINGS — %d token file(s) whose owner-only access could NOT be verified:", len(p.warnings))
		for _, w := range p.warnings {
			p.outf("   - %s", w)
		}
	}
	if p.o.avatarsDir == "" {
		p.outf("%s", ruleLine)
		p.outf("SKIPPED STEP — avatars: %s", avatarsSkippedNotice(p.o))
	}
	p.outf("%s", banner)
	p.outf("HUMAN-ONLY REMAINDER — this verb does not and cannot do these:")
	p.outf("1. Ultimate-tier settings (custom reviewer role, external status checks, pipeline execution")
	p.outf("   policy) — not scripted by this verb.")
	p.outf("2. Group token-expiry policy: set the group/instance PAT max lifetime to %d days or less.", p.o.patExpiryDays)
	p.outf("3. Create the locked ci-config project (Maintainer-humans-only, protected main, no bot membership).")
	if len(p.failures) > 0 {
		p.outf("%s", ruleLine)
		p.outf("FAILED STEPS — this run did NOT complete cleanly:")
		for _, f := range p.failures {
			p.outf("   - %s", f)
		}
		p.outf("%s", banner)
		return exitFailed
	}
	p.outf("%s", banner)
	return exitOK
}
