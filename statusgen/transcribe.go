package main

// transcribe — the R-6 verify verdict-transcription lane's brain (per the R-6
// ruling in docs/streams/issue-flow/rulings.md). It is the sole logic behind
// `verify-transcribe.yml`, which is thin glue: enactment gate, checkout main,
// `statusgen transcribe`, lint, commit, push. This file holds the trust predicate
// (per-clause, each an independent fail-closed layer), the verdict → tree delta
// derivation, and the application; the workflow holds only the transport.
//
// SHIPPED INERT. The lane evaluates NOTHING until R-6's Sign-off line in
// docs/streams/issue-flow/rulings.md resolves, via the API, to a comment by the
// blessing authority (ASSAY_BLESS_LOGIN, login:id, both halves from one response,
// non-User refused — a prior enactment fix, carried forward) whose body names R-6.
// Until then transcribeVerifyEnactmentGate reports INERT and no clause is
// evaluated. This is what makes it ship inert and arm nothing: arming stays
// gate:human, and this PR arms nothing. The workflow independently re-resolves the
// same line before the tool ever runs (defense in depth).
//
// RELATION TO THE SCAN LANE (transcribescan.go). That lane transcribes issue
// PLACEHOLDERS; this one transcribes verify VERDICTS. They share the API-read
// author triple, the comment/sign-off resolvers, the enactment-gate shape and the
// skip-log discipline (all in transcribescan.go / scanissues.go), and differ in
// the write class: R-6 appends Evidence rows and flips model-tier README status /
// Verified cells, per a SIGNED verifier verdict.
//
// SECURITY. Issue bodies are attacker-authorable DATA and are NEVER executed. The
// only executed shell is a `check:ci` row's script already committed on the
// candidate tree, re-run network-off via runHermetically. No
// content lifted from an issue body reaches a shell. Trust keys on API-read author
// identity (cl.1) and the RS256 signature over the verifier's key (cl.2), never on
// any text an issue author controls.

import (
	"crypto/rsa"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Tunables (R-6 clause tripwires)
// ---------------------------------------------------------------------------

// transcribeUnconsumedCap is the R-6 cl.9 runaway-producer tripwire: the lane
// refuses to run while more than this many unconsumed verdict issues exist. The
// loop-side ~5-minute batch cadence is the intended throttle; this is the
// tripwire, not a rate optimisation.
const transcribeUnconsumedCap = 20

// transcribeCIFailureWindow is the R-6 cl.10 main-health hold: the lane lands
// nothing while an open `ci-failure:`-titled issue younger than this exists. A red
// or freshly-red main pauses the lane; humans resume it by fixing main.
const transcribeCIFailureWindow = 6 * time.Hour

// transcribeMaxEvidenceBytes bounds the R-6 cl.4 write class per entry: the exact
// Evidence markdown a single verdict entry appends. A verify landing appends one
// Evidence row; a payload carrying a multi-kilobyte "evidence" string is refused
// as out-of-class rather than committed.
const transcribeMaxEvidenceBytes = 2048

// transcribeRowTimeout bounds a single cl.6 check:ci re-execution.
const transcribeRowTimeout = 2 * time.Minute

// ---------------------------------------------------------------------------
// Payload shape (verdict-v1) — the parsed content of a signed verdict body.
// Mirrors the verdict payload schema. The signature (cl.2) is verified
// separately over the canonical bytes; this is the CONTENT the lane acts on.
// ---------------------------------------------------------------------------

type verdictPayload struct {
	Schema  string         `json:"schema"`
	Repo    string         `json:"repo"`
	TS      string         `json:"ts"`
	Head    string         `json:"head"`
	Entries []verdictEntry `json:"entries"`
}

type verdictEntry struct {
	Brief    string `json:"brief"`    // repo-relative brief path
	Row      int    `json:"row"`      // Verify-table row number
	Class    string `json:"class"`    // check:ci | check
	Result   string `json:"result"`   // PASS | FAIL
	Evidence string `json:"evidence"` // exact Evidence markdown to append
}

// ---------------------------------------------------------------------------
// Injected seams — production shells to gh; tests inject fixtures. Fail-closed:
// an unreadable signal is a refusal, never a pass.
// ---------------------------------------------------------------------------

// verdictIssue is the subset of an open issue the verify lane reads: number,
// title, ORIGINAL body (cl.3), and creation time (cl.10 ci-failure ages).
type verdictIssue struct {
	Number    int
	Title     string
	Body      string
	CreatedAt string
}

// verdictIssueLister returns the OPEN issues of a repo (number, title, body,
// createdAt). Production shells to `gh issue list`; tests inject a fixture.
type verdictIssueLister func(repo string) ([]verdictIssue, error)

// bodyEditResolver reports whether an issue's BODY was edited since creation
// (R-6 cl.2 timeline check). Production shells to `gh api graphql` reading
// issue.lastEditedAt; a non-empty lastEditedAt means edited. An error is a
// REFUSAL for that issue (fail closed), never edited=false.
type bodyEditResolver func(repo string, issue int) (edited bool, err error)

// verdictSigVerifier verifies the RS256 signature over a signed issue body and
// returns the three-state result. Production resolves the verifier public key
// from ASSAY_VERIFIER_PUBKEY (never a committed file) and calls vvVerifyBody;
// tests inject a fixture. Independent of the author check (cl.1) by construction.
type verdictSigVerifier func(body string) (vvVerifyState, string)

// rowExecutor re-executes a `check:ci` Verify row against the candidate tree for
// the cl.6 re-verification. Production wraps runHermetically (network-off);
// tests inject a deterministic result. A couldNotRun result is
// fail-closed by the caller — a hermetic check that could not be run hermetically
// established nothing.
type rowExecutor func(root, command string) runResult

// prodRowExecutor is the production rowExecutor: re-execute network-off.
func prodRowExecutor(root, command string) runResult {
	return runHermetically(root, command, transcribeRowTimeout)
}

// ---------------------------------------------------------------------------
// Production seam implementations
// ---------------------------------------------------------------------------

// ghVerdictIssueLister lists a repo's open issues with the fields the verify lane
// needs. A gh failure is an error the caller treats as fatal for the run (a
// single-repo lane has no other repo to degrade to).
func ghVerdictIssueLister(repo string) ([]verdictIssue, error) {
	out, err := runGH("issue", "list", "--repo", repo, "--state", "open",
		"--limit", "1000", "--json", "number,title,body,createdAt")
	if err != nil {
		return nil, err
	}
	var raw []struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		Body      string `json:"body"`
		CreatedAt string `json:"createdAt"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing gh issue list for %s: %w", repo, err)
	}
	issues := make([]verdictIssue, 0, len(raw))
	for _, r := range raw {
		issues = append(issues, verdictIssue{Number: r.Number, Title: r.Title, Body: r.Body, CreatedAt: r.CreatedAt})
	}
	return issues, nil
}

// ghBodyEditResolver reads issue.lastEditedAt via GraphQL (a READ). A non-empty
// value means the body was edited after creation → cl.2 refusal.
func ghBodyEditResolver(repo string, issue int) (bool, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return true, fmt.Errorf("bad repo %q", repo) // fail closed: treat as edited
	}
	const q = `query($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){issue(number:$number){lastEditedAt}}}`
	out, err := runGH("api", "graphql", "-f", "query="+q,
		"-f", "owner="+owner, "-f", "name="+name, "-F", "number="+strconv.Itoa(issue))
	if err != nil {
		return true, err
	}
	var env struct {
		Data struct {
			Repository struct {
				Issue *struct {
					LastEditedAt string `json:"lastEditedAt"`
				} `json:"issue"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := jsonUnmarshalTrim(out, &env); err != nil {
		return true, fmt.Errorf("parsing lastEditedAt for %s#%d: %w", repo, issue, err)
	}
	if len(env.Errors) > 0 {
		return true, fmt.Errorf("lastEditedAt query error for %s#%d: %s", repo, issue, env.Errors[0].Message)
	}
	if env.Data.Repository.Issue == nil {
		return true, fmt.Errorf("lastEditedAt query returned no issue for %s#%d", repo, issue)
	}
	return strings.TrimSpace(env.Data.Repository.Issue.LastEditedAt) != "", nil
}

// prodVerdictSigVerifier resolves the verifier public key from the
// ASSAY_VERIFIER_PUBKEY variable (NEVER a committed file) and verifies the body.
// A missing/invalid key is CouldNotCheck — never a silent pass.
func prodVerdictSigVerifier(body string) (vvVerifyState, string) {
	pub, err := resolveVerifierPubkey()
	if err != nil {
		return vvCouldNotCheck, "could not check: " + err.Error()
	}
	return vvVerifyBody(body, pub)
}

func resolveVerifierPubkey() (*rsa.PublicKey, error) {
	v := strings.TrimSpace(os.Getenv(vvPubkeyVar))
	if v == "" {
		return nil, fmt.Errorf("no verifier pubkey configured: set %s (a PEM string or base64-of-PEM)", vvPubkeyVar)
	}
	pemBytes, err := vvDecodePubkeyVar(v)
	if err != nil {
		return nil, err
	}
	return vvParsePublicKeyPEM(pemBytes)
}

// runGH shells one `gh` invocation and returns stdout, folding stderr into the
// error. Kept local so the verify lane's gh calls read uniformly.
func runGH(args ...string) ([]byte, error) {
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return nil, fmt.Errorf("gh %s: %v %s", strings.Join(args, " "), err, detail)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Enactment gate (R-6) — the self-arming-excluded-by-construction gate.
// ---------------------------------------------------------------------------

// transcribeVerifyEnactmentGate reports whether the lane is ARMED — whether R-6's
// Sign-off line resolves to a bless-authority comment naming R-6. Fail closed: an
// empty sign-off, an unparseable URL, an unreadable comment, a wrong author, a
// non-User author, or a body not naming R-6 all leave the lane INERT.
//
// The EMPTY-sign-off branch is fully offline (reads only rulings.md), which is the
// state the lane ships in; resolve is called only when a URL is present.
func transcribeVerifyEnactmentGate(root string, resolve commentResolver) (armed bool, reason string) {
	rulings := filepath.Join(root, "docs", "streams", "issue-flow", "rulings.md")
	data, err := os.ReadFile(rulings)
	if err != nil {
		return false, fmt.Sprintf("cannot read %s: %v", rulings, err)
	}
	line, ok := findR6SignoffLine(string(data))
	if !ok {
		return false, "R-6 sign-off line not found in rulings.md"
	}
	url := firstHTTPSURL(line)
	if url == "" {
		return false, "R-6 sign-off is empty — the lane evaluates no clause until an authorized acceptance URL lands on it"
	}
	if resolve == nil {
		return false, "R-6 sign-off carries a URL but no resolver is available to verify it (fail closed)"
	}
	author, body, err := resolve(url)
	if err != nil {
		return false, fmt.Sprintf("R-6 sign-off URL %s could not be resolved: %v", url, err)
	}
	c := scanEffectiveConfig()
	if !c.Configured() {
		return false, "roster unconfigured — no blessing authority to verify the sign-off against"
	}
	if !strings.EqualFold(author.Login, c.Bless.Login) || author.ID == 0 || author.ID != c.Bless.ID {
		return false, fmt.Sprintf("R-6 sign-off author %s:%d is not the blessing authority %s:%d",
			author.Login, author.ID, c.Bless.Login, c.Bless.ID)
	}
	if author.Type != "User" {
		return false, fmt.Sprintf("R-6 sign-off author is type %q, not User — a non-User identity cannot arm the lane", author.Type)
	}
	if !strings.Contains(body, "R-6") {
		return false, "R-6 sign-off comment body does not name R-6"
	}
	return true, "armed: R-6 sign-off resolves to the blessing authority"
}

// findR6SignoffLine returns the first `**Sign-off:**` line at or after `## R-6`,
// bounded by the next top-level `## R-` heading so a filled R-5 (earlier) or R-7
// (later) sign-off cannot arm this lane.
func findR6SignoffLine(md string) (string, bool) {
	lines := strings.Split(md, "\n")
	inR6 := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "## R-6") {
			inR6 = true
			continue
		}
		if inR6 && strings.HasPrefix(t, "## R-") && !strings.HasPrefix(t, "## R-6") {
			return "", false
		}
		if inR6 && strings.HasPrefix(t, "**Sign-off:**") {
			return t, true
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Derivation: verdict issues → tree delta, per-clause fail-closed.
// ---------------------------------------------------------------------------

// evidenceApply is one Evidence-append + optional cell-flip the lane will make to
// a brief. It is derived (never applied) by evaluateVerdict.
type evidenceApply struct {
	Issue      int
	BriefPath  string // absolute brief file path
	Num        string // brief number within its stream
	Stream     *Stream
	Evidence   string // exact Evidence markdown to append (cl.4)
	FlipStatus bool   // flip README status implemented→verified (model-tier)
	Verified   string // Verified-cell stamp when FlipStatus
}

// verifyDelta is the fully-derived, not-yet-applied verify delta plus the
// per-issue skip log.
type verifyDelta struct {
	Applies []evidenceApply
	Skips   []clauseSkip
}

// evaluateVerdict runs the R-6 clause battery over ONE candidate verdict issue.
// Each clause is an INDEPENDENT fail-closed layer: a failure at any clause returns
// a clauseSkip naming that clause and transcribes nothing for the whole verdict
// (cl.8). Upper layers are reached only when every lower one passed, so each layer
// refuses on its own. Returns applies only for an all-PASS verdict that clears
// every clause.
func evaluateVerdict(root string, iss verdictIssue, streams []*Stream, homeRepo string,
	resolveAuthor authorResolver, resolveBodyEdited bodyEditResolver, verifySig verdictSigVerifier,
	rowExec rowExecutor) ([]evidenceApply, *clauseSkip) {

	skip := func(clause, format string, a ...any) *clauseSkip {
		return &clauseSkip{Issue: iss.Number, Clause: clause, Reason: fmt.Sprintf(format, a...)}
	}

	// --- R-6 cl.1: author identity — the verifier App, API-read at eval time:
	// login AND numeric bot USER id AND type Bot. Never a commit email, never a
	// display name, never a comment author. ---
	policy := evidenceActorPolicyFromRoster()
	if policy.Unavailable != "" {
		return nil, skip("cl.1 (author)", "no verifier identity to check against — %s", policy.Unavailable)
	}
	ident, aerr := resolveAuthor(homeRepo, iss.Number)
	if aerr != nil {
		return nil, skip("cl.1 (author)", "author identity unreadable — %v", aerr)
	}
	if !isVerifierAppAuthor(ident, policy) {
		return nil, skip("cl.1 (author)",
			"issue author %s:%d (%s) is not the bound verifier App %s:%d/Bot — never a login, commit email or display name alone",
			ident.Login, ident.ID, ident.Type, policy.Verifier.Login, policy.Verifier.ID)
	}

	// --- R-6 cl.2: signature verifies against the verifier public key. An
	// unverifiable signature, an unparseable payload, or a could-not-check is a
	// refusal, never a skip-and-trust. Independent of the author check above. ---
	state, msg := verifySig(iss.Body)
	if state != vvVerified {
		return nil, skip("cl.2 (signature)", "verdict signature not verified: %s", msg)
	}

	// --- R-6 cl.2: the body has NOT been edited since creation (timeline check).
	// An edited body is a refusal (someone changed the signed content's envelope). ---
	edited, eerr := resolveBodyEdited(homeRepo, iss.Number)
	if eerr != nil {
		return nil, skip("cl.2 (body-edited)", "body-edit timeline unreadable — %v", eerr)
	}
	if edited {
		return nil, skip("cl.2 (body-edited)", "issue body was edited after creation — the signed envelope is no longer original")
	}

	// --- R-6 cl.3: read ONLY the original issue body — never comments, never
	// labels-as-content. We parse the payload from iss.Body and nothing else. ---
	rawPayload, perr := vvExtractPayload(iss.Body)
	if perr != nil {
		return nil, skip("cl.3 (payload)", "cannot extract verdict payload from the original body: %v", perr)
	}
	var pl verdictPayload
	if uerr := json.Unmarshal([]byte(rawPayload), &pl); uerr != nil {
		return nil, skip("cl.3 (payload)", "verdict payload is not valid JSON: %v", uerr)
	}
	if pl.Schema != vvSchemaVersion {
		return nil, skip("cl.3 (payload)", "verdict payload schema %q is not %q — a consumer refuses a version it does not recognise", pl.Schema, vvSchemaVersion)
	}
	if !strings.EqualFold(strings.TrimSpace(pl.Repo), homeRepo) {
		return nil, skip("cl.3 (payload)", "verdict payload names repo %q, not this repo %q", pl.Repo, homeRepo)
	}
	if len(pl.Entries) == 0 {
		return nil, skip("cl.3 (payload)", "verdict payload carries no entries")
	}

	// --- Per-entry clause evaluation (cl.4/5/6). A FAIL entry, or any clause
	// failure, refuses the WHOLE verdict (cl.6/cl.8): the verdict stays unconsumed
	// and the workflow files one brief-freshness triage issue naming the row. ---
	var applies []evidenceApply
	for _, e := range pl.Entries {
		// cl.8: a FAIL verdict transcribes nothing.
		if strings.ToUpper(strings.TrimSpace(e.Result)) != "PASS" {
			return nil, skip("cl.8 (fail-verdict)",
				"entry %s row %d result is %q, not PASS — a FAIL verdict transcribes nothing; the verdict stays unconsumed for triage",
				e.Brief, e.Row, e.Result)
		}

		// cl.4: byte-bounded Evidence-append class.
		if len(e.Evidence) > transcribeMaxEvidenceBytes {
			return nil, skip("cl.4 (write-scope)",
				"entry %s row %d Evidence is %d bytes, over the %d-byte per-entry bound — out of the docs-only append class",
				e.Brief, e.Row, len(e.Evidence), transcribeMaxEvidenceBytes)
		}
		if strings.TrimSpace(e.Evidence) == "" {
			return nil, skip("cl.4 (write-scope)", "entry %s row %d carries no Evidence markdown to append", e.Brief, e.Row)
		}
		// cl.4: the append class is Evidence rows, NEVER new sections. A payload
		// whose "evidence" carries a Markdown heading is an attempt to inject a
		// section — including a `## Verify` table, the F-verify-self-attest surface
		// this clause exists to keep off the unattended lane. Refuse it here, before
		// any write, and back it with the post-apply byte-invariant in applyDelta.
		if evidenceInjectsHeading(e.Evidence) {
			return nil, skip("cl.4 (write-scope)",
				"entry %s row %d Evidence contains a Markdown section heading — the lane appends Evidence rows, never new sections (a Verify-table edit riding the unattended lane is exactly what cl.4 forbids)",
				e.Brief, e.Row)
		}
		// cl.5: no diff line may add or remove a `human:` stamp — the sole human:
		// writer is verify-gate-close.yml. A verdict entry that would append a
		// human: token is out of class.
		if strings.Contains(e.Evidence, "human:") {
			return nil, skip("cl.5 (human-stamp)",
				"entry %s row %d Evidence carries a `human:` token — only verify-gate-close.yml writes human: stamps", e.Brief, e.Row)
		}

		// Resolve the brief file on the candidate tree.
		briefPath := filepath.Join(root, filepath.FromSlash(e.Brief))
		bf, ok, ferr := parseBriefFile(briefPath)
		if ferr != nil {
			return nil, skip("cl.4 (write-scope)", "entry %s: brief file unreadable/invalid: %v", e.Brief, ferr)
		}
		if !ok {
			return nil, skip("cl.4 (write-scope)", "entry %s: not a brief-v1 file on the candidate tree", e.Brief)
		}
		stream, num, brief := locateBriefRow(streams, briefPath)
		if brief == nil {
			return nil, skip("cl.4 (write-scope)", "entry %s: no README row found for the brief on the candidate tree", e.Brief)
		}

		// cl.5: irreversible-brief flips remain verify-gate-close.yml's alone,
		// read from the candidate tree's frontmatter.
		if bf.Risk["irreversible"] == "yes" {
			return nil, skip("cl.5 (irreversible)",
				"entry %s targets a risk.irreversible:yes brief — irreversible flips stay with verify-gate-close.yml", e.Brief)
		}

		// cl.6: row-class discipline. Locate the Verify row and its class on the
		// candidate tree; a check:ci row's PASS is RE-EXECUTED network-off and the
		// whole verdict refused on mismatch. A `check` row rests on cl.1–3. A
		// gate:* row is outside this lane by construction.
		cls, command, found := verifyRowClassAndCommand(bf.Verify, e.Row)
		if !found {
			return nil, skip("cl.6 (row-class)", "entry %s: Verify row %d not found on the candidate tree", e.Brief, e.Row)
		}
		switch cls {
		case classGateHuman, classGateModel:
			return nil, skip("cl.6 (row-class)", "entry %s row %d is a %s row — judgment rows are outside this lane (verify-gate)", e.Brief, e.Row, cls)
		case classCheckCI:
			res := rowExec(root, command)
			if res.couldNotRun {
				return nil, skip("cl.6 (re-exec)",
					"entry %s row %d is check:ci and could not be re-executed hermetically: %s — a hermetic check that could not be run hermetically established nothing",
					e.Brief, e.Row, res.reason)
			}
			if res.exit != 0 {
				return nil, skip("cl.6 (re-exec)",
					"entry %s row %d claims PASS but its check:ci script re-executed network-off to exit %d — refusing the whole verdict",
					e.Brief, e.Row, res.exit)
			}
		case classCheck:
			// Env-bound: rests on authorship + signature (cl.1–3). Not re-executed.
		default:
			return nil, skip("cl.6 (row-class)", "entry %s row %d has unresolved class %q", e.Brief, e.Row, cls)
		}

		// Build the apply. A model-tier status flip (implemented→verified) is made
		// only for a model-gate brief currently at `implemented`; otherwise the
		// entry appends Evidence without moving the cell (e.g. a re-verify of an
		// already-verified brief). Human-gate briefs never flip here (cl.6 excludes
		// their rows above; a model brief is the only status this lane advances).
		ap := evidenceApply{
			Issue:     iss.Number,
			BriefPath: briefPath,
			Num:       num,
			Stream:    stream,
			Evidence:  e.Evidence,
		}
		if bf.Gate == "model" && brief.Status == "implemented" {
			ap.FlipStatus = true
			ap.Verified = transcribeVerifiedStamp(policy.Verifier.Login, iss.Number, pl.Head)
		}
		applies = append(applies, ap)
	}
	return applies, nil
}

// isVerifierAppAuthor reports whether an API-read author identity is the bound
// verifier App: type Bot, the login matching the rostered verifier slug in one of
// its bot renderings, and (when the roster pins an id) the numeric id matching.
func isVerifierAppAuthor(ident authorIdentity, policy evidenceActorPolicy) bool {
	if ident.Type != "Bot" {
		return false
	}
	slug := strings.ToLower(strings.TrimSpace(policy.Verifier.Login))
	if slug == "" {
		return false
	}
	login := strings.ToLower(strings.TrimSpace(ident.Login))
	if login != slug && login != slug+"[bot]" && login != "app/"+slug {
		return false
	}
	if policy.idPinned() {
		return ident.ID != 0 && ident.ID == policy.Verifier.ID
	}
	return true
}

// transcribeVerifiedStamp renders the Verified cell a model-tier flip stamps: the
// verifier App, the verdict issue it came from, and the head SHA the rows ran
// against, dated. It is the audit record back to the exact verdict.
func transcribeVerifiedStamp(verifierLogin string, issue int, head string) string {
	login := verifierLogin
	if login != "" && !strings.HasSuffix(login, "[bot]") {
		login += "[bot]"
	}
	h := head
	if len(h) > 12 {
		h = h[:12]
	}
	return fmt.Sprintf("%s %s (verdict issue #%d @ %s)", time.Now().UTC().Format("2006-01-02"), login, issue, h)
}

// locateBriefRow resolves a brief file path to its stream, brief number, and
// README row (or nil when the file has no README row).
func locateBriefRow(streams []*Stream, briefPath string) (*Stream, string, *Brief) {
	_, num, ok := expectedBriefID(briefPath)
	if !ok {
		return nil, "", nil
	}
	dir := filepath.Dir(briefPath)
	for _, s := range streams {
		if filepath.Clean(s.Dir) == filepath.Clean(dir) {
			return s, num, findRow(s, num)
		}
	}
	return nil, num, nil
}

// verifyRowClassAndCommand returns the resolved class and Command cell of the
// Verify row numbered rowNum in a Verify section, and whether it was found.
func verifyRowClassAndCommand(verify string, rowNum int) (class, command string, found bool) {
	want := strconv.Itoa(rowNum)
	verifyRowTable(verify, func(r verifyRowCells) {
		if found {
			return
		}
		if strings.TrimSpace(normalizeRowID(r.Num)) == want {
			class = r.class()
			command = strings.TrimSpace(r.Command)
			found = true
		}
	})
	return class, command, found
}

// ---------------------------------------------------------------------------
// Apply — Evidence append + README cell flip, with the cl.4 byte-invariant check.
// ---------------------------------------------------------------------------

// applyEvidenceAppend appends evidenceMarkdown to a brief file's `## Evidence`
// section and returns the new file bytes. It NEVER edits the frontmatter or any
// other section. cl.4's frontmatter/Verify byte-invariant is asserted by the
// caller after the write is composed.
func applyEvidenceAppend(raw, evidenceMarkdown string) (string, error) {
	lines := strings.Split(raw, "\n")
	head := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## Evidence" {
			head = i
			break
		}
	}
	if head < 0 {
		return "", fmt.Errorf("no `## Evidence` section to append to")
	}
	// End of the Evidence section: the next `## ` sibling heading, or EOF.
	end := len(lines)
	for i := head + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	// Insert after the last non-blank line of the section body, preserving the
	// blank line(s) that precede the next heading.
	insert := end
	for insert-1 > head && strings.TrimSpace(lines[insert-1]) == "" {
		insert--
	}
	newLines := make([]string, 0, len(lines)+2)
	newLines = append(newLines, lines[:insert]...)
	newLines = append(newLines, strings.Split(strings.TrimRight(evidenceMarkdown, "\n"), "\n")...)
	newLines = append(newLines, lines[insert:]...)
	return strings.Join(newLines, "\n"), nil
}

// flipRowToVerified sets brief num's README status cell to `verified` and stamps
// its Verified cell, preserving every other cell's exact text. Additive on the
// Verified cell if already set. Mirrors flipRowToDone's cell mechanics.
func flipRowToVerified(raw, num, verifiedStamp string) (string, error) {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		cols := splitRow(line)
		idx := map[string]int{}
		for j, c := range cols {
			idx[strings.ToLower(strings.TrimSpace(c))] = j
		}
		if _, ok := idx["#"]; !ok {
			continue
		}
		if _, ok := idx["status"]; !ok {
			continue
		}
		for k := i + 2; k < len(lines); k++ {
			if !strings.HasPrefix(strings.TrimSpace(lines[k]), "|") {
				break
			}
			cells := splitRow(lines[k])
			if len(cells) < len(cols) {
				continue
			}
			if strings.TrimSpace(cells[idx["#"]]) != num {
				continue
			}
			cells[idx["status"]] = setCell(cells[idx["status"]], "verified")
			if vi, ok := idx["verified"]; ok {
				existing := strings.TrimSpace(cells[vi])
				if normalizeMark(existing) == "" {
					cells[vi] = setCell(cells[vi], verifiedStamp)
				} else {
					cells[vi] = setCell(cells[vi], existing+"; "+verifiedStamp)
				}
			}
			lines[k] = "|" + strings.Join(cells, "|") + "|"
			return strings.Join(lines, "\n"), nil
		}
		return "", fmt.Errorf("no row for #%s in briefs table", num)
	}
	return "", fmt.Errorf("no briefs table found")
}

// applyDelta writes the derived delta to the tree: per brief, the Evidence append
// and (when flipping) the README cell flip. cl.4's byte-invariant — frontmatter
// and the `## Verify` section byte-identical before and after — is asserted per
// brief; a violation aborts the whole apply and writes nothing further.
func applyDelta(delta verifyDelta) error {
	// Group applies by brief file (a batch may carry several rows for one brief).
	byBrief := map[string][]evidenceApply{}
	order := []string{}
	for _, ap := range delta.Applies {
		if _, seen := byBrief[ap.BriefPath]; !seen {
			order = append(order, ap.BriefPath)
		}
		byBrief[ap.BriefPath] = append(byBrief[ap.BriefPath], ap)
	}
	for _, path := range order {
		before, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		raw := string(before)
		for _, ap := range byBrief[path] {
			updated, aerr := applyEvidenceAppend(raw, ap.Evidence)
			if aerr != nil {
				return fmt.Errorf("%s: %v", path, aerr)
			}
			raw = updated
		}
		// cl.4 byte-invariant: frontmatter + `## Verify` unchanged.
		if err := assertFrontmatterAndVerifyUnchanged(string(before), raw); err != nil {
			return fmt.Errorf("%s: cl.4 write-scope violated: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	// README cell flips, grouped per stream README.
	byReadme := map[string][]evidenceApply{}
	rorder := []string{}
	for _, ap := range delta.Applies {
		if !ap.FlipStatus || ap.Stream == nil {
			continue
		}
		rp := filepath.Join(ap.Stream.Dir, "README.md")
		if _, seen := byReadme[rp]; !seen {
			rorder = append(rorder, rp)
		}
		byReadme[rp] = append(byReadme[rp], ap)
	}
	for _, rp := range rorder {
		before, err := os.ReadFile(rp)
		if err != nil {
			return fmt.Errorf("%s: %w", rp, err)
		}
		raw := string(before)
		for _, ap := range byReadme[rp] {
			updated, ferr := flipRowToVerified(raw, ap.Num, ap.Verified)
			if ferr != nil {
				return fmt.Errorf("%s: %v", rp, ferr)
			}
			raw = updated
		}
		if err := os.WriteFile(rp, []byte(raw), 0o644); err != nil {
			return fmt.Errorf("%s: %w", rp, err)
		}
	}
	return nil
}

// assertFrontmatterAndVerifyUnchanged enforces cl.4: a verify landing appends
// Evidence and flips a cell; it never edits the frontmatter block or the Verify
// table. Both must be byte-identical before and after.
func assertFrontmatterAndVerifyUnchanged(before, after string) error {
	fb, ferr := frontmatterBytes(before)
	fa, ferr2 := frontmatterBytes(after)
	if ferr != nil || ferr2 != nil {
		return fmt.Errorf("could not isolate frontmatter for comparison")
	}
	if fb != fa {
		return fmt.Errorf("the frontmatter block changed")
	}
	if sectionBody(before, "Verify") != sectionBody(after, "Verify") {
		return fmt.Errorf("the `## Verify` section changed")
	}
	return nil
}

// frontmatterBytes returns the raw text of the leading `---` … `---` frontmatter
// block (inclusive), or an error when there is none.
func frontmatterBytes(s string) (string, error) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return "", fmt.Errorf("no frontmatter")
	}
	rest := s[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", fmt.Errorf("unterminated frontmatter")
	}
	return s[:len("---\n")+end+len("\n---")], nil
}

// sectionBody returns the body of the `## <name>` section (prefix-matched heading,
// decorated headings allowed), between it and the next `## ` heading — the same
// slice extractSectionByPrefix would report. Used only for the cl.4 invariant.
func sectionBody(s, name string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	head := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "## ") && strings.HasPrefix(strings.TrimSpace(t[3:]), name) {
			head = i
			break
		}
	}
	if head < 0 {
		return ""
	}
	end := len(lines)
	for i := head + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return strings.Join(lines[head+1:end], "\n")
}

// ---------------------------------------------------------------------------
// Run entrypoint (the `transcribe` subcommand's body).
// ---------------------------------------------------------------------------

// runTranscribeVerify derives and (unless check) APPLIES the R-6 verify delta to
// the candidate tree. It NEVER commits, pushes, or mutates any GitHub issue — the
// workflow commits the tree the tool wrote, runs `statusgen --lint` in-path
// (cl.7), and files the ONE brief-freshness triage issue for refusals (cl.8).
// Process exit codes (mirroring the scan lane):
//
//	0  ran (armed + applied/derived) OR INERT (evaluated no clause) OR HELD (cl.10)
//	2  REFUSED: roster unconfigured, or (write path) a primary-checkout root
//	3  tripwire: more than the cap of unconsumed verdicts (cl.9) — nothing written
//
// check is the CI-testable "--check" surface: derive and report the would-be
// delta and the per-issue skip log without touching the filesystem.
func runTranscribeVerify(root string, check bool,
	list verdictIssueLister, resolveAuthor authorResolver, resolveBodyEdited bodyEditResolver,
	verifySig verdictSigVerifier, resolveSignoff commentResolver, rowExec rowExecutor) int {

	// P1: unset roster is CLOSED — this lane WRITES to main from signed verdicts,
	// and the roster is exactly what gates the verifier identity it trusts.
	if err := scanRosterUnconfiguredError(); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen transcribe REFUSED:", err)
		return 2
	}

	// Isolation guard (write path only). A --check run writes nothing and is always
	// allowed; a live write against a shared primary checkout is refused.
	if !check && !scanInCI() {
		if reason := scanIsolationRefusal(root); reason != "" {
			fmt.Fprintln(os.Stderr, "statusgen transcribe REFUSED:", reason)
			return 2
		}
	}

	// --- Enactment gate (R-6): evaluate NOTHING until armed. This is what ships
	// the lane inert. ---
	armed, reason := transcribeVerifyEnactmentGate(root, resolveSignoff)
	if !armed {
		fmt.Println("transcribe: INERT —", reason)
		fmt.Println("transcribe: evaluating no clause; the lane is disarmed until R-6 is signed")
		return 0
	}
	fmt.Println("transcribe:", reason)

	homeRepo := verifyRepoSlug()
	if homeRepo == "" {
		fmt.Println("transcribe: no home repo configured (ASSAY_HOME_REPO) — nothing to transcribe")
		return 0
	}

	issues, err := list(homeRepo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen transcribe:", err)
		return 1
	}

	// --- R-6 cl.10: main-health hold. A red or freshly-red main (an open
	// `ci-failure:`-titled issue younger than the window) pauses the lane. ---
	if held, why := ciFailureHold(issues); held {
		fmt.Println("transcribe: HELD (cl.10 main-health) —", why)
		fmt.Println("transcribe: landing nothing; resume by fixing main, not by overriding the lane")
		return 0
	}

	// Candidate verdicts: open issues authored (by structure) as a signed verdict —
	// a body carrying a ```verdict-payload block. Non-verdict issues are ignored.
	var candidates []verdictIssue
	for _, iss := range issues {
		if _, perr := vvExtractPayload(iss.Body); perr == nil {
			candidates = append(candidates, iss)
		}
	}

	// --- R-6 cl.9: runaway-producer tripwire. Refuse the whole run while more than
	// the cap of unconsumed verdicts exist; nothing is written and the workflow
	// files one triage issue. ---
	if len(candidates) > transcribeUnconsumedCap {
		fmt.Printf("transcribe: TRIPWIRE (cl.9) — %d unconsumed verdict issues exceed the cap of %d; refusing the run (nothing written)\n",
			len(candidates), transcribeUnconsumedCap)
		return 3
	}

	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen transcribe:", err)
		return 1
	}
	attachPlaceholders(streams)

	var delta verifyDelta
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Number < candidates[j].Number })
	for _, iss := range candidates {
		applies, skip := evaluateVerdict(root, iss, streams, homeRepo, resolveAuthor, resolveBodyEdited, verifySig, rowExec)
		if skip != nil {
			delta.Skips = append(delta.Skips, *skip)
			continue
		}
		delta.Applies = append(delta.Applies, applies...)
	}

	// Surface the per-issue skip log (clause named). The workflow lifts refusals
	// into the ONE brief-freshness triage issue (cl.8); the tool only logs.
	for _, sk := range delta.Skips {
		fmt.Printf("transcribe: SKIP %s#%d — %s: %s\n", homeRepo, sk.Issue, sk.Clause, sk.Reason)
	}
	for _, ap := range delta.Applies {
		flip := ""
		if ap.FlipStatus {
			flip = " (flip implemented→verified)"
		}
		fmt.Printf("transcribe: APPEND %s#%d → %s%s\n", homeRepo, ap.Issue, filepathRel(root, ap.BriefPath), flip)
	}

	if check {
		if len(delta.Applies) == 0 {
			fmt.Println("transcribe: no verdicts cleared all clauses — nothing to transcribe")
		}
		fmt.Println("transcribe: --check — evaluated without writing")
		return 0
	}

	if len(delta.Applies) == 0 {
		fmt.Println("transcribe: nothing to transcribe")
		return 0
	}
	if err := applyDelta(delta); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen transcribe:", err)
		return 1
	}
	fmt.Printf("transcribe: transcribed %d verdict entr(ies) into the tree\n", len(delta.Applies))
	return 0
}

// ciFailureHold reports whether an open `ci-failure:`-titled issue younger than
// the cl.10 window exists among the listed open issues.
func ciFailureHold(issues []verdictIssue) (bool, string) {
	cutoff := time.Now().Add(-transcribeCIFailureWindow)
	for _, iss := range issues {
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(iss.Title)), "ci-failure:") {
			continue
		}
		created, err := time.Parse(time.RFC3339, iss.CreatedAt)
		if err != nil {
			// Unparseable timestamp on a ci-failure issue → hold (fail closed).
			return true, fmt.Sprintf("open ci-failure issue #%d with an unreadable createdAt (%q)", iss.Number, iss.CreatedAt)
		}
		if created.After(cutoff) {
			return true, fmt.Sprintf("open ci-failure issue #%d created %s (within %s)", iss.Number, iss.CreatedAt, transcribeCIFailureWindow)
		}
	}
	return false, ""
}

// filepathRel is filepath.Rel with a fallback to the absolute path on error.
func filepathRel(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// runTranscribe is the `statusgen transcribe` subcommand: it parses its own
// --root/--check flags and dispatches into runTranscribeVerify with the
// production seams (gh for issue/author/body-edit/sign-off reads, the
// ASSAY_VERIFIER_PUBKEY-resolved verifier for signatures). It owns its flag
// namespace exactly like `verifyrun` and `mergecheck`.
func runTranscribe(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("transcribe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "repository root to transcribe against (the candidate tree)")
	check := fs.Bool("check", false, "evaluate the R-6 clauses and report the would-be delta WITHOUT writing (the CI-testable surface)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return runTranscribeVerify(*root, *check,
		ghVerdictIssueLister, ghAuthorResolver, ghBodyEditResolver, prodVerdictSigVerifier, ghCommentResolver, prodRowExecutor)
}

// evidenceInjectsHeading reports whether an Evidence markdown string carries a
// Markdown ATX heading line (`# `, `## `, …) — the shape that would inject a new
// section (a `## Verify` table included) rather than append an Evidence row. cl.4.
func evidenceInjectsHeading(evidence string) bool {
	for _, ln := range strings.Split(evidence, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "#") {
			// A run of leading '#' followed by a space is an ATX heading.
			h := strings.TrimLeft(t, "#")
			if h != t && (h == "" || strings.HasPrefix(h, " ")) {
				return true
			}
		}
	}
	return false
}
