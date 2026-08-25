package main

// transcribe-verdict — the VERIFY VERDICT transcriber's brain (issue-flow rulings
// R-6; verdict-lane/03). It is the sole logic behind the `verify-transcribe.yml`
// workflow, which is thin glue: enactment gate, checkout main, `run`, lint,
// commit, push. This file holds the trust predicate (authorship + RS256 signature
// + body-edit timeline), the byte-bounded delta derivation (Evidence appends +
// model-tier status flips), the check:ci re-execution, and the clause evaluation;
// the workflow holds only the transport.
//
// SHIPPED INERT. The lane evaluates NOTHING until R-6's Sign-off line in
// docs/streams/issue-flow/rulings.md resolves, via the API, to a comment by the
// blessing authority (ASSAY_BLESS_LOGIN, login:id, non-User refused) whose body
// names R-6. Until then transcribeVerdictEnactmentGate reports INERT and no clause
// is evaluated — the same self-arming-excluded-by-construction posture as the R-7
// scan-transcribe lane. The workflow independently re-resolves the same line
// before the tool ever runs (defense in depth), so an empty or unauthorized
// sign-off keeps the lane inert from either side.
//
// RELATION TO THE R-7 TWIN (transcribescan.go). The scan lane boards intake
// PLACEHOLDERS behind an author-identity + roster trust predicate. This lane lands
// verify EVIDENCE + model-tier status flips behind a stronger predicate: the
// verdict body is RS256-SIGNED by the verifier App's existing key, so trusting the
// author login alone (spoofable by anyone who can edit an issue body) is replaced
// by a cryptographic check over the verifier PUBLIC key. Verify takes the public
// key only; the signing PEM never enters this tool or Actions secrets.
//
// SECURITY: issue bodies are attacker-authorable data. Nothing from any issue body
// is EXECUTED. The only shell this lane runs is a committed `check:ci` verify
// script already on the candidate tree, re-executed network-off (R-6 cl.6) — never
// anything drawn from the payload. The Evidence text it appends is verifier-authored
// and signature-bound; even so, a line carrying a `human:` stamp refuses the whole
// verdict (cl.5), because the sole `human:` writer is verify-gate-close.yml.

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// verdictSchemaVersion is the payload schema this build speaks. A payload naming a
// version this build does not recognise is REFUSED, never guessed (payload.md).
const verdictSchemaVersion = "verdict-v1"

// The issue-body block delimiters — the literal markers ParseVerdictBody keys on.
// The payload is a fenced code block tagged verdictFenceTag; the signature is an
// HTML-comment trailer so it does not render in the issue body a human reads.
// KEEP IN SYNC with deskkit/verdict.go (the signer): the two are separate Go
// modules that share no code, so the canonical form + markers are a documented
// cross-tree duplicate, and a change to either must be made in both. A behavioural
// coupling exists in that a body the signer emits must verify here byte-for-byte.
const (
	verdictFenceTag  = "verdict-payload"
	verdictSigMarker = "deskverdict-signature"
)

// verdictSigRE extracts the base64 signature from the trailer comment. Standard
// base64 alphabet — a raw RSA blob, never the eyJ… JWT shape a body scanner refuses.
var verdictSigRE = regexp.MustCompile(verdictSigMarker + `\b[^>]*\bsig=([A-Za-z0-9+/=]+)`)

// verdictPubkeyVar carries the verifier PUBLIC key. A repo/Actions VARIABLE, never
// a secret — the public half is public material — and NOT committed to the tree
// (payload.md's relayed re-scope: adopters running locally self-generate a keypair
// and set this to their own public half).
const verdictPubkeyVar = "ASSAY_VERIFIER_PUBKEY"

// verdictCheckCITimeout bounds a single check:ci row re-execution (cl.6).
const verdictCheckCITimeout = 2 * time.Minute

// verdictFloodThreshold is the R-6 cl.9 runaway-producer tripwire: the lane
// refuses to run while MORE THAN this many unconsumed verdict issues exist.
const verdictFloodThreshold = 20

// ---------------------------------------------------------------------------
// Payload types
// ---------------------------------------------------------------------------

type verdictPayload struct {
	Schema  string         `json:"schema"`
	Repo    string         `json:"repo"`
	TS      string         `json:"ts"`
	Head    string         `json:"head"`
	Entries []verdictEntry `json:"entries"`
}

type verdictEntry struct {
	Brief    string          `json:"brief"`
	Row      int             `json:"row"`
	Class    string          `json:"class"`
	Result   string          `json:"result"`
	Evidence string          `json:"evidence"`
	Session  *verdictSession `json:"session,omitempty"`
}

// verdictSession is the optional provenance block. Absent = ProvenanceUnknown,
// never a default-trust (payload.md); this lane records it in the flip stamp when
// present but never synthesises one.
type verdictSession struct {
	ID               string `json:"id"`
	TranscriptSHA256 string `json:"transcript_sha256"`
	Runner           string `json:"runner"`
}

// ---------------------------------------------------------------------------
// Canonicalisation + signature verify (stdlib; byte-identical to deskkit)
// ---------------------------------------------------------------------------

// verdictCanonicalizeJSON returns the byte-deterministic canonical JSON encoding of
// raw. It MUST match deskkit.CanonicalizeJSON byte-for-byte or a body the signer
// produced will not verify here: object keys sorted by UTF-8 code unit, no
// insignificant whitespace, the fixed escape rule (only the two mandatory escapes,
// the five short control escapes, \u00XX for the remaining C0 controls, every other
// rune raw UTF-8 — `<`/`>`/`&` NOT escaped), integers normalised to shortest
// decimal, trailing data rejected. Idempotent, which lets verify re-canonicalise a
// payload block that was reflowed in transit.
func verdictCanonicalizeJSON(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("payload is not valid JSON: %w", err)
	}
	if dec.More() {
		return nil, errors.New("payload has trailing data after the JSON value")
	}
	var buf bytes.Buffer
	if err := verdictWriteCanonical(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func verdictWriteCanonical(buf *bytes.Buffer, v interface{}) error {
	switch t := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if t {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case json.Number:
		buf.WriteString(verdictCanonicalNumber(t))
	case string:
		verdictWriteCanonicalString(buf, t)
	case []interface{}:
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := verdictWriteCanonical(buf, e); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case map[string]interface{}:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			verdictWriteCanonicalString(buf, k)
			buf.WriteByte(':')
			if err := verdictWriteCanonical(buf, t[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		return fmt.Errorf("unexpected JSON value of type %T during canonicalisation", v)
	}
	return nil
}

func verdictCanonicalNumber(n json.Number) string {
	s := n.String()
	if verdictIsIntegerLiteral(s) {
		if i, err := n.Int64(); err == nil {
			return strconv.FormatInt(i, 10)
		}
	}
	return s
}

func verdictIsIntegerLiteral(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	if s[0] == '-' {
		i = 1
	}
	if i == len(s) {
		return false
	}
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func verdictWriteCanonicalString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(buf, `\u%04x`, r)
			} else {
				buf.WriteRune(r)
			}
		}
	}
	buf.WriteByte('"')
}

// verdictVerifyState is the three-state result of verifying an issue body. A trust
// gate treats everything that is not Verified as "do not trust".
type verdictVerifyState int

const (
	verdictVerified      verdictVerifyState = iota // signature matches the canonical payload
	verdictRefused                                 // definite cryptographic negative (tamper)
	verdictCouldNotCheck                           // structural surprise: no block/trailer, unparseable, no pubkey
)

// verdictParseBody extracts the payload JSON and base64 signature from an issue
// body. A missing block or trailer is a structural error mapped to CouldNotCheck.
func verdictParseBody(body string) (payload []byte, sigB64 string, err error) {
	payloadStr, perr := verdictExtractFenced(body)
	if perr != nil {
		return nil, "", perr
	}
	m := verdictSigRE.FindStringSubmatch(body)
	if m == nil {
		return nil, "", errors.New("no verdict signature trailer found in body")
	}
	return []byte(payloadStr), m[1], nil
}

// verdictExtractFenced returns the inner text of the first ```verdict-payload block.
func verdictExtractFenced(body string) (string, error) {
	lines := strings.Split(body, "\n")
	open := -1
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "```") && strings.TrimSpace(strings.TrimLeft(t, "`")) == verdictFenceTag {
			open = i
			break
		}
	}
	if open < 0 {
		return "", errors.New("no ```" + verdictFenceTag + " block found in body")
	}
	for j := open + 1; j < len(lines); j++ {
		t := strings.TrimSpace(lines[j])
		if strings.HasPrefix(t, "```") && strings.TrimLeft(t, "`") == "" {
			return strings.Join(lines[open+1:j], "\n"), nil
		}
	}
	return "", errors.New("unterminated ```" + verdictFenceTag + " block in body")
}

// verdictHasPayloadBlock reports whether body carries a verdict-payload fence at
// all — the cheap test that separates a verdict issue (evaluate the clause battery)
// from an ordinary issue (skip silently, no log noise).
func verdictHasPayloadBlock(body string) bool {
	_, err := verdictExtractFenced(body)
	return err == nil
}

// verdictVerifyBody extracts, re-canonicalises, and checks the signature with the
// PUBLIC key. A nil pub is CouldNotCheck (a missing key is never trust).
func verdictVerifyBody(body string, pub *rsa.PublicKey) (verdictVerifyState, string) {
	rawPayload, sigB64, err := verdictParseBody(body)
	if err != nil {
		return verdictCouldNotCheck, "could not check: " + err.Error()
	}
	if pub == nil {
		return verdictCouldNotCheck, "could not check: no verifier public key configured (" + verdictPubkeyVar + ")"
	}
	canonical, cerr := verdictCanonicalizeJSON(rawPayload)
	if cerr != nil {
		return verdictCouldNotCheck, "could not check: " + cerr.Error()
	}
	sig, derr := base64.StdEncoding.DecodeString(strings.TrimSpace(sigB64))
	if derr != nil {
		return verdictRefused, "refused: signature is not valid base64: " + derr.Error()
	}
	sum := sha256.Sum256(canonical)
	if verr := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig); verr != nil {
		return verdictRefused, "refused: signature does not verify against the verifier public key"
	}
	return verdictVerified, "verified: signature matches the canonical verdict payload"
}

// ---------------------------------------------------------------------------
// Public-key resolution (--pubkey file, then ASSAY_VERIFIER_PUBKEY)
// ---------------------------------------------------------------------------

// verdictResolvePubkey resolves the verifier public key. Order (payload.md): an
// explicit --pubkey PEM path, then the ASSAY_VERIFIER_PUBKEY variable (a PEM string
// OR base64-of-PEM), else an error — never a silent pass. The key is read directly,
// independent of the trust roster: it is public material, not a trusted-identity list.
func verdictResolvePubkey(pubkeyPath string) (*rsa.PublicKey, error) {
	if strings.TrimSpace(pubkeyPath) != "" {
		data, err := os.ReadFile(pubkeyPath)
		if err != nil {
			return nil, fmt.Errorf("reading --pubkey %s: %w", pubkeyPath, err)
		}
		return verdictParseRSAPublicKeyPEM(data)
	}
	if v := strings.TrimSpace(os.Getenv(verdictPubkeyVar)); v != "" {
		pemBytes, err := verdictDecodePubkeyVar(v)
		if err != nil {
			return nil, err
		}
		return verdictParseRSAPublicKeyPEM(pemBytes)
	}
	return nil, fmt.Errorf("no verifier public key: pass --pubkey <file> or set %s (a missing key is could-not-check, never trust)", verdictPubkeyVar)
}

// verdictDecodePubkeyVar normalises the variable value into PKIX public-key PEM
// bytes, accepting a literal PEM string or base64-of-PEM (newline-safe across an
// Actions round-trip). Empty, or base64 that does not decode to a PEM, is an error.
func verdictDecodePubkeyVar(val string) ([]byte, error) {
	s := strings.TrimSpace(val)
	if s == "" {
		return nil, errors.New("empty verifier pubkey value")
	}
	if strings.HasPrefix(s, "-----BEGIN") {
		return []byte(val), nil
	}
	compact := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
	der, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		return nil, fmt.Errorf("%s is neither a PEM string nor valid base64-of-PEM: %w", verdictPubkeyVar, err)
	}
	if !bytes.Contains(der, []byte("-----BEGIN")) {
		return nil, fmt.Errorf("%s base64-decoded but is not a PEM (no BEGIN marker)", verdictPubkeyVar)
	}
	return der, nil
}

// verdictParseRSAPublicKeyPEM parses an RSA public key from a PKIX ("PUBLIC KEY")
// PEM, tolerating the bare PKCS#1 form too.
func verdictParseRSAPublicKeyPEM(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found in verifier public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		if rk, e2 := x509.ParsePKCS1PublicKey(block.Bytes); e2 == nil {
			return rk, nil
		}
		return nil, fmt.Errorf("cannot parse verifier public key: %w", err)
	}
	rk, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("verifier public key is %T, not RSA", pub)
	}
	return rk, nil
}

// ---------------------------------------------------------------------------
// Enactment gate (R-6): evaluate NOTHING until armed
// ---------------------------------------------------------------------------

// transcribeVerdictEnactmentGate reports whether the lane is ARMED — whether R-6's
// Sign-off line resolves to a bless-authority comment naming R-6. Fail closed: an
// empty sign-off, an unparseable URL, an unreadable comment, or an author/body that
// fails any check leaves the lane INERT. Mirrors transcribeEnactmentGate (R-7).
func transcribeVerdictEnactmentGate(root string, resolve commentResolver) (armed bool, reason string) {
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
// scoped to the R-6 section so a filled R-7 sign-off (which appears later) cannot
// arm the verdict lane.
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
// Trust seams
// ---------------------------------------------------------------------------

// verdictIssue is the API-read state of one issue the clause battery judges: its
// author identity (cl.1), its ORIGINAL body (cl.3 — never comments/labels), and
// whether that body was edited since creation (cl.2 timeline check).
type verdictIssue struct {
	Author authorIdentity
	Body   string
	Edited bool
}

// verdictIssueResolver reads one issue's author + original body + edited state. The
// production implementation shells to `gh api`; tests inject a fixture. An error is
// a per-issue could-not-read (NOTICE), never a default-trust.
type verdictIssueResolver func(repo string, issue int) (verdictIssue, error)

// ghVerdictIssueResolver is the default verdictIssueResolver. `last_edited_at` on
// the issue object is non-null only when the ISSUE BODY itself was edited (comments
// and labels do not touch it), which is exactly the cl.2 timeline signal — an
// edited body is a refusal.
func ghVerdictIssueResolver(repo string, issue int) (verdictIssue, error) {
	out, err := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/issues/%d", repo, issue),
		"--jq", "{login: .user.login, id: .user.id, type: .user.type, body: .body, edited: (.last_edited_at != null)}").Output()
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return verdictIssue{}, fmt.Errorf("gh api issue %s#%d: %v %s", repo, issue, err, detail)
	}
	var v struct {
		Login  string `json:"login"`
		ID     int64  `json:"id"`
		Type   string `json:"type"`
		Body   string `json:"body"`
		Edited bool   `json:"edited"`
	}
	if err := jsonUnmarshalTrim(out, &v); err != nil {
		return verdictIssue{}, fmt.Errorf("parsing issue %s#%d: %w", repo, issue, err)
	}
	return verdictIssue{
		Author: authorIdentity{Login: v.Login, ID: v.ID, Type: v.Type},
		Body:   v.Body,
		Edited: v.Edited,
	}, nil
}

// checkCIResult is the three-state outcome of re-executing one check:ci row
// network-off. CouldNotRun (no sandbox on this host, etc.) is NOT a pass — a
// hermetic check that could not be run hermetically established nothing (cl.6).
type checkCIResult struct {
	Passed      bool
	CouldNotRun bool
	Reason      string
}

// checkCIRunner re-executes one check:ci row's committed command network-off
// against the candidate tree. Production wraps runHermetically; tests inject a fake
// so the battery is exercised on any host.
type checkCIRunner func(root, command string) checkCIResult

// hermeticCheckCIRunner is the default checkCIRunner: runHermetically (unshare
// --net) over the candidate tree.
func hermeticCheckCIRunner(root, command string) checkCIResult {
	r := runHermetically(root, command, verdictCheckCITimeout)
	if r.couldNotRun {
		return checkCIResult{CouldNotRun: true, Reason: r.reason}
	}
	return checkCIResult{Passed: r.exit == 0, Reason: fmt.Sprintf("exit %d", r.exit)}
}

// mainHealthChecker reports the R-6 cl.10 main-health hold: an open `ci-failure:`-
// titled issue younger than 6h pauses the lane (a red or freshly-red main). An
// error is fail-closed HELD — an unreadable health signal pauses the lane rather
// than landing into a possibly-red main.
type mainHealthChecker func() (held bool, reason string, err error)

// verdictMainHealthWindow is the cl.10 hold window: an open `ci-failure:`-titled
// issue younger than this pauses the lane.
const verdictMainHealthWindow = 6 * time.Hour

// ghVerdictMainHealth is the production mainHealthChecker (cl.10). It lists open
// issues on the home repo and HOLDS the lane while any `ci-failure:`-titled one is
// younger than the 6h window. An unreadable signal is returned as an error, which
// the run treats as fail-closed HELD. It resolves the home repo itself so main.go
// can wire it before the run resolves its own; an unset home repo cannot be red, so
// it reports not-held.
func ghVerdictMainHealth(root string) mainHealthChecker {
	return func() (bool, string, error) {
		repo := scanHomeRepo()
		if repo == "" {
			return false, "", nil
		}
		out, err := exec.Command("gh", "issue", "list",
			"--repo", repo, "--state", "open", "--limit", "200",
			"--json", "number,title,createdAt").Output()
		if err != nil {
			detail := ""
			if ee, ok := err.(*exec.ExitError); ok {
				detail = strings.TrimSpace(string(ee.Stderr))
			}
			return false, "", fmt.Errorf("gh issue list --repo %s: %v %s", repo, err, detail)
		}
		var issues []struct {
			Number    int    `json:"number"`
			Title     string `json:"title"`
			CreatedAt string `json:"createdAt"`
		}
		if uerr := json.Unmarshal(out, &issues); uerr != nil {
			return false, "", fmt.Errorf("parsing gh output for %s: %w", repo, uerr)
		}
		now := time.Now()
		for _, iss := range issues {
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(iss.Title)), "ci-failure:") {
				continue
			}
			created, perr := time.Parse(time.RFC3339, iss.CreatedAt)
			if perr != nil {
				// An unparseable timestamp on a ci-failure issue is fail-closed HELD.
				return true, fmt.Sprintf("open ci-failure issue %s#%d has an unparseable createdAt %q — holding (fail closed)", repo, iss.Number, iss.CreatedAt), nil
			}
			if now.Sub(created) < verdictMainHealthWindow {
				return true, fmt.Sprintf("open ci-failure issue %s#%d is %s old (< %s) — main is red or freshly-red", repo, iss.Number, now.Sub(created).Round(time.Minute), verdictMainHealthWindow), nil
			}
		}
		return false, "", nil
	}
}

// verdictVerifierIdentity resolves the verifier App identity from the roster's
// `verifier=` role binding (ASSAY_TRUSTED_BOT_SLUGS). Returns ok=false when no such
// role is bound — the fail-closed direction: with no identity to check, no verdict
// is trusted and nothing is consumed.
func verdictVerifierIdentity() (authorIdentity, bool) {
	c := scanEffectiveConfig()
	slug := c.RoleBots["verifier"]
	if slug == "" {
		return authorIdentity{}, false
	}
	id := c.Bots[slug]
	if id == 0 {
		return authorIdentity{}, false
	}
	return authorIdentity{Login: slug + "[bot]", ID: id, Type: "Bot"}, true
}

// ---------------------------------------------------------------------------
// Delta types
// ---------------------------------------------------------------------------

// evidenceAppend is one planned append to a brief's `## Evidence` section.
type evidenceAppend struct {
	Brief string // "<stream>/<NN>"
	Path  string // absolute brief file path
	Row   int
	Line  string // the exact Evidence markdown to append
}

// verdictFlip is one planned README status flip (implemented→verified) for a
// gate:model, non-irreversible brief all of whose check:ci/check rows are attested.
type verdictFlip struct {
	Brief      string
	Num        string
	ReadmePath string
	Stamp      string // Verified-cell stamp
}

// verdictRefusal is one verdict issue the lane refused to consume, naming the R-6
// clause it failed. cl.8: a refusal transcribes nothing and the workflow files (or
// updates) one brief-freshness triage issue.
type verdictRefusal struct {
	Issue  int
	Clause string
	Reason string
}

// verdictDelta is the fully-derived, not-yet-applied delta plus the refusal log.
type verdictDelta struct {
	Appends      []evidenceAppend
	Flips        []verdictFlip
	Consumed     []int // consumed verdict issue numbers
	Refusals     []verdictRefusal
	VerdictCount int // open verdict issues seen (cl.9 tripwire counts these)
	Notices      []string
}

// ---------------------------------------------------------------------------
// Plan
// ---------------------------------------------------------------------------

// planTranscribeVerdict derives the byte-bounded R-6 delta over the candidate tree.
// It NEVER writes. For each open issue that carries a verdict-payload block it runs
// the clause battery and either CONSUMES it (planning Evidence appends + eligible
// model-tier flips) or REFUSES it (naming the failed clause). An issue with no
// payload block is not a verdict issue and is skipped silently.
func planTranscribeVerdict(root string, streams []*Stream, homeRepo string,
	list issueLister, resolveIssue verdictIssueResolver,
	pub *rsa.PublicKey, verifier authorIdentity, runCheckCI checkCIRunner) (verdictDelta, error) {

	var d verdictDelta

	issues, err := list(homeRepo)
	if err != nil {
		return d, fmt.Errorf("listing issues for %s: %w", homeRepo, err)
	}

	// Index brief files by repo-relative path and by stream, so an entry's `brief`
	// pointer resolves to a parsed file and a README row.
	briefByRel := map[string]*BriefFile{}
	rowsByRel := map[string]map[int]verifyRowCells{}
	streamByRel := map[string]*Stream{}
	numByRel := map[string]string{}
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, perr := parseBriefFile(path)
			if perr != nil || !ok {
				continue
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			briefByRel[rel] = bf
			streamByRel[rel] = s
			if _, num, okName := expectedBriefID(path); okName {
				numByRel[rel] = num
			}
			rows := map[int]verifyRowCells{}
			verifyRowTable(bf.Verify, func(r verifyRowCells) {
				if n, cerr := strconv.Atoi(strings.TrimSpace(r.Num)); cerr == nil {
					rows[n] = r
				}
			})
			rowsByRel[rel] = rows
		}
	}

	// attested[rel] = set of row numbers attested PASS across all consumed verdicts
	// (a batch is one payload per issue, but a run sweeps every open verdict issue,
	// so coverage can accrue across issues). contrib[rel] = the issues that attested.
	attested := map[string]map[int]bool{}
	contrib := map[string][]int{}

	sort.Slice(issues, func(i, j int) bool { return issues[i].Number < issues[j].Number })
	for _, iss := range issues {
		vi, rerr := resolveIssue(homeRepo, iss.Number)
		if rerr != nil {
			d.Notices = append(d.Notices, fmt.Sprintf("%s#%d could not be read: %v", homeRepo, iss.Number, rerr))
			continue
		}
		if !verdictHasPayloadBlock(vi.Body) {
			continue // not a verdict issue
		}
		d.VerdictCount++

		refuse := func(clause, reason string) {
			d.Refusals = append(d.Refusals, verdictRefusal{Issue: iss.Number, Clause: clause, Reason: reason})
		}

		// --- clause 1: author is the verifier App (API-read login+id+type). ---
		if !strings.EqualFold(vi.Author.Login, verifier.Login) || vi.Author.ID == 0 ||
			vi.Author.ID != verifier.ID || vi.Author.Type != "Bot" {
			refuse("clause-1 (author)", fmt.Sprintf("issue author %s:%d (%s) is not the verifier App %s:%d — an author login alone is spoofable; only the API-read verifier identity is trusted",
				vi.Author.Login, vi.Author.ID, vi.Author.Type, verifier.Login, verifier.ID))
			continue
		}

		// --- clause 2: signature verifies + body not edited since creation. ---
		state, msg := verdictVerifyBody(vi.Body, pub)
		if state != verdictVerified {
			refuse("clause-2 (signature)", msg)
			continue
		}
		if vi.Edited {
			refuse("clause-2 (timeline)", "the issue body was edited after creation — an edited body is a refusal, never a skip-and-trust")
			continue
		}

		// Parse the (already signature-checked) payload for its structured entries.
		rawPayload, _, perr := verdictParseBody(vi.Body)
		if perr != nil {
			refuse("clause-2 (payload)", "could not check: "+perr.Error())
			continue
		}
		var pl verdictPayload
		if uerr := json.Unmarshal(rawPayload, &pl); uerr != nil {
			refuse("clause-2 (payload)", "could not parse the verdict payload JSON: "+uerr.Error())
			continue
		}
		if pl.Schema != verdictSchemaVersion {
			refuse("clause-2 (schema)", fmt.Sprintf("payload schema %q is not %q — a version this build does not recognise is refused, never guessed", pl.Schema, verdictSchemaVersion))
			continue
		}
		if pl.Repo != homeRepo {
			refuse("clause-2 (repo)", fmt.Sprintf("payload repo %q is not this repo %q", pl.Repo, homeRepo))
			continue
		}
		if len(pl.Entries) == 0 {
			refuse("clause-2 (payload)", "the verdict carries no entries")
			continue
		}

		// Evaluate every entry BEFORE planning any append — a verdict is consumed
		// whole or not at all (cl.8: a FAIL verdict / any refusal transcribes
		// nothing). Stage the appends; commit them to the delta only if all pass.
		var staged []evidenceAppend
		verdictOK := true
		for _, e := range pl.Entries {
			// cl.8: a FAIL result transcribes nothing from the whole verdict.
			if !strings.EqualFold(e.Result, "PASS") {
				refuse("clause-8 (fail)", fmt.Sprintf("entry for %s row %d is %q, not PASS — a FAIL verdict transcribes nothing; the verdict issue stays open and the workflow files a brief-freshness triage issue", e.Brief, e.Row, e.Result))
				verdictOK = false
				break
			}
			rel := filepath.ToSlash(strings.TrimSpace(e.Brief))
			bf, ok := briefByRel[rel]
			if !ok {
				refuse("clause-4 (scope)", fmt.Sprintf("entry names brief %q, which is not a brief file under this tree", e.Brief))
				verdictOK = false
				break
			}
			// cl.5: no consumed verdict touches an irreversible brief.
			if strings.EqualFold(strings.TrimSpace(bf.Risk["irreversible"]), "yes") {
				refuse("clause-5 (irreversible)", fmt.Sprintf("brief %s is risk.irreversible: yes — irreversible-brief landings remain verify-gate-close.yml's alone", rel))
				verdictOK = false
				break
			}
			// cl.5: the lane never writes a `human:` stamp (sole writer is verify-gate-close).
			if strings.Contains(e.Evidence, "human:") {
				refuse("clause-5 (human-stamp)", fmt.Sprintf("the Evidence for %s row %d carries a `human:` stamp — the sole human: writer is verify-gate-close.yml; refusing", rel, e.Row))
				verdictOK = false
				break
			}
			if strings.TrimSpace(e.Evidence) == "" {
				refuse("clause-4 (scope)", fmt.Sprintf("entry for %s row %d carries no Evidence markdown to append", rel, e.Row))
				verdictOK = false
				break
			}
			// cl.4: byte-bounded append class. A verdict carrying a multi-kilobyte
			// Evidence string is out of the docs-only class the lane is authorized
			// for — the brief's gate-why (c) requires the write scope cannot exceed
			// the cl.4 byte-bounds.
			if len(e.Evidence) > verdictMaxEvidenceBytes {
				refuse("clause-4 (scope)", fmt.Sprintf("the Evidence for %s row %d is %d bytes, over the %d-byte per-entry bound — out of the byte-bounded docs-only append class", rel, e.Row, len(e.Evidence), verdictMaxEvidenceBytes))
				verdictOK = false
				break
			}
			// cl.4: the append class is Evidence ROWS, never new sections. Evidence
			// carrying a Markdown heading would inject a section — a `## Verify` table
			// included, the F-verify-self-attest surface cl.4 keeps off this lane.
			if verdictEvidenceInjectsHeading(e.Evidence) {
				refuse("clause-4 (scope)", fmt.Sprintf("the Evidence for %s row %d contains a Markdown section heading — the lane appends Evidence rows, never new sections (a Verify-table edit riding the unattended lane is what cl.4 forbids)", rel, e.Row))
				verdictOK = false
				break
			}
			// cl.6: re-execute a check:ci row network-off; refuse the whole verdict
			// on mismatch. `check` (env-bound) rows rest on clauses 1–3 (no re-exec).
			row, hasRow := rowsByRel[rel][e.Row]
			if !hasRow {
				refuse("clause-6 (row)", fmt.Sprintf("brief %s has no Verify row %d to attest", rel, e.Row))
				verdictOK = false
				break
			}
			if row.class() == classCheckCI {
				res := runCheckCI(root, row.Command)
				if res.CouldNotRun {
					refuse("clause-6 (check:ci)", fmt.Sprintf("check:ci row %d of %s could not be re-executed hermetically (%s) — a hermetic check that could not be run hermetically established nothing", e.Row, rel, res.Reason))
					verdictOK = false
					break
				}
				if !res.Passed {
					refuse("clause-6 (check:ci)", fmt.Sprintf("check:ci row %d of %s re-executed network-off and did NOT pass (%s) — the verdict's PASS is not trusted; refusing the whole verdict", e.Row, rel, res.Reason))
					verdictOK = false
					break
				}
			}
			staged = append(staged, evidenceAppend{Brief: rel, Path: bf.Path, Row: e.Row, Line: strings.TrimRight(e.Evidence, "\n")})
		}
		if !verdictOK {
			continue
		}

		// Consumed. Commit its staged appends and record row attestation for flips.
		d.Appends = append(d.Appends, staged...)
		d.Consumed = append(d.Consumed, iss.Number)
		for _, a := range staged {
			if attested[a.Brief] == nil {
				attested[a.Brief] = map[int]bool{}
			}
			attested[a.Brief][a.Row] = true
			contrib[a.Brief] = append(contrib[a.Brief], iss.Number)
		}
	}

	// --- Model-tier flips (cl.4). A brief flips implemented→verified only when it
	// is gate:model, not irreversible, its README row reads exactly `implemented`,
	// and EVERY check:ci/check row in its Verify table is attested PASS by the
	// consumed verdicts. gate:human briefs get their Evidence appended but their
	// flip stays with verify-gate-close.yml (cl.5); the lane never flips them. ---
	for rel, rows := range attested {
		bf := briefByRel[rel]
		s := streamByRel[rel]
		num := numByRel[rel]
		if bf == nil || s == nil || num == "" {
			continue
		}
		if bf.Gate != "model" {
			continue // gate:human / legacy: Evidence only, no model-tier flip
		}
		if strings.EqualFold(strings.TrimSpace(bf.Risk["irreversible"]), "yes") {
			continue
		}
		br := findRow(s, num)
		if br == nil || br.Status != "implemented" {
			continue
		}
		// Coverage: every check:ci/check row attested.
		covered := true
		for n, r := range rowsByRel[rel] {
			cls := r.class()
			if cls != classCheckCI && cls != classCheck {
				continue // gate rows are judgment, outside this lane
			}
			if !rows[n] {
				covered = false
				break
			}
		}
		if !covered {
			continue
		}
		refs := verdictIssueRefs(homeRepo, contrib[rel])
		d.Flips = append(d.Flips, verdictFlip{
			Brief:      s.Name + "/" + num,
			Num:        num,
			ReadmePath: filepath.Join(s.Dir, "README.md"),
			Stamp:      verdictVerifiedStamp(time.Now(), verifier.Login, refs),
		})
	}

	sort.Slice(d.Appends, func(i, j int) bool {
		if d.Appends[i].Path != d.Appends[j].Path {
			return d.Appends[i].Path < d.Appends[j].Path
		}
		return d.Appends[i].Row < d.Appends[j].Row
	})
	sort.Slice(d.Flips, func(i, j int) bool { return d.Flips[i].Brief < d.Flips[j].Brief })
	sort.Slice(d.Refusals, func(i, j int) bool { return d.Refusals[i].Issue < d.Refusals[j].Issue })
	sort.Ints(d.Consumed)
	return d, nil
}

// verdictIssueRefs renders the consumed issue numbers as a deduped, sorted
// `repo#n` reference list for the flip stamp's provenance.
func verdictIssueRefs(repo string, issues []int) string {
	seen := map[int]bool{}
	var uniq []int
	for _, n := range issues {
		if !seen[n] {
			seen[n] = true
			uniq = append(uniq, n)
		}
	}
	sort.Ints(uniq)
	parts := make([]string, 0, len(uniq))
	for _, n := range uniq {
		parts = append(parts, fmt.Sprintf("%s#%d", repo, n))
	}
	return strings.Join(parts, ", ")
}

// verdictVerifiedStamp renders the Verified cell: date first (the repo-wide
// convention the done-shape lint requires), then the verifier App, then the
// verdict issue references so any flip is auditable back to the signed verdicts.
func verdictVerifiedStamp(now time.Time, verifier, refs string) string {
	return fmt.Sprintf("%s %s (verdict %s)", now.Format("2006-01-02"), verifier, refs)
}

// ---------------------------------------------------------------------------
// Apply
// ---------------------------------------------------------------------------

// appendEvidenceLine inserts line at the END of a raw brief file's `## Evidence`
// section body (before the next `## ` heading, or at EOF). It touches nothing else
// — the frontmatter and the `## Verify` section are byte-identical before and after
// (cl.4). Returns an error when the brief has no `## Evidence` heading.
func appendEvidenceLine(raw, line string) (string, error) {
	lines := strings.Split(raw, "\n")
	head := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## Evidence" {
			head = i
			break
		}
	}
	if head < 0 {
		return "", errors.New("brief has no `## Evidence` section to append to")
	}
	// End of section = the line before the next `## ` heading, or EOF.
	end := len(lines)
	for i := head + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	// Insert after the last non-blank line of the section body, so the append lands
	// flush against existing content rather than after trailing blanks.
	insert := head
	for i := head + 1; i < end; i++ {
		if strings.TrimSpace(lines[i]) != "" {
			insert = i
		}
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:insert+1]...)
	out = append(out, line)
	out = append(out, lines[insert+1:]...)
	return strings.Join(out, "\n"), nil
}

// flipRowToVerified rewrites the briefs-table row for brief num: status → verified
// and the Verified cell → stamp. Every other cell's exact text is preserved. It
// reuses the same table-locate + setCell machinery as flipRowToDone. Returns an
// error if the table or row is not found.
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
		if _, ok := idx["brief"]; !ok {
			continue
		}
		si, ok := idx["status"]
		if !ok {
			continue
		}
		for k := i + 2; k < len(lines); k++ { // i+1 is the |---| separator
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
			cells[si] = setCell(cells[si], "verified")
			if vi, ok := idx["verified"]; ok {
				cells[vi] = setCell(cells[vi], verifiedStamp)
			}
			lines[k] = "|" + strings.Join(cells, "|") + "|"
			return strings.Join(lines, "\n"), nil
		}
		return "", fmt.Errorf("no row for #%s in briefs table", num)
	}
	return "", fmt.Errorf("no briefs table found")
}

// applyVerdictDelta writes the derived delta to the tree: Evidence appends first
// (grouped per file), then README flips (grouped per README). It NEVER commits,
// pushes, or mutates any GitHub issue — the workflow commits the tree the tool
// wrote. Returns the count of files touched.
func applyVerdictDelta(d verdictDelta) (int, error) {
	touched := 0
	// Group appends by file so a brief with several attested rows is read and
	// written once, in row order (d.Appends is already sorted by path then row).
	i := 0
	for i < len(d.Appends) {
		path := d.Appends[i].Path
		b, err := os.ReadFile(path)
		if err != nil {
			return touched, err
		}
		raw := string(b)
		for i < len(d.Appends) && d.Appends[i].Path == path {
			updated, aerr := appendEvidenceLine(raw, d.Appends[i].Line)
			if aerr != nil {
				return touched, fmt.Errorf("%s: %w", path, aerr)
			}
			raw = updated
			i++
		}
		// cl.4 backstop: the frontmatter block and the `## Verify` section must be
		// byte-identical before and after — a verify landing appends Evidence and
		// flips a cell; it never edits a Verify table (which verifyrun later
		// executes from merged main) or the frontmatter. Independent of the
		// eval-time byte-bound/heading checks; a violation writes nothing.
		if err := assertVerdictWriteScope(string(b), raw); err != nil {
			return touched, fmt.Errorf("%s: cl.4 write-scope invariant: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			return touched, err
		}
		touched++
	}
	for _, f := range d.Flips {
		b, err := os.ReadFile(f.ReadmePath)
		if err != nil {
			return touched, err
		}
		updated, ferr := flipRowToVerified(string(b), f.Num, f.Stamp)
		if ferr != nil {
			return touched, fmt.Errorf("%s: %w", f.ReadmePath, ferr)
		}
		if err := os.WriteFile(f.ReadmePath, []byte(updated), 0o644); err != nil {
			return touched, err
		}
		touched++
	}
	return touched, nil
}

// ---------------------------------------------------------------------------
// Run
// ---------------------------------------------------------------------------

// runTranscribeVerdict is the --transcribe-verdict entrypoint (the workflow's "run"
// step). It derives and, unless dryRun, APPLIES the R-6 delta to the tree. It NEVER
// commits, pushes, or mutates any GitHub issue. Process exit code:
//
//	0  ran (armed + applied/derived), INERT (evaluated no clause), or HELD
//	   (cl.10 main-health hold) — all neutral
//	2  REFUSED: roster unconfigured, no verifier App bound, or (local write path) a
//	   primary-checkout root
//	3  FLOOD: more than the cl.9 threshold of unconsumed verdict issues — nothing is
//	   written; the workflow files one triage issue
//
// dryRun is the CI-testable "--check" surface: it derives and reports the would-be
// delta and the per-verdict refusal log without touching the filesystem.
func runTranscribeVerdict(root string, dryRun bool, pubkeyPath string,
	list issueLister, resolveIssue verdictIssueResolver,
	runCheckCI checkCIRunner, mainHealth mainHealthChecker,
	resolveSignoff commentResolver) int {

	// P1: an unconfigured roster is CLOSED — this lane WRITES durable Evidence/flips
	// keyed on identities the roster names (the verifier App, the bless authority).
	if !scanEffectiveConfig().Configured() {
		fmt.Fprintln(os.Stderr, "statusgen --transcribe-verdict REFUSED: the trust roster is NOT CONFIGURED, so no verifier identity is trusted and no bless authority can arm the lane (fail closed).")
		return 2
	}

	// Isolation guard (local write path only). In CI the checkout is an ephemeral
	// tree the workflow exists to write and commit; the guard stops a LOCAL run from
	// dirtying a live session's shared checkout. --dry-run writes nothing.
	if !dryRun && !scanInCI() {
		if reason := scanIsolationRefusal(root); reason != "" {
			fmt.Fprintln(os.Stderr, "statusgen --transcribe-verdict REFUSED:", reason)
			return 2
		}
	}

	// --- Enactment gate (R-6): evaluate NOTHING until armed. ---
	armed, reason := transcribeVerdictEnactmentGate(root, resolveSignoff)
	if !armed {
		fmt.Println("transcribe-verdict: INERT —", reason)
		fmt.Println("transcribe-verdict: evaluating no clause; the lane is disarmed until R-6 is signed")
		return 0
	}
	fmt.Println("transcribe-verdict:", reason)

	// --- cl.10: main-health hold. A red or freshly-red main pauses the lane. ---
	if mainHealth != nil {
		held, why, herr := mainHealth()
		if herr != nil {
			fmt.Println("transcribe-verdict: HELD — main-health signal unreadable, pausing the lane (fail closed):", herr)
			return 0
		}
		if held {
			fmt.Println("transcribe-verdict: HELD —", why)
			fmt.Println("transcribe-verdict: landing nothing while main is red or freshly-red; resume by fixing main, not by overriding the lane")
			return 0
		}
	}

	verifier, ok := verdictVerifierIdentity()
	if !ok {
		fmt.Fprintln(os.Stderr, "statusgen --transcribe-verdict REFUSED: no `verifier=` App bound in ASSAY_TRUSTED_BOT_SLUGS with a numeric id — there is no verifier identity whose verdicts could be trusted.")
		return 2
	}

	homeRepo := scanHomeRepo()
	if homeRepo == "" {
		fmt.Println("transcribe-verdict: no home repo configured (ASSAY_HOME_REPO) — the lane has nothing to sweep")
		return 0
	}

	pub, perr := verdictResolvePubkey(pubkeyPath)
	if perr != nil {
		// A missing/unreadable public key is could-not-check for EVERY verdict, not a
		// crash: the lane reports it and consumes nothing (a missing key is never trust).
		fmt.Println("transcribe-verdict: could-not-check — no usable verifier public key:", perr)
		pub = nil
	}

	streams, _, err := loadStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen --transcribe-verdict:", err)
		return 1
	}

	delta, err := planTranscribeVerdict(root, streams, homeRepo, list, resolveIssue, pub, verifier, runCheckCI)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen --transcribe-verdict:", err)
		return 1
	}

	// --- cl.9: flood tripwire. Refuse the WHOLE run; the workflow files one triage
	// issue. Nothing is written. ---
	if delta.VerdictCount > verdictFloodThreshold {
		fmt.Printf("transcribe-verdict: FLOOD — clause-9 tripwire: %d unconsumed verdict issues exceed the threshold of %d; refusing the run (nothing written)\n",
			delta.VerdictCount, verdictFloodThreshold)
		return 3
	}

	for _, n := range delta.Notices {
		fmt.Println("transcribe-verdict: NOTICE —", n)
	}
	// Refusals (clause named). cl.8: each transcribes nothing; the verdict issue
	// stays open and the workflow files/updates ONE brief-freshness triage issue.
	for _, r := range delta.Refusals {
		fmt.Printf("transcribe-verdict: REFUSE %s#%d — %s: %s\n", homeRepo, r.Issue, r.Clause, r.Reason)
	}
	for _, n := range delta.Consumed {
		fmt.Printf("transcribe-verdict: CONSUME %s#%d\n", homeRepo, n)
	}
	for _, a := range delta.Appends {
		fmt.Printf("transcribe-verdict: EVIDENCE %s row %d += %s\n", a.Brief, a.Row, a.Line)
	}
	for _, f := range delta.Flips {
		fmt.Printf("transcribe-verdict: FLIP %s implemented→verified — %s\n", f.Brief, f.Stamp)
	}

	if dryRun {
		if len(delta.Appends) == 0 && len(delta.Flips) == 0 {
			fmt.Println("transcribe-verdict: no changes — nothing to append or flip")
		}
		return 0
	}

	touched, aerr := applyVerdictDelta(delta)
	if aerr != nil {
		fmt.Fprintln(os.Stderr, "statusgen --transcribe-verdict:", aerr)
		return 1
	}
	fmt.Printf("transcribe-verdict: applied — %d file(s) touched (%d Evidence append(s), %d flip(s))\n",
		touched, len(delta.Appends), len(delta.Flips))
	return 0
}
