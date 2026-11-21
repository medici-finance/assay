package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// snapFixture builds "<cache>/assay/verify-snap" and returns it. A file's inode change time
// cannot be set back, so the tests judge at a clock AHEAD of the real one: everything written
// now is "old" at that clock unless its modification time is moved forward to match.
func snapFixture(t *testing.T) string {
	t.Helper()
	base := filepath.Join(t.TempDir(), "assay", verifySnapDirName)
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatal(err)
	}
	return base
}

func snapWrite(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

// snapAgeTree sets every directory's modification time under dir, the deepest first.
func snapAgeTree(t *testing.T, dir string, mtime time.Time) {
	t.Helper()
	var dirs []string
	if err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			dirs = append(dirs, p)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Chtimes(dirs[i], mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
}

// needChangeTime skips a test that expects a removal on a build that reads no change time:
// there the prune removes nothing by design (TestVerifySnapNoChangeTimeKeeps pins that).
func needChangeTime(t *testing.T) {
	t.Helper()
	if !changeTimeKnown {
		t.Skip("this build reads no inode change time; the prune keeps every snapshot here")
	}
}

func snapVerdictFor(t *testing.T, plan []snapVerdict, name string) snapVerdict {
	t.Helper()
	for _, v := range plan {
		if v.name == name {
			return v
		}
	}
	t.Fatalf("no verdict for %s in %+v", name, plan)
	return snapVerdict{}
}

// regression: #7275
func TestVerifySnapPruneKeepsLiveRuns(t *testing.T) {
	needChangeTime(t)
	base := snapFixture(t)
	real := time.Now()
	now := real.Add(verifySnapMaxAge + 24*time.Hour) // every inode change is a day past retention
	old := real.Add(-time.Hour)

	// idle: nothing in it touched since well before retention, with a read-only directory
	// (a copied module cache) that removal must make writable first.
	snapWrite(t, filepath.Join(base, "idle--01", "src", "main.go"), old)
	snapWrite(t, filepath.Join(base, "idle--01", "home", "mod", "x.go"), old)
	snapAgeTree(t, filepath.Join(base, "idle--01"), old)
	if err := os.Chmod(filepath.Join(base, "idle--01", "home", "mod"), 0o500); err != nil {
		t.Fatal(err)
	}
	// live: an old tree, but a row just wrote its output deep inside it.
	snapWrite(t, filepath.Join(base, "live--02", "src", "main.go"), old)
	snapWrite(t, filepath.Join(base, "live--02", "out", "deep", "row3.out"), now.Add(-time.Hour))
	snapAgeTree(t, filepath.Join(base, "live--02"), old)
	// own: idle, but it is the claim being dispatched.
	snapWrite(t, filepath.Join(base, "own--03", "f"), old)
	snapAgeTree(t, filepath.Join(base, "own--03"), old)
	// a symlink to an idle tree outside the base, and a stray file.
	outside := filepath.Join(t.TempDir(), "outside")
	snapWrite(t, filepath.Join(outside, "keep.txt"), old)
	snapAgeTree(t, outside, old)
	if err := os.Symlink(outside, filepath.Join(base, "link--04")); err != nil {
		t.Fatal(err)
	}
	snapWrite(t, filepath.Join(base, "notes.txt"), old)

	dry, err := planVerifySnaps(base, "own--03", now, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if v := snapVerdictFor(t, dry, "idle--01"); !v.remove || !strings.Contains(v.reason, "idle") {
		t.Errorf("idle snapshot: %+v", v)
	}
	for _, name := range []string{"live--02", "own--03", "link--04", "notes.txt"} {
		if v := snapVerdictFor(t, dry, name); v.remove || v.reason == "" {
			t.Errorf("%s must be kept with a reason: %+v", name, v)
		}
	}
	lines := strings.Join(verifySnapLines(base, dry, true), "\n")
	if !strings.Contains(lines, "would remove "+filepath.Join(base, "idle--01")) || !strings.Contains(lines, "keep "+filepath.Join(base, "live--02")) {
		t.Errorf("dry-run listing does not name both verdicts:\n%s", lines)
	}
	if _, err := os.Stat(filepath.Join(base, "idle--01")); err != nil {
		t.Fatalf("the plan removed something: %v", err)
	}

	if _, err := pruneVerifySnaps(base, "own--03", now, 1000); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(base, "idle--01")); !os.IsNotExist(err) {
		t.Errorf("idle snapshot kept: %v", err)
	}
	for _, keep := range []string{
		filepath.Join(base, "live--02", "out", "deep", "row3.out"),
		filepath.Join(base, "own--03", "f"),
		filepath.Join(base, "notes.txt"),
		filepath.Join(outside, "keep.txt"),
	} {
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("%s was removed: %v", keep, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(base, "link--04")); err != nil {
		t.Errorf("the symlink entry was removed: %v", err)
	}
}

// An archive extracted into a snapshot keeps the archive's old modification times; the change
// time is what shows the extraction just happened.
//
// regression: #7275
func TestVerifySnapChangeTimeIsActivity(t *testing.T) {
	base := snapFixture(t)
	now := time.Now().Add(time.Hour)
	ancient := now.Add(-10 * verifySnapMaxAge)
	snapWrite(t, filepath.Join(base, "extracted--01", "src", "a.go"), ancient)
	snapAgeTree(t, filepath.Join(base, "extracted--01"), ancient)
	plan, err := planVerifySnaps(base, "", now, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if v := snapVerdictFor(t, plan, "extracted--01"); v.remove {
		t.Fatalf("a just-extracted snapshot with archive-old modification times was judged idle: %+v", v)
	}
}

// A snapshot that cannot be read in full is kept, never removed on a partial view.
func TestVerifySnapUnreadKept(t *testing.T) {
	needChangeTime(t)
	base := snapFixture(t)
	real := time.Now()
	now := real.Add(verifySnapMaxAge + 24*time.Hour)
	for i := 0; i < 5; i++ {
		snapWrite(t, filepath.Join(base, "big--01", "d", string(rune('a'+i))), real.Add(-time.Hour))
	}
	snapAgeTree(t, filepath.Join(base, "big--01"), real.Add(-time.Hour))
	plan, err := pruneVerifySnaps(base, "", now, 3)
	if err != nil {
		t.Fatal(err)
	}
	if v := snapVerdictFor(t, plan, "big--01"); v.remove || !strings.Contains(v.reason, "could not read") {
		t.Fatalf("an over-bound snapshot was not kept as could-not-read: %+v", v)
	}
	if _, err := os.Stat(filepath.Join(base, "big--01")); err != nil {
		t.Fatal(err)
	}
}

// A symlinked base is refused: the prune removes trees and must only see its own directory.
func TestVerifySnapSymlinkBaseRefused(t *testing.T) {
	target := snapFixture(t)
	snapWrite(t, filepath.Join(target, "idle--01", "f"), time.Now().Add(-time.Hour))
	parent := filepath.Join(t.TempDir(), "assay")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(parent, verifySnapDirName)
	if err := os.Symlink(target, base); err != nil {
		t.Fatal(err)
	}
	if _, err := pruneVerifySnaps(base, "", time.Now().Add(30*24*time.Hour), 1000); err == nil {
		t.Fatal("a symlinked verify-snap base was pruned through")
	}
	if _, err := os.Stat(filepath.Join(target, "idle--01", "f")); err != nil {
		t.Fatal(err)
	}
}

// The dispatch hook: a dry run removes nothing, a real verifier dispatch removes the idle
// snapshot and keeps its own claim's.
func TestVerifySnapDispatchHook(t *testing.T) {
	needChangeTime(t)
	base := snapFixture(t)
	cache := filepath.Dir(filepath.Dir(base))
	real := time.Now()
	oldCache, oldNow, oldRoot := packetCacheDir, packetNow, verifySnapRoot
	t.Cleanup(func() { packetCacheDir, packetNow, verifySnapRoot = oldCache, oldNow, oldRoot })
	packetCacheDir = func() (string, error) { return cache, nil }
	verifySnapRoot = verifySnapBase
	packetNow = func() time.Time { return real.Add(verifySnapMaxAge + 24*time.Hour) }
	for _, name := range []string{"idle--01", "own--02"} {
		snapWrite(t, filepath.Join(base, name, "f"), real.Add(-time.Hour))
		snapAgeTree(t, filepath.Join(base, name), real.Add(-time.Hour))
	}
	reportVerifySnaps("own--02", true)
	if _, err := os.Stat(filepath.Join(base, "idle--01")); err != nil {
		t.Fatalf("a dry run removed a snapshot: %v", err)
	}
	reportVerifySnaps("own--02", false)
	if _, err := os.Stat(filepath.Join(base, "idle--01")); !os.IsNotExist(err) {
		t.Errorf("the dispatch kept an idle snapshot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "own--02", "f")); err != nil {
		t.Errorf("the dispatch removed its own claim's snapshot: %v", err)
	}
}

// Where the build reads no inode change time (every platform but macOS and Linux), the prune
// cannot tell a timestamp-preserving copy from an idle tree, so it keeps everything and says so.
//
// regression: #7275
func TestVerifySnapNoChangeTimeKeeps(t *testing.T) {
	old := snapChangeTimeKnown
	t.Cleanup(func() { snapChangeTimeKnown = old })
	snapChangeTimeKnown = false
	base := snapFixture(t)
	real := time.Now()
	snapWrite(t, filepath.Join(base, "idle--01", "f"), real.Add(-time.Hour))
	snapAgeTree(t, filepath.Join(base, "idle--01"), real.Add(-time.Hour))
	plan, err := pruneVerifySnaps(base, "", real.Add(verifySnapMaxAge+24*time.Hour), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if v := snapVerdictFor(t, plan, "idle--01"); v.remove || !strings.HasPrefix(v.reason, "could-not-check") {
		t.Fatalf("a platform with no change time removed or did not report could-not-check: %+v", v)
	}
	if _, err := os.Stat(filepath.Join(base, "idle--01", "f")); err != nil {
		t.Fatal(err)
	}
}

// A snapshot a run touches after the plan judged it idle is kept: removal reads its activity
// again through the same root.
//
// regression: #7275
func TestVerifySnapRecheckBeforeRemoval(t *testing.T) {
	needChangeTime(t)
	base := snapFixture(t)
	real := time.Now()
	now := real.Add(verifySnapMaxAge + 24*time.Hour)
	snapWrite(t, filepath.Join(base, "idle--01", "f"), real.Add(-time.Hour))
	snapAgeTree(t, filepath.Join(base, "idle--01"), real.Add(-time.Hour))
	root, err := openSnapBase(base)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	plan, err := planSnapsIn(root, "", now, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if v := snapVerdictFor(t, plan, "idle--01"); !v.remove {
		t.Fatalf("fixture not idle: %+v", v)
	}
	snapWrite(t, filepath.Join(base, "idle--01", "row.out"), now) // a run writes after the plan
	if err := removeVerifySnap(root, "idle--01", now, 1000); err != errSnapActive {
		t.Fatalf("removal of a snapshot touched after the plan: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "idle--01", "row.out")); err != nil {
		t.Fatal(err)
	}
}

// The hook has exactly two call sites in dispatch(): the dry-run plan (lists, removes nothing)
// and the real verifier dispatch. Dropping or swapping either would not be caught by the
// direct hook test above.
func TestVerifySnapCallSites(t *testing.T) {
	src, err := os.ReadFile("dispatch.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if n := strings.Count(s, "reportVerifySnaps("); n != 2 {
		t.Fatalf("dispatch.go calls reportVerifySnaps %d times, want 2", n)
	}
	for _, call := range []string{"reportVerifySnaps(plan.claimKey, true)", "reportVerifySnaps(plan.claimKey, false)"} {
		if !strings.Contains(s, call) {
			t.Errorf("dispatch.go lacks %s", call)
		}
	}
}

// On a build with no change time, a real dispatch keeps every snapshot and must still say so
// on stderr: retention is off there, and the operator needs a signal on the dispatches they run.
//
// regression: #7275
func TestVerifySnapReportNoCtime(t *testing.T) {
	base := snapFixture(t)
	cache := filepath.Dir(filepath.Dir(base))
	real := time.Now()
	oldCache, oldNow, oldRoot, oldKnown := packetCacheDir, packetNow, verifySnapRoot, snapChangeTimeKnown
	t.Cleanup(func() {
		packetCacheDir, packetNow, verifySnapRoot, snapChangeTimeKnown = oldCache, oldNow, oldRoot, oldKnown
	})
	packetCacheDir = func() (string, error) { return cache, nil }
	verifySnapRoot = verifySnapBase
	packetNow = func() time.Time { return real.Add(verifySnapMaxAge + 24*time.Hour) }
	snapChangeTimeKnown = false
	snapWrite(t, filepath.Join(base, "idle--01", "f"), real.Add(-time.Hour))
	snapAgeTree(t, filepath.Join(base, "idle--01"), real.Add(-time.Hour))

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	reportVerifySnaps("", false)
	os.Stderr = oldStderr
	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	want := "keep " + filepath.Join(base, "idle--01") + " — could-not-check"
	if !strings.Contains(string(out), want) {
		t.Fatalf("a real dispatch with no change time did not report the kept snapshot; stderr: %q", out)
	}
	if _, err := os.Stat(filepath.Join(base, "idle--01", "f")); err != nil {
		t.Fatal(err)
	}
}

// The chmod before removal goes through a descriptor, and os.Root.OpenFile follows a final
// link inside the root, so a link in place of a directory must leave its target unchanged.
//
// regression: #7275
func TestVerifySnapChmodNoFollow(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "target"), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Join(dir, "target"), 0o700) })
	if err := os.Mkdir(filepath.Join(dir, "plain"), 0o500); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := makeSnapDirWritable(root, "link"); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(dir, "target")); err != nil || st.Mode().Perm() != 0o500 {
		t.Fatalf("chmod through a link changed its target: %v %v", st.Mode(), err)
	}
	if err := makeSnapDirWritable(root, "plain"); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(dir, "plain")); err != nil || st.Mode().Perm() != 0o700 {
		t.Fatalf("a read-only snapshot directory was not made writable: %v %v", st.Mode(), err)
	}
}
