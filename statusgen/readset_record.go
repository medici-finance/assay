package main

// Read-set recorder, platform-neutral half (build-less-brittle/15, Task
// steps 2 and 3). The Linux tracer (readset_trace_linux.go) turns a row's
// system calls into a traceLog; this file classifies that log into read-set
// entries, or into the could-not-check reason that keeps the witness's
// whole-tree scope, and writes the file. It has no build constraint, so every
// rule here is tested on every host.
//
// Fail closed throughout: anything this file cannot place is a reason, and a
// reason means no read set is written. An empty set is never written.

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/statusgen/evaluation"
)

// traceAccess is how a traced process touched a path.
type traceAccess int

const (
	accRead traceAccess = iota + 1 // opened for reading, mapped, or a link's source
	accStat                        // stat-ed, probed, or written without being read
	accList                        // directory listed (getdents)
	accExec                        // executed
)

// traceEvent is one classified access: Path is absolute and symlink-free;
// Links are the in-path symbolic links the lexical path went through
// (absolute, unresolved), each of which is recorded too. Exe is the reading
// process's own executable, which the GOCACHE/GOMODCACHE rule needs.
type traceEvent struct {
	Kind   traceAccess
	Path   string
	Links  []string
	Absent bool
	Exe    string
}

// traceGoRun is one exec of the go tool, as the tracer saw it.
type traceGoRun struct {
	Cwd  string
	Args []string
	Env  map[string]string
}

// traceLog is everything the tracer observed for one row.
type traceLog struct {
	Events  []traceEvent
	Created map[string]bool // paths a traced process created in this row
	GoRuns  []traceGoRun
	GitDirs []string     // GIT_DIR / GIT_COMMON_DIR values a traced process set
	Pids    map[int]bool // every traced process (tgid)
	Denied  string       // the first default-deny reason the tracer met
}

// recordEnv is the per-session context the classification needs.
type recordEnv struct {
	Top     string   // the tree's toplevel, absolute and symlink-free
	GitDirs []string // git dir and common dir at session start
	// StateDir is the worktree's own git dir, under which finish writes the
	// read-set files (evaluation.ReadSetStateDir), outside the worktree.
	StateDir   string
	GOROOT     string
	GOMODCACHE string
	GOCACHE    string
	// GOTELEMETRYDIR holds the go tool's own usage counters, which it
	// rewrites on every run: toolchain state, like GOCACHE.
	GOTELEMETRYDIR string
	// TempRoot is the row's temporary-directory root ($TMPDIR, else /tmp).
	// The go tool opens it (never lists it) to make its work directory.
	TempRoot  string
	Home      string
	GOOS      string
	GOARCH    string
	Start     time.Time       // when the verify session began
	Files     map[string]bool // tracked files at the witness commit
	Dirs      map[string]bool // directories holding a tracked file
	ToolNames map[string]bool // main-package dir names and executable file names in the tree
	// ModTime is an out-of-tree file's modification time (os.Lstat); a test
	// seam.
	ModTime func(string) (time.Time, bool)
}

// Reasons the recorder writes after `could-not-check:`. They carry no
// absolute host path: the token lands in a public Evidence row.
const (
	reasonEmptyReadSet  = "the row recorded no in-tree input"
	reasonOutOfTreeRead = "the row read an out-of-tree file outside the environment set"
	reasonGoOutsideTree = "the go tool ran outside the tree"
)

func reasonUntracked(rel string) string {
	return "the row read an in-tree path untracked at the witness commit (" + rel + ")"
}

func reasonForeignExec(name string) string {
	return "the row executed an out-of-tree binary it did not build (" + name + ")"
}

// under reports whether p is dir or beneath it.
func pathUnder(dir, p string) bool {
	if dir == "" {
		return false
	}
	return p == dir || strings.HasPrefix(p, strings.TrimSuffix(dir, "/")+"/")
}

// inTree is p's tree-relative path when p is in the tree.
func (e *recordEnv) inTree(p string) (string, bool) {
	if !pathUnder(e.Top, p) {
		return "", false
	}
	if p == e.Top {
		return ".", true
	}
	return strings.TrimPrefix(p, strings.TrimSuffix(e.Top, "/")+"/"), true
}

// toolchain reports whether exe is one of the go toolchain's own binaries:
// $GOROOT/bin/go, $GOROOT/bin/gofmt, or a file in $GOROOT/pkg/tool/<goos>_<goarch>/.
func (e *recordEnv) toolchain(exe string) bool {
	if e.GOROOT == "" || exe == "" {
		return false
	}
	switch exe {
	case filepath.Join(e.GOROOT, "bin", "go"), filepath.Join(e.GOROOT, "bin", "gofmt"):
		return true
	}
	return filepath.Dir(exe) == filepath.Join(e.GOROOT, "pkg", "tool", e.GOOS+"_"+e.GOARCH)
}

// unmodified reports whether p has not changed since the session began.
func (e *recordEnv) unmodified(p string) bool {
	if e.ModTime == nil {
		return false
	}
	t, ok := e.ModTime(p)
	return ok && !t.After(e.Start)
}

// distroPath reports whether p is under a distribution-managed system path
// the environment set names. /usr/local is never one.
func distroPath(p string) bool {
	if pathUnder("/usr/local", p) {
		return false
	}
	for _, d := range []string{"/usr", "/bin", "/sbin", "/etc", "/dev", "/sys"} {
		if pathUnder(d, p) {
			return true
		}
	}
	first := strings.SplitN(strings.TrimPrefix(p, "/"), "/", 2)[0]
	return strings.HasPrefix(first, "lib")
}

// execPrefix reports whether p is under a prefix an out-of-tree exec may come
// from: /usr/bin, /usr/sbin, /usr/lib*, /usr/libexec, /bin, /sbin.
func execPrefix(p string) bool {
	for _, d := range []string{"/usr/bin", "/usr/sbin", "/usr/libexec", "/bin", "/sbin"} {
		if pathUnder(d, p) {
			return true
		}
	}
	if rest, ok := strings.CutPrefix(p, "/usr/"); ok {
		return strings.HasPrefix(strings.SplitN(rest, "/", 2)[0], "lib")
	}
	return false
}

// procOfTraced reports whether p is a /proc entry of a traced process, or
// one of the kernel's own (non-process) /proc files such as /proc/filesystems
// or /proc/sys, which hold kernel state as /sys does, never tree content.
func procOfTraced(p string, pids map[int]bool) bool {
	rest, ok := strings.CutPrefix(p, "/proc/")
	if !ok {
		return false
	}
	var pid int
	n := strings.SplitN(rest, "/", 2)[0]
	if n == "" {
		return false
	}
	if n[0] < '0' || n[0] > '9' {
		return true
	}
	if _, err := fmt.Sscanf(n, "%d", &pid); err != nil || fmt.Sprint(pid) != n {
		return false
	}
	return pids[pid]
}

// envOK reports whether an out-of-tree read of p by exe is inside the
// environment set (Task step 2, "Environment").
func (e *recordEnv) envOK(p, exe string, pids map[int]bool) bool {
	switch {
	case distroPath(p), procOfTraced(p, pids):
		return true
	case e.GOROOT != "" && pathUnder(e.GOROOT, p):
		return e.unmodified(p)
	case pathUnder(e.GOMODCACHE, p), pathUnder(e.GOCACHE, p), pathUnder(e.GOTELEMETRYDIR, p):
		return e.toolchain(exe)
	}
	if e.Home != "" {
		for _, c := range []string{".gitconfig", ".config/git", ".config/go"} {
			if pathUnder(filepath.Join(e.Home, c), p) {
				return e.unmodified(p)
			}
		}
	}
	return false
}

// createdUnder reports whether p, or a directory above it, was created in
// this row.
func createdUnder(created map[string]bool, p string) bool {
	for q := p; ; q = filepath.Dir(q) {
		if created[q] {
			return true
		}
		if q == "/" || q == "." {
			return false
		}
	}
}

// classifyTrace turns a row's trace into sorted read-set entries, or the
// could-not-check reason that writes none.
func (e *recordEnv) classifyTrace(l traceLog) ([]string, string) {
	if l.Denied != "" {
		return nil, l.Denied
	}
	gitDirs := append(append([]string{filepath.Join(e.Top, ".git")}, e.GitDirs...), l.GitDirs...)
	set := map[string]bool{}
	for _, ev := range l.Events {
		for _, link := range ev.Links {
			if why := e.one(traceEvent{Kind: accStat, Path: link, Exe: ev.Exe}, gitDirs, l, set); why != "" {
				return nil, why
			}
		}
		if why := e.one(ev, gitDirs, l, set); why != "" {
			return nil, why
		}
	}
	for _, g := range l.GoRuns {
		rel, ok := e.inTree(g.Cwd)
		if !ok {
			return nil, reasonGoOutsideTree
		}
		set[evaluation.GoRunEntry(evaluation.GoRun{
			Dir: rel, Args: g.Args, GOFLAGS: g.Env["GOFLAGS"], GOWORK: g.Env["GOWORK"],
			GOOS: g.Env["GOOS"], GOARCH: g.Env["GOARCH"], CGOEnabled: g.Env["CGO_ENABLED"],
		})] = true
	}
	inTree := false
	for k := range set {
		if !strings.HasPrefix(k, evaluation.EntryHistory) && !strings.HasPrefix(k, evaluation.EntryGoRun+" ") {
			inTree = true
		}
	}
	if !inTree {
		return nil, reasonEmptyReadSet
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, ""
}

// one classifies one access into set, or returns the reason it denies.
func (e *recordEnv) one(ev traceEvent, gitDirs []string, l traceLog, set map[string]bool) string {
	p := ev.Path
	for _, g := range gitDirs {
		if pathUnder(g, p) {
			set[evaluation.EntryHistory] = true
			return ""
		}
	}
	if createdUnder(l.Created, p) {
		return ""
	}
	if rel, ok := e.inTree(p); ok {
		if ev.Absent {
			set[evaluation.EntryAbsent+" "+rel] = true
			return ""
		}
		isDir, isFile := rel == "." || e.Dirs[rel], e.Files[rel]
		if !isDir && !isFile {
			return reasonUntracked(rel)
		}
		switch {
		case ev.Kind == accList && isDir:
			set[evaluation.EntryList+" "+rel] = true
		case isDir:
			set[evaluation.EntryDir+" "+rel] = true
		case ev.Kind == accRead || ev.Kind == accExec:
			set[evaluation.EntryRead+" "+rel] = true
		default:
			set[evaluation.EntryStat+" "+rel] = true
		}
		return ""
	}
	// Out of the tree: a stat or a miss is not an input; a read, a listing
	// or an exec must sit inside the environment set.
	if ev.Absent || ev.Kind == accStat {
		return ""
	}
	if ev.Kind == accRead && p == e.TempRoot && e.toolchain(ev.Exe) {
		// The go tool opening the temp root itself reads no content: a
		// listing would be an accList, and the work directory it then
		// creates is in Created.
		return ""
	}
	if ev.Kind == accExec {
		name := filepath.Base(p)
		if !(execPrefix(p) || e.toolchain(p)) || e.ToolNames[name] {
			return reasonForeignExec(name)
		}
	}
	if !e.envOK(p, ev.Exe, l.Pids) {
		return reasonOutOfTreeRead
	}
	return ""
}

// readSetTarget names where one row's read set is written and what its header
// says.
type readSetTarget struct {
	env      *recordEnv
	brief    string // repository path of the brief
	commit   string // the full commit the row runs on
	row      string
	command  string
	armExecs int // execs before the row's own shell (the network-off wrapper's)

	unavailable string // known reason a host run cannot record this brief
}

// forRow is the target for one Verify row.
func (t *readSetTarget) forRow(id, command string) *readSetTarget {
	if t == nil {
		return nil
	}
	c := *t
	c.row, c.command = id, command
	return &c
}

// readSetStatePath is where a run writes the read-set file named digest:
// under the worktree's own git dir (stateDir), outside the worktree, so a
// dispatched verifier home's source closure, which refuses any untracked
// worktree file, still admits the run's Evidence (evaluation.ReadSetStateDir).
func readSetStatePath(stateDir, digest string) string {
	return filepath.Join(stateDir, evaluation.ReadSetStateDir, digest)
}

// finish classifies a row's trace and, when it yields a read set, writes the
// file and returns its `sha256:` token; otherwise the could-not-check token.
func (t *readSetTarget) finish(l traceLog) string {
	entries, why := t.env.classifyTrace(l)
	if why != "" {
		return evaluation.ReadSetCellToken("", why)
	}
	content := evaluation.FormatReadSet(evaluation.ReadSetHeader{
		Commit: t.commit, Command: evaluation.ReadSetCommandDigest(t.command), Brief: t.brief, Row: t.row,
	}, entries)
	digest := evaluation.ReadSetDigest(content)
	if t.env.StateDir == "" {
		return evaluation.ReadSetCellToken("", "the read-set file could not be written")
	}
	if err := writeReadSetFile(t.env.StateDir, digest, content, randomTempSuffix); err != nil {
		return evaluation.ReadSetCellToken("", "the read-set file could not be written")
	}
	return evaluation.ReadSetCellToken(digest, "")
}

// Retention for the state directory: a read-set file waits there only until
// the landing copies it beside its brief, so pruneReadSets keeps files
// younger than readSetMaxAge (a file exactly that old goes) and, of those,
// the newest readSetMaxFiles.
const (
	readSetMaxAge   = 30 * 24 * time.Hour
	readSetMaxFiles = 256
)

// writeReadSetFile writes content as the read-set file named digest under
// stateDir (readSetStatePath), then prunes the directory (pruneReadSets).
// It never follows a link: the state directory must be a real directory
// (Lstat), a non-regular entry at the file's name (a planted symlink) is
// refused, and the bytes go to a fresh temp file opened
// O_CREATE|O_EXCL|O_NOFOLLOW and renamed into place, so an existing file is
// replaced, never rewritten in place, and a reader sees the old file or the
// whole new one. suffix names each temp-file attempt; the caller passes
// randomTempSuffix, and a test passes a fixed name to plant an entry there.
func writeReadSetFile(stateDir, digest string, content []byte, suffix func() (string, error)) error {
	dst := readSetStatePath(stateDir, digest)
	dir := filepath.Dir(dst)
	if err := os.Mkdir(dir, 0o755); err != nil && !os.IsExist(err) {
		return err
	}
	if fi, err := os.Lstat(dir); err != nil {
		return err
	} else if !fi.IsDir() {
		return fmt.Errorf("read-set state directory %s is not a directory (%s)", dir, fi.Mode().Type())
	}
	if fi, err := os.Lstat(dst); err == nil && !fi.Mode().IsRegular() {
		return fmt.Errorf("read-set file %s is not a regular file (%s)", dst, fi.Mode().Type())
	}
	f, tmp, err := createReadSetTemp(dir, digest, suffix)
	if err != nil {
		return err
	}
	_, werr := f.Write(content)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(tmp, dst)
	}
	if werr != nil {
		_ = os.Remove(tmp)
		return werr
	}
	pruneReadSets(dir, digest, time.Now())
	return nil
}

// randomTempSuffix is the production temp-name source: 64 random bits.
func randomTempSuffix() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// createReadSetTemp opens a new temp file beside the read-set file, never
// one that already exists and never through a link.
func createReadSetTemp(dir, digest string, suffix func() (string, error)) (*os.File, string, error) {
	var err error
	for i := 0; i < 16; i++ {
		var sfx string
		if sfx, err = suffix(); err != nil {
			return nil, "", err
		}
		p := filepath.Join(dir, "."+digest+"."+sfx+".tmp")
		f, oerr := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL|readSetNoFollow, 0o644)
		if oerr == nil {
			return f, p, nil
		}
		if err = oerr; !os.IsExist(oerr) {
			break
		}
	}
	return nil, "", err
}

// pruneReadSets bounds the state directory dir: it removes every read-set
// file (a 64-hex name) or leftover temp file aged readSetMaxAge or more, then
// the oldest read-set files beyond readSetMaxFiles. The file named keep, just
// written, always stays. Any other entry is left alone. Best effort: a file
// it cannot stat or remove is skipped.
func pruneReadSets(dir, keep string, now time.Time) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type aged struct {
		name string
		mod  time.Time
	}
	var fresh []aged
	for _, e := range ents {
		n := e.Name()
		isSet, isTmp := isReadSetName(n), strings.HasPrefix(n, ".") && strings.HasSuffix(n, ".tmp")
		if n == keep || (!isSet && !isTmp) || !e.Type().IsRegular() {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if now.Sub(fi.ModTime()) >= readSetMaxAge {
			_ = os.Remove(filepath.Join(dir, n))
		} else if isSet {
			fresh = append(fresh, aged{n, fi.ModTime()})
		}
	}
	sort.Slice(fresh, func(i, j int) bool { return fresh[i].mod.After(fresh[j].mod) })
	for i, a := range fresh {
		if i+1 >= readSetMaxFiles { // keep occupies one slot
			_ = os.Remove(filepath.Join(dir, a.name))
		}
	}
}

// isReadSetName reports whether n is a read-set file name: 64 lowercase hex.
func isReadSetName(n string) bool {
	if len(n) != 64 {
		return false
	}
	for _, c := range n {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return false
		}
	}
	return true
}

// newRecordEnv observes the session context once: the tree's toplevel, git
// and common dirs, the go environment the rows will see, the tracked files at
// HEAD and the tree's tool names. Any failure returns nil: the run then
// records no read set.
func newRecordEnv(root string, rowEnv []string, start time.Time) (*recordEnv, string, error) {
	// The repository reads go through the existing adapters (gitcore, the
	// coverage repository and the scan guard's git-dir read), so the recorder
	// adds no Git launch site of its own (statusgen/git-launches.json).
	top, err := gitcore.Toplevel(root)
	if err != nil {
		return nil, "", err
	}
	if top, err = filepath.EvalSymlinks(top); err != nil {
		return nil, "", err
	}
	repo := newCoverageRepo(top)
	commit, ok := repo.ResolveCommit("HEAD")
	if !ok || commit == "" {
		return nil, "", fmt.Errorf("read-set recorder: HEAD of %s does not resolve to a commit", top)
	}
	gitDir, err := scanGitDir(root, "absolute-git-dir")
	if err != nil {
		return nil, "", err
	}
	commonDir, err := scanGitDir(root, "git-common-dir")
	if err != nil {
		return nil, "", err
	}
	dirs := gitDir + "\n" + commonDir
	e := &recordEnv{Top: top, Start: start, Files: map[string]bool{}, Dirs: map[string]bool{}, ToolNames: map[string]bool{},
		ModTime: func(p string) (time.Time, bool) {
			fi, err := os.Lstat(p)
			if err != nil {
				return time.Time{}, false
			}
			return fi.ModTime(), true
		}}
	for _, d := range strings.Split(dirs, "\n") {
		if d = strings.TrimSpace(d); d == "" {
			continue
		}
		if !filepath.IsAbs(d) {
			d = filepath.Join(root, d)
		}
		if r, err := filepath.EvalSymlinks(d); err == nil {
			d = r
		}
		e.GitDirs = append(e.GitDirs, d)
	}
	if len(e.GitDirs) > 0 {
		e.StateDir = e.GitDirs[0] // the absolute git dir: this worktree's own
	}
	e.TempRoot = "/tmp"
	for _, kv := range rowEnv {
		if v, ok := strings.CutPrefix(kv, "HOME="); ok {
			e.Home = resolveBest(v)
		}
		if v, ok := strings.CutPrefix(kv, "TMPDIR="); ok && filepath.IsAbs(v) {
			e.TempRoot = filepath.Clean(v)
		}
	}
	e.TempRoot = resolveBest(e.TempRoot)
	goEnv := exec.Command("go", "env", "GOROOT", "GOMODCACHE", "GOCACHE", "GOOS", "GOARCH", "GOTELEMETRYDIR")
	goEnv.Env = rowEnv
	if b, err := goEnv.Output(); err == nil {
		// One line per variable; an unset GOTELEMETRYDIR is an empty last line.
		v := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
		if len(v) == 5 {
			v = append(v, "")
		}
		if len(v) == 6 {
			for i, p := range []*string{&e.GOROOT, &e.GOMODCACHE, &e.GOCACHE} {
				if r, err := filepath.EvalSymlinks(strings.TrimSpace(v[i])); err == nil {
					*p = r
				}
			}
			e.GOOS, e.GOARCH = strings.TrimSpace(v[3]), strings.TrimSpace(v[4])
			if tdir := strings.TrimSpace(v[5]); tdir != "" {
				e.GOTELEMETRYDIR = resolveBest(tdir)
			}
		}
	}
	tree, ok := repo.TreeModes(commit)
	if !ok {
		return nil, "", fmt.Errorf("read-set recorder: the tree of %s could not be listed", commit)
	}
	mains := map[string]bool{}
	for _, entry := range tree {
		meta, name := entry.Mode, entry.Path
		e.Files[name] = true
		for d := path.Dir(name); d != "."; d = path.Dir(d) {
			e.Dirs[d] = true
		}
		if meta == "100755" {
			e.ToolNames[path.Base(name)] = true
		}
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") && !mains[path.Dir(name)] && goPackageMain(filepath.Join(top, filepath.FromSlash(name))) {
			mains[path.Dir(name)] = true
		}
	}
	for d := range mains {
		if d == "." {
			e.ToolNames[filepath.Base(top)] = true
		} else {
			e.ToolNames[path.Base(d)] = true
		}
	}
	return e, commit, nil
}

// goPackageMain reports whether the Go file at p declares package main.
func goPackageMain(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	var head bytes.Buffer
	sc := bufio.NewScanner(f)
	for n := 0; sc.Scan() && n < 200; n++ {
		head.Write(sc.Bytes())
		head.WriteByte('\n')
	}
	af, err := parser.ParseFile(token.NewFileSet(), p, head.Bytes(), parser.PackageClauseOnly)
	return err == nil && af.Name.Name == "main"
}

// resolveBest is p with every symbolic link resolved, the deepest existing
// ancestor resolved when p itself does not exist.
func resolveBest(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	d, b := filepath.Dir(p), filepath.Base(p)
	if d == p {
		return p
	}
	return filepath.Join(resolveBest(d), b)
}
