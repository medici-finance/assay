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
	floors  map[string]evaluation.GoFloor // GoFloorKey -> the floor computed for it
	// tool is the evaluator's go tool, read once on first use; toolWhy says
	// why it could not be read. scratch holds the floor runs' HOME, GOPATH,
	// build cache and temporary directory.
	tool    *floorTool
	toolWhy string
	scratch string
}

// introRow is the commit that introduced a witness row with a given identity
// key, and that row's Runner cell there.
type introRow struct {
	commit string
	runner string
}

func newReadSetSource(root string) *readSetSource {
	return &readSetSource{root: root, exports: map[string]string{}, failed: map[string]string{}, intro: map[string]map[string]introRow{}, floors: map[string]evaluation.GoFloor{}}
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
}

// observe fills w.ReadSet for a witness whose Runner cell carries a
// `reads sha256:` token. briefPath is the brief file on disk, briefRel its
// repository path, rowID/date the witness's `#` and Date cells.
func (s *readSetSource) observe(briefPath, briefRel, rowID string, w *evaluation.Witness) {
	digest, _, ok := evaluation.ReadSetToken(w.Runner)
	if !ok || digest == "" {
		return
	}
	o := &evaluation.ReadSetObservation{}
	w.ReadSet = o
	if b, err := os.ReadFile(filepath.Join(filepath.Dir(briefPath), evaluation.ReadSetDir, digest)); err == nil {
		o.Content, o.Found = b, true
	}
	key := evaluation.WitnessRowKey(rowID, w.Command, w.Date, w.Runner)
	in, found := s.introductions(briefRel)[key]
	if !found {
		return
	}
	o.IntroFound, o.IntroRunner = true, in.runner
	spec := in.commit + ":" + evaluation.ReadSetPath(filepath.ToSlash(filepath.Dir(briefRel)), digest)
	got, ok := coverageRepo{root: s.root}.CatObjects([]string{spec})
	_, o.IntroFile = got[spec]
	o.IntroFile = o.IntroFile && ok
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
// are never read) and each distinct brief blob, the parents' included, is read
// once through the coverage port's batch object read. A repository whose
// profile the in-process reader does not cover (a shallow clone, a native
// discovery or config environment), or a blob that cannot be read, yields no
// introduction, so every read set there is ignored: a history this reader
// cannot see never binds a read set.
func (s *readSetSource) introductions(briefRel string) map[string]introRow {
	if m, ok := s.intro[briefRel]; ok {
		return m
	}
	none := map[string]introRow{}
	s.intro[briefRel] = none
	read := gitCoreFor(s.root)
	if read.err != nil || read.native != "" {
		return none
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
		return none
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
	objs, ok := coverageRepo{root: s.root}.CatObjects(blobs)
	if !ok {
		return none
	}
	rowsOf := map[string]map[string]string{} // blob -> key -> Runner cell
	rows := func(blob string) (map[string]string, bool) {
		if r, done := rowsOf[blob]; done {
			return r, r != nil
		}
		obj, found := objs[blob]
		if !found || obj.Type != "blob" {
			rowsOf[blob] = nil
			return nil, false
		}
		r := map[string]string{}
		for _, line := range strings.Split(string(obj.Content), "\n") {
			cells := witnessCells(line)
			if cells == nil {
				continue
			}
			key := evaluation.WitnessRowKey(cells[0], witnessCommandOf(line), cells[witnessCellDate], cells[witnessCellRunner])
			if _, seen := r[key]; !seen {
				r[key] = strings.TrimSpace(cells[witnessCellRunner])
			}
		}
		rowsOf[blob] = r
		return r, true
	}
	introducers := map[string][]introRow{}
	for _, t := range touches {
		held, ok := rows(t.blob)
		if !ok {
			return none
		}
		var parents []map[string]string
		for _, p := range t.parents {
			if p == "" {
				continue // the brief is absent in that parent
			}
			pr, ok := rows(p)
			if !ok {
				return none
			}
			parents = append(parents, pr)
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
	return m
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
	dir, why := s.export(commit)
	if why != "" {
		return evaluation.GoFloor{Problem: why}
	}
	return runFloor(tool, s.scratch, dir, plan)
}

// export writes commit's tree, byte for byte as git stores it (no checkout
// filters, no export-ignore), to a fresh temporary directory.
func (s *readSetSource) export(commit string) (string, string) {
	if d, ok := s.exports[commit]; ok {
		return d, s.failed[commit]
	}
	d, err := exportTree(s.root, commit)
	if err != nil {
		s.exports[commit], s.failed[commit] = "", "the witness tree could not be exported: "+err.Error()
		return "", s.failed[commit]
	}
	s.exports[commit] = d
	return d, ""
}

// exportMaxEntries and exportMaxBytes cap one exported witness tree.
var (
	exportMaxEntries       = 200000
	exportMaxBytes   int64 = 4 << 30
)

// exportTree writes the tree of commit to a new temporary directory through
// the coverage port's tree listing and batch object read (`git ls-tree` and
// `git cat-file --batch`, replacement objects disabled). Every write stays
// under that directory: exportPlan refuses an entry path that is absolute,
// has an empty, `.`, `..` or `.git` component, or collides with another
// entry (the same path ignoring case, or one entry below another); every file
// is created fresh (O_EXCL), so nothing is written through an existing name;
// and symbolic links are created last, only with a relative target that has
// no `..` component, so no write ever passes through one and no link points
// out of the tree. Submodule entries are skipped. A refused tree is an
// error, and the directory is removed.
func exportTree(root, commit string) (string, error) {
	repo := coverageRepo{root: root}
	listed, ok := repo.ListTree(commit)
	if !ok {
		return "", fmt.Errorf("git ls-tree %s failed", commit)
	}
	if len(listed) > exportMaxEntries {
		return "", fmt.Errorf("the tree has %d entries, over the export cap of %d", len(listed), exportMaxEntries)
	}
	ents, err := exportPlan(listed)
	if err != nil {
		return "", err
	}
	oids := make([]string, 0, len(ents))
	for _, e := range ents {
		oids = append(oids, e.OID)
	}
	objs, ok := repo.CatObjects(oids)
	if !ok {
		return "", errors.New("git cat-file --batch failed")
	}
	dir, err := os.MkdirTemp("", "readset-floor-")
	if err != nil {
		return "", err
	}
	fail := func(err error) (string, error) {
		_ = os.RemoveAll(dir)
		return "", err
	}
	var written int64
	var links []evaluation.TreeBlob
	for _, e := range ents {
		obj, found := objs[e.OID]
		if !found || obj.Type != "blob" {
			return fail(fmt.Errorf("git cat-file --batch: %s (%s) is not a readable blob", e.Path, e.OID))
		}
		if written += int64(len(obj.Content)); written > exportMaxBytes {
			return fail(fmt.Errorf("the tree holds more than the export cap of %d bytes", exportMaxBytes))
		}
		if e.Mode == "120000" {
			links = append(links, e)
			continue
		}
		dst := filepath.Join(dir, filepath.FromSlash(e.Path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fail(err)
		}
		mode := os.FileMode(0o644)
		if e.Mode == "100755" {
			mode = 0o755
		}
		f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return fail(err)
		}
		_, werr := f.Write(obj.Content)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return fail(werr)
		}
	}
	for _, e := range links {
		target := string(objs[e.OID].Content)
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
	return dir, nil
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

// floorTimeout bounds each go tool run the floor starts.
var floorTimeout = 2 * time.Minute

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
	cmd.Dir, cmd.Env = filepath.Join(scratch, "home"), env
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
	cmd.Dir, cmd.Env = dir, env
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
	add := func(f string) bool {
		rel, ok := inRoots(roots, f)
		if ok && !seen[rel] {
			seen[rel] = true
			files = append(files, rel)
		}
		return ok
	}
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
				if add(filepath.Join(pkg.Dir, f)) {
					inTreeFiles++
				}
			}
		}
	}
	if inTreeFiles == 0 {
		return evaluation.GoFloor{Problem: "go list named no file of the witness tree, so the floor would be empty"}
	}
	return evaluation.GoFloor{Files: files}
}
