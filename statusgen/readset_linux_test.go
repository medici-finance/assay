//go:build linux && (amd64 || arm64)

package main

// The recorder rows of build-less-brittle/15 (Verify rows 14, 16, 21, 23 and
// 25). Each drives verifyrun over a fixture repository with the tracer on and
// reads back the Runner cell's read-set token and the file it names. Where
// the host refuses the tracer each test skips as its first statement, so the
// brief's row prints 0 and can never pass.

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
	"golang.org/x/sys/unix"
)

// needRecorder skips (could-not-check, never a pass) when this host refuses
// the tracer.
func needRecorder(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("SKIP (could-not-check, not a pass): no bash on this host: %v", err)
	}
	tr := runTraced(t.Context(), exec.Command("/bin/true"), 0, nil)
	if tr.startErr != nil {
		t.Skipf("SKIP (could-not-check, not a pass): the host refused the read-set tracer: %v", tr.startErr)
	}
	if !tr.exited || tr.exit != 0 || tr.log.Denied != "" {
		t.Skipf("SKIP (could-not-check, not a pass): the tracer could not trace /bin/true (exit %d, %q)", tr.exit, tr.log.Denied)
	}
}

// recFixture is a committed fixture repository holding one brief.
type recFixture struct {
	root, brief string
}

// newRecFixture writes files (a value starting "->" is a symbolic link to the
// rest; a key ending "*" is executable), a brief whose Verify table holds
// rows (one command each, IDs 1..n), and commits it all.
func newRecFixture(t *testing.T, files map[string]string, rows ...string) recFixture {
	t.Helper()
	t.Setenv("DESK_LOOP", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITHUB_ACTOR", "")
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := recFixture{root: root, brief: filepath.Join(root, "docs", "brief.md")}
	gitInit(t, root, "fixture", "fixture@example.invalid")
	for name, body := range files {
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, "*") {
			name, mode = strings.TrimSuffix(name, "*"), 0o755
		}
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if target, ok := strings.CutPrefix(body, "->"); ok {
			if err := os.Symlink(target, p); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	var b strings.Builder
	b.WriteString("| # | Command | Expect |\n|---|---|---|\n")
	for i, r := range rows {
		fmt.Fprintf(&b, "| %d | `%s` | exit 0 |\n", i+1, strings.ReplaceAll(r, "|", `\|`))
	}
	if err := os.MkdirAll(filepath.Dir(f.brief), 0o755); err != nil {
		t.Fatal(err)
	}
	putFile(t, f.brief, "# Fixture\n\n## Verify\n\n"+b.String()+"\n## Evidence\n\n## Review\n\nnone\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "fixture")
	resetReadMemo()
	return f
}

func (f recFixture) head(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "-C", f.root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// run runs verifyrun over the given rows and returns each row's Runner cell
// from the newest witness.
func (f recFixture) run(t *testing.T, ids ...string) map[string]string {
	t.Helper()
	pinClock(t)
	res, out := captureVerifyrunOut(t, []string{"--root", f.root, "--brief", f.brief, "--rows", strings.Join(ids, ",")})
	if res.code != verifyrunExitPass {
		t.Fatalf("verifyrun --rows %v: exit %d\n%s", ids, res.code, out)
	}
	rows := parseEvidenceRows(evidenceOf(t, f.brief))
	got := map[string]string{}
	for _, id := range ids {
		ws := rows[id]
		if len(ws) == 0 {
			t.Fatalf("row %s: no witness\n%s", id, out)
		}
		cells := witnessCells(ws[len(ws)-1].Text)
		if cells == nil {
			t.Fatalf("row %s witness does not parse: %q", id, ws[len(ws)-1].Text)
		}
		got[id] = strings.TrimSpace(cells[witnessCellRunner])
	}
	return got
}

// readSetOf returns the header and entries of the read set a Runner cell
// names, checking the file's digest is its name.
func (f recFixture) readSetOf(t *testing.T, runner string) (evaluation.ReadSetHeader, []string) {
	t.Helper()
	digest, couldNot, ok := evaluation.ReadSetToken(runner)
	if !ok || digest == "" {
		t.Fatalf("Runner cell %q carries no read set (could-not-check %q)", runner, couldNot)
	}
	p := readSetStatePath(gitDirOf(t, f.root), digest)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read-set file: %v", err)
	}
	if got := evaluation.ReadSetDigest(raw); got != digest {
		t.Fatalf("read-set file digest %s, token names %s", got, digest)
	}
	h, entries, err := evaluation.ParseReadSet(raw)
	if err != nil {
		t.Fatalf("read-set file does not parse: %v\n%s", err, raw)
	}
	return h, entries
}

// readsDirs lists every reads/ directory under root, and the recorder's state
// directory under root's git dir when it exists.
func readsDirs(root string) []string {
	var out []string
	if b, err := exec.Command("git", "-C", root, "rev-parse", "--absolute-git-dir").Output(); err == nil {
		d := filepath.Join(strings.TrimSpace(string(b)), evaluation.ReadSetStateDir)
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			out = append(out, d)
		}
	}
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if err == nil && d.IsDir() && d.Name() == evaluation.ReadSetDir {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// buildProbe builds a small helper binary for system calls a shell cannot
// make, into dst.
func buildProbe(t *testing.T, dst string) {
	t.Helper()
	src := t.TempDir()
	putFile(t, filepath.Join(src, "go.mod"), "module probe\n\ngo 1.22\n")
	putFile(t, filepath.Join(src, "main.go"), probeMain())
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("go", "build", "-o", dst, ".")
	c.Dir = src
	c.Env = append(os.Environ(), "GOFLAGS=", "GOWORK=off", "CGO_ENABLED=0")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("build probe: %v\n%s", err, out)
	}
}

// probeMain is the probe's source with this architecture's syscall numbers
// filled in: the frozen syscall package names open_by_handle_at and
// process_vm_readv on arm64 but not on amd64, so the probe takes them from
// x/sys/unix at build time instead.
func probeMain() string {
	return probeSource + fmt.Sprintf("\nconst (\n\tsysOpenByHandleAt = %d\n\tsysProcessVMReadv = %d\n)\n", unix.SYS_OPEN_BY_HANDLE_AT, unix.SYS_PROCESS_VM_READV)
}

const probeSource = `package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

func main() {
	switch os.Args[1] {
	case "dirfd":
		dfd, err := syscall.Open(os.Args[2], syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
		if err != nil {
			panic(err)
		}
		fd, err := syscall.Openat(dfd, os.Args[3], syscall.O_RDONLY, 0)
		if err != nil {
			panic(err)
		}
		b := make([]byte, 64)
		n, _ := syscall.Read(fd, b)
		fmt.Printf("%s", b[:n])
	case "handle":
		syscall.Syscall(sysOpenByHandleAt, uintptr(0xffffff9c), 0, 0)
	case "vmread":
		pid, _ := strconv.Atoi(os.Args[2])
		var buf [8]byte
		local := syscall.Iovec{Base: &buf[0], Len: 8}
		remote := syscall.Iovec{Base: &buf[0], Len: 8}
		syscall.Syscall6(sysProcessVMReadv, uintptr(pid), uintptr(unsafe.Pointer(&local)), 1, uintptr(unsafe.Pointer(&remote)), 1, 0)
	}
}
`

// regression: #6274
func TestReadSetTargetWithoutRepository(t *testing.T) {
	root := t.TempDir()
	brief := filepath.Join(t.TempDir(), "brief.md")
	if target := newReadSetTarget(root, brief, nil); target != nil {
		t.Fatal("reported a recorder target without establishing a checkout")
	}
}

// regression: #2026
// TestVerifyrunRecordsReadSet — build-less-brittle/15 row 14.
func TestVerifyrunRecordsReadSet(t *testing.T) {
	needRecorder(t)
	files := map[string]string{
		"data/child.txt":       "child\n",
		"data/script-in.txt":   "script\n",
		"data/target.txt":      "target\n",
		"data/link.txt":        "->target.txt",
		"data/dir/inner.txt":   "inner\n",
		"data/listed/a.txt":    "a\n",
		"data/stated.txt":      "stated\n",
		"tools/run.sh*":        "#!/bin/sh\ncat data/script-in.txt\n",
		"tools/placeholder.md": "x\n",
	}
	row := "sh -c 'cat data/child.txt' && tools/run.sh && cat data/link.txt && bin/probe dirfd data/dir inner.txt && ls data/listed && test -f data/stated.txt && ! test -e data/missing.txt"
	f := newRecFixture(t, files, row)
	buildProbe(t, filepath.Join(f.root, "bin", "probe"))
	runGit(t, f.root, "add", "bin/probe")
	runGit(t, f.root, "commit", "-q", "-m", "probe")
	resetReadMemo()
	runner := f.run(t, "1")["1"]
	h, entries := f.readSetOf(t, runner)
	// The file lands under the git dir, never in the worktree: a dispatched
	// verifier home refuses any untracked worktree file.
	st, err := exec.Command("git", "-C", f.root, "status", "--porcelain", "--untracked-files=all").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(st), evaluation.ReadSetDir+"/") {
		t.Errorf("the recorder wrote a read-set file into the worktree:\n%s", st)
	}
	if h.Commit != f.head(t) {
		t.Errorf("header commit %q, want the full witness commit %q", h.Commit, f.head(t))
	}
	if want := evaluation.ReadSetCommandDigest(row); h.Command != want {
		t.Errorf("header command %q, want %q", h.Command, want)
	}
	if h.Brief != "docs/brief.md" || h.Row != "1" {
		t.Errorf("header brief/row %q/%q", h.Brief, h.Row)
	}
	have := map[string]bool{}
	for _, e := range entries {
		have[e] = true
	}
	for _, want := range []string{
		"read data/child.txt",     // through a child process
		"read tools/run.sh",       // an in-tree script executed
		"read data/script-in.txt", // read by that script
		"stat data/link.txt",      // the link itself
		"read data/target.txt",    // and its target
		"read data/dir/inner.txt", // relative to a directory descriptor
		"list data/listed",        // a directory listed
		"stat data/stated.txt",    // a file stat-ed
		"absent data/missing.txt", // an absent probe
		"read bin/probe",          // the probe itself
	} {
		if !have[want] {
			t.Errorf("read set lacks %q; entries:\n%s", want, strings.Join(entries, "\n"))
		}
	}
}

// regression: #2026
// TestVerifyrunUntraceableReadOmitsReadSet — build-less-brittle/15 row 16.
func TestVerifyrunUntraceableReadOmitsReadSet(t *testing.T) {
	needRecorder(t)
	if os.Getenv("BLB15_MOUNTNS") != "" {
		// The re-run inside the private mount namespace: the planted cases only.
		t.Run("tree tool copies", treeToolCopies)
		return
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	outside := filepath.Join(t.TempDir(), "outside.txt")
	putFile(t, outside, "outside\n")
	built := filepath.Join(t.TempDir(), "built-tool")
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".config", "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(home, ".config", "git", "ignore"), "x\n")
	t.Setenv("HOME", home)
	gocache, err := exec.Command("go", "env", "GOCACHE").Output()
	if err != nil {
		t.Fatal(err)
	}
	cacheReadme := filepath.Join(strings.TrimSpace(string(gocache)), "README")
	if _, err := os.Stat(cacheReadme); err != nil {
		t.Skipf("SKIP (could-not-check, not a pass): no GOCACHE README to read: %v", err)
	}
	cases := []struct{ name, row, reason string }{
		{"non-loopback connect", "cat data/in.txt && (timeout 2 bash -c 'echo > /dev/tcp/192.0.2.1/9' || true)", "non-loopback"},
		{"untraced listener", fmt.Sprintf("cat data/in.txt && (bash -c 'echo > /dev/tcp/127.0.0.1/%d' || true)", port), "no traced process listens"},
		{"unshare", "cat data/in.txt && (unshare -U true || true)", "unshare"},
		{"open_by_handle_at", "cat data/in.txt && bin/probe handle", "open_by_handle_at"},
		{"out-of-tree read", "cat data/in.txt && cat " + outside, "out-of-tree file"},
		{"built by an earlier row", "cat data/in.txt && " + built, "did not build (built-tool)"},
		{"process_vm_readv", fmt.Sprintf("cat data/in.txt && bin/probe vmread %d", os.Getpid()), "process_vm_readv"},
		{"GOCACHE by non-go", "cat data/in.txt && cat " + cacheReadme, "out-of-tree file"},
		{"modified git config", "cat data/in.txt && echo y >> " + filepath.Join(home, ".config", "git", "ignore") + " && cat " + filepath.Join(home, ".config", "git", "ignore"), "out-of-tree file"},
	}
	rows := []string{"cp /bin/true " + built}
	for _, c := range cases {
		rows = append(rows, c.row)
	}
	f := newRecFixture(t, map[string]string{"data/in.txt": "in\n"}, rows...)
	buildProbe(t, filepath.Join(f.root, "bin", "probe"))
	runGit(t, f.root, "add", "bin/probe")
	runGit(t, f.root, "commit", "-q", "-m", "probe")
	resetReadMemo()
	ids := []string{}
	for i := range rows {
		ids = append(ids, fmt.Sprint(i+1))
	}
	got := f.run(t, ids...)
	for i, c := range cases {
		runner := got[fmt.Sprint(i+2)]
		t.Run(c.name, func(t *testing.T) {
			digest, couldNot, ok := evaluation.ReadSetToken(runner)
			if !ok || digest != "" || couldNot == "" {
				t.Fatalf("Runner cell %q: want a reads could-not-check token", runner)
			}
			if !strings.Contains(runner, "(reads could-not-check:") || !strings.Contains(couldNot, c.reason) {
				t.Errorf("Runner cell %q: reason %q lacks %q", runner, couldNot, c.reason)
			}
		})
	}
	// Row 1 (the build) may record its own read set; no denied row may leave
	// a read-set file beside it.
	allowed := map[string]bool{}
	if digest, _, ok := evaluation.ReadSetToken(got["1"]); ok && digest != "" {
		allowed[digest] = true
	}
	for _, d := range readsDirs(f.root) {
		ents, _ := os.ReadDir(d)
		for _, e := range ents {
			if !allowed[e.Name()] {
				t.Errorf("a denied row wrote read-set file %s", filepath.Join(d, e.Name()))
			}
		}
	}
	t.Run("tree tool copies", treeToolCopies)
}

// treeToolCopies covers the two planted-binary cases of row 16 in a private
// mount namespace: a copy of a tree tool in /usr/local/bin, and a binary in
// an allowed prefix named like a tree main package. The test re-runs itself
// under `unshare --mount` so nothing is planted on the host.
func treeToolCopies(t *testing.T) {
	if os.Getenv("BLB15_MOUNTNS") == "" {
		if os.Geteuid() != 0 {
			t.Skipf("SKIP (could-not-check, not a pass): planting binaries in a private mount namespace needs root")
		}
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		script := `set -e
mount --make-rprivate /
mount -t tmpfs none /usr/local/bin
mount -t tmpfs none /usr/libexec
cp /bin/true /usr/local/bin/run.sh
cp /bin/true /usr/libexec/probe-tool
exec "$0" -test.run '^TestVerifyrunUntraceableReadOmitsReadSet$/^tree_tool_copies$' -test.count=1 -test.v`
		c := exec.Command("unshare", "--mount", "sh", "-c", script, self)
		c.Env = append(os.Environ(), "BLB15_MOUNTNS=1")
		out, err := c.CombinedOutput()
		if err != nil {
			if strings.Contains(string(out), "unshare failed") || strings.Contains(string(out), "mount:") {
				t.Skipf("SKIP (could-not-check, not a pass): no private mount namespace: %s", out)
			}
			t.Fatalf("mount-namespace run: %v\n%s", err, out)
		}
		if !regexp.MustCompile(`(?m)^\s*--- PASS: TestVerifyrunUntraceableReadOmitsReadSet/tree_tool_copies/usr_local_copy`).Match(out) ||
			!regexp.MustCompile(`(?m)^\s*--- PASS: TestVerifyrunUntraceableReadOmitsReadSet/tree_tool_copies/allowed_prefix_main_name`).Match(out) {
			t.Fatalf("mount-namespace run did not pass both cases:\n%s", out)
		}
		return
	}
	files := map[string]string{
		"data/in.txt":            "in\n",
		"tools/run.sh*":          "#!/bin/sh\ntrue\n",
		"cmd/probe-tool/main.go": "package main\n\nfunc main() {}\n",
	}
	f := newRecFixture(t, files, "cat data/in.txt && /usr/local/bin/run.sh", "cat data/in.txt && /usr/libexec/probe-tool")
	got := f.run(t, "1", "2")
	for id, c := range map[string]struct{ name, reason string }{
		"1": {"usr_local_copy", "did not build (run.sh)"},
		"2": {"allowed_prefix_main_name", "did not build (probe-tool)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(got[id], "(reads could-not-check:") || !strings.Contains(got[id], c.reason) {
				t.Errorf("Runner cell %q lacks a could-not-check naming %q", got[id], c.reason)
			}
		})
	}
	if d := readsDirs(f.root); len(d) > 0 {
		t.Errorf("a read-set directory was written: %v", d)
	}
}

// regression: #2026
// TestVerifyrunCachedGoTestRecordsFixtures — build-less-brittle/15 row 21.
func TestVerifyrunCachedGoTestRecordsFixtures(t *testing.T) {
	needRecorder(t)
	t.Setenv("GOCACHE", t.TempDir())
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	files := map[string]string{
		"go.mod":                   "module fixture\n\ngo 1.22\n",
		"pkg/pkg.go":               "package pkg\n",
		"pkg/testdata/fixture.txt": "fixture\n",
		"pkg/pkg_test.go": `package pkg

import (
	"os"
	"testing"
)

func TestFixture(t *testing.T) {
	for i := 0; i < 2; i++ {
		if _, err := os.ReadFile("testdata/fixture.txt"); err != nil {
			t.Fatal(err)
		}
	}
}
`,
	}
	row := "go test ./pkg/"
	f := newRecFixture(t, files, row, row)
	first := f.run(t, "1")["1"]
	second := f.run(t, "2")["2"]
	for _, c := range []struct{ name, runner string }{{"first", first}, {"cached", second}} {
		_, entries := f.readSetOf(t, c.runner)
		has := false
		for _, e := range entries {
			// A cached run only stats the fixture (the go tool checks it has
			// not changed); a stat entry is judged on content (row 20).
			has = has || e == "read pkg/testdata/fixture.txt" || e == "stat pkg/testdata/fixture.txt"
		}
		if !has {
			t.Errorf("%s run: read set lacks the fixture:\n%s", c.name, strings.Join(entries, "\n"))
		}
	}
}

// regression: #2026
// TestVerifyrunHistoryReadInLinkedWorktree — build-less-brittle/15 row 23.
func TestVerifyrunHistoryReadInLinkedWorktree(t *testing.T) {
	needRecorder(t)
	row := "git log -1 --format=%H -- README.md && test -f README.md"
	f := newRecFixture(t, map[string]string{"README.md": "readme\n"}, row)
	check := func(t *testing.T, fx recFixture) {
		_, entries := fx.readSetOf(t, fx.run(t, "1")["1"])
		has := false
		for _, e := range entries {
			has = has || e == evaluation.EntryHistory
		}
		if !has {
			t.Errorf("read set lacks %q:\n%s", evaluation.EntryHistory, strings.Join(entries, "\n"))
		}
	}
	t.Run("primary", func(t *testing.T) { check(t, f) })
	t.Run("linked worktree", func(t *testing.T) {
		wt := filepath.Join(t.TempDir(), "wt")
		runGit(t, f.root, "worktree", "add", "-q", "--detach", wt, "HEAD")
		resetReadMemo()
		check(t, recFixture{root: wt, brief: filepath.Join(wt, "docs", "brief.md")})
	})
}

// regression: #2026
// TestVerifyrunReadSetFilesAndDirty — build-less-brittle/15 row 25.
func TestVerifyrunReadSetFilesAndDirty(t *testing.T) {
	needRecorder(t)
	f := newRecFixture(t, map[string]string{"data/in.txt": "in\n", "other/brief.md": "# Other\n"}, "cat data/in.txt")
	first := f.run(t, "1")["1"]
	if _, _, ok := evaluation.ReadSetToken(first); !ok || !strings.Contains(first, "(reads sha256:") {
		t.Fatalf("first run wrote no read set: %q", first)
	}
	// A resumed run over the brief's own uncommitted Evidence records a clean
	// tree: the read-set file sits under the git dir. So does one over the
	// file a landing copies beside the brief.
	resumed := f.run(t, "1")["1"]
	if strings.Contains(resumed, "+dirty") {
		t.Errorf("resumed run over its own Evidence is +dirty: %q", resumed)
	}
	digest, _, _ := evaluation.ReadSetToken(resumed)
	raw, err := os.ReadFile(readSetStatePath(gitDirOf(t, f.root), digest))
	if err != nil {
		t.Fatalf("read-set file under the git dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(f.root, "docs", evaluation.ReadSetDir), 0o755); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(f.root, "docs", evaluation.ReadSetDir, digest), string(raw))
	resetReadMemo()
	resumed = f.run(t, "1")["1"]
	if strings.Contains(resumed, "+dirty") {
		t.Errorf("resumed run over its own read-set file is +dirty: %q", resumed)
	}
	// Another brief's uncommitted read-set file dirties the tree.
	other := filepath.Join(f.root, "other", evaluation.ReadSetDir)
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(other, strings.Repeat("a", 64)), "x\n")
	resetReadMemo()
	dirty := f.run(t, "1")["1"]
	if !strings.Contains(dirty, "+dirty") {
		t.Errorf("a run with another brief's read-set file is not +dirty: %q", dirty)
	}
}
