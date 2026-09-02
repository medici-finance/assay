package attribution

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// dossier.go — DETERMINISTIC, content-addressed dossier assembly (task item 2).
// For one brief-07 trace it gathers (a) the inducing brief/plan text as it stood at
// inducing-merge time, (b) the inducing diff, (c) the at-head review verdicts, and
// (d) any postdating rulings, into a stable, ORDERED, content-addressed dossier:
// the same inputs yield a byte-identical dossier and the same content hash.
//
// The dossier is the DEFENSIBLE layer of M3 (spec §6, §10): it is assembled
// INDEPENDENTLY of the stage call and is re-derivable and spot-auditable, so a
// wrong call is catchable against a fixed dossier rather than trusted blind. That
// independence is the single control this design depends on
// (brief single-point-of-failure line), which is why assembly takes no stage
// signal and the classifier (stage.go) is a pure function OF the dossier.

// Trace mirrors the FROZEN `defects.jsonl` JSON contract brief-07 (quality/07)
// writes — the subset M3 consumes. It is decoded from the on-disk artifact, never
// imported from the mining package, so attribution stays a decoupled reader. Field
// tags match brief-07's DefectTrace exactly; unlisted fields are ignored on decode.
type Trace struct {
	FixCommit       string   `json:"fix_commit"`
	FixPR           int      `json:"fix_pr,omitempty"`
	InducingCommits []string `json:"inducing_commits"`
	InducingPRs     []string `json:"inducing_prs"`
	EvidenceTier    int      `json:"evidence_tier,omitempty"`
	TraceState      string   `json:"trace_state"`
}

// traceStateCouldNotTrace / traceStateTracedNone mirror brief-07's TraceState
// string values (the on-disk contract). A could-not-trace fix has no reachable
// inducing change, so M3 attributes it `untraceable` up front (spec §3.2).
const (
	traceStateTraced        = "traced"
	traceStateTracedNone    = "traced-none"
	traceStateCouldNotTrace = "could-not-trace"
)

// Traceable reports whether the trace resolved to at least one inducing change to
// attribute. A traced-none (measured zero inducers) and a could-not-trace both have
// nothing to walk back — both attribute `untraceable`, never a stage guess.
func (t Trace) Traceable() bool {
	return t.TraceState == traceStateTraced && len(t.InducingCommits) > 0
}

// DefectID is the stable identifier for the defect this trace anchors: its fix PR
// when known, else its fix commit. Used as the ledger filename stem and the dossier
// key.
func (t Trace) DefectID() string {
	if t.FixPR > 0 {
		return fmt.Sprintf("pr-%d", t.FixPR)
	}
	return "commit-" + t.FixCommit
}

// LoadTraces reads a brief-07 `defects.jsonl` file (one JSON Trace per line) into a
// slice, preserving file order. A blank line is skipped; a malformed line is an
// error naming the line number — a partial read is never silently truncated.
func LoadTraces(path string) ([]Trace, error) {
	f, err := os.Open(path) //nolint:gosec // operator-supplied tracking-root path
	if err != nil {
		return nil, fmt.Errorf("open defects.jsonl: %w", err)
	}
	defer f.Close()
	return decodeTraces(f)
}

func decodeTraces(r io.Reader) ([]Trace, error) {
	var out []Trace
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		b := strings.TrimSpace(sc.Text())
		if b == "" {
			continue
		}
		var t Trace
		if err := json.Unmarshal([]byte(b), &t); err != nil {
			return nil, fmt.Errorf("defects.jsonl line %d: %w", line, err)
		}
		out = append(out, t)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan defects.jsonl: %w", err)
	}
	return out, nil
}

// Signal is a three-state judgment input (spec §3.2) for a dossier fact that cannot
// be COMPUTED from surfaces and must be RECORDED: "did the brief faithfully reflect
// the spec as it stood." It is never rounded — an Unknown drives the classifier to
// the conservative stage, never to a spec-stage claim it has no positive evidence
// for.
type Signal string

const (
	SignalTrue    Signal = "true"
	SignalFalse   Signal = "false"
	SignalUnknown Signal = "unknown"
)

// BriefSnapshot is the inducing brief/plan text as of inducing-merge time (dossier
// input (a)). Coverage is the set of defect-surface tokens the brief DECLARED it
// covers (files, packages, or named surfaces) — the deterministic classifier
// compares the defect surface against it to decide plan-gap vs plan-violation.
type BriefSnapshot struct {
	Path         string   `json:"path,omitempty"`
	AtMergeSHA   string   `json:"at_merge_sha,omitempty"`
	Content      string   `json:"content,omitempty"`
	Coverage     []string `json:"coverage"`
	Present      bool     `json:"present"`
	ReflectsSpec Signal   `json:"reflects_spec"`
}

// InducingDiff is the inducing change's diff (dossier input (b)). Surface is the set
// of surface tokens the change actually touched; Patch is the raw unified diff (kept
// for spot-audit, hashed into the dossier).
type InducingDiff struct {
	Files   []string `json:"files"`
	Surface []string `json:"surface"`
	Patch   string   `json:"patch,omitempty"`
}

// ReviewVerdict is one at-head review verdict (dossier input (c)) — a lane and its
// verdict on the inducing PR at the head it was merged at. Lane is generic
// ("correctness", "security", "style", …), never an individual; Approved feeds the
// review-escape overlay.
type ReviewVerdict struct {
	Lane     string `json:"lane"`
	Verdict  string `json:"verdict"`
	Approved bool   `json:"approved"`
	Ref      string `json:"ref,omitempty"`
}

// Ruling is a postdating ruling (dossier input (d)) that bears on the inducing
// change but was recorded AFTER it merged — carried in the dossier for the
// spot-audit trail, ordered by ref for determinism.
type Ruling struct {
	Ref  string `json:"ref"`
	Date string `json:"date,omitempty"`
	Text string `json:"text,omitempty"`
}

// DossierInput is the complete raw material for one defect's dossier. Every field
// is supplied by the caller (its adapters resolve the brief snapshot, diff,
// verdicts and rulings); assembly is a pure, deterministic function of this input.
type DossierInput struct {
	Trace         Trace
	Chain         Chain
	Brief         BriefSnapshot
	InducingDiff  InducingDiff
	Reviews       []ReviewVerdict
	Rulings       []Ruling
	DefectSurface []string
}

// Dossier is the assembled, content-addressed evidence for one defect. Every slice
// is stored in a canonical (sorted, de-duplicated) order, so two assemblies of the
// same input are byte-identical and share Hash. Hash is the sha256 hex of the
// canonical JSON of everything ABOVE it — the content address a spot-audit and the
// stage call both pin to.
type Dossier struct {
	DefectID      string          `json:"defect_id"`
	Trace         Trace           `json:"trace"`
	Chain         Chain           `json:"chain"`
	Brief         BriefSnapshot   `json:"brief"`
	InducingDiff  InducingDiff    `json:"inducing_diff"`
	Reviews       []ReviewVerdict `json:"reviews"`
	Rulings       []Ruling        `json:"rulings"`
	DefectSurface []string        `json:"defect_surface"`
	Hash          string          `json:"hash"`
}

// AssembleDossier builds the deterministic dossier for one defect. It sorts and
// de-duplicates every collection into a canonical order, then content-addresses the
// result: identical inputs -> byte-identical dossier and identical Hash. It reads no
// clock, no filesystem, and no map iteration order, so determinism holds across runs
// and machines (Verify #3).
func AssembleDossier(in DossierInput) Dossier {
	d := Dossier{
		DefectID:      in.Trace.DefectID(),
		Trace:         canonicalTrace(in.Trace),
		Chain:         in.Chain,
		Brief:         canonicalBrief(in.Brief),
		InducingDiff:  canonicalDiff(in.InducingDiff),
		Reviews:       canonicalReviews(in.Reviews),
		Rulings:       canonicalRulings(in.Rulings),
		DefectSurface: sortedUnique(in.DefectSurface),
	}
	d.Hash = contentHash(d)
	return d
}

// contentHash is the sha256 hex over the canonical JSON of the dossier with its Hash
// field cleared — the content address. json.Marshal emits struct fields in
// declaration order and every slice is already canonically ordered, so the byte
// stream is stable.
func contentHash(d Dossier) string {
	d.Hash = ""
	b, err := json.Marshal(d)
	if err != nil {
		// The dossier is composed only of strings, ints and string slices; Marshal
		// cannot fail. A panic here would signal a type change that broke the
		// contract, which is exactly what a test should catch.
		panic(fmt.Sprintf("attribution: dossier marshal: %v", err))
	}
	return sha256Hex(b)
}

// sha256Hex is the shared content-address helper: lowercase hex of the sha256 of b.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum)
}

func canonicalTrace(t Trace) Trace {
	t.InducingCommits = sortedUnique(t.InducingCommits)
	t.InducingPRs = sortedUnique(t.InducingPRs)
	return t
}

func canonicalBrief(b BriefSnapshot) BriefSnapshot {
	b.Coverage = sortedUnique(b.Coverage)
	if b.ReflectsSpec == "" {
		b.ReflectsSpec = SignalUnknown
	}
	return b
}

func canonicalDiff(d InducingDiff) InducingDiff {
	d.Files = sortedUnique(d.Files)
	d.Surface = sortedUnique(d.Surface)
	return d
}

func canonicalReviews(rs []ReviewVerdict) []ReviewVerdict {
	out := append([]ReviewVerdict(nil), rs...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Lane != out[j].Lane {
			return out[i].Lane < out[j].Lane
		}
		if out[i].Verdict != out[j].Verdict {
			return out[i].Verdict < out[j].Verdict
		}
		return out[i].Ref < out[j].Ref
	})
	return out
}

func canonicalRulings(rs []Ruling) []Ruling {
	out := append([]Ruling(nil), rs...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Ref != out[j].Ref {
			return out[i].Ref < out[j].Ref
		}
		return out[i].Date < out[j].Date
	})
	return out
}

// sortedUnique returns a new sorted, de-duplicated slice; a nil/empty input yields
// an empty non-nil slice so the marshalled form is stable (`[]`, never `null`).
func sortedUnique(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
