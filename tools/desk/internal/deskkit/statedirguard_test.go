package deskkit

// Class guard: no test in this package may resolve the operator's REAL state directory.
//
// The real state directory (~/.config/assay) holds the audit log that scan overrides and
// every outward write are reviewed from, plus the kill switch and the roster beacons. A test
// that appends there pollutes that trail with fixture rows carrying the local identity.
// Per-test discipline (call setup, or t.Setenv a temp HOME) is what a new test forgets, so
// the guard is package-wide instead: TestMain resolves the real directory BEFORE the fixture
// redirects the home, and installs the deskDir test hook (stateDirGuard) that REFUSES it.
//
// The hook judges where a resolved directory lies ON THE FILE SYSTEM, not how it is
// spelled: a relative path, a case variant on a case-insensitive file system, a symlinked
// path, a ".." through a symlink, and a subdirectory that does not exist yet are all
// refused (withinDir, and the spelling cases in TestStateDirGuard).
//
// Refusing at resolution — not snapshotting the file afterwards — is deliberate on two
// counts. It stops the write before it happens, because every writer resolves the
// directory before it creates or opens anything there, so the guard cannot itself be the
// pollution; and a stat/size comparison of a live audit log is racy on any machine where a
// real desk is appending to it while the suite runs. Every writer in this package reaches
// the state directory through deskDir (audit.go, auditrecover.go, killswitch.go, and the
// StateDir consumers), so one hook covers the whole class.
//
// A refused resolution is recorded with the test that caused it, and TestMain fails the run
// when any were recorded — so a test that swallows the refusal still turns the suite red.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// realStateDir is the operator's real state directory, resolved before the fixture home is
// installed. Empty when the home could not be resolved (the guard is then could-not-check,
// reported as such by TestMain and TestStateDirGuard).
var realStateDir string

type stateDirHit struct {
	test string
	dir  string
}

var (
	stateDirHitsMu sync.Mutex
	stateDirHits   []stateDirHit
)

// installStateDirGuard resolves the real state directory from the process's current home
// and installs the refusing hook. It must run before anything changes the home variables.
func installStateDirGuard() (uninstall func()) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		fmt.Fprintf(os.Stderr, "statedirguard: could-not-check — cannot resolve the real home (%v); "+
			"the real-state-directory guard is NOT installed for this run\n", err)
		return func() {}
	}
	realStateDir = filepath.Join(home, ".config", "assay")
	prev := stateDirGuard
	stateDirGuard = refuseRealStateDir
	return func() { stateDirGuard = prev }
}

// refuseRealStateDir is the hook body: dir inside the real state directory is recorded and
// refused; anything else passes through untouched.
func refuseRealStateDir(dir string) error {
	if realStateDir == "" || !withinDir(dir, realStateDir) {
		return nil
	}
	stateDirHitsMu.Lock()
	stateDirHits = append(stateDirHits, stateDirHit{test: callingTest(), dir: dir})
	stateDirHitsMu.Unlock()
	return Unverifiable("test guard: refusing the REAL desk-tools state directory "+dir+
		" — point the test at t.TempDir() (setup(t), or t.Setenv HOME)", nil)
}

// withinDir reports whether p names base, or a path beneath it, ON THE FILE SYSTEM — not
// only as text. Neither path has to exist yet: a writer's MkdirAll comes after deskDir, so
// the guard has to judge the directory it is about to create. Three comparisons, any one
// of which is enough:
//
//  1. The absolute, cleaned spellings share a prefix (the plain case, and the only one that
//     needs no file system at all).
//  2. The physical spellings share a prefix. physicalPath resolves a relative path against
//     the working directory, follows every symlink component, and applies ".." to the
//     directory a symlink points INTO, as the kernel does — lexical cleaning would apply it
//     to the link's own parent instead.
//  3. The file identities match. The nearest existing ancestor of base is compared by
//     os.SameFile with each existing ancestor of p, and the rest of the path is compared
//     component by component, ignoring case. That catches spellings the text never shows:
//     a case variant on a case-insensitive file system (the macOS and Windows default) and
//     a second mount of the same directory. Ignoring case can only over-refuse a directory
//     that differs from the real one by case alone, beneath the real home — no fixture does.
//
// A symlink loop, or a link that cannot be read, stops physicalPath. The kernel refuses to
// write through such a path as well, so comparisons 2 and 3 are skipped for it.
func withinDir(p, base string) bool {
	in := func(p, base string) bool {
		return p == base || strings.HasPrefix(p, strings.TrimSuffix(base, string(filepath.Separator))+string(filepath.Separator))
	}
	ap, aerr := filepath.Abs(p)
	ab, berr := filepath.Abs(base)
	if aerr != nil || berr != nil {
		ap, ab = filepath.Clean(p), filepath.Clean(base)
	}
	if in(ap, ab) {
		return true
	}
	pp, pok := physicalPath(p)
	pb, bok := physicalPath(base)
	if !pok || !bok {
		return false
	}
	if in(pp, pb) {
		return true
	}
	return sameFileWithin(pp, pb)
}

// physicalPath returns the symlink-free absolute path that p names, whether or not p (or
// any tail of it) exists yet. ok is false on a symlink loop or an unreadable link.
func physicalPath(p string) (string, bool) { return resolvePhysical(p, 0) }

func resolvePhysical(p string, links int) (string, bool) {
	const maxLinks = 40 // the Linux kernel's own limit on symlinks followed in one lookup
	if links > maxLinks {
		return "", false
	}
	sep := string(filepath.Separator)
	if !filepath.IsAbs(p) {
		if filepath.VolumeName(p) != "" || strings.HasPrefix(p, sep) {
			// Windows only: drive-relative ("C:x") or rooted without a drive ("\x"). Abs
			// cleans lexically, which the kernel also does for these forms.
			abs, err := filepath.Abs(p)
			if err != nil {
				return "", false
			}
			p = abs
		} else {
			wd, err := os.Getwd()
			if err != nil {
				return "", false
			}
			p = wd + sep + p // NOT filepath.Join: that would clean ".." lexically
		}
	}
	vol := filepath.VolumeName(p)
	r := vol + sep
	parts := strings.FieldsFunc(p[len(vol):], func(c rune) bool { return c < 0x80 && os.IsPathSeparator(uint8(c)) })
	for _, c := range parts {
		switch c {
		case ".":
			continue
		case "..":
			// r is already symlink-free, so its lexical parent is its physical parent.
			r = filepath.Dir(r)
			continue
		}
		next := filepath.Join(r, c)
		fi, err := os.Lstat(next)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			r = next // an ordinary directory, or one that does not exist yet
			continue
		}
		target, err := os.Readlink(next)
		if err != nil {
			return "", false
		}
		if !filepath.IsAbs(target) && filepath.VolumeName(target) == "" && !strings.HasPrefix(target, sep) {
			target = r + sep + target
		}
		resolved, ok := resolvePhysical(target, links+1)
		if !ok {
			return "", false
		}
		r = resolved
	}
	return r, true
}

// sameFileWithin is comparison 3 of withinDir, over two physical paths.
func sameFileWithin(p, base string) bool {
	// Find base's nearest existing ancestor; the components below it are compared by name.
	anchor, rest := base, []string(nil)
	ai, err := os.Stat(anchor)
	for err != nil {
		parent := filepath.Dir(anchor)
		if parent == anchor {
			return false
		}
		rest = append([]string{filepath.Base(anchor)}, rest...)
		anchor = parent
		ai, err = os.Stat(anchor)
	}
	// Walk p upward. Every level where an existing directory is the anchor, check whether
	// the components below it start with rest.
	below := []string(nil)
	for q := p; ; {
		if qi, err := os.Stat(q); err == nil && os.SameFile(qi, ai) && len(below) >= len(rest) {
			match := true
			for i := range rest {
				if !strings.EqualFold(below[i], rest[i]) {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
		parent := filepath.Dir(q)
		if parent == q {
			return false
		}
		below = append([]string{filepath.Base(q)}, below...)
		q = parent
	}
}

// callingTest names the Test function on the current stack, so the failure report says
// which test to fix rather than only that one exists.
func callingTest() string {
	pcs := make([]uintptr, 64)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for {
		f, more := frames.Next()
		name := f.Function[strings.LastIndex(f.Function, "/")+1:]
		name = strings.TrimPrefix(name, "deskkit.")
		if strings.HasPrefix(name, "Test") && strings.HasSuffix(f.File, "_test.go") {
			return name
		}
		if !more {
			return "unknown (no Test frame on the stack)"
		}
	}
}

func stateDirHitCount() int {
	stateDirHitsMu.Lock()
	defer stateDirHitsMu.Unlock()
	return len(stateDirHits)
}

// withoutHitsOf returns hits with the entries at index from onward that are attributed to
// test removed. Entries before from, and entries from any other test, are kept.
func withoutHitsOf(hits []stateDirHit, from int, test string) []stateDirHit {
	kept := append([]stateDirHit(nil), hits[:from]...)
	for _, h := range hits[from:] {
		if h.test != test {
			kept = append(kept, h)
		}
	}
	return kept
}

// TestWithoutHitsOf pins the self-test's cleanup: it drops only its own planted hits, so a
// hit another test records in the same window still reaches TestMain and fails the run.
func TestWithoutHitsOf(t *testing.T) {
	hits := []stateDirHit{
		{test: "TestEarlier", dir: "a"},
		{test: "TestStateDirGuard", dir: "b"},
		{test: "TestLeakedGoroutine", dir: "c"},
		{test: "TestStateDirGuard", dir: "d"},
	}
	got := withoutHitsOf(hits, 1, "TestStateDirGuard")
	want := []stateDirHit{{test: "TestEarlier", dir: "a"}, {test: "TestLeakedGoroutine", dir: "c"}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("withoutHitsOf = %+v, want %+v", got, want)
	}
	if fmt.Sprint(hits[3]) != fmt.Sprint(stateDirHit{test: "TestStateDirGuard", dir: "d"}) {
		t.Fatalf("withoutHitsOf modified its input: %+v", hits)
	}
}

// reportStateDirHits prints every recorded hit and reports whether there were any.
func reportStateDirHits(w io.Writer) bool {
	stateDirHitsMu.Lock()
	defer stateDirHitsMu.Unlock()
	if len(stateDirHits) == 0 {
		return false
	}
	fmt.Fprintf(w, "FAIL: statedirguard: %d attempt(s) to resolve the REAL state directory %s:\n",
		len(stateDirHits), realStateDir)
	for _, h := range stateDirHits {
		fmt.Fprintf(w, "  %s -> %s\n", h.test, h.dir)
	}
	return true
}

// TestStateDirGuard proves the guard is live: with the home pointed back at the real one
// (and, separately, the override pointed at the real directory) deskDir refuses, Log writes
// nothing, and each refusal is recorded against this test. It then removes its own planted
// records so they do not fail the run. It is not parallel (t.Setenv), and top-level parallel
// tests do not start until the sequential ones finish, so nothing else records meanwhile.
func TestStateDirGuard(t *testing.T) {
	if realStateDir == "" {
		t.Skip("could-not-check: the real home was not resolvable, so the guard is not installed")
	}
	// Control: the fixture home TestMain installed resolves, and is not the real one.
	if dir, err := deskDir(); err != nil || withinDir(dir, realStateDir) {
		t.Fatalf("control: fixture state dir = %q, %v; want a non-real dir and no error", dir, err)
	}

	n0 := stateDirHitCount()
	t.Cleanup(func() {
		// Remove only this test's own planted hits. A hit some other test's leaked goroutine
		// records in this window must survive, so TestMain still fails the run on it.
		stateDirHitsMu.Lock()
		stateDirHits = withoutHitsOf(stateDirHits, n0, "TestStateDirGuard")
		stateDirHitsMu.Unlock()
	})

	realHome := filepath.Dir(filepath.Dir(realStateDir))
	t.Setenv("HOME", realHome)
	t.Setenv("USERPROFILE", realHome)
	old := dirOverride
	dirOverride = ""
	t.Cleanup(func() { dirOverride = old })

	if dir, err := deskDir(); err == nil || dir != "" {
		t.Fatalf("deskDir under the real home = %q, %v; want a refusal", dir, err)
	}
	if err := Log(Entry{Tool: "deskpr", Verb: "guard-probe", Result: ResultOK}); err == nil {
		t.Fatal("Log under the real home succeeded; the guard did not stop the audit write")
	}
	dirOverride = filepath.Join(realStateDir, "sub")
	if _, err := deskDir(); err == nil {
		t.Fatal("deskDir with the override inside the real state dir was not refused")
	}

	// Other spellings of the same directory. Each case goes through deskDir only, never Log,
	// so a spelling the guard misses fails an assertion here instead of writing to the real
	// directory. The loop runs inline (no t.Run closure) so every hit is attributed to this
	// test by name.
	spellings := stateDirSpellings(t, realHome)
	for _, s := range spellings {
		home, override := realHome, s.override
		if s.home != "" {
			home, override = s.home, ""
		}
		t.Setenv("HOME", home)
		t.Setenv("USERPROFILE", home)
		dirOverride = override
		if dir, err := deskDir(); err == nil {
			t.Errorf("spelling %q (home %q, override %q): deskDir resolved %q; want a refusal",
				s.name, home, override, dir)
		}
	}

	stateDirHitsMu.Lock()
	got := append([]stateDirHit(nil), stateDirHits[n0:]...)
	stateDirHitsMu.Unlock()
	if want := 3 + len(spellings); len(got) != want {
		t.Fatalf("recorded hits = %d, want %d: %+v", len(got), want, got)
	}
	for _, h := range got {
		if h.test != "TestStateDirGuard" {
			t.Errorf("hit attributed to %q, want TestStateDirGuard", h.test)
		}
	}
}

// stateDirSpelling is one way of naming the real state directory (or a path beneath it)
// that is not its plain cleaned text: either a home variable value or a dirOverride.
type stateDirSpelling struct {
	name, home, override string
}

// stateDirSpellings builds the spellings this machine can express. A spelling the platform
// cannot produce (a case-sensitive file system, no symlink permission, no relative path
// across volumes) is logged as could-not-check and left out, never counted as a pass.
func stateDirSpellings(t *testing.T, realHome string) []stateDirSpelling {
	t.Helper()
	sep := string(filepath.Separator)
	// A subdirectory that does not exist: the guard has to refuse it BEFORE a writer's
	// MkdirAll creates it. Nothing here creates it, because the cases resolve only.
	missing := "statedirguard-not-created"
	if _, err := os.Lstat(filepath.Join(realStateDir, missing)); err == nil {
		t.Fatalf("%s exists under the real state dir; the missing-subdir cases need it absent", missing)
	}
	var out []stateDirSpelling

	if wd, err := os.Getwd(); err != nil {
		t.Logf("could-not-check: relative spelling (no working directory: %v)", err)
	} else if rel, err := filepath.Rel(wd, realStateDir); err != nil || filepath.IsAbs(rel) {
		t.Logf("could-not-check: relative spelling (no relative path from %s: %v)", wd, err)
	} else {
		out = append(out, stateDirSpelling{name: "relative override", override: rel})
	}

	if v := swapCase(realHome); v == realHome {
		t.Log("could-not-check: case-variant spelling (the home path has no letters)")
	} else if vi, err := os.Stat(v); err != nil {
		t.Log("could-not-check: case-variant spelling (the file system is case-sensitive here)")
	} else if hi, err := os.Stat(realHome); err != nil || !os.SameFile(vi, hi) {
		t.Log("could-not-check: case-variant spelling (it names a different directory here)")
	} else {
		out = append(out, stateDirSpelling{name: "case-variant home", home: v})
	}

	link := filepath.Join(t.TempDir(), "home-link")
	if err := os.Symlink(realHome, link); err != nil {
		t.Logf("could-not-check: symlink spellings (%v)", err)
	} else {
		out = append(out,
			stateDirSpelling{
				name:     "symlinked override to a missing subdir",
				override: filepath.Join(link, ".config", "assay", missing),
			},
			// Built by hand, not filepath.Join, which would clean the ".." away lexically.
			// Lexically this lies beside the link; on the file system ".." is the parent of
			// the link's TARGET, so it names the real home again.
			stateDirSpelling{
				name: "dot-dot through a symlink",
				override: strings.Join([]string{link, "..", filepath.Base(realHome),
					".config", "assay", missing}, sep),
			},
		)
	}
	return out
}

// swapCase inverts the case of every ASCII letter in s.
func swapCase(s string) string {
	b := []byte(s)
	for i, c := range b {
		switch {
		case 'a' <= c && c <= 'z':
			b[i] = c - 'a' + 'A'
		case 'A' <= c && c <= 'Z':
			b[i] = c - 'A' + 'a'
		}
	}
	return string(b)
}
