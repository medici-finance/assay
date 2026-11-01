package main

// readset_adapter.go — the adapter half of build-less-brittle/15: it observes
// each witness's recorded read set (the `docs/streams/<stream>/reads/<sha256>`
// file a Runner cell's `(reads sha256:…)` token names), the introduction
// observation for that witness row, and the go compile-input floor for every
// `go-run` entry the read set carries. The library (evaluation/readset.go)
// judges all three; this file only observes. coverageOptionsFor is the one
// place a caller gets these observations, so no caller can skip the floor
// (a read set with a build-family go run and no floor fails closed in the
// library either way).
//
// The floor is a read, not a re-run. The library asks for it only for a read
// set it has admitted (evaluation.GoFloorSource). The recorded invocation is
// read through an allow-list: a recorded flag, GOFLAGS entry, directory or
// package argument the floor does not reproduce makes the floor
// could-not-check, and nothing recorded is handed to the go tool except the
// reproduced values (-tags, -mod, -race, the platform and the package
// arguments). The go tool runs with an environment built here, not inherited:
// a fresh HOME and build cache, no network, no toolchain switch, no GOFLAGS.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/statusgen/evaluation"
)

// readSetSource observes read sets for one evaluation run. Tree exports are
// shared by every floor computed at the same commit and removed by close.
type readSetSource struct {
	root    string
	exports map[string]string // full commit -> exported tree dir ("" when the export failed)
	failed  map[string]string // full commit -> why the export failed
	intro   map[string]map[string]introRow
	// introWhy is why a brief's history was refused (introductions).
	introWhy map[string]string
	floors   map[string]evaluation.GoFloor // GoFloorKey -> the floor computed for it
	// tool is the evaluator's go tool, read once on first use; toolWhy says
	// why it could not be read. scratch holds the floor runs' HOME, GOPATH,
	// build cache and temporary directory.
	tool    *floorTool
	toolWhy string
	scratch string
	// exported is the bytes every export so far may have written, charged
	// when its writing starts; floorRuns counts the go tool runs started and
	// floorSpent the wall time of floors, exports included, for the
	// per-evaluation caps.
	exported   int64
	floorRuns  int
	floorSpent time.Duration
	// wanted is, per brief, the identity keys of its token rows at head: the
	// only keys introductions keeps. files memoizes each read-set file read,
	// by path. observed is the bytes the evaluation's observations have read
	// or parsed together, held under observeEvalBytes (charge).
	wanted   map[string]map[string]bool
	files    map[string]readSetRead
	observed int64
}

// readSetRead is one read-set file read: its content, whether it was found,
// and why it was refused.
type readSetRead struct {
	content []byte
	found   bool
	problem string
}

// observeEvalBytes bounds, across one evaluation, the bytes the read-set
// observations read and parse together, whatever the number of briefs, rows
// or touching commits: every brief's history (its distinct versions, sized
// before any is read, charged again for each brief even when briefs share
// versions), the row keys each touching commit compares, and the read-set
// content each token row hands the library to digest and parse. Past it an
// observation is refused, which the library judges could-not-check.
var observeEvalBytes int64 = 256 << 20

// charge takes n bytes from the evaluation's observation budget, or returns
// the refusal when they do not fit; a refused charge takes nothing.
func (s *readSetSource) charge(n int64) string {
	if n > observeEvalBytes-s.observed {
		return fmt.Sprintf("the evaluation's read-set observations already hold the cap of %d bytes", observeEvalBytes)
	}
	s.observed += n
	return ""
}

// introRow is the commit that introduced a witness row with a given identity
// key, and that row's Runner cell there.
type introRow struct {
	commit string
	runner string
}

func newReadSetSource(root string) *readSetSource {
	return &readSetSource{root: root, exports: map[string]string{}, failed: map[string]string{}, intro: map[string]map[string]introRow{}, introWhy: map[string]string{}, floors: map[string]evaluation.GoFloor{},
		wanted: map[string]map[string]bool{}, files: map[string]readSetRead{}}
}

// close removes every exported tree and the floor runs' scratch directory.
func (s *readSetSource) close() {
	if s == nil {
		return
	}
	for _, d := range s.exports {
		if d != "" {
			_ = os.RemoveAll(d)
		}
	}
	if s.scratch != "" {
		_ = os.RemoveAll(s.scratch)
	}
	s.exports, s.failed, s.floors, s.files = map[string]string{}, map[string]string{}, map[string]evaluation.GoFloor{}, map[string]readSetRead{}
	s.tool, s.toolWhy, s.scratch = nil, "", ""
	s.exported, s.floorRuns, s.floorSpent = 0, 0, 0
}

// observeBrief fills w.ReadSet for every witness of one brief whose Runner
// cell carries a `reads sha256:` token (observe), in row order. It first
// names the brief's token row keys, so the history walk keeps those keys
// only.
func (s *readSetSource) observeBrief(briefPath, briefRel string, witnesses map[string][]evaluation.Witness) {
	ids := make([]string, 0, len(witnesses))
	wanted := map[string]bool{}
	for id, ws := range witnesses {
		ids = append(ids, id)
		for _, w := range ws {
			if d, _, ok := evaluation.ReadSetToken(w.Runner); ok && d != "" {
				wanted[evaluation.WitnessRowKey(id, w.Command, w.Date, w.Runner)] = true
			}
		}
	}
	if s.wanted[briefRel] == nil {
		s.wanted[briefRel] = wanted
	}
	slices.Sort(ids)
	for _, id := range ids {
		ws := witnesses[id]
		for i := range ws {
			s.observe(briefPath, briefRel, id, &ws[i])
		}
	}
}

// observe fills w.ReadSet for a witness whose Runner cell carries a
// `reads sha256:` token. briefPath is the brief file on disk, briefRel its
// repository path, rowID/date the witness's `#` and Date cells. Nothing it
// reads is unbounded: the read-set file is read only when it is a regular
// file within readSetMaxBytes, once per path, and each row's content is
// charged to the evaluation's budget (readSet); the introduction's history is
// read within its caps and that budget (introductions), and the file's
// existence at the introduction commit is a batch check that reads no
// content. A refusal is the observation's Problem, which the library judges
// could-not-check. A brief observed row by row, not through observeBrief,
// keeps the key of the first row observed; a later row's introduction is then
// not found, so its read set is ignored, never bound.
func (s *readSetSource) observe(briefPath, briefRel, rowID string, w *evaluation.Witness) {
	digest, _, ok := evaluation.ReadSetToken(w.Runner)
	if !ok || digest == "" {
		return
	}
	o := &evaluation.ReadSetObservation{}
	w.ReadSet = o
	key := evaluation.WitnessRowKey(rowID, w.Command, w.Date, w.Runner)
	if s.wanted[briefRel] == nil {
		s.wanted[briefRel] = map[string]bool{key: true}
	}
	o.Content, o.Found, o.Problem = s.readSet(filepath.Join(filepath.Dir(briefPath), evaluation.ReadSetDir, digest))
	if o.Problem != "" {
		return
	}
	intro, problem := s.introductions(briefRel)
	if problem != "" {
		o.Problem = problem
		return
	}
	in, found := intro[key]
	if !found {
		return
	}
	o.IntroFound, o.IntroRunner = true, in.runner
	spec := in.commit + ":" + evaluation.ReadSetPath(filepath.ToSlash(filepath.Dir(briefRel)), digest)
	sizes, ok := coverageRepo{root: s.root}.ObjectSizes([]string{spec})
	o.IntroFile = ok && sizes[spec].Type == "blob"
}

// readSetMaxBytes caps one read-set file; introMaxBlobBytes one historic
// brief blob and introMaxHistoryBytes every historic brief blob of one brief
// together. A brief or read set over them is refused, never read.
var (
	readSetMaxBytes      int64 = 16 << 20
	introMaxBlobBytes    int64 = 4 << 20
	introMaxHistoryBytes int64 = 64 << 20
)

// readSet is the read-set file at p for one token row. The file is read
// once per evaluation (readSetFile), no further than the bytes the budget has
// left, and the rows naming it share that one content; each row's content is
// charged to the budget, since the library digests and parses it per row.
func (s *readSetSource) readSet(p string) ([]byte, bool, string) {
	r, ok := s.files[p]
	if !ok {
		r.content, r.found, r.problem = readSetFile(p, observeEvalBytes-s.observed)
		if r.found {
			if why := s.charge(int64(len(r.content))); why != "" {
				r = readSetRead{problem: why}
			}
		}
		s.files[p] = r
		return r.content, r.found, r.problem
	}
	if !r.found || r.problem != "" {
		return nil, r.found, r.problem
	}
	if why := s.charge(int64(len(r.content))); why != "" {
		return nil, false, why
	}
	return r.content, true, ""
}

// readSetFile reads the read-set file at p: absent is not found, and a file
// that is not a regular file, is over readSetMaxBytes or over left (what the
// evaluation's budget has left) is a problem, read no further than the cap.
func readSetFile(p string, left int64) (content []byte, found bool, problem string) {
	fi, err := os.Lstat(p)
	if err != nil {
		return nil, false, ""
	}
	if !fi.Mode().IsRegular() {
		return nil, false, "the read-set file is not a regular file"
	}
	if fi.Size() > readSetMaxBytes {
		return nil, false, fmt.Sprintf("the read-set file holds %d bytes, over the cap of %d", fi.Size(), readSetMaxBytes)
	}
	if fi.Size() > left {
		return nil, false, fmt.Sprintf("the evaluation's read-set observations already hold the cap of %d bytes", observeEvalBytes)
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, false, ""
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, readSetMaxBytes+1))
	if err != nil {
		return nil, false, ""
	}
	if int64(len(b)) > readSetMaxBytes {
		return nil, false, fmt.Sprintf("the read-set file holds more than the cap of %d bytes", readSetMaxBytes)
	}
	return b, true, ""
}

// introductions maps each witness row identity key in the brief's history to
// the commit that introduced it, with that commit's Runner cell. A commit
// introduces a key when its brief file holds a row with that key and none of
// its parents' brief files does. The order commits are visited in plays no
// part, so a commit date (which its author sets) cannot choose the
// introduction. A key more than one commit introduces (two branches that
// each added it, or a row removed and added again) has no introduction: an
// ambiguous history never binds a read set. Only the keys the brief's token
// rows ask for (s.wanted) are kept, and each is counted to two introducers at
// most, so what the walk holds follows the head brief, not its history.
//
// The history is walked in process (gitcore PathHistory: replacement objects
// are never read). Each distinct brief blob, the parents' included, is sized
// first by one batch check that reads no content: one over
// introMaxBlobBytes, or all of them together over introMaxHistoryBytes, is
// the returned problem and nothing is read; the distinct blobs' bytes are
// then charged to the evaluation's budget (charge), and so is each touching
// commit's comparison, the bytes of the kept keys its version holds. The blobs
// are streamed once through the coverage port and parsed line by line, never
// held whole. A
// repository whose profile the in-process reader does not cover (a shallow
// clone, a native discovery or config environment), or a blob that cannot be
// read, yields no introduction, so every read set there is ignored: a history
// this reader cannot see never binds a read set.
func (s *readSetSource) introductions(briefRel string) (map[string]introRow, string) {
	if m, ok := s.intro[briefRel]; ok {
		return m, s.introWhy[briefRel]
	}
	none := map[string]introRow{}
	s.intro[briefRel] = none
	read := gitCoreFor(s.root)
	if read.err != nil || read.native != "" {
		return none, ""
	}
	type touch struct {
		commit, blob string
		parents      []string
	}
	var touches []touch
	err := read.repo.PathHistory("HEAD", read.prefix+briefRel, func(pc gitcore.PathCommit) bool {
		if pc.PathID != "" && !slices.Contains(pc.ParentPathIDs, pc.PathID) {
			touches = append(touches, touch{pc.Hash, pc.PathID, pc.ParentPathIDs})
		}
		return true
	})
	if err != nil {
		return none, ""
	}
	var blobs []string
	listed := map[string]bool{}
	want := func(b string) {
		if b != "" && !listed[b] {
			listed[b] = true
			blobs = append(blobs, b)
		}
	}
	for _, t := range touches {
		want(t.blob)
		for _, p := range t.parents {
			want(p)
		}
	}
	repo := coverageRepo{root: s.root}
	sizes, ok := repo.ObjectSizes(blobs)
	if !ok {
		return none, ""
	}
	var total int64
	for _, b := range blobs {
		sz, found := sizes[b]
		if !found || sz.Type != "blob" {
			return none, ""
		}
		if sz.Size > introMaxBlobBytes {
			s.introWhy[briefRel] = fmt.Sprintf("a historic version of the brief holds %d bytes, over the cap of %d", sz.Size, introMaxBlobBytes)
			return none, s.introWhy[briefRel]
		}
		if total += sz.Size; total > introMaxHistoryBytes {
			s.introWhy[briefRel] = fmt.Sprintf("the brief's history holds more than the cap of %d bytes", introMaxHistoryBytes)
			return none, s.introWhy[briefRel]
		}
	}
	if why := s.charge(total); why != "" {
		s.introWhy[briefRel] = why
		return none, why
	}
	wanted := s.wanted[briefRel]
	rowsOf := map[string]map[string]string{} // blob -> wanted key -> Runner cell (only a cell carrying a read-set token is kept)
	err = repo.StreamObjects(blobs, func(oid, typ string, size int64, body io.Reader) error {
		if typ != "blob" || size != sizes[oid].Size {
			return fmt.Errorf("%s changed between the size check and the read", oid)
		}
		r := map[string]string{}
		sc := bufio.NewScanner(body)
		sc.Buffer(nil, int(introMaxBlobBytes)+1)
		for sc.Scan() {
			line := sc.Text()
			cells := witnessCells(line)
			if cells == nil {
				continue
			}
			key := evaluation.WitnessRowKey(cells[0], witnessCommandOf(line), cells[witnessCellDate], cells[witnessCellRunner])
			if !wanted[key] {
				continue
			}
			if _, seen := r[key]; !seen {
				runner := strings.Clone(strings.TrimSpace(cells[witnessCellRunner]))
				if _, _, tok := evaluation.ReadSetToken(runner); !tok {
					runner = "" // a row with no token can bind no read set
				}
				r[key] = runner
			}
		}
		if err := sc.Err(); err != nil {
			return err
		}
		rowsOf[oid] = r
		return nil
	})
	if err != nil {
		return none, ""
	}
	type introducer struct {
		row introRow
		n   int // introducers seen, counted to 2
	}
	introducers := map[string]introducer{}
	for _, t := range touches {
		held := rowsOf[t.blob]
		var cost int64
		for key := range held {
			cost += int64(len(key))
		}
		if why := s.charge(cost); why != "" {
			s.introWhy[briefRel] = why
			return none, why
		}
		var parents []map[string]string
		for _, p := range t.parents {
			if p == "" {
				continue // the brief is absent in that parent
			}
			parents = append(parents, rowsOf[p])
		}
		for key, runner := range held {
			inParent := false
			for _, pr := range parents {
				if _, has := pr[key]; has {
					inParent = true
					break
				}
			}
			if in := introducers[key]; !inParent && in.n < 2 {
				if in.n == 0 {
					in.row = introRow{commit: t.commit, runner: runner}
				}
				in.n++
				introducers[key] = in
			}
		}
	}
	m := map[string]introRow{}
	for key, in := range introducers {
		if in.n == 1 {
			m[key] = in.row
		}
	}
	s.intro[briefRel] = m
	return m, ""
}

// GoFloor is the floor the library asks for one build-family `go-run` entry
// of a read set it has admitted (evaluation.GoFloorSource). Each entry is
// computed at most once per run.
func (s *readSetSource) GoFloor(commit, entry string) (evaluation.GoFloor, bool) {
	k := evaluation.GoFloorKey(commit, entry)
	if f, ok := s.floors[k]; ok {
		return f, true
	}
	g, ok := evaluation.ParseGoRun(entry)
	if !ok || !g.BuildFamily() {
		return evaluation.GoFloor{}, false
	}
	f := s.goFloor(commit, g)
	s.floors[k] = f
	return f, true
}

// goFloor reads the recorded invocation through the allow-list, exports the
// commit's tree and lists, through the go tool, every in-tree file the
// invocation compiles. Its wall time, the export's included, is charged to
// floorSpent whether it succeeds or not, and each go tool run is charged to
// floorRuns as it starts (startGo).
func (s *readSetSource) goFloor(commit string, g evaluation.GoRun) evaluation.GoFloor {
	plan, problem := planFloor(g)
	if problem != "" {
		return evaluation.GoFloor{Problem: problem}
	}
	start := time.Now()
	defer func() { s.floorSpent += time.Since(start) }()
	gate := func() string { return s.startGo(time.Since(start)) }
	tool, why := s.goTool(gate)
	if why != "" {
		return evaluation.GoFloor{Problem: why}
	}
	if why := s.floorLeft(time.Since(start)); why != "" {
		return evaluation.GoFloor{Problem: why}
	}
	dir, why := s.export(commit)
	if why != "" {
		return evaluation.GoFloor{Problem: why}
	}
	return runFloor(tool, s.scratch, dir, plan, gate)
}

// floorLeft is the refusal once the evaluation's floor budget is spent: the
// go tool runs already started, or the wall time floors have taken, running
// (the current floor's time so far) included.
func (s *readSetSource) floorLeft(running time.Duration) string {
	if s.floorRuns >= floorEvalRuns {
		return fmt.Sprintf("the evaluation has already run the floor's go tool %d times, its cap", floorEvalRuns)
	}
	if s.floorSpent+running >= floorEvalBudget {
		return fmt.Sprintf("the evaluation has already spent %s on compile-input floors, its cap", floorEvalBudget)
	}
	return ""
}

// startGo charges one go tool run to the evaluation as it starts, or returns
// the refusal when the budget is spent; running is the current floor's time
// so far.
func (s *readSetSource) startGo(running time.Duration) string {
	if why := s.floorLeft(running); why != "" {
		return why
	}
	s.floorRuns++
	return ""
}

// export writes commit's tree, byte for byte as git stores it (no checkout
// filters, no export-ignore), to a fresh temporary directory. The bytes an
// export may write are charged to s.exported when its writing starts, whether
// it then succeeds or fails.
func (s *readSetSource) export(commit string) (string, string) {
	if d, ok := s.exports[commit]; ok {
		return d, s.failed[commit]
	}
	left := exportEvalBytes - s.exported
	if left <= 0 {
		s.exports[commit], s.failed[commit] = "", fmt.Sprintf("the evaluation's exports already hold the cap of %d bytes", exportEvalBytes)
		return "", s.failed[commit]
	}
	d, n, err := exportTreeWithin(s.root, commit, min(left, exportMaxBytes))
	s.exported += n
	if err != nil {
		s.exports[commit], s.failed[commit] = "", "the witness tree could not be exported: "+err.Error()
		return "", s.failed[commit]
	}
	s.exports[commit] = d
	return d, ""
}

// exportMaxEntries, exportMaxListBytes, exportMaxBytes and
// exportMaxLinkBytes cap one exported witness tree. The listing is read as it
// arrives and stops past exportMaxEntries entries or exportMaxListBytes
// bytes, so the listing held in memory is bounded by them. exportMaxBytes
// bounds the bytes the export writes, every entry counted, a repeated object
// once per entry; the bytes read from git are each distinct object once,
// never more than that (a link's object twice: once to check its target
// before any write, once more if a file shares it). exportMaxLinkBytes bounds
// the link targets, the only object content held in memory. exportEvalBytes
// bounds the bytes every export of one evaluation may write together, the
// floor's scratch disk, each export charged when its writing starts.
var (
	exportMaxEntries         = 200000
	exportMaxListBytes int64 = 64 << 20
	exportMaxBytes     int64 = 4 << 30
	exportMaxLinkBytes int64 = 1 << 20
	exportEvalBytes    int64 = 8 << 30
)

// floorEvalRuns and floorEvalBudget bound the floor across one evaluation,
// whatever the number of read sets: at most floorEvalRuns go tool runs, each
// counted as it starts (the one `go env -json` read of the evaluator's tool,
// each `go env GOWORK` and each `go list`), and no run or export starts once
// floorEvalBudget of wall time has gone to floors, exports included (a run
// already started finishes within its own floorTimeout, an export within its
// byte caps).
var (
	floorEvalRuns   = 64
	floorEvalBudget = 10 * time.Minute
)

// exportTree writes the tree of commit to a new temporary directory through
// the coverage port's streamed tree listing, size check and streamed object
// read (`git cat-file --batch-check`, then `--batch`, replacement objects
// disabled). The listing stops at its own caps as it is read; the byte caps
// are checked against the listed sizes before any content is read; each
// link's object is read and its target checked before anything is written;
// each distinct file object is then read once and streamed to its first
// file, and a repeated object is copied from that file, so object content in
// memory is a copy buffer and the link targets, and the listing in memory is
// bounded by the listing caps. Every write stays under that
// directory: exportPlan refuses an entry path that is absolute, has an
// empty, `.`, `..` or `.git` component, or collides with another entry (the
// same path ignoring case, or one entry below another); every file is
// created fresh (O_EXCL), so nothing is written through an existing name;
// and symbolic links are created last, only with a relative target that has
// no `..` component, so no write ever passes through one and no link points
// out of the tree. Submodule entries are skipped. A refused tree is an
// error, and the directory is removed.
func exportTree(root, commit string) (string, error) {
	dir, _, err := exportTreeWithin(root, commit, exportMaxBytes)
	return dir, err
}

// exportTreeWithin is exportTree with maxBytes (at most exportMaxBytes) as
// the byte cap; it also returns the bytes the export may have written: the
// tree's total once writing starts, on success or failure, and 0 for a tree
// refused before any write.
func exportTreeWithin(root, commit string, maxBytes int64) (string, int64, error) {
	repo := coverageRepo{root: root}
	listed, over, ok := repo.listTree(commit, exportMaxEntries, exportMaxListBytes)
	switch over {
	case "entries":
		return "", 0, fmt.Errorf("the tree has more than %d entries, over the export cap of %d", exportMaxEntries, exportMaxEntries)
	case "bytes":
		return "", 0, fmt.Errorf("the tree's listing holds more than %d bytes, over the export cap", exportMaxListBytes)
	}
	if !ok {
		return "", 0, fmt.Errorf("the tree of %s could not be listed", commit)
	}
	ents, err := exportPlan(listed)
	if err != nil {
		return "", 0, err
	}
	var distinct []string
	seen := map[string]bool{}
	files := map[string][]string{} // oid -> the file paths it fills, in plan order
	isLink := map[string]bool{}
	for _, e := range ents {
		if !seen[e.OID] {
			seen[e.OID] = true
			distinct = append(distinct, e.OID)
		}
		if e.Mode == "120000" {
			isLink[e.OID] = true
		} else {
			files[e.OID] = append(files[e.OID], e.Path)
		}
	}
	sizes, ok := repo.ObjectSizes(distinct)
	if !ok {
		return "", 0, errors.New("git cat-file --batch-check failed")
	}
	var total, linkTotal int64
	for _, e := range ents {
		sz, found := sizes[e.OID]
		if !found || sz.Type != "blob" {
			return "", 0, fmt.Errorf("%s (%s) is not a readable blob", e.Path, e.OID)
		}
		if total += sz.Size; total > maxBytes {
			return "", 0, fmt.Errorf("the tree holds more than the export cap of %d bytes", maxBytes)
		}
		if e.Mode == "120000" {
			if linkTotal += sz.Size; linkTotal > exportMaxLinkBytes {
				return "", 0, fmt.Errorf("the tree's link targets hold more than the export cap of %d bytes", exportMaxLinkBytes)
			}
		}
	}
	var linkOIDs, fileOIDs []string
	for _, oid := range distinct {
		if isLink[oid] {
			linkOIDs = append(linkOIDs, oid)
		}
		if len(files[oid]) > 0 {
			fileOIDs = append(fileOIDs, oid)
		}
	}
	targets := map[string]string{} // oid -> link target
	err = repo.StreamObjects(linkOIDs, func(oid, typ string, size int64, body io.Reader) error {
		if typ != "blob" || size != sizes[oid].Size {
			return fmt.Errorf("%s changed between the size check and the read", oid)
		}
		var link bytes.Buffer
		n, err := io.Copy(&link, body)
		if err == nil && n != size {
			err = fmt.Errorf("%s: read %d of %d bytes", oid, n, size)
		}
		targets[oid] = link.String()
		return err
	})
	if err != nil {
		return "", 0, err
	}
	for _, e := range ents {
		if e.Mode == "120000" && !linkTargetInTree(targets[e.OID]) {
			return "", 0, fmt.Errorf("%s is a symbolic link whose target is absolute or climbs with ..", e.Path)
		}
	}
	dir, err := os.MkdirTemp("", "readset-floor-")
	if err != nil {
		return "", 0, err
	}
	fail := func(err error) (string, int64, error) {
		_ = os.RemoveAll(dir)
		return "", total, err
	}
	create := func(p string, mode os.FileMode) (*os.File, error) {
		dst := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		return os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	}
	modes := map[string]os.FileMode{}
	for _, e := range ents {
		modes[e.Path] = 0o644
		if e.Mode == "100755" {
			modes[e.Path] = 0o755
		}
	}
	err = repo.StreamObjects(fileOIDs, func(oid, typ string, size int64, body io.Reader) error {
		if typ != "blob" || size != sizes[oid].Size {
			return fmt.Errorf("%s changed between the size check and the read", oid)
		}
		ps := files[oid]
		f, err := create(ps[0], modes[ps[0]])
		if err != nil {
			return err
		}
		n, err := io.Copy(f, body)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err == nil && n != size {
			err = fmt.Errorf("%s: read %d of %d bytes", oid, n, size)
		}
		return err
	})
	if err != nil {
		return fail(err)
	}
	for _, ps := range files {
		for _, p := range ps[1:] {
			if err := copyExported(dir, ps[0], p, modes[p], create); err != nil {
				return fail(err)
			}
		}
	}
	for _, e := range ents {
		if e.Mode != "120000" {
			continue
		}
		target := targets[e.OID]
		dst := filepath.Join(dir, filepath.FromSlash(e.Path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fail(err)
		}
		if err := os.Symlink(target, dst); err != nil {
			return fail(err)
		}
	}
	return dir, total, nil
}

// copyExported copies the exported file from to a fresh file at p, both
// relative to dir; no link exists yet, so neither path passes through one.
func copyExported(dir, from, p string, mode os.FileMode, create func(string, os.FileMode) (*os.File, error)) error {
	src, err := os.Open(filepath.Join(dir, filepath.FromSlash(from)))
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := create(p, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	if cerr := dst.Close(); err == nil {
		err = cerr
	}
	return err
}

// exportPlan checks every listed entry's path and mode before anything is
// written and returns the entries to write (submodules dropped).
func exportPlan(listed []evaluation.TreeBlob) ([]evaluation.TreeBlob, error) {
	folded := map[string]string{} // case-folded path -> the entry's path
	var ents []evaluation.TreeBlob
	for _, e := range listed {
		if e.Mode == "160000" {
			continue // a submodule commit, not a blob
		}
		if e.Mode != "100644" && e.Mode != "100755" && e.Mode != "120000" {
			return nil, fmt.Errorf("%s has mode %s, which the export does not write", e.Path, e.Mode)
		}
		if !exportPathOK(e.Path) {
			return nil, fmt.Errorf("%q is not a plain relative path inside the tree", e.Path)
		}
		k := strings.ToLower(e.Path)
		if other, dup := folded[k]; dup {
			return nil, fmt.Errorf("%s and %s name the same file on a case-insensitive file system", other, e.Path)
		}
		folded[k] = e.Path
		ents = append(ents, e)
	}
	for _, e := range ents {
		for d := path.Dir(strings.ToLower(e.Path)); d != "."; d = path.Dir(d) {
			if other, ok := folded[d]; ok {
				return nil, fmt.Errorf("%s sits below %s, which is not a directory", e.Path, other)
			}
		}
	}
	return ents, nil
}

// exportPathOK reports whether p is a slash-separated relative path whose
// every component is a plain name: not empty, `.`, `..` or `.git`.
func exportPathOK(p string) bool {
	if p == "" || strings.ContainsAny(p, "\x00\\") || strings.HasPrefix(p, "/") || filepath.IsAbs(filepath.FromSlash(p)) {
		return false
	}
	for _, c := range strings.Split(p, "/") {
		if c == "" || c == "." || c == ".." || strings.EqualFold(c, ".git") {
			return false
		}
	}
	return true
}

// linkTargetInTree reports whether a symbolic link target is relative and
// has no `..` component, so resolving it can only descend from the link's
// own directory.
func linkTargetInTree(t string) bool {
	if t == "" || strings.ContainsAny(t, "\x00\\") || strings.HasPrefix(t, "/") || filepath.IsAbs(filepath.FromSlash(t)) {
		return false
	}
	for _, c := range strings.Split(t, "/") {
		if c == ".." {
			return false
		}
	}
	return true
}

// goValueFlags are the go build and test flags that take a separate value.
var goValueFlags = map[string]bool{
	"-C": true, "-tags": true, "-run": true, "-skip": true, "-count": true, "-timeout": true,
	"-bench": true, "-benchtime": true, "-cpu": true, "-parallel": true, "-o": true, "-p": true,
	"-ldflags": true, "-gcflags": true, "-asmflags": true, "-gccgoflags": true, "-mod": true,
	"-pkgdir": true, "-toolexec": true, "-coverpkg": true, "-covermode": true, "-coverprofile": true,
	"-exec": true, "-installsuffix": true, "-buildmode": true, "-compiler": true, "-pgo": true,
	"-cpuprofile": true, "-memprofile": true, "-blockprofile": true, "-mutexprofile": true,
	"-trace": true, "-outputdir": true, "-fuzz": true, "-fuzztime": true, "-shuffle": true,
	"-vet": true, "-f": true, "-reuse": true, "-modfile": true, "-overlay": true,
}

// floorFlagUse says what the floor does with one allow-listed go flag.
type floorFlagUse uint8

const (
	// flagIgnored never changes which in-tree files compile, so the floor
	// drops it: it does not reach the go tool.
	flagIgnored floorFlagUse = iota + 1
	// flagReproduced changes which files compile, so the floor passes its
	// checked value to `go list`.
	flagReproduced
)

// floorFlags is the allow-list of recorded go flags, by their one-dash name.
// Any flag not here makes the floor could-not-check: it may start a program,
// read a file the floor does not see, tolerate a broken package or change
// which files compile in a way the floor does not reproduce.
var floorFlags = map[string]floorFlagUse{
	"-tags": flagReproduced, "-mod": flagReproduced, "-race": flagReproduced,
	"-v": flagIgnored, "-x": flagIgnored, "-n": flagIgnored, "-a": flagIgnored, "-work": flagIgnored,
	"-count": flagIgnored, "-run": flagIgnored, "-skip": flagIgnored, "-timeout": flagIgnored,
	"-short": flagIgnored, "-failfast": flagIgnored, "-json": flagIgnored, "-bench": flagIgnored,
	"-benchtime": flagIgnored, "-benchmem": flagIgnored, "-cpu": flagIgnored, "-parallel": flagIgnored,
	"-p": flagIgnored, "-shuffle": flagIgnored, "-fullpath": flagIgnored, "-trimpath": flagIgnored,
	"-buildvcs": flagIgnored, "-cover": flagIgnored, "-covermode": flagIgnored, "-vet": flagIgnored,
	"-deps": flagIgnored, "-test": flagIgnored, "-f": flagIgnored,
}

var (
	// floorTagsRe is a -tags value the floor passes on: comma-separated names.
	floorTagsRe = regexp.MustCompile(`^[A-Za-z0-9_.,-]*$`)
	// floorFlagNameRe is a flag name the floor can name in a problem.
	floorFlagNameRe = regexp.MustCompile(`^-[A-Za-z][A-Za-z0-9_.-]*$`)
	// floorPlatformRe is a GOOS or GOARCH value the floor passes on.
	floorPlatformRe = regexp.MustCompile(`^[a-z0-9]+$`)
)

// floorPlan is what the floor reproduces of one recorded invocation.
type floorPlan struct {
	dir                string   // working directory, slash form, relative to the tree
	args               []string // `go list` arguments
	goos, goarch, cgo  string   // the recorded platform ("" = the evaluator's)
	gowork             string   // "off" or "" (the run's own go.work discovery)
	tags, mod          string
	race               bool
	pkgs               []string
	sawTags, sawModArg bool
}

// unreproduced is the problem for a recorded flag outside the allow-list.
func unreproduced(name string) string {
	if !floorFlagNameRe.MatchString(name) {
		return "the recorded go invocation carries a flag the floor cannot read, so it does not reproduce it"
	}
	return "the recorded go invocation uses " + name + ", which the floor does not reproduce"
}

// apply records one allow-listed flag.
func (p *floorPlan) apply(name, val string, hasVal bool) string {
	switch floorFlags[name] {
	case flagIgnored:
		return ""
	case flagReproduced:
	default:
		return unreproduced(name)
	}
	switch name {
	case "-tags":
		if !floorTagsRe.MatchString(val) {
			return "the recorded -tags value is not a comma-separated list of names, so the floor does not reproduce it"
		}
		p.tags = val
	case "-mod":
		if val != "readonly" && val != "vendor" {
			return "the recorded -mod value is not readonly or vendor, so the floor does not reproduce it"
		}
		p.mod = val
	case "-race":
		on := true
		if hasVal {
			b, err := strconv.ParseBool(val)
			if err != nil {
				return "the recorded -race value is not a boolean"
			}
			on = b
		}
		p.race = on
	}
	return ""
}

// splitFlag reads one command-line or GOFLAGS word as a flag: its one-dash
// name, its value and whether the word carried one. ok is false for a word
// with more than two leading dashes.
func splitFlag(word string) (name, val string, hasVal, ok bool) {
	name, val, hasVal = strings.Cut(word, "=")
	bare := strings.TrimPrefix(strings.TrimPrefix(name, "-"), "-")
	if strings.HasPrefix(bare, "-") || bare == "" {
		return "", "", false, false
	}
	return "-" + bare, val, hasVal, true
}

// floorRelPathOK reports whether p is a relative slash path with no `..`
// component ("" and "." are the toplevel).
func floorRelPathOK(p string) bool {
	if strings.ContainsAny(p, "\x00\\") || strings.HasPrefix(p, "/") || filepath.IsAbs(filepath.FromSlash(p)) {
		return false
	}
	for _, c := range strings.Split(p, "/") {
		if c == ".." {
			return false
		}
	}
	return true
}

// floorPkgArgOK reports whether a recorded package argument is one the floor
// passes on: a relative path, pattern or import path, never absolute, never
// climbing with `..`, never a version query (which would fetch a module).
func floorPkgArgOK(a string) bool {
	return a != "" && !strings.HasPrefix(a, "-") && !strings.ContainsAny(a, "@:") && floorRelPathOK(a)
}

// planFloor reads a recorded go invocation through the allow-list. Recorded
// GOFLAGS are read first and the command line after, as the go tool does, so
// a command-line value wins.
func planFloor(g evaluation.GoRun) (floorPlan, string) {
	p := floorPlan{dir: path.Clean("./" + filepath.ToSlash(g.Dir))}
	if !floorRelPathOK(g.Dir) {
		return p, "the recorded working directory is outside the tree"
	}
	switch g.GOWORK {
	case "", "off":
		p.gowork = g.GOWORK
	default:
		return p, "the recorded go invocation set GOWORK to a path, which the floor does not reproduce"
	}
	for _, v := range []string{g.GOOS, g.GOARCH} {
		if v != "" && !floorPlatformRe.MatchString(v) {
			return p, "the recorded GOOS or GOARCH is not a platform name"
		}
	}
	if g.CGOEnabled != "" && g.CGOEnabled != "0" && g.CGOEnabled != "1" {
		return p, "the recorded CGO_ENABLED is not 0 or 1"
	}
	p.goos, p.goarch, p.cgo = g.GOOS, g.GOARCH, g.CGOEnabled
	for _, w := range strings.Fields(g.GOFLAGS) {
		name, val, hasVal, ok := splitFlag(w)
		if !ok || !strings.HasPrefix(w, "-") {
			return p, "the recorded GOFLAGS carries a word that is not a flag, so the floor does not reproduce it"
		}
		if goValueFlags[name] && !hasVal {
			return p, "the recorded GOFLAGS carries " + name + " without a value"
		}
		if why := p.apply(name, val, hasVal); why != "" {
			return p, "GOFLAGS: " + why
		}
	}
	a := g.Args
	if len(a) < 2 {
		return p, "the recorded go invocation names no subcommand"
	}
	i := 1
	if name, val, hasVal, ok := splitFlag(a[i]); ok && strings.HasPrefix(a[i], "-") && name == "-C" {
		if !hasVal {
			if i+1 >= len(a) {
				return p, "the recorded -C has no directory"
			}
			val, i = a[i+1], i+1
		}
		if !floorRelPathOK(val) {
			return p, "the recorded -C directory is absolute or climbs with .., which the floor does not reproduce"
		}
		p.dir = path.Join(p.dir, filepath.ToSlash(val))
		i++
	}
	if i >= len(a) {
		return p, "the recorded go invocation names no subcommand"
	}
	sub := a[i]
	i++
	for ; i < len(a); i++ {
		arg := a[i]
		if arg == "-args" || arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") {
			name, val, hasVal, ok := splitFlag(arg)
			if !ok {
				return p, unreproduced("")
			}
			if floorFlags[name] != 0 && !hasVal && goValueFlags[name] {
				if i+1 >= len(a) {
					return p, "the recorded " + name + " has no value"
				}
				i++
				val, hasVal = a[i], true
			}
			if why := p.apply(name, val, hasVal); why != "" {
				return p, why
			}
			continue
		}
		if !floorPkgArgOK(arg) {
			return p, "a recorded package argument is absolute, climbs with .. or names a version, which the floor does not reproduce"
		}
		p.pkgs = append(p.pkgs, arg)
		if sub == "run" {
			// `go run` compiles its package, or every .go file named
			// together; the rest are the program's own arguments.
			for strings.HasSuffix(arg, ".go") && i+1 < len(a) && strings.HasSuffix(a[i+1], ".go") && floorPkgArgOK(a[i+1]) {
				i++
				p.pkgs = append(p.pkgs, a[i])
			}
			break
		}
	}
	p.args = []string{"list", "-deps", "-test", "-json"}
	if p.mod != "" {
		p.args = append(p.args, "-mod="+p.mod)
	}
	if p.tags != "" {
		p.args = append(p.args, "-tags="+p.tags)
	}
	if p.race {
		p.args = append(p.args, "-race")
	}
	p.args = append(p.args, p.pkgs...)
	return p, ""
}

// floorTimeout bounds each go tool run the floor starts; floorWaitDelay
// bounds the wait for its output pipes once it is killed.
var (
	floorTimeout   = 2 * time.Minute
	floorWaitDelay = 5 * time.Second
)

// floorTool is the evaluator's own go tool and the settings the floor takes
// from it.
type floorTool struct {
	goroot, modcache, goos, goarch, cgo string
}

// goToolPassThrough are the variables the one `go env` read of the
// evaluator's go tool sees from the caller: where its settings and module
// cache live. It runs in an empty scratch directory and reads no tree.
var goToolPassThrough = []string{"PATH", "HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "GOENV", "GOROOT", "GOPATH", "GOMODCACHE", "GOOS", "GOARCH", "CGO_ENABLED", "CC", "SYSTEMROOT", "TMPDIR"}

// goTool reads the evaluator's go tool once, the read charged to the
// evaluation's go tool runs through start: its GOROOT, module cache and
// default platform. The floor runs the same `go` the evaluator's PATH names
// (looked up by the process, never in the exported tree), pinned to that
// GOROOT with no toolchain switch.
func (s *readSetSource) goTool(start func() string) (*floorTool, string) {
	if s.tool != nil || s.toolWhy != "" {
		return s.tool, s.toolWhy
	}
	if why := start(); why != "" {
		return nil, why // not memoized: the budget refused this floor, not the tool
	}
	t, why := s.readGoTool()
	s.tool, s.toolWhy = t, why
	return t, why
}

func (s *readSetSource) readGoTool() (*floorTool, string) {
	scratch, err := os.MkdirTemp("", "readset-floor-env-")
	if err != nil {
		return nil, "the floor's scratch directory could not be made: " + err.Error()
	}
	s.scratch = scratch
	for _, d := range []string{"home", "tmp", "gopath", "cache"} {
		if err := os.Mkdir(filepath.Join(scratch, d), 0o700); err != nil {
			return nil, "the floor's scratch directory could not be made: " + err.Error()
		}
	}
	env := []string{"GOTOOLCHAIN=local", "GOFLAGS=", "GOWORK=off", "GO111MODULE=on"}
	for _, k := range goToolPassThrough {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), floorTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "env", "-json", "GOROOT", "GOMODCACHE", "GOOS", "GOARCH", "CGO_ENABLED")
	cmd.Dir, cmd.Env, cmd.WaitDelay = filepath.Join(scratch, "home"), env, floorWaitDelay
	stdout := &cappedOutput{max: floorMaxOutputBytes, stop: cancel}
	cmd.Stdout = stdout
	err = cmd.Run()
	out := stdout.buf.Bytes()
	if stdout.over {
		err = fmt.Errorf("it wrote more than the cap of %d bytes", floorMaxOutputBytes)
	}
	if err != nil {
		return nil, "the evaluator's go tool could not be read (go env failed: " + firstLine(err.Error()) + ")"
	}
	var v struct{ GOROOT, GOMODCACHE, GOOS, GOARCH, CGO_ENABLED string }
	if err := json.Unmarshal(out, &v); err != nil || !filepath.IsAbs(v.GOROOT) || !filepath.IsAbs(v.GOMODCACHE) {
		return nil, "the evaluator's go tool could not be read (go env gave no absolute GOROOT and GOMODCACHE)"
	}
	return &floorTool{goroot: v.GOROOT, modcache: v.GOMODCACHE, goos: v.GOOS, goarch: v.GOARCH, cgo: v.CGO_ENABLED}, ""
}

// floorEnv is the whole environment of a floor run: nothing is inherited.
// HOME, GOPATH, the build cache and the temporary directory are the run's
// own scratch; the module cache is the evaluator's, read with no network
// (GOPROXY=off, GOSUMDB=off, every VCS off) and no toolchain switch; GOENV
// and GOFLAGS are off, so no settings file or ambient flag reaches the tool;
// the platform is the recorded one, else the evaluator's.
func floorEnv(t *floorTool, scratch string, p floorPlan) []string {
	or := func(a, b string) string {
		if a != "" {
			return a
		}
		return b
	}
	env := []string{
		"PATH=" + filepath.Join(t.goroot, "bin"),
		"HOME=" + filepath.Join(scratch, "home"),
		"TMPDIR=" + filepath.Join(scratch, "tmp"),
		"GOROOT=" + t.goroot,
		"GOPATH=" + filepath.Join(scratch, "gopath"),
		"GOMODCACHE=" + t.modcache,
		"GOCACHE=" + filepath.Join(scratch, "cache"),
		"GOENV=off", "GOFLAGS=", "GO111MODULE=on", "GOTOOLCHAIN=local", "GOEXPERIMENT=",
		"GOPROXY=off", "GOSUMDB=off", "GOVCS=*:off", "GOPRIVATE=", "GONOPROXY=", "GONOSUMDB=", "GOINSECURE=",
		"GOOS=" + or(p.goos, t.goos), "GOARCH=" + or(p.goarch, t.goarch), "CGO_ENABLED=" + or(p.cgo, t.cgo),
	}
	if p.gowork == "off" {
		env = append(env, "GOWORK=off")
	}
	if runtime.GOOS == "windows" {
		env = append(env, "SYSTEMROOT="+os.Getenv("SYSTEMROOT"), "TEMP="+filepath.Join(scratch, "tmp"), "TMP="+filepath.Join(scratch, "tmp"), "USERPROFILE="+filepath.Join(scratch, "home"))
	}
	return env
}

// floorMaxOutputBytes caps what one go tool run of the floor may write to
// its standard output, read as it arrives: past it the run is killed and the
// floor is a problem. floorMaxStderrBytes is what is kept of its standard
// error, the rest discarded as it arrives.
var (
	floorMaxOutputBytes int64 = 64 << 20
	floorMaxStderrBytes int64 = 64 << 10
)

// cappedOutput holds at most max bytes written to it. Past max, a capped
// output (stop set) stops the run and refuses the write, and a kept prefix
// (stop nil) discards the rest.
type cappedOutput struct {
	buf  bytes.Buffer
	max  int64
	over bool
	stop func()
}

func (c *cappedOutput) Write(p []byte) (int, error) {
	if left := c.max - int64(c.buf.Len()); int64(len(p)) > left {
		c.over = true
		if c.stop != nil {
			c.stop()
			return 0, errors.New("output over the cap")
		}
		c.buf.Write(p[:max(left, 0)])
		return len(p), nil
	}
	return c.buf.Write(p)
}

// runFloorGo runs the evaluator's go tool once, bounded by floorTimeout and
// floorMaxOutputBytes, in dir with env, once start has charged the run. The
// problem names what failed, the tree path shown as `.`.
func runFloorGo(t *floorTool, tree, dir string, env []string, start func() string, what string, args ...string) ([]byte, string) {
	if why := start(); why != "" {
		return nil, why
	}
	ctx, cancel := context.WithTimeout(context.Background(), floorTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	// WaitDelay: a child that keeps the output pipes open past the timeout
	// is not waited for without end.
	cmd.Dir, cmd.Env, cmd.WaitDelay = dir, env, floorWaitDelay
	stdout := &cappedOutput{max: floorMaxOutputBytes, stop: cancel}
	stderr := &cappedOutput{max: floorMaxStderrBytes}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	out := stdout.buf.Bytes()
	if stdout.over {
		return nil, fmt.Sprintf("%s wrote more than the cap of %d bytes", what, floorMaxOutputBytes)
	}
	if ctx.Err() != nil {
		return nil, what + " did not finish within " + floorTimeout.String()
	}
	if err != nil {
		msg := firstLine(stderr.buf.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, what + " failed: " + strings.ReplaceAll(msg, tree, ".")
	}
	return out, ""
}

// listedPkg is the part of `go list -json` output the floor reads.
type listedPkg struct {
	ImportPath                                          string
	Dir                                                 string
	Standard, Incomplete                                bool
	Error                                               *struct{ Err string }
	DepsErrors                                          []struct{ Err string }
	GoFiles, CgoFiles, CFiles, CXXFiles, MFiles, HFiles []string
	FFiles, SFiles, SwigFiles, SwigCXXFiles, SysoFiles  []string
	IgnoredGoFiles, IgnoredOtherFiles                   []string
	TestGoFiles, XTestGoFiles                           []string
	EmbedFiles, TestEmbedFiles, XTestEmbedFiles         []string
	Module                                              *struct {
		GoMod string
		Main  bool
	}
}

// inRoots returns p relative to the first of roots that holds it.
func inRoots(roots []string, p string) (string, bool) {
	for _, r := range roots {
		if rel, err := filepath.Rel(r, p); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return filepath.ToSlash(rel), true
		}
	}
	return "", false
}

// withReal is dir and, when it differs, its symlink-resolved form.
func withReal(dir string) []string {
	out := []string{dir}
	if real, err := filepath.EvalSymlinks(dir); err == nil && real != dir {
		out = append(out, real)
	}
	return out
}

// runFloor runs the floor's `go list` in an exported tree and returns every
// listed file inside it, as repository paths. A floor that would be empty or
// partial is a problem, never a narrower floor: a package go list reports
// incomplete, a package compiled from outside the tree, the module cache and
// GOROOT, a main module whose go.mod is outside the tree, a go.work outside
// the tree, or no in-tree file at all. Each go tool run starts only once
// start has charged it.
func runFloor(t *floorTool, scratch, tree string, p floorPlan, start func() string) evaluation.GoFloor {
	roots := withReal(tree)
	dir := filepath.Join(tree, filepath.FromSlash(p.dir))
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return evaluation.GoFloor{Problem: "the recorded working directory is not in the witness tree"}
	}
	if _, ok := inRoots(roots, real); !ok {
		return evaluation.GoFloor{Problem: "the recorded working directory is outside the tree"}
	}
	env := floorEnv(t, scratch, p)
	seen := map[string]bool{}
	var files []string
	phantom := ""
	add := func(f string) bool {
		rel, ok := inRoots(roots, f)
		if !ok {
			return false
		}
		if _, err := os.Lstat(f); err != nil && phantom == "" {
			phantom = rel
		}
		if !seen[rel] {
			seen[rel] = true
			files = append(files, rel)
		}
		return true
	}
	own := withReal(scratch)
	if p.gowork != "off" {
		out, why := runFloorGo(t, tree, dir, env, start, "go env GOWORK", "env", "GOWORK")
		if why != "" {
			return evaluation.GoFloor{Problem: why}
		}
		if w := strings.TrimSpace(string(out)); w != "" && w != "off" {
			if !add(w) {
				return evaluation.GoFloor{Problem: "the run would use a go.work outside the witness tree"}
			}
			if _, err := os.Lstat(w + ".sum"); err == nil {
				add(w + ".sum")
			}
		}
	}
	out, why := runFloorGo(t, tree, dir, env, start, "go list", p.args...)
	if why != "" {
		return evaluation.GoFloor{Problem: why}
	}
	cache := withReal(t.modcache)
	inTreeFiles := 0
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var pkg listedPkg
		if err := dec.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return evaluation.GoFloor{Problem: "go list output could not be read: " + err.Error()}
		}
		if pkg.Error != nil || pkg.Incomplete || len(pkg.DepsErrors) > 0 {
			return evaluation.GoFloor{Problem: "go list reported package " + pkg.ImportPath + " incomplete, so the floor would be partial"}
		}
		if pkg.Module != nil && pkg.Module.GoMod != "" {
			if add(pkg.Module.GoMod) {
				sum := filepath.Join(filepath.Dir(pkg.Module.GoMod), "go.sum")
				if _, err := os.Lstat(sum); err == nil {
					add(sum)
				}
			} else if pkg.Module.Main {
				return evaluation.GoFloor{Problem: "the run's main module is outside the witness tree"}
			}
		}
		if _, ok := inRoots(roots, pkg.Dir); !ok {
			if _, cached := inRoots(cache, pkg.Dir); pkg.Standard || (cached && pkg.Module != nil && !pkg.Module.Main) {
				continue // GOROOT or a module cache dependency: not tree input
			}
			return evaluation.GoFloor{Problem: "package " + pkg.ImportPath + " compiles from outside the witness tree, so the floor would be partial"}
		}
		for _, list := range [][]string{pkg.GoFiles, pkg.CgoFiles, pkg.CFiles, pkg.CXXFiles, pkg.MFiles, pkg.HFiles,
			pkg.FFiles, pkg.SFiles, pkg.SwigFiles, pkg.SwigCXXFiles, pkg.SysoFiles, pkg.IgnoredGoFiles, pkg.IgnoredOtherFiles,
			pkg.TestGoFiles, pkg.XTestGoFiles, pkg.EmbedFiles, pkg.TestEmbedFiles, pkg.XTestEmbedFiles} {
			for _, f := range list {
				if !filepath.IsAbs(f) {
					f = filepath.Join(pkg.Dir, f)
				} else if _, in := inRoots(roots, f); !in {
					// An absolute name is a file go generated (the test
					// main) in the run's own build cache: not tree input.
					if _, gen := inRoots(own, f); gen {
						continue
					}
					return evaluation.GoFloor{Problem: "package " + pkg.ImportPath + " compiles a file from outside the witness tree, so the floor would be partial"}
				}
				if add(f) {
					inTreeFiles++
				}
			}
		}
	}
	if inTreeFiles == 0 {
		return evaluation.GoFloor{Problem: "go list named no file of the witness tree, so the floor would be empty"}
	}
	if phantom != "" {
		return evaluation.GoFloor{Problem: "go list named " + phantom + ", which the witness tree does not hold"}
	}
	return evaluation.GoFloor{Files: files}
}
