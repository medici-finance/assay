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

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// readSetSource observes read sets for one evaluation run. Tree exports are
// shared by every floor computed at the same commit and removed by close.
type readSetSource struct {
	root    string
	exports map[string]string // full commit -> exported tree dir ("" when the export failed)
	failed  map[string]string // full commit -> why the export failed
	intro   map[string]map[string]introRow
}

// introRow is the first commit that holds a witness row with a given identity
// key, that row's Runner cell, and the commit itself.
type introRow struct {
	commit string
	runner string
}

func newReadSetSource(root string) *readSetSource {
	return &readSetSource{root: root, exports: map[string]string{}, failed: map[string]string{}, intro: map[string]map[string]introRow{}}
}

// close removes every exported tree.
func (s *readSetSource) close() {
	if s == nil {
		return
	}
	for _, d := range s.exports {
		if d != "" {
			_ = os.RemoveAll(d)
		}
	}
	s.exports, s.failed = map[string]string{}, map[string]string{}
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
	file := evaluation.ReadSetPath(filepath.ToSlash(filepath.Dir(briefRel)), digest)
	o.IntroFile = coverageGit(s.root, "cat-file", "-e", in.commit+":"+file).Run() == nil
}

// introductions maps each witness row identity key in the brief's history to
// the earliest commit (first-parent or not: every commit touching the file)
// whose brief file holds it, with that commit's Runner cell.
func (s *readSetSource) introductions(briefRel string) map[string]introRow {
	if m, ok := s.intro[briefRel]; ok {
		return m
	}
	m := map[string]introRow{}
	s.intro[briefRel] = m
	out, err := coverageGit(s.root, "log", "--reverse", "--format=%H", "--", briefRel).Output()
	if err != nil {
		return m
	}
	for _, c := range strings.Fields(string(out)) {
		body, err := coverageGit(s.root, "show", c+":"+briefRel).Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(body), "\n") {
			cells := witnessCells(line)
			if cells == nil {
				continue
			}
			key := evaluation.WitnessRowKey(cells[0], witnessCommandOf(line), cells[witnessCellDate], cells[witnessCellRunner])
			if _, seen := m[key]; !seen {
				m[key] = introRow{commit: c, runner: strings.TrimSpace(cells[witnessCellRunner])}
			}
		}
	}
	return m
}

// floors computes the go compile-input floor for every build-family `go-run`
// entry of every read set observed on the briefs, keyed as the library
// reads them (GoFloorKey of the header's commit and the entry line).
func (s *readSetSource) floors(briefs []evaluation.BriefObservation) map[string]evaluation.GoFloor {
	out := map[string]evaluation.GoFloor{}
	for _, b := range briefs {
		for _, ws := range b.Witnesses {
			for _, w := range ws {
				if w.ReadSet == nil || !w.ReadSet.Found {
					continue
				}
				h, entries, err := evaluation.ParseReadSet(w.ReadSet.Content)
				if err != nil {
					continue
				}
				for _, e := range entries {
					g, ok := evaluation.ParseGoRun(e)
					if !ok || !g.BuildFamily() {
						continue
					}
					k := evaluation.GoFloorKey(h.Commit, e)
					if _, done := out[k]; !done {
						out[k] = s.goFloor(h.Commit, g)
					}
				}
			}
		}
	}
	return out
}

// goFloor exports the commit's tree and lists, through the go tool, every
// in-tree file the recorded invocation compiles.
func (s *readSetSource) goFloor(commit string, g evaluation.GoRun) evaluation.GoFloor {
	dir, why := s.export(commit)
	if why != "" {
		return evaluation.GoFloor{Problem: why}
	}
	return goListFloor(dir, g)
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

// exportTree writes the tree of commit to a new temporary directory using
// `git ls-tree` and `git cat-file --batch`. Symbolic links are recreated;
// submodule entries are skipped.
func exportTree(root, commit string) (string, error) {
	ls, err := coverageGit(root, "ls-tree", "-r", "-z", "--full-tree", commit).Output()
	if err != nil {
		return "", fmt.Errorf("git ls-tree %s: %w", commit, err)
	}
	type ent struct{ mode, sha, path string }
	var ents []ent
	for _, rec := range strings.Split(strings.TrimSuffix(string(ls), "\x00"), "\x00") {
		meta, p, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 3 || f[1] != "blob" {
			continue
		}
		ents = append(ents, ent{f[0], f[2], p})
	}
	dir, err := os.MkdirTemp("", "readset-floor-")
	if err != nil {
		return "", err
	}
	cmd := coverageGit(root, "cat-file", "--batch")
	var in bytes.Buffer
	for _, e := range ents {
		in.WriteString(e.sha + "\n")
	}
	cmd.Stdin = &in
	raw, err := cmd.Output()
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("git cat-file --batch: %w", err)
	}
	r := bufio.NewReader(bytes.NewReader(raw))
	for _, e := range ents {
		head, err := r.ReadString('\n')
		f := strings.Fields(head)
		if err != nil || len(f) != 3 {
			_ = os.RemoveAll(dir)
			return "", errors.New("short git cat-file --batch output")
		}
		n, _ := strconv.Atoi(f[2])
		body := make([]byte, n+1)
		if _, err := io.ReadFull(r, body); err != nil {
			_ = os.RemoveAll(dir)
			return "", err
		}
		dst := filepath.Join(dir, filepath.FromSlash(e.path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			_ = os.RemoveAll(dir)
			return "", err
		}
		switch e.mode {
		case "120000":
			err = os.Symlink(string(body[:n]), dst)
		case "100755":
			err = os.WriteFile(dst, body[:n], 0o755)
		default:
			err = os.WriteFile(dst, body[:n], 0o644)
		}
		if err != nil {
			_ = os.RemoveAll(dir)
			return "", err
		}
	}
	return dir, nil
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
	"-vet": true, "-f": true, "-reuse": true,
}

// goListArgs maps a recorded go invocation onto `go list -deps -test -json`
// arguments: its -C directory, its -tags value and its package arguments.
// A -modfile or -overlay the floor cannot reproduce is a problem.
func goListArgs(g evaluation.GoRun) (args []string, problem string) {
	a := g.Args
	if len(a) < 2 {
		return nil, "the recorded go invocation names no subcommand"
	}
	i := 1
	var chdir string
	if v, ok := strings.CutPrefix(a[i], "-C="); ok {
		chdir, i = v, i+1
	} else if a[i] == "-C" && i+1 < len(a) {
		chdir, i = a[i+1], i+2
	}
	sub := g.Subcommand()
	i++ // the subcommand
	var tags string
	var pkgs []string
	for ; i < len(a); i++ {
		arg := a[i]
		if arg == "-args" || arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "-") {
			name, val, hasVal := strings.Cut(strings.TrimPrefix(arg, "-"), "=")
			name = "-" + strings.TrimPrefix(name, "-")
			if name == "-modfile" || name == "-overlay" {
				return nil, "the recorded go invocation uses " + name + ", which the floor does not reproduce"
			}
			if !hasVal && goValueFlags[name] && i+1 < len(a) {
				i++
				val = a[i]
			}
			if name == "-tags" {
				tags = val
			}
			continue
		}
		pkgs = append(pkgs, arg)
		if sub == "run" {
			break // the rest are the program's own arguments
		}
	}
	if chdir != "" {
		args = append(args, "-C", chdir)
	}
	args = append(args, "list", "-deps", "-test", "-json")
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	return append(args, pkgs...), ""
}

// goListEnv is the floor's environment: the recorded GOFLAGS plus
// -mod=readonly, the recorded platform, and no network or toolchain switch.
func goListEnv(g evaluation.GoRun) []string {
	drop := map[string]bool{"GOFLAGS": true, "GOWORK": true, "GOOS": true, "GOARCH": true, "CGO_ENABLED": true, "GOPROXY": true, "GOTOOLCHAIN": true}
	var env []string
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); !drop[k] {
			env = append(env, kv)
		}
	}
	env = append(env, "GOFLAGS="+strings.TrimSpace(g.GOFLAGS+" -mod=readonly"), "GOPROXY=off", "GOTOOLCHAIN=local")
	for k, v := range map[string]string{"GOWORK": g.GOWORK, "GOOS": g.GOOS, "GOARCH": g.GOARCH, "CGO_ENABLED": g.CGOEnabled} {
		if v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// listedPkg is the part of `go list -json` output the floor reads.
type listedPkg struct {
	Dir                                                 string
	GoFiles, CgoFiles, CFiles, CXXFiles, MFiles, HFiles []string
	FFiles, SFiles, SwigFiles, SwigCXXFiles, SysoFiles  []string
	IgnoredGoFiles, IgnoredOtherFiles                   []string
	TestGoFiles, XTestGoFiles                           []string
	EmbedFiles, TestEmbedFiles, XTestEmbedFiles         []string
	Module                                              *struct{ GoMod string }
}

// goListFloor runs the floor's `go list` in an exported tree and returns
// every listed file inside it, as repository paths.
func goListFloor(tree string, g evaluation.GoRun) evaluation.GoFloor {
	if g.GOWORK != "" && g.GOWORK != "off" {
		return evaluation.GoFloor{Problem: "the recorded go invocation set GOWORK to a path, which the floor does not reproduce"}
	}
	for _, f := range strings.Fields(g.GOFLAGS) {
		if strings.HasPrefix(f, "-modfile") || strings.HasPrefix(f, "-overlay") {
			return evaluation.GoFloor{Problem: "the recorded GOFLAGS carries " + f + ", which the floor does not reproduce"}
		}
	}
	args, problem := goListArgs(g)
	if problem != "" {
		return evaluation.GoFloor{Problem: problem}
	}
	dir := filepath.Join(tree, filepath.FromSlash(g.Dir))
	if rel, err := filepath.Rel(tree, dir); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return evaluation.GoFloor{Problem: "the recorded working directory is outside the tree"}
	}
	cmd := exec.Command("go", args...)
	cmd.Dir, cmd.Env = dir, goListEnv(g)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if i := strings.IndexByte(msg, '\n'); i >= 0 {
			msg = msg[:i]
		}
		return evaluation.GoFloor{Problem: "go list failed: " + msg}
	}
	roots := []string{tree}
	if real, err := filepath.EvalSymlinks(tree); err == nil && real != tree {
		roots = append(roots, real)
	}
	inTree := func(p string) (string, bool) {
		for _, r := range roots {
			if rel, err := filepath.Rel(r, p); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
				return filepath.ToSlash(rel), true
			}
		}
		return "", false
	}
	seen := map[string]bool{}
	var files []string
	add := func(p string) {
		if rel, ok := inTree(p); ok && !seen[rel] {
			seen[rel] = true
			files = append(files, rel)
		}
	}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p listedPkg
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			return evaluation.GoFloor{Problem: "go list output could not be read: " + err.Error()}
		}
		if p.Module != nil && p.Module.GoMod != "" {
			add(p.Module.GoMod)
			sum := filepath.Join(filepath.Dir(p.Module.GoMod), "go.sum")
			if _, err := os.Lstat(sum); err == nil {
				add(sum)
			}
		}
		if _, ok := inTree(p.Dir); !ok {
			continue
		}
		for _, list := range [][]string{p.GoFiles, p.CgoFiles, p.CFiles, p.CXXFiles, p.MFiles, p.HFiles,
			p.FFiles, p.SFiles, p.SwigFiles, p.SwigCXXFiles, p.SysoFiles, p.IgnoredGoFiles, p.IgnoredOtherFiles,
			p.TestGoFiles, p.XTestGoFiles, p.EmbedFiles, p.TestEmbedFiles, p.XTestEmbedFiles} {
			for _, f := range list {
				add(filepath.Join(p.Dir, f))
			}
		}
	}
	if g.GOWORK != "off" {
		envArgs := []string{"env", "GOWORK"}
		if args[0] == "-C" {
			envArgs = append([]string{"-C", args[1]}, envArgs...)
		}
		env := exec.Command("go", envArgs...)
		env.Dir, env.Env = dir, goListEnv(g)
		if w, err := env.Output(); err == nil {
			if p := strings.TrimSpace(string(w)); p != "" && p != "off" {
				add(p)
				if _, err := os.Lstat(p + ".sum"); err == nil {
					add(p + ".sum")
				}
			}
		}
	}
	return evaluation.GoFloor{Files: files}
}
