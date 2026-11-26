//go:build darwin || linux

package cellcache

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// workDir plants tmp/<name> holding one nested file, with the file and every
// directory stamped at mtime.
func workDir(t *testing.T, p Policy, name string, mtime time.Time) string {
	t.Helper()
	dir := filepath.Join(p.Root, tmpDir, name)
	file := filepath.Join(dir, "b001", "exe", "a.out")
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{file, filepath.Dir(file), filepath.Join(dir, "b001"), dir} {
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func present(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

// TestCacheTmpPolicy: a managed cache exports GOTMPDIR under its root and
// enrollment creates it; a policy recorded before the field existed
// serializes unchanged, exports no GOTMPDIR and keeps its custody.
//
// regression: #7275
func TestCacheTmpPolicy(t *testing.T) {
	p := fixture(t)
	want := "GOTMPDIR=" + filepath.Join(p.Root, tmpDir)
	if !p.GoTmp || !slices.Contains(p.Env(), want) {
		t.Fatalf("default policy does not export %s: %v", want, p.Env())
	}
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(p.Root, tmpDir)); err != nil || !st.IsDir() || st.Mode().Perm() != 0o700 {
		t.Fatalf("enrollment did not create an owner-only GOTMPDIR: %v %v", st, err)
	}
	l.Finish(true)

	legacy := fixture(t)
	legacy.GoTmp = false
	b, _ := json.Marshal(legacy)
	if strings.Contains(string(b), "gotmpdir") {
		t.Fatalf("legacy policy serialization changed: %s", b)
	}
	for _, kv := range legacy.Env() {
		if strings.HasPrefix(kv, "GOTMPDIR=") {
			t.Fatalf("legacy policy exports %s", kv)
		}
	}
	l, err = Acquire(legacy.Env())
	if err != nil {
		t.Fatal("legacy consumer refused", err)
	}
	l.Finish(true)
	if present(t, filepath.Join(legacy.Root, tmpDir)) {
		t.Fatal("a legacy policy's enrollment created a GOTMPDIR it does not export")
	}
	// GOTMPDIR must match the policy exactly, like GOCACHE.
	env := slices.Clone(p.Env())
	for i, kv := range env {
		if strings.HasPrefix(kv, "GOTMPDIR=") {
			env[i] = "GOTMPDIR=/tmp"
		}
	}
	if _, err := Acquire(env); err == nil {
		t.Fatal("consumer whose GOTMPDIR differs from its policy was admitted")
	}
}

// TestCacheTmpGoEnv: inside a managed consumer's environment, the Go
// toolchain resolves GOTMPDIR under the cache root and builds there.
//
// regression: #7275
func TestCacheTmpGoEnv(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go unavailable")
	}
	p := fixture(t)
	p.Budget = 1 << 30
	env, _ := goEnv(t, goBin, p)
	l, err := Acquire(env)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Finish(true)
	cmd := exec.Command(goBin, "env", "GOTMPDIR")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != filepath.Join(p.Root, tmpDir) {
		t.Fatalf("go env GOTMPDIR = %q (%v), want %s", out, err, filepath.Join(p.Root, tmpDir))
	}
	work := t.TempDir()
	if err = os.WriteFile(filepath.Join(work, "go.mod"), []byte("module tmp.fixture\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(work, "m.go"), []byte("package main\nfunc main(){}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(goBin, "build", "-work", "-o", os.DevNull, ".")
	cmd.Dir, cmd.Env = work, env
	out, err = cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "WORK="+filepath.Join(p.Root, tmpDir, goBuildPrefix)) {
		t.Fatalf("the build's work directory is not under the managed GOTMPDIR: %s %v", out, err)
	}
}

// TestCacheTmpSweep: a maintenance pass removes only go-build directories in
// which nothing changed within the build trim age; a dry run lists the same
// directory and removes nothing.
//
// regression: #7275
func TestCacheTmpSweep(t *testing.T) {
	p := fixture(t)
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	l.Finish(true) // no live consumer: only the age rule applies
	now := time.Now()
	old := now.Add(-2 * p.BuildAge())
	idle := workDir(t, p, "go-build111", old)
	deep := workDir(t, p, "go-build222", old)
	if err := os.Chtimes(filepath.Join(deep, "b001", "exe", "a.out"), now, now); err != nil {
		t.Fatal(err)
	}
	recent := workDir(t, p, "go-build333", now.Add(-time.Hour))
	foreign := workDir(t, p, "not-a-build", old)

	dry, err := Check(p, true)
	if err != nil {
		t.Fatal(err)
	}
	if dry.Tmp == nil || !slices.Equal(dry.Tmp.Removed, []string{"go-build111"}) || dry.Tmp.Kept != 2 || dry.Tmp.Reason != "" || len(dry.Tmp.Held) != 0 {
		t.Fatalf("dry sweep: %+v", dry.Tmp)
	}
	if !present(t, idle) {
		t.Fatal("a dry run removed a work directory")
	}
	r, err := Check(p, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Tmp == nil || !slices.Equal(r.Tmp.Removed, []string{"go-build111"}) || r.Tmp.RemovedBytes != int64(len("binary")) {
		t.Fatalf("sweep: %+v", r.Tmp)
	}
	if present(t, idle) {
		t.Fatal("an idle work directory past the build trim age was kept")
	}
	for _, keep := range []string{deep, recent, foreign} {
		if !present(t, keep) {
			t.Errorf("%s was removed", filepath.Base(keep))
		}
	}
}

// TestCacheTmpSweepHeldByRecord: a work directory older than the trim age but
// created after a live consumer enrolled may belong to it, and is kept; one
// that predates every live consumer is removed. A record whose enrollment time
// cannot be read refuses the whole sweep.
//
// regression: #7275
func TestCacheTmpSweepHeldByRecord(t *testing.T) {
	p := fixture(t)
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Finish(true)
	now := time.Now()
	enrolled := now.Add(-3 * p.BuildAge())
	record := filepath.Join(p.Root, l.record)
	if err := os.WriteFile(record, []byte(enrolled.UTC().Format(time.RFC3339Nano)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(record, enrolled, enrolled); err != nil {
		t.Fatal(err)
	}
	held := workDir(t, p, "go-build444", now.Add(-2*p.BuildAge()))
	orphan := workDir(t, p, "go-build555", now.Add(-4*p.BuildAge()))
	r, err := Check(p, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Tmp == nil || !slices.Equal(r.Tmp.Removed, []string{"go-build555"}) {
		t.Fatalf("sweep under a live record: %+v", r.Tmp)
	}
	if !present(t, held) || present(t, orphan) {
		t.Fatalf("held kept=%v orphan kept=%v", present(t, held), present(t, orphan))
	}
	// The keep is visible: the held directory is named with the record that holds it.
	if len(r.Tmp.Held) != 1 || !strings.HasPrefix(r.Tmp.Held[0], "go-build444: ") || !strings.Contains(r.Tmp.Held[0], l.record) {
		t.Fatalf("held directory not reported with its holding record: %q", r.Tmp.Held)
	}

	orphan = workDir(t, p, "go-build666", now.Add(-4*p.BuildAge()))
	if err := os.WriteFile(record, []byte("not a time\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err = Check(p, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Tmp == nil || len(r.Tmp.Removed) != 0 || !strings.HasPrefix(r.Tmp.Reason, "could-not-check") || !present(t, orphan) {
		t.Fatalf("an unreadable record did not refuse the sweep: %+v", r.Tmp)
	}
}

// TestCacheTmpMaintainFresh: maintenance that skips the measurement because
// the recorded one is fresh still sweeps GOTMPDIR, and still records nothing.
//
// regression: #7275
func TestCacheTmpMaintainFresh(t *testing.T) {
	p := fixture(t)
	p.Budget = 1 << 30
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	l.Finish(true)
	old := time.Now().Add(-2 * AdmitMtimeSlack)
	for _, name := range []string{"build", "mod"} {
		if err := os.Chtimes(filepath.Join(p.Root, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	idle := workDir(t, p, "go-build777", time.Now().Add(-2*p.BuildAge()))
	before, err := os.ReadFile(p.ReportPath())
	if err != nil {
		t.Fatal(err)
	}
	r, err := Maintain(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(p.ReportPath())
	if err != nil {
		t.Fatal(err)
	}
	if r.Admission != "precheck" || string(before) != string(after) {
		t.Fatalf("fresh maintenance re-measured: admission %q", r.Admission)
	}
	if r.Tmp == nil || !slices.Equal(r.Tmp.Removed, []string{"go-build777"}) || present(t, idle) {
		t.Fatalf("fresh maintenance did not sweep GOTMPDIR: %+v", r.Tmp)
	}
}
