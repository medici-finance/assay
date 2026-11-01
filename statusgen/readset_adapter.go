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
	// exported is the bytes every export so far wrote; floorRuns and
	// floorSpent count the floor's go tool runs and their wall time, for the
	// per-evaluation caps.
	exported   int64
	floorRuns  int
	floorSpent time.Duration
}

// introRow is the commit that introduced a witness row with a given identity
// key, and that row's Runner cell there.
type introRow struct {
	commit string
	runner string
}

func newReadSetSource(root string) *readSetSource {
	return &readSetSource{root: root, exports: map[string]string{}, failed: map[string]string{}, intro: map[string]map[string]introRow{}, introWhy: map[string]string{}, floors: map[string]evaluation.GoFloor{}}
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
	s.exports, s.failed, s.floors = map[string]string{}, map[string]string{}, map[string]evaluation.GoFloor{}
	s.tool, s.toolWhy, s.scratch = nil, "", ""
	s.exported, s.floorRuns, s.floorSpent = 0, 0, 0
}

// observe fills w.ReadSet for a witness whose Runner cell carries a
// `reads sha256:` token. briefPath is the brief file on disk, briefRel its
// repository path, rowID/date the witness's `#` and Date cells. Nothing it
// reads is unbounded: the read-set file is read only when it is a regular
// file within readSetMaxBytes, the introduction's history within its caps
// (introductions), and the file's existence at the introduction commit is a
// batch check that reads no content. A refusal is the observation's Problem,
// which the library judges could-not-check.
func (s *readSetSource) observe(briefPath, briefRel, rowID string, w *evaluation.Witness) {
	digest, _, ok := evaluation.ReadSetToken(w.Runner)
	if !ok || digest == "" {
		return
	}
	o := &evaluation.ReadSetObservation{}
	w.ReadSet = o
	o.Content, o.Found, o.Problem = readSetFile(filepath.Join(filepath.Dir(briefPath), evaluation.ReadSetDir, digest))
	if o.Problem != "" {
		return
	}
	key := evaluation.WitnessRowKey(rowID, w.Command, w.Date, w.Runner)
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

// readSetFile reads the read-set file at p: absent is not found, and a file
// that is not a regular file or is over readSetMaxBytes is a problem, read
// no further than the cap.
func readSetFile(p string) (content []byte, found bool, problem string) {
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
// ambiguous history never binds a read set.
//
// The history is walked in process (gitcore PathHistory: replacement objects
// are never read). Each distinct brief blob, the parents' included, is sized
// first by one batch check that reads no content: one over
// introMaxBlobBytes, or all of them together over introMaxHistoryBytes, is
// the returned problem and nothing is read. The blobs are then streamed once
// through the coverage port and parsed line by line, never held whole. A
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
	rowsOf := map[string]map[string]string{} // blob -> key -> Runner cell (only a cell carrying a read-set token is kept)
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
			if _, seen := r[key]; !seen {
				runner := strings.TrimSpace(cells[witnessCellRunner])
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
	introducers := map[string][]introRow{}
	for _, t := range touches {
		held := rowsOf[t.blob]
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
			if !inParent {
				introducers[key] = append(introducers[key], introRow{commit: t.commit, runner: runner})
			}
		}
	}
	m := map[string]introRow{}
	for key, ins := range introducers {
		if len(ins) == 1 {
			m[key] = ins[0]
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
// invocation compiles.
func (s *readSetSource) goFloor(commit string, g evaluation.GoRun) evaluation.GoFloor {
	plan, problem := planFloor(g)
	if problem != "" {
		return evaluation.GoFloor{Problem: problem}
	}
	tool, why := s.goTool()
	if why != "" {
		return evaluation.GoFloor{Problem: why}
	}
	if s.floorRuns >= floorEvalRuns {
		return evaluation.GoFloor{Problem: fmt.Sprintf("the evaluation has already run the compile-input floor %d times, its cap", floorEvalRuns)}
	}
	if s.floorSpent >= floorEvalBudget {
		return evaluation.GoFloor{Problem: fmt.Sprintf("the evaluation has already spent %s on compile-input floors, its cap", floorEvalBudget)}
	}
	dir, why := s.export(commit)
	if why != "" {
		return evaluation.GoFloor{Problem: why}
	}
	start := time.Now()
	f := runFloor(tool, s.scratch, dir, plan)
	s.floorRuns++
	s.floorSpent += time.Since(start)
	return f
}

// export writes commit's tree, byte for byte as git stores it (no checkout
// filters, no export-ignore), to a fresh temporary directory.
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
	if err != nil {
		s.exports[commit], s.failed[commit] = "", "the witness tree could not be exported: "+err.Error()
		return "", s.failed[commit]
	}
	s.exports[commit] = d
	s.exported += n
	return d, ""
}

// exportMaxEntries, exportMaxListBytes, exportMaxBytes and
// exportMaxLinkBytes cap one exported witness tree. The listing is read as it
// arrives and stops past exportMaxEntries entries or exportMaxListBytes
// bytes, so the listing held in memory is bounded by them. exportMaxBytes
// bounds the bytes the export writes, every entry counted, a repeated object
// once per entry; the bytes read from git are each distinct object once,
// never more than that. exportMaxLinkBytes bounds the link targets, the only
// object content held in memory. exportEvalBytes bounds the bytes every
// export of one evaluation writes together, the floor's scratch disk.
var (
	exportMaxEntries         = 200000
	exportMaxListBytes int64 = 64 << 20
	exportMaxBytes     int64 = 4 << 30
	exportMaxLinkBytes int64 = 1 << 20
	exportEvalBytes    int64 = 8 << 30
)

// floorEvalRuns and floorEvalBudget bound the floor across one evaluation,
// whatever the number of read sets: at most floorEvalRuns go tool runs, and
// none starts once floorEvalBudget of wall time has gone to floor runs (one
// run already started finishes within its own floorTimeout).
var (
	floorEvalRuns   = 64
	floorEvalBudget = 10 * time.Minute
)

// exportTree writes the tree of commit to a new temporary directory through
// the coverage port's streamed tree listing, size check and streamed object
// read (`git cat-file --batch-check`, then `--batch`, replacement objects
// disabled). The listing stops at its own caps as it is read; the byte caps
// are checked against the listed sizes before any content is read; each
// distinct object is then read once and streamed to its first file, and a
// repeated object is copied from that file, so object content in memory is a
// copy buffer and the link targets, and the listing in memory is bounded by
// the listing caps. Every write stays under that
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
// the byte cap; it also returns the bytes the export wrote.
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
	dir, err := os.MkdirTemp("", "readset-floor-")
	if err != nil {
		return "", 0, err
	}
	fail := func(err error) (string, int64, error) {
		_ = os.RemoveAll(dir)
		return "", 0, err
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
	targets := map[string]string{} // oid -> link target
	err = repo.StreamObjects(distinct, func(oid, typ string, size int64, body io.Reader) error {
		if typ != "blob" || size != sizes[oid].Size {
			return fmt.Errorf("%s changed between the size check and the read", oid)
		}
		var link bytes.Buffer
		var sink io.Writer = io.Discard
		var f *os.File
		if ps := files[oid]; len(ps) > 0 {
			var err error
			if f, err = create(ps[0], modes[ps[0]]); err != nil {
				return err
			}
			sink = f
		}
		if isLink[oid] {
			sink = io.MultiWriter(sink, &link)
		}
		n, err := io.Copy(sink, body)
		if f != nil {
			if cerr := f.Close(); err == nil {
				err = cerr
			}
		}
		if err == nil && n != size {
			err = fmt.Errorf("%s: read %d of %d bytes", oid, n, size)
		}
		if isLink[oid] {
			targets[oid] = link.String()
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
		if !linkTargetInTree(target) {
			return fail(fmt.Errorf("%s is a symbolic link whose target is absolute or climbs with ..", e.Path))
		}
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

// goTool reads the evaluator's go tool once: its GOROOT, module cache and
// default platform. The floor runs the same `go` the evaluator's PATH names
// (looked up by the process, never in the exported tree), pinned to that
// GOROOT with no toolchain switch.
func (s *readSetSource) goTool() (*floorTool, string) {
	if s.tool != nil || s.toolWhy != "" {
		return s.tool, s.toolWhy
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
	out, err := cmd.Output()
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

// runFloorGo runs the evaluator's go tool once, bounded by floorTimeout,
// in dir with env. The problem names what failed, the tree path shown as `.`.
func runFloorGo(t *floorTool, tree, dir string, env []string, what string, args ...string) ([]byte, string) {
	ctx, cancel := context.WithTimeout(context.Background(), floorTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	// WaitDelay: a child that keeps the output pipes open past the timeout
	// is not waited for without end.
	cmd.Dir, cmd.Env, cmd.WaitDelay = dir, env, floorWaitDelay
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, what + " did not finish within " + floorTimeout.String()
	}
	if err != nil {
		msg := firstLine(stderr.String())
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
// the tree, or no in-tree file at all.
func runFloor(t *floorTool, scratch, tree string, p floorPlan) evaluation.GoFloor {
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
		out, why := runFloorGo(t, tree, dir, env, "go env GOWORK", "env", "GOWORK")
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
	out, why := runFloorGo(t, tree, dir, env, "go list", p.args...)
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
