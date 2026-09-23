package main

// auditpack.go — `statusgen --export-audit-pack --release <tag>` (sdlc/08): a
// RELEASE-keyed sibling of --export-evidence's DATE-keyed bundler.
//
// evidenceexport.go's collector keys on frontmatter/register DATES. A release
// asks a different question — "show me everything behind release Y" — which
// means walking a different graph: release -> the PRs/briefs in it -> the
// requirements those briefs satisfy -> the Evidence and review verdicts behind
// each one. sdlc/01 (the REQUIREMENTS register, requirements.go) and sdlc/02
// (the traceability checks and the per-requirement rollup, traceability.go /
// rollup.go) already make the brief<->requirement half of that graph walkable;
// this file adds the missing release<->brief edge and assembles the pack.
//
// # Release resolution
//
// A release tag resolves against docs/release-notes/<tag>.md — the EXISTING
// release-notes convention, not a new parallel register. That file already IS
// where a release's story lives; this brief extends it with an OPTIONAL
// machine-readable frontmatter block (`briefs:` / `requirements:`) rather than
// forking a second "releases" directory next to intake/findings/requirements.
// Three states, deliberately:
//
//   - no docs/release-notes/<tag>.md at all: the release is NOT FOUND
//     (errReleaseNotFound) — a hard refusal, not an empty pack.
//   - the file exists with no frontmatter (every release note written before
//     this brief): a legitimate "release found, empty declared scope" state.
//     The note IS the release; it simply predates machine-readable scoping.
//   - the file exists WITH frontmatter that fails to parse: a real error
//     (could-not-check), never silently rounded to an empty scope.
//
// # The single point of failure, and its second layer
//
// The brief's own Context names it: the `omitted` list is the ONE thing
// standing between an incomplete pack and an auditor who believes they have
// the whole chain — and a bundler can have a bug that drops something WITHOUT
// ever populating `omitted`, in which case that layer alone is silent about
// its own failure. The second, INDEPENDENT layer here is
// auditPackCoverageAgreement: this file's own requirement->brief walk
// (collectAuditPackRequirements, a fresh loadStreams/parseRequirementsDir/
// scanTracedBriefs call) is compared against a FRESH, unmodified call into
// rollup.go's buildRollupReport — sdlc/02's existing rollup, walking the same
// tree via ITS OWN separate calls into the same three functions. Two
// components, two derivations (two independent reads off disk), one
// comparison: a bug that drops a backing brief from ONLY this file's
// collection step moves this file's tally without moving rollup's, and the
// pack REFUSES to write rather than ship the disagreement quietly.
//
// # Manifest shape
//
// manifest.json is NOT forked. The bundle is written by the SAME
// writeEvidenceBundle (evidenceexport.go) that --export-evidence uses, so it
// carries the identical {generated, version, files[{path,sha256}], omitted}
// shape. The requirement/brief/PR/review detail this brief adds lives in one
// bundled content file, auditPackReportPath, whose own sha256 is just another
// entry in that same `files` array.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// releaseNotesDirName is where a release's story — and, as of this brief, its
// optional machine-readable scope — lives. Sibling of docs/, NOT of
// docs/streams/: it is not a stream and not a registers-v1 register, so it
// needs no entry in reservedRegisterNames and cannot collide with either.
const releaseNotesDirName = "release-notes"

// auditPackReportPath is the one SYNTHESIZED file this collector adds to the
// bundle — generated at pack-build time, not read off disk. The name is
// deliberately outside every real repo-relative path shape (no repo file is
// ever named exactly this) so it cannot collide with a bundled register/brief
// entry.
const auditPackReportPath = "audit-pack-report.json"

// releaseTagRe bounds --release BEFORE it is joined onto a filesystem path.
// Release tags are not required to be semver in every adopter (hence no
// v-prefix requirement here), but the value is about to become
// docs/release-notes/<tag>.md and must not carry a path separator or escape
// that directory.
var releaseTagRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// errReleaseNotFound is loadReleaseRecord's sentinel for "no
// docs/release-notes/<tag>.md exists for this tag" — the DEREFERENCING case
// (Verify row 7): a well-formed but nonexistent release must refuse, naming
// itself as not found, never emit an empty-but-well-formed pack.
var errReleaseNotFound = errors.New("release not found")

// releaseRecord is the optional machine-readable frontmatter a release note
// carries (this brief's one addition to that file shape).
type releaseRecord struct {
	Release      string   `yaml:"release"`
	Briefs       []string `yaml:"briefs"`
	Requirements []string `yaml:"requirements"`
}

// loadReleaseRecord reads docs/release-notes/<tag>.md and parses its optional
// frontmatter block. See the file-level doc comment for the three states.
func loadReleaseRecord(root, tag string) (*releaseRecord, error) {
	path := filepath.Join(root, "docs", releaseNotesDirName, tag+".md")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errReleaseNotFound
		}
		return nil, fmt.Errorf("release record %s: %w", path, err)
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	first, _, _ := strings.Cut(content, "\n")
	if strings.TrimSpace(first) != "---" {
		// A legacy, prose-only release note: found, empty declared scope.
		return &releaseRecord{Release: tag}, nil
	}
	fm, _, ferr := splitFrontmatter(content)
	if ferr != nil {
		return nil, fmt.Errorf("release record %s: unparseable frontmatter: %w", path, ferr)
	}
	var rec releaseRecord
	if uerr := yaml.Unmarshal([]byte(fm), &rec); uerr != nil {
		return nil, fmt.Errorf("release record %s: %w", path, uerr)
	}
	if strings.TrimSpace(rec.Release) == "" {
		rec.Release = tag
	}
	return &rec, nil
}

// ---- the pack's own requirement/brief/PR/review records -------------------

// auditPackBacking is one backing brief of a requirement in the pack,
// enriched with the PR/review detail the plain rollup does not carry: the PR
// number and the reviewer identity+date, parsed from the Reviewed cell's
// EXISTING "YYYY-MM-DD <runner> (approved PR #<N> @ <sha>)" shape (the
// derived-board convention this file reads rather than re-derives — see
// parseReviewedCell).
type auditPackBacking struct {
	Brief        string `json:"brief"`
	Status       string `json:"status"`
	Resolved     bool   `json:"resolved"`
	Evidence     string `json:"evidence,omitempty"`
	Verified     string `json:"verified,omitempty"` // the raw Verified cell: the human/model sign-off
	PR           int    `json:"pr,omitempty"`       // 0 = not recorded
	ReviewedBy   string `json:"reviewed-by,omitempty"`
	ReviewedDate string `json:"reviewed-date,omitempty"`
	MergedSHA    string `json:"merged-sha,omitempty"`
}

// auditPackRequirement is one requirement's record in the pack: the
// requirement and its acceptance criteria, the briefs backing it, and a
// three-state verdict. State is never "satisfied" for an empty or unresolved
// backing set (rollupState, reused verbatim — the terminal classification is
// a small pure function; the WALK that feeds it is this file's own, separate
// from rollup.go's).
type auditPackRequirement struct {
	ID         string             `json:"id"`
	Title      string             `json:"title"`
	Impact     string             `json:"impact"`
	Status     string             `json:"status"`
	Date       string             `json:"date"`
	Acceptance []string           `json:"acceptance"`
	Briefs     []auditPackBacking `json:"briefs"`
	State      string             `json:"state"` // satisfied | partial | could-not-check
	Reason     string             `json:"could-not-check-reason,omitempty"`
}

// auditPackCoverage is the independent completeness assertion's own printed
// line (task item 2's "coverage line"): the count of RESOLVED backing links
// this pack's collector found, across every requirement in scope, and the
// same count as sdlc/02's rollup independently computes for the identical
// requirement-id set. Agreement is what let the pack write; Note says so.
type auditPackCoverage struct {
	ResolvedBackingLinks int    `json:"resolved-backing-links"`
	RollupBackingLinks   int    `json:"rollup-backing-links"`
	RequirementsInScope  int    `json:"requirements-in-scope"`
	Note                 string `json:"note"`
}

// auditPackDocument is the whole bundled auditPackReportPath content.
type auditPackDocument struct {
	Release      string                 `json:"release"`
	Note         string                 `json:"note"`
	Requirements []auditPackRequirement `json:"requirements"`
	Coverage     auditPackCoverage      `json:"coverage"`
}

// auditPackNote mirrors rollup.go's own honesty note (rollupAuthoredNote) plus
// the release-scoping caveat: this pack reports what the release record NAMES
// and what the corpus's own board rows say, never a re-measurement.
const auditPackNote = rollupAuthoredNote + "; scope is exactly what the release record's " +
	"`requirements:`/`briefs:` frontmatter names — a release note with no such frontmatter " +
	"is a real release with an EMPTY declared scope, not an error"

// reviewedCellRe parses the derived-board Reviewed-cell convention already in
// use across every stream README this repo writes:
// "YYYY-MM-DD <identity> (approved PR #<N> @ <sha>)" — the PR number and merge
// sha are OPTIONAL trailing detail; a bare dated identity still matches with
// pr="" sha="".
var reviewedCellRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})\s+(.+?)(?:\s+\(approved PR #(\d+)(?:\s+@\s+([0-9a-fA-F]{7,40}))?\))?$`)

// parseReviewedCell extracts the review verdict (identity + date) and, when
// present, the PR number and merge sha, from a Reviewed cell already sitting
// in a stream README board row — this file reads that existing convention
// rather than re-deriving PR/review data from a second source.
func parseReviewedCell(cell string) (date, identity string, pr int, sha string) {
	m := reviewedCellRe.FindStringSubmatch(strings.TrimSpace(cell))
	if m == nil {
		return "", "", 0, ""
	}
	date = m[1]
	identity = strings.TrimSpace(m[2])
	if m[3] != "" {
		if n, err := strconv.Atoi(m[3]); err == nil {
			pr = n
		}
	}
	sha = m[4]
	return
}

// auditBriefRow is the Verified/Reviewed/Status facts this file reads
// straight off a Stream's OWN in-memory Briefs rows (already loaded by THIS
// file's own loadStreams call in collectAuditPackRequirements — never a
// second file read, and never rollup.go's copies).
type auditBriefRow struct {
	Status   string
	Verified string
	Reviewed string
}

// auditBriefRows indexes every stream's board rows by "<stream>/<NN>" key.
func auditBriefRows(streams []*Stream) map[string]auditBriefRow {
	out := map[string]auditBriefRow{}
	for _, s := range streams {
		for i := range s.Briefs {
			key := s.Name + "/" + s.Briefs[i].Num
			out[key] = auditBriefRow{
				Status:   s.Briefs[i].Status,
				Verified: s.Briefs[i].Verified,
				Reviewed: s.Briefs[i].Reviewed,
			}
		}
	}
	return out
}

// auditBriefPaths indexes every parseable brief file's absolute path by
// "<stream>/<NN>" key, walked independently of scanTracedBriefs (which
// returns tracedBrief records, not paths) so the bundler can locate and read
// each backing brief's own file.
func auditBriefPaths(streams []*Stream) map[string]string {
	out := map[string]string{}
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			_, num, ok := expectedBriefID(path)
			if !ok {
				continue
			}
			out[s.Name+"/"+num] = path
		}
	}
	return out
}

// auditStreamReadmes indexes every stream's README path by stream name.
func auditStreamReadmes(streams []*Stream) map[string]string {
	out := map[string]string{}
	for _, s := range streams {
		out[s.Name] = filepath.Join(s.Dir, "README.md")
	}
	return out
}

// collectAuditPackRequirements is this file's OWN independent
// requirement->backing-brief walk: a SEPARATE loadStreams +
// parseRequirementsDir + scanTracedBriefs call from the one
// buildRollupReport makes internally. A defect that only THIS walk's
// collection step introduces is what the completeness comparison
// (auditPackCoverageAgreement) exists to catch.
func collectAuditPackRequirements(root string, rec *releaseRecord) (reqs []auditPackRequirement, paths map[string]string, readmes map[string]string, omitted []omittedEntry, err error) {
	entries, rerr := parseRequirementsDir(root)
	if rerr != nil {
		return nil, nil, nil, nil, fmt.Errorf("requirements register unreadable: %w", rerr)
	}
	byID := map[string]requirementEntry{}
	for _, e := range entries {
		byID[e.ID] = e
	}

	streams, _, lerr := loadStreams(root)
	if lerr != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not load streams: %w", lerr)
	}
	attachPlaceholders(streams)
	briefs, _ := scanTracedBriefs(streams)

	byKey := map[string]tracedBrief{}
	citedBy := map[string][]tracedBrief{}
	for _, b := range briefs {
		byKey[b.Key] = b
		for _, ref := range b.Satisfies {
			if id, cross := requirementRefKind(ref); id != "" && !cross {
				citedBy[id] = append(citedBy[id], b)
			}
		}
	}
	rows := auditBriefRows(streams)
	paths = auditBriefPaths(streams)
	readmes = auditStreamReadmes(streams)

	seenReq := map[string]bool{}
	for _, reqID := range rec.Requirements {
		reqID = strings.TrimSpace(reqID)
		if reqID == "" || seenReq[reqID] {
			continue
		}
		seenReq[reqID] = true
		e, ok := byID[reqID]
		if !ok {
			reason := fmt.Sprintf("release %s names requirement %s, which docs/streams/requirements/ does not define", rec.Release, reqID)
			omitted = append(omitted, omittedEntry{Path: "requirement:" + reqID, Reason: "could-not-check: " + reason})
			reqs = append(reqs, auditPackRequirement{ID: reqID, State: rollupStateCouldNotCheck, Reason: reason})
			continue
		}
		ar := buildAuditPackRequirement(e, citedBy, byKey, rows)
		if ar.State == rollupStateCouldNotCheck {
			omitted = append(omitted, omittedEntry{Path: "requirement:" + ar.ID, Reason: "could-not-check: " + ar.Reason})
		}
		reqs = append(reqs, ar)
	}
	sort.SliceStable(reqs, func(i, j int) bool { return reqs[i].ID < reqs[j].ID })
	sort.Slice(omitted, func(i, j int) bool { return omitted[i].Path < omitted[j].Path })
	return reqs, paths, readmes, omitted, nil
}

// buildAuditPackRequirement assembles one requirement's record, resolving
// both traceability directions exactly as rollup.go's buildRollupRequirement
// does — the SAME rule, applied over THIS file's own separately-collected
// data (byKey/citedBy come from THIS call's scanTracedBriefs, not rollup's).
func buildAuditPackRequirement(e requirementEntry, citedBy map[string][]tracedBrief, byKey map[string]tracedBrief, rows map[string]auditBriefRow) auditPackRequirement {
	seen := map[string]bool{}
	var backing []auditPackBacking
	var unresolvedReason string

	add := func(key, status string, resolved bool, evidence string) {
		if seen[key] {
			return
		}
		seen[key] = true
		b := auditPackBacking{Brief: key, Status: status, Resolved: resolved, Evidence: strings.TrimSpace(evidence)}
		if row, ok := rows[key]; ok {
			b.Verified = strings.TrimSpace(row.Verified)
			if date, identity, pr, sha := parseReviewedCell(row.Reviewed); date != "" {
				b.ReviewedDate = date
				b.ReviewedBy = identity
				b.PR = pr
				b.MergedSHA = sha
			}
		}
		backing = append(backing, b)
	}

	for _, b := range citedBy[e.ID] {
		add(b.Key, b.Status, true, b.Evidence)
	}
	for _, ref := range nonEmptyStrings(e.SatisfiedBy) {
		if b, ok := byKey[ref]; ok {
			add(b.Key, b.Status, true, b.Evidence)
			continue
		}
		if unresolvedReason == "" {
			unresolvedReason = fmt.Sprintf("satisfied-by names %s, which does not resolve to a brief board row", ref)
		}
		add(ref, "", false, "")
	}

	sort.SliceStable(backing, func(i, j int) bool { return backing[i].Brief < backing[j].Brief })

	anyUnresolved := false
	rb := make([]rollupBrief, len(backing))
	for i, b := range backing {
		rb[i] = rollupBrief{Brief: b.Brief, Status: b.Status, Resolved: b.Resolved, Evidence: b.Evidence}
		if !b.Resolved {
			anyUnresolved = true
		}
	}
	state := rollupState(rb, anyUnresolved)

	ar := auditPackRequirement{
		ID:         e.ID,
		Title:      e.Title,
		Impact:     strings.TrimSpace(e.Impact),
		Status:     strings.TrimSpace(e.Status),
		Date:       strings.TrimSpace(e.Date),
		Acceptance: nonEmptyStrings(e.Acceptance),
		Briefs:     backing,
		State:      state,
	}
	if state == rollupStateCouldNotCheck {
		ar.Reason = unresolvedReason
	}
	return ar
}

// resolvedBackingCount sums resolved backing entries — the metric both sides
// of the completeness comparison report.
func resolvedBackingCount(bs []rollupBrief) int {
	n := 0
	for _, b := range bs {
		if b.Resolved {
			n++
		}
	}
	return n
}

func toRollupBriefs(bs []auditPackBacking) []rollupBrief {
	out := make([]rollupBrief, len(bs))
	for i, b := range bs {
		out[i] = rollupBrief{Brief: b.Brief, Status: b.Status, Resolved: b.Resolved, Evidence: b.Evidence}
	}
	return out
}

// auditPackCoverageAgreement is the SECOND, INDEPENDENT layer (task item 2):
// it compares THIS file's own per-requirement backing tally against a FRESH,
// unmodified call into rollup.go's buildRollupReport (sdlc/02) over the same
// requirement-id set. Two components, two derivations (two separate reads off
// disk), one comparison — see the file-level doc comment.
func auditPackCoverageAgreement(root string, mine []auditPackRequirement) (agree bool, mineCount, rollupCount int, detail string) {
	report, code := buildRollupReport(root, "")
	if code != rollupExitOK {
		return false, 0, 0, "the sdlc/02 rollup itself could not be read — refusing rather than comparing against nothing"
	}
	rollupByID := map[string]rollupRequirement{}
	for _, r := range report.Requirements {
		rollupByID[r.ID] = r
	}

	var mismatches []string
	for _, r := range mine {
		myN := resolvedBackingCount(toRollupBriefs(r.Briefs))
		mineCount += myN
		rr, ok := rollupByID[r.ID]
		if !ok {
			// Not in the register at all is already reported as its own
			// could-not-check requirement; nothing to compare against here.
			continue
		}
		rollupN := resolvedBackingCount(rr.Briefs)
		rollupCount += rollupN
		if myN != rollupN {
			mismatches = append(mismatches, fmt.Sprintf("%s: this pack's collector found %d resolved backing brief(s), the rollup independently found %d", r.ID, myN, rollupN))
		}
	}
	if len(mismatches) > 0 {
		return false, mineCount, rollupCount, strings.Join(mismatches, "; ")
	}
	return true, mineCount, rollupCount, ""
}

// buildAuditPackBundle assembles the whole pack: the requirement/brief walk,
// the completeness comparison (which may REFUSE, returning a non-nil error
// and no bundle), and the file set to hand to writeEvidenceBundle.
//
// The manifest.json this produces is NOT forked — writeEvidenceBundle
// (evidenceexport.go) is reused verbatim, so the existing per-file sha256 and
// `omitted` shape survive unchanged (Verify row 2). A could-not-check
// requirement is recorded as an `omitted` entry (never omitted silently — task
// item 3) but does NOT by itself fail the export: the pack was still written
// completely, honestly labelling the one thing it could not resolve. Only a
// REAL I/O failure to read a file the walk expected to bundle, or a
// completeness disagreement, refuses the write.
func buildAuditPackBundle(root string, rec *releaseRecord) (paths []string, bundle map[string][]byte, omitted []omittedEntry, doc auditPackDocument, err error) {
	reqs, briefPaths, readmes, reqOmitted, cerr := collectAuditPackRequirements(root, rec)
	if cerr != nil {
		return nil, nil, nil, auditPackDocument{}, cerr
	}
	omitted = append(omitted, reqOmitted...)
	bundle = map[string][]byte{}

	addFile := func(abs, reasonPrefix string) {
		rel, rerr := filepath.Rel(root, abs)
		if rerr != nil {
			rel = abs
		}
		rel = filepath.ToSlash(rel)
		if _, already := bundle[rel]; already {
			return
		}
		raw, ferr := os.ReadFile(abs)
		if ferr != nil {
			omitted = append(omitted, omittedEntry{Path: rel, Reason: reasonPrefix + ferr.Error()})
			return
		}
		bundle[rel] = raw
	}

	// The release record itself: the definitional statement of what is in
	// scope travels with the pack it defines.
	addFile(filepath.Join(root, "docs", releaseNotesDirName, rec.Release+".md"), "release record: ")

	// Each in-scope requirement's register entry, each resolved backing
	// brief's own file, and (deduped) each contributing stream's README —
	// the same "README travels with its briefs" rule --export-evidence
	// already keeps, for the same reason: the lifecycle/Verified/Reviewed
	// cells exist only there.
	seenStream := map[string]bool{}
	for _, r := range reqs {
		if entryPath, ok := requirementEntryPath(root, r.ID); ok {
			addFile(entryPath, "requirement entry: ")
		}
		for _, b := range r.Briefs {
			if !b.Resolved {
				continue // nothing on disk to bundle for an unresolved reference
			}
			p, ok := briefPaths[b.Brief]
			if !ok {
				omitted = append(omitted, omittedEntry{Path: "brief:" + b.Brief, Reason: "named in the corpus but its file could not be located on disk"})
				continue
			}
			addFile(p, "backing brief: ")
			stream := b.Brief
			if i := strings.IndexByte(stream, '/'); i >= 0 {
				stream = stream[:i]
			}
			if !seenStream[stream] {
				seenStream[stream] = true
				if rp, ok := readmes[stream]; ok {
					addFile(rp, "stream README: ")
				}
			}
		}
	}

	agree, mineCount, rollupCount, detail := auditPackCoverageAgreement(root, reqs)
	coverageNote := fmt.Sprintf("collector: %d resolved backing link(s); rollup (sdlc/02, independent): %d resolved backing link(s) — agree", mineCount, rollupCount)
	if !agree {
		coverageNote = fmt.Sprintf("DISAGREEMENT — collector: %d resolved backing link(s); rollup (sdlc/02, independent): %d resolved backing link(s); %s", mineCount, rollupCount, detail)
	}
	doc = auditPackDocument{
		Release:      rec.Release,
		Note:         auditPackNote,
		Requirements: reqs,
		Coverage: auditPackCoverage{
			ResolvedBackingLinks: mineCount,
			RollupBackingLinks:   rollupCount,
			RequirementsInScope:  len(reqs),
			Note:                 coverageNote,
		},
	}
	if !agree {
		return nil, nil, nil, doc, fmt.Errorf(
			"completeness disagreement: this pack's collector found %d resolved backing link(s) across %d requirement(s) in scope, the sdlc/02 rollup independently found %d — %s",
			mineCount, len(reqs), rollupCount, detail)
	}

	docJSON, jerr := json.MarshalIndent(doc, "", "  ")
	if jerr != nil {
		return nil, nil, nil, doc, jerr
	}
	bundle[auditPackReportPath] = docJSON

	paths = make([]string, 0, len(bundle))
	for p := range bundle {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	sort.Slice(omitted, func(i, j int) bool { return omitted[i].Path < omitted[j].Path })
	return paths, bundle, omitted, doc, nil
}

// requirementEntryPath resolves a requirement id to its register entry's
// absolute path, by re-reading the same parseRequirementsDir this file
// already called — the id->filename join lives in the register's own
// `File` field (basename), so no second lookup table is needed. A miss here
// after a hit in collectAuditPackRequirements would mean the register changed
// underfoot between the two calls; ok=false lets the caller treat it as an
// omission rather than panic.
func requirementEntryPath(root, id string) (string, bool) {
	entries, err := parseRequirementsDir(root)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.ID == id && e.File != "" {
			return filepath.Join(root, "docs", "streams", requirementsDirName, e.File), true
		}
	}
	return "", false
}

// runAuditPackExport is the `--export-audit-pack --release <tag>` entry
// point. Exit codes: 0 clean (including a pack that carries could-not-check
// requirements — those are recorded, honestly, never silently, but they do
// not by themselves fail the export) · 1 error (release not found, an
// unreadable register/stream tree, a real I/O failure, or a completeness
// disagreement) · 2 usage error (a malformed --release value).
func runAuditPackExport(root, release, output string, generated time.Time) int {
	if !releaseTagRe.MatchString(release) {
		fmt.Fprintf(os.Stderr, "statusgen: --release %q is not a valid release tag (want [A-Za-z0-9][A-Za-z0-9._-]*)\n", release)
		return 2
	}
	rec, err := loadReleaseRecord(root, release)
	if errors.Is(err, errReleaseNotFound) {
		fmt.Fprintf(os.Stderr, "statusgen: --export-audit-pack: release %q not found (no docs/%s/%s.md)\n", release, releaseNotesDirName, release)
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}

	paths, bundle, omitted, doc, berr := buildAuditPackBundle(root, rec)
	if berr != nil {
		fmt.Fprintln(os.Stderr, "statusgen: --export-audit-pack: REFUSING to write pack —", berr)
		return 1
	}

	if generated.IsZero() {
		generated = nowFunc()
	}
	if err := writeEvidenceBundle(output, paths, bundle, omitted, generated); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	fmt.Printf("exported audit pack for release %s to %s (%d requirement(s) in scope, %d resolved backing link(s))\n",
		release, output, len(doc.Requirements), doc.Coverage.ResolvedBackingLinks)
	if len(omitted) > 0 {
		fmt.Fprintf(os.Stderr, "statusgen: NOTE — %d item(s) recorded as could-not-check / omitted (see manifest.json \"omitted\" and %s):\n", len(omitted), auditPackReportPath)
		for _, o := range omitted {
			fmt.Fprintf(os.Stderr, "  - %s: %s\n", o.Path, o.Reason)
		}
	}
	return 0
}
