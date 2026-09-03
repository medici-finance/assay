// Package verifier holds qualgen's pluggable agent-verification boundary for
// the code-slop forensic sweep lane (brief quality/16, spec §3): the
// AgentVerifier interface plus the plain data types that cross it. Like the
// sibling adapters package, it is deliberately self-contained — no dependency
// on the qualgen command package — so a live coding-agent CLI adapter is pure
// CONFIGURATION shipped separately, never new orchestration code wired into
// main. The offline scripted-fixture reference adapter (fixture.go) lives here
// so the whole lane is testable with no live agent.
//
// The two-stage split the lane implements (deterministic linters nominate;
// an agent verifies each nominee against its surrounding code) exists because
// each stage covers the other's failure mode: linters are context-blind (false
// positives/negatives) and agents overclaim (a persuasive paragraph is not
// evidence). This package carries the agent half of that contract; the
// evidence-enforcement backstop that reclassifies an evidence-free verdict
// lives EMITTER-side in package main, deliberately a DIFFERENT component so a
// buggy or overconfident verifier cannot self-confirm.
package verifier

// Suspect is one deterministic lead nominated by leg 1 (the linter front-end)
// and carried, unchanged, through all three legs — one schema, so evidence is
// never misattributed as it flows linter → verdict → report. It is the record
// appended to suspects.jsonl (package main embeds it) and the input handed to
// AgentVerifier.Verify.
type Suspect struct {
	// Fingerprint is the suspect's stable identity: category + normalized
	// path + line-window, hashed. It is what the standing lane diffs across
	// runs to partition new / persistent / cleared, and the key a verdict
	// points back to.
	Fingerprint string `json:"fingerprint"`
	// Category is one of the canonical sweep categories (dead-code,
	// swallowed-error, module-size, duplication). It is NOT the tool name —
	// several tools can feed one category, and one category may have no tool
	// configured (a could-not-measure category, never a silent zero).
	Category string `json:"category"`
	// File is the repo-relative, slash-separated path the linter flagged.
	File string `json:"file"`
	// LineStart / LineEnd bound the flagged region (1-based, inclusive).
	// LineEnd == LineStart when the tool reported a single line.
	LineStart int `json:"line_start"`
	LineEnd   int `json:"line_end"`
	// Tool is the executable that nominated this suspect (argv[0] of the
	// configured command), recorded so the report can attribute the lead.
	Tool string `json:"tool"`
	// Rule is the tool's rule/check identifier (e.g. "U1000"), carried so the
	// report names WHICH check flagged the region, not merely that one did.
	Rule string `json:"rule"`
	// RawEvidence is the verbatim tool output line(s) for this suspect — the
	// deterministic half of the evidence the report shows a human.
	RawEvidence string `json:"raw_evidence"`
}

// ContextPack is the size-capped evidence bundle assembled per suspect and
// handed to the verifier: enough of the surrounding code to adjudicate the
// lead, never the whole tree. Assembly and the size cap live in package main;
// this is the boundary shape the verifier reads.
type ContextPack struct {
	// Category is the suspect's category, so a verifier can select a
	// per-category prompt.
	Category string
	// FileRegion is the suspect's file window (the flagged lines plus a
	// bounded margin), size-capped.
	FileRegion string
	// RawEvidence is the linter's verbatim output for the suspect.
	RawEvidence string
	// RelatedExcerpts are optional bounded excerpts of related code
	// (caller/callee regions) the assembler could cheaply gather.
	RelatedExcerpts []string
}

// VerdictClass is the four-state adjudication outcome. confirmed and
// needs-human are the two ACTIONABLE classes and are exactly the two the
// emitter-side evidence gate guards: either MUST carry a non-empty evidence
// pointer or it is reclassified could-not-verify before it can be rendered as
// actionable.
type VerdictClass string

const (
	// ClassConfirmed — the agent confirms the lead is a real defect. Actionable
	// ONLY with a non-empty evidence pointer.
	ClassConfirmed VerdictClass = "confirmed"
	// ClassFalsePositive — the agent judged the lead a linter false positive.
	// Rendered in a suppressed section WITH its reason, never silently dropped.
	ClassFalsePositive VerdictClass = "false-positive"
	// ClassNeedsHuman — the agent cannot decide; a human must. Actionable ONLY
	// with a non-empty evidence pointer.
	ClassNeedsHuman VerdictClass = "needs-human"
	// ClassCouldNotVerify — the three-state could-not-measure of this lane: the
	// verifier errored, or the emitter reclassified an evidence-free
	// confirmed/needs-human here. Listed as such, never dropped, never
	// actionable.
	ClassCouldNotVerify VerdictClass = "could-not-verify"
)

// Actionable reports whether a class is one a human triager should act on. Only
// confirmed and needs-human are — and only the emitter, after the evidence
// gate, ever renders them so.
func (c VerdictClass) Actionable() bool {
	return c == ClassConfirmed || c == ClassNeedsHuman
}

// Verdict is the agent's adjudication of one suspect. EvidencePointer is the
// load-bearing field: an actionable class with an empty pointer is malformed by
// construction (a persuasive Rationale is not evidence), and the emitter-side
// gate reclassifies it could-not-verify rather than trust it.
type Verdict struct {
	// Fingerprint ties the verdict back to its suspect. The verifier need not
	// set it; the emitter stamps it from the suspect it verified.
	Fingerprint string `json:"fingerprint"`
	// Class is the adjudication outcome.
	Class VerdictClass `json:"class"`
	// EvidencePointer is file:line plus a quoted excerpt — the concrete
	// pointer that makes a confirmed/needs-human verdict re-adjudicable by a
	// human cheaply. Empty is only legitimate for false-positive /
	// could-not-verify.
	EvidencePointer string `json:"evidence_pointer"`
	// Rationale is the agent's prose reasoning. It is shown to the human but is
	// NEVER accepted in place of an evidence pointer.
	Rationale string `json:"rationale"`
}

// AgentVerifier is the pluggable verification adapter (spec §3, profile-B):
// given a suspect and its assembled context pack, return a verdict. A non-nil
// error means the verdict could not be produced (a live agent CLI failed, a
// script had no entry) — the emitter records that as could-not-verify, never a
// silent confirm or drop. The reference implementation is Fixture; a live
// headless-agent adapter is configuration, added without touching this
// interface.
type AgentVerifier interface {
	Verify(s Suspect, pack ContextPack) (Verdict, error)
}

// MatchKey is the stable, human-readable key the scripted Fixture adapter keys
// verdicts on: category + file. It is intentionally independent of the opaque
// Fingerprint hash so a testdata verdict script stays legible and does not have
// to embed hashes. The planted fixtures carry at most one suspect per
// (category, file), so this key is unambiguous there.
func MatchKey(s Suspect) string {
	return s.Category + "|" + s.File
}
