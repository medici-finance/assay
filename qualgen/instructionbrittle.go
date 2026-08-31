package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// instructionbrittle.go is the instruction-layer brittleness pass (spec §4.6):
// over a CONFIGURED set of instruction-bearing docs it (a) extracts and
// resolves references (file paths, code symbols, typed IDs) and scores
// reference-validity, TRENDED over history, and (b) computes doc↔code
// co-change staleness by applying the §4.5 coupling analysis in the
// doc→code direction. It reads the miner skeleton's Store/Commit/FileDiff
// tables (quality/01) and resolves individual references through the shared
// driftdetect capability rather than a second copy of that logic.

// defaultTypedIDPattern matches a typed ID token such as a ticket/brief ID
// (e.g. TASK-1234). Configurable per fact 1 (a per-target glob/pattern set).
const defaultTypedIDPattern = `^[A-Z][A-Z0-9]{1,9}-\d+$`

// defaultWindowCount is how many chronological windows trended
// reference-validity partitions history into when the caller does not
// configure one.
const defaultWindowCount = 4

// defaultStaleCoChangeThreshold is the minimum number of doc-absent code
// changes, after a coupling is established, before a pair is flagged
// presumptively stale.
const defaultStaleCoChangeThreshold = 2

// InstructionBrittleConfig configures the pass. The instruction-doc set is
// CONFIG, never hardcoded (fact 1): an empty/unconfigured Globs set yields
// could-not-measure end-to-end, never a silent zero.
type InstructionBrittleConfig struct {
	// Globs selects instruction-bearing paths (relative to the repo root) at
	// the commit being evaluated — config/instruction files, briefs, specs,
	// skills. Supports "*" (any run of non-slash characters) and "**" (any
	// depth, including zero).
	Globs []string
	// TypedIDPattern is the configured regexp recognizing a typed ID token.
	// Defaults to defaultTypedIDPattern when empty.
	TypedIDPattern string
	// WindowCount partitions the mined commit history into this many
	// chronological windows for the trended reference-validity curve (fact 3).
	// Defaults to defaultWindowCount when <= 0; clamped to the number of
	// mined commits when larger.
	WindowCount int
	// StaleCoChangeThreshold is the minimum number of code-only changes,
	// after a coupling is established, before doc↔code staleness is flagged.
	// Defaults to defaultStaleCoChangeThreshold when <= 0.
	StaleCoChangeThreshold int
}

// ResolvedReference is one reference extracted from an instruction doc,
// classified and (when classifiable) resolved via driftdetect.
type ResolvedReference struct {
	DocPath string
	Token   string
	Target  Target // zero value when the token could not be classified
	// Validity is the three-state instrument value (quality/01's Measure[T]):
	// Measured(true) = live, Measured(false) = dead, CouldNotMeasure = the
	// token could not be classified resolvable/unresolvable at all (fact 2,
	// last sentence) — a state driftdetect is never even consulted for.
	Validity Measure[bool]
	Reason   string
}

// WindowResult is one point on the trended reference-validity curve: the
// aggregate live/dead/could-not-measure counts across every configured doc,
// resolved at one history window's representative (newest) commit.
type WindowResult struct {
	Index  int
	AtSHA  string
	AtWhen time.Time

	Live            int
	Dead            int
	CouldNotMeasure int

	// Refs carries the full per-reference evidence for this window, so a
	// consumer (or a test dereferencing a known-answer fixture) can inspect
	// exactly which token produced which verdict, not just the tallies.
	Refs []ResolvedReference
}

// CoChangePair is one (doc, code) pairing whose coupling was established by
// at least one historical commit that changed both together.
type CoChangePair struct {
	DocPath       string
	CodePath      string
	EstablishedAt string // SHA of the first commit that changed both together
}

// StalenessResult is the doc↔code co-change staleness verdict for one pair
// (spec §4.6, applying §4.5 in the doc→code direction).
type StalenessResult struct {
	Pair CoChangePair
	// CodeOnlyChanges counts commits AFTER EstablishedAt that touched
	// CodePath without touching DocPath in the same commit — the doc "left
	// behind" while its described code kept moving. This is the evidence
	// (fact 3: "co-change counts as evidence").
	CodeOnlyChanges int
	Stale           bool
}

// BrittleReport is the pass's whole output: emitted as M1 aggregates through
// the skeleton's aggregation path by the caller (task 4); this type is the
// in-process shape that emission reads from.
type BrittleReport struct {
	// Configured records whether an instruction-doc glob set was supplied at
	// all. Unconfigured is could-not-measure end-to-end (fact 1) — Trend and
	// Staleness are left nil rather than populated with a manufactured zero,
	// so "never looked" is never presented as "looked and found nothing"
	// (Verify row 7).
	Configured Measure[bool]
	Trend      []WindowResult
	Staleness  []StalenessResult
}

// RunInstructionBrittleness runs the whole pass: reference-validity trended
// over history windows, plus doc↔code co-change staleness for every
// instruction doc matched at HEAD. repoPath is the mined repository (opened
// read-only for tree access); store is the miner skeleton's artifact store
// for the SAME repository, already mined (quality/01).
func RunInstructionBrittleness(repoPath string, store *Store, cfg InstructionBrittleConfig) (BrittleReport, error) {
	if len(cfg.Globs) == 0 {
		return BrittleReport{
			Configured: CouldNotMeasure[bool]("no instruction-doc glob set configured (spec §4.6 fact 1: an empty/unconfigured set is could-not-measure, never a silent zero)"),
		}, nil
	}

	// EnableDotGitCommonDir: the miner drives this pass over the repo it just
	// mined, which in the desk's operating model is a linked worktree (a `.git`
	// file pointing at the common dir, per mine.go's own PlainOpenWithOptions).
	// Plain PlainOpen does not follow the worktree's commondir, so r.Head() would
	// fail to resolve the branch ref living in the common dir — the same failure
	// mine.go already guards against.
	r, err := git.PlainOpenWithOptions(repoPath, &git.PlainOpenOptions{EnableDotGitCommonDir: true})
	if err != nil {
		return BrittleReport{}, fmt.Errorf("qualgen: instructionbrittle: open repo: %w", err)
	}

	commits, err := store.ReadCommits()
	if err != nil {
		return BrittleReport{}, fmt.Errorf("qualgen: instructionbrittle: read commits: %w", err)
	}
	diffs, err := store.ReadDiffs()
	if err != nil {
		return BrittleReport{}, fmt.Errorf("qualgen: instructionbrittle: read diffs: %w", err)
	}
	filesByCommit := groupFilesByCommit(diffs)

	typedIDPattern := cfg.TypedIDPattern
	if typedIDPattern == "" {
		typedIDPattern = defaultTypedIDPattern
	}
	typedIDRe, err := regexp.Compile(typedIDPattern)
	if err != nil {
		return BrittleReport{}, fmt.Errorf("qualgen: instructionbrittle: invalid typed-ID pattern %q: %w", typedIDPattern, err)
	}

	trend, err := trendedReferenceValidity(r, commits, cfg.Globs, cfg.WindowCount, typedIDRe)
	if err != nil {
		return BrittleReport{}, err
	}

	staleness, err := docCodeStalenessAtHead(r, commits, filesByCommit, cfg.Globs, cfg.StaleCoChangeThreshold)
	if err != nil {
		return BrittleReport{}, err
	}

	return BrittleReport{Configured: Measured(true), Trend: trend, Staleness: staleness}, nil
}

// ---------------------------------------------------------------------------
// M1 emission — the instruction-brittleness family on metrics.jsonl
// ---------------------------------------------------------------------------
//
// brief-04 task 4 asks this pass to emit BOTH signals "as M1 aggregates through
// the skeleton's aggregation path". The pass computes them in-process
// (BrittleReport); this is the append-only emission the miner drives so the
// numbers land in metrics.jsonl beside the hotspot / ownership / coupling /
// taxonomy families and the quality/05 report view can render them instead of a
// placeholder. Each `mine` run appends a fresh full snapshot (extend, never
// rewrite), so a trend consumer reads the most recent snapshot per family.

// Metric-name discriminators for the instruction-brittleness family — the
// on-disk contract the report reader keys on (mirrors "hotspot" / "ownership").
const (
	MetricReferenceValidity = "reference_validity"
	MetricDocCodeStaleness  = "doc_code_staleness"
)

// ReferenceValidityRecord is one history-window point of the trended
// instruction reference-validity curve (spec §4.6), appended to metrics.jsonl.
// A decaying Validity across windows is the context-rot signal. The unconfigured
// / could-not-measure end-to-end case emits ONE marker record with an empty
// AtSHA and a could-not-measure Validity carrying the reason (three-state:
// never a silent zero, fact 1) — the report renders it as unmeasured.
type ReferenceValidityRecord struct {
	Metric      string    `json:"metric"` // "reference_validity"
	WindowIndex int       `json:"window_index"`
	AtSHA       string    `json:"at_sha,omitempty"`
	AtWhen      time.Time `json:"at_when,omitempty"`

	Live            int `json:"live"`
	Dead            int `json:"dead"`
	CouldNotMeasure int `json:"could_not_measure"`

	// Validity is the three-state reference-validity ratio live/(live+dead) over
	// the CLASSIFIABLE references in this window: measured when there were
	// classifiable refs, measured-zero when some were classifiable but none live,
	// could-not-measure when the window had no classifiable reference at all (or
	// the instruction-doc set was unconfigured).
	Validity Measure[float64] `json:"validity"`
	MinedAt  time.Time        `json:"mined_at"`
}

// DocCodeStalenessRecord is one doc↔code co-change staleness pair (spec §4.6,
// §4.5 applied doc→code), appended to metrics.jsonl. Stale is set when the coupled
// code moved CodeOnlyChanges times without the doc following it, above the
// configured threshold.
type DocCodeStalenessRecord struct {
	Metric          string    `json:"metric"` // "doc_code_staleness"
	DocPath         string    `json:"doc_path"`
	CodePath        string    `json:"code_path"`
	EstablishedAt   string    `json:"established_at"`
	CodeOnlyChanges int       `json:"code_only_changes"`
	Stale           bool      `json:"stale"`
	MinedAt         time.Time `json:"mined_at"`
}

// referenceValidityRatio scores one window's live/(live+dead) over classifiable
// references, three-state: could-not-measure when the window had no classifiable
// reference to score from (only could-not-classify tokens), measured-zero when
// some were classifiable but none live, measured otherwise — never a silent zero.
func referenceValidityRatio(w WindowResult) Measure[float64] {
	denom := w.Live + w.Dead
	if denom == 0 {
		return CouldNotMeasure[float64]("no classifiable reference in this window to score reference-validity from")
	}
	if w.Live == 0 {
		return MeasuredZero[float64]()
	}
	return Measured(float64(w.Live) / float64(denom))
}

// appendInstructionBrittleness runs the instruction-brittleness pass under cfg
// over the freshly-mined repoPath and appends its reference-validity trend and
// doc↔code staleness records to the metrics table as one fresh snapshot. An
// unconfigured instruction-doc set emits a single could-not-measure
// reference-validity marker (fact 1: could-not-measure, never a silent zero).
// READ-ONLY against the target repo; the only writes land under the tracking root.
func appendInstructionBrittleness(store *Store, repoPath string, cfg InstructionBrittleConfig, minedAt time.Time) error {
	rep, err := RunInstructionBrittleness(repoPath, store, cfg)
	if err != nil {
		return err
	}
	if rep.Configured.State != StateMeasured || !rep.Configured.Value {
		// Unconfigured / could-not-measure end-to-end: one marker record so the
		// report renders "not measured (<reason>)", never a fabricated zero.
		reason := rep.Configured.Reason
		if reason == "" {
			reason = "instruction-brittleness pass reported no measured result"
		}
		return store.Append(KindMetric, ReferenceValidityRecord{
			Metric:   MetricReferenceValidity,
			Validity: CouldNotMeasure[float64](reason),
			MinedAt:  minedAt,
		})
	}
	for _, w := range rep.Trend {
		if err := store.Append(KindMetric, ReferenceValidityRecord{
			Metric:          MetricReferenceValidity,
			WindowIndex:     w.Index,
			AtSHA:           w.AtSHA,
			AtWhen:          w.AtWhen,
			Live:            w.Live,
			Dead:            w.Dead,
			CouldNotMeasure: w.CouldNotMeasure,
			Validity:        referenceValidityRatio(w),
			MinedAt:         minedAt,
		}); err != nil {
			return fmt.Errorf("append reference-validity metric: %w", err)
		}
	}
	for _, s := range rep.Staleness {
		if err := store.Append(KindMetric, DocCodeStalenessRecord{
			Metric:          MetricDocCodeStaleness,
			DocPath:         s.Pair.DocPath,
			CodePath:        s.Pair.CodePath,
			EstablishedAt:   s.Pair.EstablishedAt,
			CodeOnlyChanges: s.CodeOnlyChanges,
			Stale:           s.Stale,
			MinedAt:         minedAt,
		}); err != nil {
			return fmt.Errorf("append doc-code-staleness metric: %w", err)
		}
	}
	return nil
}

// trendedReferenceValidity partitions commits into chronological windows and
// resolves every configured doc's references at each window's newest commit
// (task 2: "computed per history window so the output is a TREND, not a
// snapshot").
func trendedReferenceValidity(r *git.Repository, commits []Commit, globs []string, windowCount int, typedIDRe *regexp.Regexp) ([]WindowResult, error) {
	windows := windowizeCommits(commits, windowCount)
	out := make([]WindowResult, 0, len(windows))
	for i, w := range windows {
		last := w[len(w)-1]
		wr := WindowResult{Index: i, AtSHA: last.SHA, AtWhen: last.CommitterWhen}

		commitObj, err := r.CommitObject(plumbing.NewHash(last.SHA))
		if err != nil {
			return nil, fmt.Errorf("qualgen: instructionbrittle: resolve commit %s: %w", last.SHA, err)
		}
		tree, err := commitObj.Tree()
		if err != nil {
			return nil, fmt.Errorf("qualgen: instructionbrittle: resolve tree at %s: %w", last.SHA, err)
		}
		docs, err := listInstructionDocs(tree, globs)
		if err != nil {
			return nil, err
		}
		for _, doc := range docs {
			refs, err := ResolveDocReferences(commitObj, doc, typedIDRe)
			if err != nil {
				return nil, err
			}
			for _, ref := range refs {
				switch ref.Validity.State {
				case StateMeasured:
					if ref.Validity.Value {
						wr.Live++
					} else {
						wr.Dead++
					}
				case StateCouldNotMeasure:
					wr.CouldNotMeasure++
				}
			}
			wr.Refs = append(wr.Refs, refs...)
		}
		out = append(out, wr)
	}
	return out, nil
}

// docCodeStalenessAtHead computes co-change staleness for every instruction
// doc matched at HEAD against the whole mined commit history.
func docCodeStalenessAtHead(r *git.Repository, commits []Commit, filesByCommit map[string][]string, globs []string, threshold int) ([]StalenessResult, error) {
	if threshold <= 0 {
		threshold = defaultStaleCoChangeThreshold
	}
	head, err := r.Head()
	if err != nil {
		return nil, fmt.Errorf("qualgen: instructionbrittle: resolve HEAD: %w", err)
	}
	headCommit, err := r.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("qualgen: instructionbrittle: resolve HEAD commit: %w", err)
	}
	headTree, err := headCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("qualgen: instructionbrittle: resolve HEAD tree: %w", err)
	}
	docs, err := listInstructionDocs(headTree, globs)
	if err != nil {
		return nil, err
	}
	var out []StalenessResult
	for _, doc := range docs {
		out = append(out, coChangeStaleness(commits, filesByCommit, doc, threshold)...)
	}
	return out, nil
}

// coChangeStaleness applies §4.5 change coupling in the doc→code direction
// (task 3) for one doc: every other file that has EVER changed in the same
// commit as the doc establishes a coupling pair (EstablishedAt = the first
// such commit); staleness is how many commits after that touched the code
// file WITHOUT touching the doc.
func coChangeStaleness(commits []Commit, filesByCommit map[string][]string, docPath string, threshold int) []StalenessResult {
	established := map[string]string{}
	codeOnly := map[string]int{}
	for _, c := range commits {
		files := filesByCommit[c.SHA]
		if len(files) == 0 {
			continue
		}
		touchesDoc := false
		for _, f := range files {
			if f == docPath {
				touchesDoc = true
				break
			}
		}
		for _, f := range files {
			if f == docPath {
				continue
			}
			if touchesDoc {
				if _, ok := established[f]; !ok {
					established[f] = c.SHA
				}
			} else if _, ok := established[f]; ok {
				codeOnly[f]++
			}
		}
	}
	codePaths := make([]string, 0, len(established))
	for p := range established {
		codePaths = append(codePaths, p)
	}
	sort.Strings(codePaths)

	out := make([]StalenessResult, 0, len(codePaths))
	for _, codePath := range codePaths {
		n := codeOnly[codePath]
		out = append(out, StalenessResult{
			Pair: CoChangePair{
				DocPath:       docPath,
				CodePath:      codePath,
				EstablishedAt: established[codePath],
			},
			CodeOnlyChanges: n,
			Stale:           n >= threshold,
		})
	}
	return out
}

// groupFilesByCommit collapses the diff table into commit SHA -> the distinct
// set of paths touched by that commit (NewPath, falling back to OldPath for a
// delete), in first-seen order.
func groupFilesByCommit(diffs []FileDiff) map[string][]string {
	out := map[string][]string{}
	seen := map[string]map[string]bool{}
	for _, fd := range diffs {
		path := fd.NewPath
		if path == "" {
			path = fd.OldPath
		}
		if path == "" {
			continue
		}
		if seen[fd.CommitSHA] == nil {
			seen[fd.CommitSHA] = map[string]bool{}
		}
		if seen[fd.CommitSHA][path] {
			continue
		}
		seen[fd.CommitSHA][path] = true
		out[fd.CommitSHA] = append(out[fd.CommitSHA], path)
	}
	return out
}

// windowizeCommits partitions a chronologically-ordered (oldest-first, the
// Store's write order) commit slice into windowCount contiguous, roughly
// equal windows — earlier windows absorb any remainder so the trend still
// reads oldest-to-newest. Returns nil for an empty history; clamps
// windowCount down to len(commits) so every window is non-empty.
func windowizeCommits(commits []Commit, windowCount int) [][]Commit {
	if len(commits) == 0 {
		return nil
	}
	if windowCount <= 0 {
		windowCount = defaultWindowCount
	}
	if windowCount > len(commits) {
		windowCount = len(commits)
	}
	base := len(commits) / windowCount
	rem := len(commits) % windowCount
	windows := make([][]Commit, 0, windowCount)
	idx := 0
	for w := 0; w < windowCount; w++ {
		size := base
		if w < rem {
			size++
		}
		if size == 0 {
			continue
		}
		windows = append(windows, commits[idx:idx+size])
		idx += size
	}
	return windows
}

// listInstructionDocs returns every path in tree matching any of globs, the
// configured instruction-doc set (fact 1).
func listInstructionDocs(tree *object.Tree, globs []string) ([]string, error) {
	var out []string
	iter := tree.Files()
	defer iter.Close()
	err := iter.ForEach(func(f *object.File) error {
		for _, g := range globs {
			if matchGlob(g, f.Name) {
				out = append(out, f.Name)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("qualgen: instructionbrittle: list instruction docs: %w", err)
	}
	sort.Strings(out)
	return out, nil
}

// matchGlob matches a path against a glob pattern supporting "*" (any run of
// non-slash characters) and "**" (any depth, including zero). This is
// deliberately a small hand-rolled matcher rather than filepath.Match, which
// treats "/" as a literal in a way that makes "**" unusable for the
// recursive doc-tree globs the instruction-doc config needs
// (e.g. "docs/streams/**/*.md").
func matchGlob(pattern, path string) bool {
	re := globToRegexp(pattern)
	return re.MatchString(path)
}

var globCache = map[string]*regexp.Regexp{}

func globToRegexp(pattern string) *regexp.Regexp {
	if re, ok := globCache[pattern]; ok {
		return re
	}
	var b strings.Builder
	b.WriteString("^")
	i := 0
	for i < len(pattern) {
		switch {
		case strings.HasPrefix(pattern[i:], "**"):
			b.WriteString(".*")
			i += 2
		case pattern[i] == '*':
			b.WriteString("[^/]*")
			i++
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
			i++
		}
	}
	b.WriteString("$")
	re := regexp.MustCompile(b.String())
	globCache[pattern] = re
	return re
}

// --- reference extraction and classification ---

// backtickSpan finds inline code spans in a markdown-shaped instruction doc —
// the reference-bearing convention this repo's own instruction docs already
// use (CLAUDE.md, briefs, skills all mark paths/symbols/IDs in backticks).
var backtickSpan = regexp.MustCompile("`([^`\n]+)`")

// rawReference is an unclassified token extracted from a doc, before
// classification decides whether it is even a resolvable reference kind.
type rawReference struct {
	DocPath string
	Token   string
}

// extractReferences pulls every backtick-span token out of a doc's content.
func extractReferences(docPath, content string) []rawReference {
	var out []rawReference
	for _, m := range backtickSpan.FindAllStringSubmatch(content, -1) {
		out = append(out, rawReference{DocPath: docPath, Token: m[1]})
	}
	return out
}

// classifyReference decides whether a token is a file path, a symbol, or a
// typed ID, or cannot be classified at all (fact 2, last sentence). A token
// containing whitespace is never a single reference token — it is the
// could-not-classify case, reported distinct from measured-dead.
func classifyReference(token string, typedIDRe *regexp.Regexp) (Target, bool) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" || strings.ContainsAny(trimmed, " \t") {
		return Target{}, false
	}
	if typedIDRe.MatchString(trimmed) {
		return Target{Kind: TargetTypedID, Value: trimmed}, true
	}
	if looksLikeFilePath(trimmed) {
		return Target{Kind: TargetFilePath, Value: trimmed}, true
	}
	if looksLikeSymbol(trimmed) {
		return Target{Kind: TargetSymbol, Value: trimmed}, true
	}
	return Target{}, false
}

// looksLikeFilePath reports whether a token is shaped like a file path: it
// carries a path separator, or a recognizable file extension.
func looksLikeFilePath(s string) bool {
	if strings.Contains(s, "/") {
		return true
	}
	dot := strings.LastIndex(s, ".")
	if dot <= 0 || dot == len(s)-1 {
		return false
	}
	ext := s[dot+1:]
	for _, r := range ext {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return !strings.ContainsAny(s, "()")
}

// looksLikeSymbol reports whether a token is shaped like a code symbol: an
// identifier (optionally dotted, package.Symbol-style), optionally suffixed
// with "()".
func looksLikeSymbol(s string) bool {
	name := strings.TrimSuffix(s, "()")
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case i == 0:
			if !(unicode.IsLetter(r) || r == '_') {
				return false
			}
		default:
			if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.') {
				return false
			}
		}
	}
	return true
}

// ResolveDocReferences extracts every reference from the doc at docPath's
// content in the given commit's tree, classifies each, and resolves the
// classifiable ones via driftdetect (tasks 1/2). A token this pass cannot
// classify resolvable/unresolvable never reaches driftdetect at all — it is
// reported could-not-measure directly (fact 2, last sentence), distinct from
// a measured-dead reference.
func ResolveDocReferences(commit *object.Commit, docPath string, typedIDRe *regexp.Regexp) ([]ResolvedReference, error) {
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("qualgen: instructionbrittle: resolve tree at %s: %w", commit.Hash, err)
	}
	f, err := tree.File(docPath)
	if err != nil {
		return nil, fmt.Errorf("qualgen: instructionbrittle: doc %q not found at %s: %w", docPath, commit.Hash, err)
	}
	content, err := f.Contents()
	if err != nil {
		return nil, fmt.Errorf("qualgen: instructionbrittle: read doc %q at %s: %w", docPath, commit.Hash, err)
	}

	var out []ResolvedReference
	for _, raw := range extractReferences(docPath, content) {
		target, ok := classifyReference(raw.Token, typedIDRe)
		if !ok {
			out = append(out, ResolvedReference{
				DocPath: docPath,
				Token:   raw.Token,
				Validity: CouldNotMeasure[bool](fmt.Sprintf(
					"reference %q in %s could not be classified as a file path, symbol, or typed ID", raw.Token, docPath)),
			})
			continue
		}
		ctx := Context{Tree: tree, CommitSHA: commit.Hash.String(), SourcePath: docPath}
		res := driftdetect(ctx, target)
		out = append(out, ResolvedReference{
			DocPath:  docPath,
			Token:    raw.Token,
			Target:   target,
			Validity: verdictToValidity(res),
			Reason:   res.Reason,
		})
	}
	return out, nil
}

// verdictToValidity translates a driftdetect Verdict into the frozen
// Measure[bool] three-state (quality/01): in-sync -> Measured(true) (live),
// drifted -> Measured(false) (dead — a real measurement, not a
// measured-zero), could-not-check -> CouldNotMeasure.
func verdictToValidity(res Result) Measure[bool] {
	switch res.Verdict {
	case VerdictInSync:
		return Measured(true)
	case VerdictDrifted:
		return Measured(false)
	default:
		reason := res.Reason
		if reason == "" {
			reason = fmt.Sprintf("driftdetect could not resolve %q", res.Target.Value)
		}
		return CouldNotMeasure[bool](reason)
	}
}
