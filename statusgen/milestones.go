package main

// milestones.go — the milestone-v1 record loader (spec/milestone-v1.md;
// milestone-support/03).
//
// OWNER. statusgen milestone discovery owns reading versioned milestone records
// for a stream that negotiated them through its README `reader-requirements`
// envelope (spec/reader-capabilities-v1.md). A stream that did not opt in is a
// legacy stream: its milestones/ directory, if any, is never read here.
//
// SHAPE. loadMilestonePlan is the pure core: it takes explicit values (the owning
// root, the alias registry, the directory entries already read by a host adapter
// and a read-only repository view for provenance checks) and returns a typed
// MilestonePlan. It never fetches, claims, approves, launches or mutates.
// readMilestoneSource is the host adapter that reads one checkout through
// os.Root, so a symlink or `..` cannot carry a read outside the repository.
//
// THREE STATES. A record is either loaded, refused (MilestoneDiagnostic: a defect
// in that record) or its coverage is unknown (MilestoneUnknown: something it
// depends on could not be read or resolved). Coverage is known only when neither
// list has an entry. Missing or unreadable input is never rounded to "no
// milestones" or "nothing left to do".
//
// NOT ADVERTISED. statusgen's reader-capabilities advertisement does not list
// milestone-v1 (briefschemaversions.go); the tree version gate therefore still
// refuses an opted-in tree before this loader is reached in production. The loader
// is the qualified reader the later milestone-support briefs build on.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/statusgen/briefdoc"
	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/readercaps/record"
	"github.com/medici-finance/assay/statusgen/workref"
)

// milestoneSchema is the record format and the name a stream README lists under
// `reader-requirements` (as a required schema or semantic) to opt in.
const milestoneSchema = "milestone-v1"

// MilestoneCoverage is the plan-level reading: known only when every record was
// read, validated and every reference it depends on was resolved.
type MilestoneCoverage string

const (
	MilestoneCoverageKnown   MilestoneCoverage = "known"
	MilestoneCoverageUnknown MilestoneCoverage = "unknown"
)

// MilestoneCriterion is one Acceptance criterion and its Coverage mapping.
// Contributors are canonical logical brief keys from the record's contributes list;
// Evidence holds repository-relative paths.
type MilestoneCriterion struct {
	ID           string
	Contributors []string
	Evidence     []string
}

// MilestoneRecord is one loaded (identity, contract-version) record. Identity and
// version come only from the record's keys; Path is a locator.
type MilestoneRecord struct {
	Identity           workref.Identity
	Key                string // canonical <cell>:<repo>:<stream>:m-<NN>
	ContractVersion    uint64
	Title              string
	Owner              string
	Sources            []string
	Contributes        []workref.Result // exact optional brief revisions preserved
	RequiresMilestones []workref.Result // exact contract versions
	Criteria           []MilestoneCriterion
	Path               string // stream-relative locator, e.g. milestones/m-03-x.md
}

// Ref is the exact versioned reference to this record.
func (r MilestoneRecord) Ref() string {
	return r.Key + "@" + strconv.FormatUint(r.ContractVersion, 10)
}

// MilestoneDiagnostic is a refused record: it is excluded from Records.
type MilestoneDiagnostic struct {
	Path   string
	Reason string
}

// MilestoneUnknown is a coverage gap that must stay visible as unknown.
type MilestoneUnknown struct {
	Path   string // "" for a stream-level gap
	Reason string
}

// MilestonePlan is the loader's typed result for one stream.
type MilestonePlan struct {
	Stream      string
	OptedIn     bool
	Coverage    MilestoneCoverage
	Records     []MilestoneRecord
	Diagnostics []MilestoneDiagnostic
	Unknowns    []MilestoneUnknown
}

func (p *MilestonePlan) refuse(where, format string, args ...any) {
	p.Diagnostics = append(p.Diagnostics, MilestoneDiagnostic{Path: where, Reason: fmt.Sprintf(format, args...)})
}

func (p *MilestonePlan) unknown(where, format string, args ...any) {
	p.Unknowns = append(p.Unknowns, MilestoneUnknown{Path: where, Reason: fmt.Sprintf(format, args...)})
}

func (p *MilestonePlan) settle() {
	p.Coverage = MilestoneCoverageKnown
	if len(p.Diagnostics) > 0 || len(p.Unknowns) > 0 {
		p.Coverage = MilestoneCoverageUnknown
	}
	sort.SliceStable(p.Records, func(i, j int) bool {
		if p.Records[i].Key != p.Records[j].Key {
			return p.Records[i].Key < p.Records[j].Key
		}
		return p.Records[i].ContractVersion < p.Records[j].ContractVersion
	})
}

// MilestoneEntry is one directory entry under docs/streams/<stream>/milestones,
// as read by the host adapter. Mode carries the entry's own type bits (never a
// followed symlink's). Err is a read failure of a regular file.
type MilestoneEntry struct {
	Name    string
	Mode    fs.FileMode
	Content []byte
	Err     error
}

// MilestoneSource is every explicit input to the pure loader.
type MilestoneSource struct {
	Stream string
	// Owner is the canonical owning cell/repository of the checkout; nil when the
	// owning root could not be established (registry absent/unreadable/ambiguous).
	Owner    *workref.Context
	OwnerErr string
	Registry workref.Registry
	// DirMissing is true when the canonical milestones directory is absent; DirErr
	// is a failure to read it. Entries are meaningful only when both are unset.
	DirMissing bool
	DirErr     error
	Entries    []MilestoneEntry
	// Repo is a read-only view of the repository for source/evidence provenance.
	// nil means provenance cannot be checked (unknown, never assumed present).
	Repo fs.FS
}

var (
	milestoneFileRe  = regexp.MustCompile(`^m-[0-9]{2,}-[a-z0-9][a-z0-9-]*\.md$`)
	milestoneCritRe  = regexp.MustCompile(`^C[0-9]{2,}$`)
	milestoneLabelRe = regexp.MustCompile(`(?i)^(m-?[0-9]+|wave[ -]?[0-9]+|w[0-9]+)$`)
	milestoneKeys    = map[string]bool{
		"schema": true, "milestone": true, "contract-version": true, "title": true,
		"owner": true, "sources": true, "contributes": true, "requires-milestones": true,
	}
	milestoneSections = []string{
		"Outcome", "Supported scope", "Acceptance", "Coverage", "Prerequisites",
		"Acceptance authority", "Change and open decisions",
	}
)

// milestoneOptIn reports whether a stream README negotiated milestone-v1 through
// its reader-requirements envelope. A README without the envelope is legacy.
func milestoneOptIn(readme []byte) (bool, error) {
	req, err := record.Requirements(readme)
	if err != nil {
		return false, err
	}
	return requiresMilestoneSchema(req), nil
}

// An operation that names milestone-v1 as a required schema or semantic opts the
// stream in: a stream that requires milestone meaning anywhere has its records read.
func requiresMilestoneSchema(req readercaps.Requirements) bool {
	for _, op := range req.Required {
		for _, s := range append(append([]string{}, op.Schemas...), op.Semantics...) {
			if s == milestoneSchema {
				return true
			}
		}
	}
	return false
}

// loadMilestonePlan is the pure loader for one opted-in stream.
func loadMilestonePlan(in MilestoneSource) *MilestonePlan {
	plan := &MilestonePlan{Stream: in.Stream, OptedIn: true}
	defer plan.settle()
	if in.Owner == nil {
		plan.unknown("", "owning root is not established (%s); no record can be attributed, coverage unknown", in.OwnerErr)
		return plan
	}
	if in.DirErr != nil {
		plan.unknown("milestones", "milestones directory unreadable: %v; coverage unknown, not an empty plan", in.DirErr)
		return plan
	}
	if in.DirMissing {
		plan.unknown("milestones", "opted-in stream has no canonical milestones directory; coverage unknown, not an empty plan")
		return plan
	}
	type located struct {
		rec MilestoneRecord
		at  string
	}
	seen := map[string]located{}
	dups := map[string]bool{}
	var loaded []located
	for _, entry := range in.Entries {
		where := "milestones/" + entry.Name
		if entry.Mode&fs.ModeType != 0 {
			// A symlink, nested directory or special file can relocate or hide a
			// record; the location is ambiguous, so it is refused and stays unknown.
			plan.refuse(where, "not a regular file (%s); a milestone location must not be a link or nested path", entry.Mode.Type())
			continue
		}
		if entry.Name == "README.md" {
			continue
		}
		if !milestoneFileRe.MatchString(entry.Name) {
			plan.unknown(where, "unrecognized entry in the milestones directory; it may hide a record")
			continue
		}
		if entry.Err != nil {
			plan.unknown(where, "record unreadable: %v", entry.Err)
			continue
		}
		rec, ok := parseMilestoneRecord(plan, in, where, entry.Content)
		if !ok {
			continue
		}
		k := rec.Ref()
		if prior, ok := seen[k]; ok {
			plan.refuse(where, "duplicate record for %s (also %s); exactly one record per identity and contract version", k, prior.at)
			dups[k] = true
			continue
		}
		seen[k] = located{rec, where}
		loaded = append(loaded, located{rec, where})
	}
	for _, l := range loaded {
		if dups[l.rec.Ref()] {
			plan.refuse(l.at, "duplicate record for %s; neither copy is loaded", l.rec.Ref())
			continue
		}
		plan.Records = append(plan.Records, l.rec)
	}
	return plan
}

// parseMilestoneRecord validates one record's closed field and body contract.
func parseMilestoneRecord(plan *MilestonePlan, in MilestoneSource, where string, raw []byte) (MilestoneRecord, bool) {
	var rec MilestoneRecord
	block, body, err := briefdoc.Split(string(raw))
	if err != nil {
		plan.refuse(where, "frontmatter: %v", err)
		return rec, false
	}
	fields, err := briefdoc.ParseMapping(block)
	if err != nil {
		plan.refuse(where, "frontmatter: %v", err)
		return rec, false
	}
	ok := true
	bad := func(format string, args ...any) { plan.refuse(where, format, args...); ok = false }
	var keys []string
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !milestoneKeys[k] {
			bad("unknown key %q (closed milestone-v1 key set)", k)
		}
	}
	for k := range milestoneKeys {
		if _, present := fields[k]; !present {
			bad("missing required key %q", k)
		}
	}
	if !ok {
		return rec, false
	}
	if s, isStr := fields["schema"].(string); !isStr || s != milestoneSchema {
		bad("schema must be %s", milestoneSchema)
	}
	version, isInt := fields["contract-version"].(int)
	if !isInt || version < 1 {
		bad("contract-version must be a positive integer")
	}
	rec.Path = where
	rec.ContractVersion = uint64(version)
	rec.Title, _ = fields["title"].(string)
	if strings.TrimSpace(rec.Title) == "" || milestoneLabelRe.MatchString(strings.TrimSpace(rec.Title)) {
		bad("title must state a useful outcome, not only an ID or wave label")
	}
	rec.Owner, _ = fields["owner"].(string)
	if strings.TrimSpace(rec.Owner) == "" {
		bad("owner must be a non-empty string")
	}
	key, isStr := fields["milestone"].(string)
	switch {
	case !isStr:
		bad("milestone must be a string")
	case strings.Contains(key, "@"):
		bad("milestone key carries a version selector; the version comes only from contract-version")
	case ok:
		res, err := workref.Resolve(key+"@"+strconv.Itoa(version), nil, in.Registry, workref.TypedMilestone)
		if err != nil {
			bad("milestone %q: %v", key, err)
			break
		}
		id := res.Identity
		if id.Cell != in.Owner.Cell || id.Repo != in.Owner.Repo || id.Stream != in.Stream {
			bad("milestone %q is owned by %s:%s:%s, not this record's location %s:%s:%s", key, id.Cell, id.Repo, id.Stream, in.Owner.Cell, in.Owner.Repo, in.Stream)
		}
		rec.Identity, rec.Key = id, id.String()
	}
	sources, isList := milestoneStrings(fields["sources"])
	if !isList || len(sources) == 0 {
		bad("sources must be a non-empty array of strings")
	}
	rec.Sources = sources
	contributes, isList := milestoneStrings(fields["contributes"])
	if !isList {
		bad("contributes must be an array of strings")
	}
	logical := map[string]bool{}
	for _, ref := range contributes {
		res, err := workref.Resolve(ref, in.Owner, in.Registry, workref.TypedBrief)
		if err != nil {
			bad("contributes %q: %v", ref, err)
			continue
		}
		if logical[res.LogicalKey] {
			bad("contributes names %s twice", res.LogicalKey)
			continue
		}
		logical[res.LogicalKey] = true
		rec.Contributes = append(rec.Contributes, res)
	}
	requires, isList := milestoneStrings(fields["requires-milestones"])
	if !isList {
		bad("requires-milestones must be an array of strings")
	}
	for _, ref := range requires {
		res, err := workref.Resolve(ref, in.Owner, in.Registry, workref.TypedMilestone)
		if err != nil {
			bad("requires-milestones %q: %v", ref, err)
			continue
		}
		rec.RequiresMilestones = append(rec.RequiresMilestones, res)
	}
	for _, title := range milestoneSections {
		if !briefdoc.HasContent(briefdoc.Section(body, title, false)) {
			bad("body section %q is missing or empty", title)
		}
	}
	if !ok {
		return rec, false
	}
	rec.Criteria, ok = milestoneCoverage(plan, in, where, body, rec.Contributes)
	if !ok {
		return rec, false
	}
	for _, src := range rec.Sources {
		checkMilestoneProvenance(plan, in, where, "source", src)
	}
	return rec, true
}

// milestoneCoverage reads the Acceptance criterion IDs and the Coverage mapping.
// Defects refuse the record; gaps (uncovered criteria, unmapped contributions,
// missing evidence) leave it loaded with unknown coverage.
func milestoneCoverage(plan *MilestonePlan, in MilestoneSource, where, body string, contributes []workref.Result) ([]MilestoneCriterion, bool) {
	ok := true
	bad := func(format string, args ...any) { plan.refuse(where, format, args...); ok = false }
	acceptance := markdownTable(briefdoc.Section(body, "Acceptance", false))
	if len(acceptance) < 2 || !strings.EqualFold(acceptance[0][0], "ID") {
		bad("Acceptance must hold a criterion table whose first column is ID")
		return nil, false
	}
	var criteria []MilestoneCriterion
	index := map[string]int{}
	for _, row := range acceptance[1:] {
		id := row[0]
		if !milestoneCritRe.MatchString(id) {
			bad("criterion ID %q must match C<NN>", id)
			continue
		}
		if _, dup := index[id]; dup {
			bad("criterion %s is defined twice", id)
			continue
		}
		index[id] = len(criteria)
		criteria = append(criteria, MilestoneCriterion{ID: id})
	}
	coverage := markdownTable(briefdoc.Section(body, "Coverage", false))
	if len(coverage) < 1 || len(coverage[0]) < 3 || !strings.EqualFold(coverage[0][0], "Criterion") ||
		!strings.EqualFold(coverage[0][1], "Contributors") || !strings.EqualFold(coverage[0][2], "Evidence") {
		bad("Coverage must hold a Criterion | Contributors | Evidence table")
		return nil, false
	}
	declared := map[string]bool{}
	for _, c := range contributes {
		declared[c.LogicalKey] = false
	}
	mapped := map[string]bool{}
	for _, row := range coverage[1:] {
		if len(row) < 3 {
			bad("coverage row %q has fewer than three cells", strings.Join(row, " | "))
			continue
		}
		i, defined := index[row[0]]
		if !defined {
			bad("coverage names undefined criterion %q", row[0])
			continue
		}
		if mapped[row[0]] {
			bad("criterion %s is mapped twice in Coverage", row[0])
			continue
		}
		mapped[row[0]] = true
		for _, ref := range cellList(row[1]) {
			res, err := workref.Resolve(ref, in.Owner, in.Registry, workref.TypedBrief)
			if err != nil {
				bad("criterion %s contributor %q: %v", row[0], ref, err)
				continue
			}
			if _, listed := declared[res.LogicalKey]; !listed {
				bad("criterion %s names contributor %s, which is not in contributes", row[0], res.LogicalKey)
				continue
			}
			declared[res.LogicalKey] = true
			criteria[i].Contributors = append(criteria[i].Contributors, res.LogicalKey)
		}
		criteria[i].Evidence = cellList(row[2])
	}
	if !ok {
		return nil, false
	}
	for _, c := range criteria {
		if !mapped[c.ID] {
			plan.unknown(where, "criterion %s has no coverage row; it is uncovered, not satisfied", c.ID)
			continue
		}
		if len(c.Contributors) == 0 && len(c.Evidence) == 0 {
			plan.unknown(where, "criterion %s has neither a contributing brief nor existing evidence (unmapped work)", c.ID)
		}
		for _, ev := range c.Evidence {
			checkMilestoneProvenance(plan, in, where, "criterion "+c.ID+" evidence", ev)
		}
	}
	var unmapped []string
	for key, used := range declared {
		if !used {
			unmapped = append(unmapped, key)
		}
	}
	sort.Strings(unmapped)
	for _, key := range unmapped {
		plan.unknown(where, "contribution %s maps to no acceptance criterion (unmapped work)", key)
	}
	return criteria, true
}

// checkMilestoneProvenance resolves a repository-relative source/evidence path.
// A path that escapes the repository refuses; a missing, unreadable or
// non-path reference is unknown, never assumed present.
func checkMilestoneProvenance(plan *MilestonePlan, in MilestoneSource, where, what, ref string) {
	p, _, _ := strings.Cut(ref, "#")
	if strings.Contains(p, "://") || strings.ContainsAny(ref, " \t") || p == "" {
		plan.unknown(where, "%s %q is not a repository path; it cannot be checked offline", what, ref)
		return
	}
	if strings.HasPrefix(p, "/") || !fs.ValidPath(path.Clean(p)) || path.Clean(p) != strings.TrimSuffix(p, "/") {
		plan.refuse(where, "%s %q escapes or is not a clean repository-relative path", what, ref)
		return
	}
	if in.Repo == nil {
		plan.unknown(where, "%s %q: repository tree unavailable; provenance unknown", what, ref)
		return
	}
	if _, err := fs.Stat(in.Repo, path.Clean(p)); err != nil {
		plan.unknown(where, "%s %q could not be read: %v", what, ref, err)
	}
}

// resolveMilestonePrerequisites checks every exact prerequisite version against
// the plans loaded in this run. A prerequisite outside them, or a plan whose
// own coverage is unknown, leaves the dependant's coverage unknown.
func resolveMilestonePrerequisites(plans []*MilestonePlan) {
	have := map[string]bool{}
	for _, p := range plans {
		for _, r := range p.Records {
			have[r.Ref()] = true
		}
	}
	for _, p := range plans {
		for _, r := range p.Records {
			for _, req := range r.RequiresMilestones {
				ref := req.Identity.String() + "@" + strconv.FormatUint(req.Revision, 10)
				if !have[ref] {
					p.unknown(r.Path, "prerequisite %s is not a loaded record version; coverage unknown", ref)
				}
			}
		}
		p.settle()
	}
}

// markdownTable returns the cells of the first Markdown table in a section,
// header first, separator row dropped, backticks stripped.
func markdownTable(section string) [][]string {
	var rows [][]string
	started := false
	for _, line := range strings.Split(section, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "|") {
			if started {
				break
			}
			continue
		}
		started = true
		t = strings.TrimSuffix(strings.TrimPrefix(t, "|"), "|")
		var cells []string
		separator := true
		for _, c := range strings.Split(t, "|") {
			c = strings.TrimSpace(strings.ReplaceAll(c, "`", ""))
			if strings.Trim(c, ":-") != "" {
				separator = false
			}
			cells = append(cells, c)
		}
		if separator {
			continue
		}
		rows = append(rows, cells)
	}
	return rows
}

// cellList splits a comma-separated table cell; an em dash or "none" is empty.
func cellList(cell string) []string {
	var out []string
	for _, v := range strings.Split(cell, ",") {
		v = strings.TrimSpace(v)
		if v == "" || v == "—" || v == "-" || strings.EqualFold(v, "none") {
			continue
		}
		out = append(out, v)
	}
	return out
}

func milestoneStrings(v any) ([]string, bool) {
	list, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// milestoneOwner derives the canonical owning context and the workref registry
// from the existing graph alias registry. The self alias is the registry's
// explicit `self:` key or its single published alias; anything else is unknown.
func milestoneOwner(reg *graphRepos) (*workref.Context, workref.Registry, string) {
	if reg == nil {
		return nil, workref.Registry{}, "no docs/streams/graph-repos.yaml alias registry"
	}
	var roots []workref.Root
	var published []string
	for alias, e := range reg.Aliases {
		cell := e.Cell
		if cell == "" {
			cell = reg.Cell
		}
		roots = append(roots, workref.Root{Cell: cell, Repo: alias})
		if !e.Unpublished {
			published = append(published, alias)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Repo < roots[j].Repo })
	registry, err := workref.NewRegistry(roots, nil)
	if err != nil {
		return nil, workref.Registry{}, "alias registry: " + err.Error()
	}
	self := reg.Self
	if self == "" {
		if len(published) != 1 {
			return nil, registry, "alias registry names no self alias and not exactly one published alias"
		}
		self = published[0]
	}
	if _, ok := reg.Aliases[self]; !ok {
		return nil, registry, fmt.Sprintf("self alias %q is not in the alias registry", self)
	}
	return &workref.Context{Cell: reg.Cell, Repo: self}, registry, ""
}

// readMilestoneSource is the host adapter: it reads docs/streams/<stream>/milestones
// under one repository root through os.Root (no symlink or `..` escape) and returns
// the explicit loader input plus the root to close once the plan is built.
func readMilestoneSource(root, stream string, reg *graphRepos, regErr error) (MilestoneSource, func()) {
	in := MilestoneSource{Stream: stream}
	in.Owner, in.Registry, in.OwnerErr = milestoneOwner(reg)
	if regErr != nil {
		in.Owner, in.OwnerErr = nil, regErr.Error()
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		in.DirErr = err
		return in, func() {}
	}
	in.Repo = r.FS()
	dir := path.Join("docs", "streams", stream, "milestones")
	info, err := r.Lstat(filepath.FromSlash(dir))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		in.DirMissing = true
		return in, func() { r.Close() }
	case err != nil:
		in.DirErr = err
		return in, func() { r.Close() }
	case !info.IsDir():
		in.DirErr = fmt.Errorf("%s is not a directory (%s)", dir, info.Mode().Type())
		return in, func() { r.Close() }
	}
	entries, err := fs.ReadDir(in.Repo, dir)
	if err != nil {
		in.DirErr = err
		return in, func() { r.Close() }
	}
	for _, e := range entries {
		entry := MilestoneEntry{Name: e.Name(), Mode: e.Type()}
		if entry.Mode&fs.ModeType == 0 {
			entry.Content, entry.Err = fs.ReadFile(in.Repo, path.Join(dir, e.Name()))
		}
		in.Entries = append(in.Entries, entry)
	}
	return in, func() { r.Close() }
}

// attachMilestonePlans loads a plan for every stream that negotiated milestone-v1.
// Legacy streams are left untouched (Milestones stays nil). The alias registry is
// read only when at least one stream opted in, so legacy trees are unaffected.
func attachMilestonePlans(root string, streams []*Stream) {
	var plans []*MilestonePlan
	var reg *graphRepos
	var regErr error
	loaded := false
	for _, s := range streams {
		if s.MilestoneOptInErr != nil {
			s.Milestones = &MilestonePlan{Stream: filepath.Base(s.Dir), OptedIn: true}
			s.Milestones.unknown("README.md", "reader-requirements unreadable: %v; milestone opt-in could not be checked", s.MilestoneOptInErr)
			s.Milestones.settle()
			continue
		}
		if !s.MilestoneOptIn {
			continue
		}
		if !loaded {
			reg, _, regErr = loadGraphRepos(root)
			loaded = true
		}
		in, done := readMilestoneSource(root, filepath.Base(s.Dir), reg, regErr)
		s.Milestones = loadMilestonePlan(in)
		done()
		plans = append(plans, s.Milestones)
	}
	resolveMilestonePrerequisites(plans)
}
