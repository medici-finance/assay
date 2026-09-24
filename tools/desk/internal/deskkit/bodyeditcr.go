package deskkit

// bodyeditcr.go is the CANONICAL reader and decision for the third same-head exemption:
// the DOCUMENTED BODY-EDIT RE-VERIFICATION class. It is the sibling of checkonlycr.go and
// externalprerequisite.go, and it follows their discipline exactly — an EXPLICIT, TYPED
// declaration, never prose; grant-direction markers that skip fenced code; and a pure
// decision that fails closed on every clause it cannot positively establish.
//
// THE CASE. A CHANGES_REQUESTED whose ONLY blocker is the PR BODY (the change description)
// asserting something false or stale. The sanctioned answer to that finding is a body edit,
// and a body edit never moves the head — so the unchanged-head rule ("an APPROVED at an
// unchanged head cannot be a re-verification; only a new commit clears this") had no path out
// except a no-op push (which games the rule) or a human dismissing the review by hand. The
// ruling that authorises this class accepts a same-head APPROVE when the APPROVE's own body
// DOCUMENTS three things: (i) it re-read the live PR body via the forge API, (ii) it verified
// the specific standing finding resolved, and (iii) CI is green at that head. The flip gate
// keeps its own mechanical checks and the merge stays human.
//
// THE NARROWEST READING, and why each clause is typed. "Documented" is read as a fixed,
// machine-checkable shape, never as English, for the same reason the check-only and
// external-prerequisite exemptions refuse prose: guessing on a GRANT path is how a
// laundering hole opens. Each element of the ruling maps to one line:
//
//	on the CHANGES_REQUESTED (written when the block is posted):
//	  Blocked-On-Body: <finding-id> <body-digest-at-CR>
//	on the APPROVE that answers it:
//	  Resolved-Body-Finding: <finding-id>          (ii) the specific finding, by id
//	  Body-Reread-Digest: <body-digest-re-read>    (i)  the live body, as re-read
//	  CI-Green-At: <full head sha>                 (iii) CI green at THAT head
//
// and the decision verifies what it can INDEPENDENTLY rather than trusting the citation:
//
//   - the CR's declaration is the whole of its blocking findings (a typed finding block that
//     carries any OTHER blocking finding makes the CR mixed, and mixed never qualifies — a
//     code finding needs a code change);
//   - the re-read digest EQUALS the digest of the live body the caller read at the gate, so
//     the documented re-read is of the body that is actually there now;
//   - the re-read digest DIFFERS from the digest the CR recorded, so the body really was
//     edited after the block (an APPROVE over an unedited body is exactly the no-op
//     re-approval the unchanged-head rule refuses);
//   - the APPROVE was submitted AFTER the CR, and cites the finding id the CR declared and
//     the head both reviews are pinned to.
//
// (iii)'s citation is checked for SHAPE and HEAD here; whether CI actually IS green stays the
// caller's own mechanical condition (deskflip `checks-green`, the board's CI verdict), which
// this exemption never replaces — "the flip gate keeps its own mechanical check".
//
// WHAT IS NOT EXEMPTED. Clearing the block is not an approval: the caller still needs an
// APPROVED to govern at head. A CR that declares BOTH this class and another exemption class
// (Blocked-On-Check / External-Prereq-Only) has made two contradictory "sole blocker" claims
// and qualifies for neither here.
//
// THE DIGEST. PRBodyDigest is lowercase hex SHA-256 over the body with every carriage return
// removed and trailing newlines trimmed — the form a shell recipe produces without any
// further normalisation:
//
//	printf '%s' "$(gh api repos/<owner>/<repo>/pulls/<N> --jq .body | tr -d '\r')" | shasum -a 256
//
// (command substitution strips the trailing newlines; `tr` strips the carriage returns).

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"
)

// blockedOnBody matches the CR-side declaration. The value is two whitespace-separated
// fields — the finding id and the digest of the body the reviewer blocked on — checked for
// shape by ParseBodyEditDeclaration, not by this line regexp, so a malformed value is a
// DECLARED-but-unusable claim (and says so) rather than silently no claim at all.
var blockedOnBody = regexp.MustCompile(`(?i)^[ \t]*Blocked-On-Body:[ \t]*(\S.*?)[ \t\r]*$`)

// resolvedBodyFinding matches the APPROVE-side finding citation: one id token.
var resolvedBodyFinding = regexp.MustCompile(`(?i)^[ \t]*Resolved-Body-Finding:[ \t]*(\S+)[ \t\r]*$`)

// bodyRereadDigest matches the APPROVE-side re-read citation: a 64-hex SHA-256 digest.
var bodyRereadDigest = regexp.MustCompile(`(?i)^[ \t]*Body-Reread-Digest:[ \t]*([0-9a-f]{64})[ \t\r]*$`)

// ciGreenAt matches the APPROVE-side CI citation: a FULL commit sha (40 hex, or 64 on a
// SHA-256 object-format repo). An abbreviated sha does not match — a prefix is a claim about
// every commit that shares it, and the class is about exactly one head.
var ciGreenAt = regexp.MustCompile(`(?i)^[ \t]*CI-Green-At:[ \t]*([0-9a-f]{40}|[0-9a-f]{64})[ \t\r]*$`)

// hex64 is the digest shape the CR declaration's second field must carry.
var hex64 = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// PRBodyDigest is the canonical digest of a PR body for the body-edit class (see the file
// doc for the normalisation and the matching shell recipe).
func PRBodyDigest(body string) string {
	norm := strings.TrimRight(strings.ReplaceAll(body, "\r", ""), "\n")
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}

// BodyEditDeclared reports whether a CHANGES_REQUESTED body carries a `Blocked-On-Body:`
// declaration line (well-formed or not). It is the shared ROUTER: a caller uses it to send a
// standing CR to this class's decision rather than another exemption's. It answers only "was
// the class claimed", never "is it granted".
func BodyEditDeclared(body string) bool {
	return len(VerdictMarkerValues(body, blockedOnBody, SkipFenced)) > 0
}

// BodyEditDeclaration is a CR body parsed for the body-edit class.
type BodyEditDeclaration struct {
	Declared  bool
	FindingID string
	CRDigest  string // lowercase
	// Problem is non-empty when the declaration is present but cannot qualify — malformed,
	// ambiguous, mixed, or co-declared with another exemption class. It names the clause.
	Problem string
}

// ParseBodyEditDeclaration reads a CHANGES_REQUESTED body's body-edit declaration.
func ParseBodyEditDeclaration(crBody string) BodyEditDeclaration {
	d := BodyEditDeclaration{Declared: BodyEditDeclared(crBody)}
	if !d.Declared {
		return d
	}
	val := SoleVerdictMarkerValue(crBody, blockedOnBody, SkipFenced)
	if val == "" {
		d.Problem = "the CR carries two `Blocked-On-Body:` lines that disagree — an ambiguous declaration establishes nothing"
		return d
	}
	fields := strings.Fields(val)
	if len(fields) != 2 || !hex64.MatchString(fields[1]) {
		d.Problem = "the CR's `Blocked-On-Body: " + val + "` is malformed — it must be exactly `<finding-id> <64-hex body digest at the CR>`"
		return d
	}
	d.FindingID, d.CRDigest = fields[0], strings.ToLower(fields[1])

	if BlockedOnCheckName(crBody) != "" || ExternalPrereqOnlyDeclared(crBody) {
		d.Problem = "the CR declares the body-edit class AND another sole-blocker class (Blocked-On-Check / " +
			"External-Prereq-Only) — two contradictory sole-blocker claims qualify for neither"
		return d
	}
	block, present, err := ParseFindingBlock(crBody)
	if err != nil {
		d.Problem = "the CR's typed finding block is unreadable (" + err.Error() +
			") — an unreadable declaration is could-not-check, never a clearance"
		return d
	}
	if present {
		for _, f := range block.Findings {
			if f.Severity != SeverityBlocking {
				continue
			}
			if strings.TrimSpace(f.ID) != d.FindingID || f.Blocker != BlockerCodeContent {
				d.Problem = "the CR's finding block carries blocking finding " + strings.TrimSpace(f.ID) +
					" (" + string(f.Blocker) + ") beyond the declared body finding " + d.FindingID +
					" — a mixed rejection is never body-edit-only; a code finding needs a code change"
				return d
			}
		}
	}
	return d
}

// BodyEditApprove is one candidate clearing APPROVE. The CALLER supplies only reviews that are
// from the reviewer identity, APPROVED, at the same head as the CR, and in the correctness
// lane (no security marker) — the decision reasons over what it is given.
type BodyEditApprove struct {
	Body        string
	SubmittedAt string // RFC3339
}

// BodyEditInput is everything EvaluateBodyEditReverification needs.
type BodyEditInput struct {
	CRBody        string
	CRSubmittedAt string // RFC3339
	Head          string // the head both the CR and the approves are pinned to
	Approves      []BodyEditApprove
	// LiveBody is the PR body as the caller read it AT THE GATE — fresh, never cached from
	// the reviewer's citation.
	LiveBody string
}

// BodyEditDecision is the result. Declared says the class was claimed; Cleared says the
// standing CR is lifted. Reason is always populated, so a refusal names its clause.
type BodyEditDecision struct {
	Declared bool
	Cleared  bool
	Reason   string
}

// EvaluateBodyEditReverification decides whether a standing CHANGES_REQUESTED clears at the
// unchanged head under the documented body-edit class. PURE: same inputs, same decision.
func EvaluateBodyEditReverification(in BodyEditInput) BodyEditDecision {
	d := ParseBodyEditDeclaration(in.CRBody)
	dec := BodyEditDecision{Declared: d.Declared}
	if !d.Declared {
		dec.Reason = "no `Blocked-On-Body:` declaration on the standing CHANGES_REQUESTED — the unchanged-head rule stands"
		return dec
	}
	if d.Problem != "" {
		dec.Reason = d.Problem
		return dec
	}
	head := strings.TrimSpace(in.Head)
	if head == "" {
		dec.Reason = "the head is unknown — a same-head exemption cannot be established against no head"
		return dec
	}
	crAt, err := time.Parse(time.RFC3339, strings.TrimSpace(in.CRSubmittedAt))
	if err != nil {
		dec.Reason = "the CR's submission time " + in.CRSubmittedAt + " is not readable as RFC3339 — whether the " +
			"APPROVE came after it cannot be established"
		return dec
	}
	live := PRBodyDigest(in.LiveBody)

	considered := 0
	for _, a := range in.Approves {
		at, perr := time.Parse(time.RFC3339, strings.TrimSpace(a.SubmittedAt))
		if perr != nil || !at.After(crAt) {
			continue
		}
		considered++
		why := bodyEditApproveProblem(a.Body, d, head, live)
		if why == "" {
			dec.Cleared = true
			dec.Reason = "documented body-edit re-verification: finding " + d.FindingID + " resolved, live body " +
				"re-read (digest " + live[:12] + ", edited since the CR), CI-green cited at " + short12(head)
			return dec
		}
		dec.Reason = why // the latest after-CR approve's failure is the one reported
	}
	if considered == 0 {
		dec.Reason = "the CR declares `Blocked-On-Body: " + d.FindingID + " …` but no correctness APPROVE from the " +
			"reviewer at this head was submitted after it"
	}
	return dec
}

// bodyEditApproveProblem returns "" when one APPROVE body documents all three elements and
// they verify, else the first clause it fails on.
func bodyEditApproveProblem(body string, d BodyEditDeclaration, head, liveDigest string) string {
	id := SoleVerdictMarkerValue(body, resolvedBodyFinding, SkipFenced)
	if id == "" {
		return "the APPROVE does not document which finding it verified resolved — it must carry " +
			"`Resolved-Body-Finding: " + d.FindingID + "` (an undocumented re-approve is the no-op the unchanged-head rule refuses)"
	}
	if id != d.FindingID {
		return "the APPROVE cites `Resolved-Body-Finding: " + id + "`, not the finding the CR declared (" + d.FindingID + ")"
	}
	dg := strings.ToLower(SoleVerdictMarkerValue(body, bodyRereadDigest, SkipFenced))
	if dg == "" {
		return "the APPROVE does not document a live-API re-read of the PR body — it must carry " +
			"`Body-Reread-Digest: <64-hex>`"
	}
	if dg != liveDigest {
		return "the APPROVE's `Body-Reread-Digest` (" + dg[:12] + ") is not the digest of the live PR body (" +
			liveDigest[:12] + ") — the body it re-read is not the body that is there now"
	}
	if dg == d.CRDigest {
		return "the APPROVE's re-read digest equals the digest the CR recorded — the body was not edited after the " +
			"block, so there is nothing new to verify"
	}
	ci := SoleVerdictMarkerValue(body, ciGreenAt, SkipFenced)
	if ci == "" {
		return "the APPROVE does not document CI green at this head — it must carry `CI-Green-At: <full head sha>`"
	}
	if !strings.EqualFold(ci, head) {
		return "the APPROVE cites `CI-Green-At: " + short12(ci) + "`, not the current head " + short12(head)
	}
	return ""
}
