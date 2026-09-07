package deskkit

// briefrisk.go — resolving a PR's OWNING BRIEF from its body's `Brief:` trailer, and
// reading that brief's OWN gate/risk frontmatter as a risk-classification signal.
//
// WHY THIS EXISTS. Risk classification for the ready-flip gate was reading the diff and
// the repo visibility, but never the brief the PR delivers. A brief declares its own
// sensitivity in its frontmatter — `gate: human` and the four `risk:` flags — and that
// declaration is authoritative for work whose sensitivity lives in intent rather than in
// a touched path. A PR whose brief is human-gated (or answers any risk flag `yes`) that
// changes only code the compiled path-triggers do not name was therefore classed NOT
// risk-classed, and the `Security-Review: pass` requirement the gate exists to enforce
// never fired. This resolves the owning brief from the trailer and reads that frontmatter
// so the declared sensitivity is consulted.
//
// TRAILER PARSE is the shared ParseTrailers (trailer.go); FILE RESOLUTION is against the
// CONFIGURED stream roots (roots.go), the same roots the multi-repo board covers. The
// value-splitter here is a small LOCAL helper (splitBriefRef) rather than a shared export,
// deliberately — the shared canonicalisers are being introduced elsewhere, and this change
// stays self-contained to avoid colliding with them; a later consolidation can fold this
// onto the shared splitter once that lands.
//
// TWO CASES, and only-widening within each:
//
//   - TRAILER ABSENT (no `Brief:` trailer at all). This term makes NO risk claim and the
//     diff/visibility/label terms decide alone. A PR that declares no brief is not the
//     concern here (whether a no-trailer PR should be treated differently in a world where
//     App PRs must carry a trailer is a separate policy call, deliberately not made here).
//
//   - TRAILER PRESENT. A brief is DECLARED, so its gate/risk answer is authoritative — and
//     a declaration we cannot READ is UNVERIFIABLE, never clean. Every failure to resolve
//     or read the named brief (no configured root, no brief file, an unreadable file, a
//     malformed trailer value, or a brief with no parseable frontmatter fence) is therefore
//     RISK-CLASSED, fail closed — the same "a short read is UNVERIFIABLE, not clean"
//     treatment the changed-file gate already uses. You cannot prove a brief you could not
//     read is not `gate: human` / `risk: yes`, so the safe answer is to require the review.
//     Only a brief that RESOLVED and whose frontmatter POSITIVELY says non-risk answers
//     false. Like every other risk term (riskclassifier.go) it only ever WIDENS — it never
//     WAIVES a gate another term set.

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// BriefRisk is the result of resolving a PR body's `Brief:` trailer.
type BriefRisk struct {
	// OwningBrief is the canonical "<stream>/<NN>" id the body's `Brief:` trailer names,
	// or "" when the body carries no `Brief:` trailer (or one whose value does not split
	// into <stream>/<NN>). It is set from the trailer whether or not the brief FILE could
	// be found, so a caller can attribute the PR to its brief even on a root this process
	// is not configured for.
	OwningBrief string
	// Resolved is true when OwningBrief named a brief file that was found, read, and whose
	// frontmatter fence parsed. Only a Resolved brief's RiskClassed is a positive statement
	// about the brief's declaration; when false, a true RiskClassed is the fail-closed
	// UNVERIFIABLE answer (see Unverifiable).
	Resolved bool
	// RiskClassed is true when the brief risk-classes the PR — either a resolved brief whose
	// frontmatter gates on a human (`gate: human`) or answers any `risk:` flag `yes`, OR a
	// DECLARED-but-unverifiable brief (trailer present, brief not readable). A trailer-absent
	// body is never risk-classed by this term.
	RiskClassed bool
	// Unverifiable is true when RiskClassed was set NOT by a positive declaration but because
	// a declared brief could not be resolved/read/parsed, so its gate/risk answer is unknown
	// and the term fails closed. A caller may use it to word the difference ("declared but
	// unresolvable" vs "the brief declares risk"); both require the security review.
	Unverifiable bool
	// Reason is a short human-readable explanation, for a refusal message. "" when not
	// risk-classed.
	Reason string
}

// briefGateLine matches the top-level `gate:` frontmatter line.
var briefGateLine = regexp.MustCompile(`(?m)^gate:\s*(.*)$`)

// briefRiskKeys are the four canonical risk-answer keys, in the brief-v1 order.
var briefRiskKeys = []string{"regulatory", "customer", "irreversible", "sensitive-data"}

// BriefRiskFromBody parses body for a `Brief:` trailer, resolves it to a brief file under
// the repo's configured stream root, reads the brief frontmatter, and reports the owning
// brief id plus whether the brief risk-classes the PR.
//
// It never errors. A body with NO `Brief:` trailer makes no risk claim. A body WITH one
// that cannot be resolved/read/parsed is RISK-CLASSED and Unverifiable (fail closed — see
// the file header). Both the slash form `<stream>/<NN>` and the colon forms
// `<...>:<stream>:<NN>` are accepted.
func BriefRiskFromBody(repo, body string) BriefRisk {
	trs, err := ParseTrailers([]byte(body))
	if err != nil {
		// A malformed trailer SET. If a Brief: trailer is implicated, a brief is declared but
		// cannot be cleanly identified → unverifiable, fail closed. A malformation implicating
		// no Brief: trailer (e.g. a duplicate Issue:) declares no brief, so it is the
		// trailer-absent case: no risk claim, the other terms decide.
		if briefImplicatedInTrailerError(err) {
			return unverifiableBrief("", "the PR body's trailer set is malformed ("+err.Error()+
				") and a Brief: trailer is implicated — the owning brief cannot be identified")
		}
		return BriefRisk{}
	}
	var val string
	for _, t := range trs {
		if t.Kind == TrailerBrief {
			val = t.Value
			break
		}
	}
	if val == "" {
		// TRAILER ABSENT — no risk claim; the caller falls back to path/visibility/label.
		return BriefRisk{}
	}

	// From here a Brief: trailer is PRESENT: every failure below is UNVERIFIABLE, not clean.
	stream, nn, ok := splitBriefRef(val)
	if !ok {
		return unverifiableBrief("", "owning brief trailer "+strconv.Quote(val)+
			" does not name a resolvable <stream>/<NN>")
	}
	owning := stream + "/" + nn

	root := RootForRepo(repo)
	if root == "" {
		return unverifiableBrief(owning, "owning brief "+owning+" is declared but this process has "+
			"no configured stream root for "+repo+" — the brief could not be consulted")
	}
	matches, _ := filepath.Glob(filepath.Join(root, "docs", "streams", stream, "brief-"+nn+"-*.md"))
	if len(matches) == 0 {
		return unverifiableBrief(owning, "owning brief "+owning+" is declared but no brief file "+
			"resolves under "+repo+"'s configured stream root")
	}
	raw, rerr := os.ReadFile(matches[0])
	if rerr != nil {
		return unverifiableBrief(owning, "owning brief "+owning+" resolved to a file that could not "+
			"be read ("+rerr.Error()+")")
	}
	classed, reason, hasFrontmatter := briefFrontmatterRisk(string(raw))
	if !hasFrontmatter {
		return unverifiableBrief(owning, "owning brief "+owning+" has no parseable frontmatter fence — "+
			"its gate/risk declaration cannot be read")
	}
	return BriefRisk{OwningBrief: owning, Resolved: true, RiskClassed: classed, Reason: reason}
}

// unverifiableBrief builds the fail-closed result for a DECLARED brief whose gate/risk
// answer could not be established. It is always risk-classed — you cannot prove an
// unreadable brief is not gate:human/risk:yes — with a reason that names the failure and
// says the safe conclusion out loud.
func unverifiableBrief(owning, why string) BriefRisk {
	return BriefRisk{
		OwningBrief:  owning,
		RiskClassed:  true,
		Unverifiable: true,
		Reason:       why + " — unverifiable, fail closed (a declared brief that cannot be read is not clean)",
	}
}

// briefImplicatedInTrailerError reports whether a ParseTrailers error involves a Brief:
// trailer (both-kinds present, or a duplicate Brief:). A duplicate Issue: with no Brief:
// declares no brief and so is NOT implicated.
func briefImplicatedInTrailerError(err error) bool {
	var both *ErrTrailerBoth
	if errors.As(err, &both) {
		return true
	}
	var dup *ErrTrailerDuplicate
	if errors.As(err, &dup) {
		return dup.Kind == TrailerBrief
	}
	return false
}

// briefFrontmatterRisk reads a brief file's frontmatter and reports whether it declares
// the PR risk-bearing: `gate: human`, OR any of the four `risk:` flags answered `yes`
// (both the inline flow map and per-line forms). Matching is confined to the frontmatter
// fence so a `sensitive-data: yes` in prose cannot risk-class a PR by accident. The third
// return, hasFrontmatter, is FALSE when the content carries no parseable `---` fence — the
// caller treats that as unverifiable (a resolved brief with no readable frontmatter cannot
// be called clean), distinct from a fence that parsed and positively answered non-risk.
func briefFrontmatterRisk(content string) (classed bool, reason string, hasFrontmatter bool) {
	block := briefFrontmatterFence(content)
	if block == "" {
		return false, "", false
	}
	if m := briefGateLine.FindStringSubmatch(block); m != nil {
		if strings.EqualFold(strings.Trim(strings.TrimSpace(m[1]), `"'`), "human") {
			return true, "owning brief is human-gated (gate: human)", true
		}
	}
	for _, k := range briefRiskKeys {
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(k) + `\s*:\s*yes`)
		if re.MatchString(block) {
			return true, "owning brief declares risk: " + k + " = yes", true
		}
	}
	return false, "", true
}

// briefFrontmatterFence returns the text between the leading `---` fences, or "" when the
// content does not open with one. Mirrors the small extractors verifyloop/deskdispatch
// carry — deskkit keeps its own so it does not import a command package.
func briefFrontmatterFence(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n")
		}
	}
	return ""
}

// splitBriefRef reduces the accepted `Brief:` trailer value forms to (stream, NN): the
// slash form `<stream>/<NN>`, and the colon forms `<stream>:<NN>`, `<repo>:<stream>:<NN>`,
// and `<cell>:<repo>:<stream>:<NN>`. For the colon forms the LAST two parts are stream and
// NN; any repo/cell prefixes are not needed for the file resolution here. NN must be
// numeric. It is a package-LOCAL helper on purpose — see the file header.
func splitBriefRef(v string) (stream, nn string, ok bool) {
	v = strings.TrimSpace(v)
	var parts []string
	if strings.Contains(v, ":") {
		parts = strings.Split(v, ":")
		if len(parts) < 2 {
			return "", "", false
		}
		stream, nn = parts[len(parts)-2], parts[len(parts)-1]
	} else {
		parts = strings.Split(v, "/")
		if len(parts) != 2 {
			return "", "", false
		}
		stream, nn = parts[0], parts[1]
	}
	stream, nn = strings.TrimSpace(stream), strings.TrimSpace(nn)
	if stream == "" || nn == "" {
		return "", "", false
	}
	for _, c := range nn {
		if c < '0' || c > '9' {
			return "", "", false
		}
	}
	return stream, nn, true
}
