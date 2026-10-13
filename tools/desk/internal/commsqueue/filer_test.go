package commsqueue

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeDeskfileOnPath installs testdata/fake-deskfile.sh as `deskfile` at the
// front of PATH and returns the file it logs a successful filing to. The fake
// enforces cmd/deskfile's `new` argv contract, so a filer whose argv the real
// verb would refuse is refused here too.
func fakeDeskfileOnPath(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake deskfile is a POSIX shell script")
	}
	src, err := os.ReadFile(filepath.Join("testdata", "fake-deskfile.sh"))
	if err != nil {
		t.Fatalf("read fake deskfile: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "deskfile"), src, 0o755); err != nil {
		t.Fatalf("install fake deskfile: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	log := filepath.Join(dir, "filed.log")
	t.Setenv("FAKE_DESKFILE_LOG", log)
	return log
}

// The quarantine filer's argv must be one `deskfile new` accepts: -R (there is
// no --repo), a --body-file (there is no inline --body), a roster role for
// --raised-by (not the loop name), and the Evidence block a "help wanted"
// filing must carry. Otherwise every held message's issue is refused.
//
// regression: #2726
func TestFilerArgvMatchesDeskfile(t *testing.T) {
	log := fakeDeskfileOnPath(t)
	env := sampleEnvelope("q-argv")
	if err := (DeskfileIssueFiler{Repo: "owner/repo"}).File(env, "outbound prose gate: hold-for-human"); err != nil {
		t.Fatalf("DeskfileIssueFiler.File = %v, want deskfile to accept the filing", err)
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("fake deskfile recorded nothing: %v", err)
	}
	got := string(raw)
	for _, want := range []string{"repo=owner/repo\n", "raised-by=worker\n", "labels=|help wanted\n", "- id: q-argv", "outbound prose gate: hold-for-human"} {
		if !strings.Contains(got, want) {
			t.Errorf("filing missing %q:\n%s", want, got)
		}
	}
}

// captureStderr runs fn with os.Stderr redirected and returns what it wrote.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	done := make(chan []byte)
	go func() {
		b, _ := io.ReadAll(r)
		done <- b
	}()
	fn()
	_ = w.Close()
	return string(<-done)
}

// Every caller discards the ErrQuarantined-wrapped filing error (the message is
// safe, so the return is informational), so a filing failure must say so
// itself: one stderr line, the message still held, the return unchanged.
//
// regression: #2726
func TestQuarantineFilingVisible(t *testing.T) {
	root := t.TempDir()
	filer := &fakeFiler{err: errors.New("deskfile new failed: exit status 5: refused: bad flags\nsecond line")}
	var qerr error
	out := captureStderr(t, func() {
		qerr = Quarantine(root, sampleEnvelope("q-loud"), "test reason", time.Now().UTC(), filer)
	})
	if !errors.Is(qerr, ErrQuarantined) {
		t.Fatalf("Quarantine = %v, want ErrQuarantined (the message is held)", qerr)
	}
	if held, _ := ListHeld(root); len(held) != 1 {
		t.Fatalf("held = %d items, want the message held", len(held))
	}
	if n := bytes.Count([]byte(out), []byte("\n")); n != 1 || !strings.Contains(out, "q-loud") || !strings.Contains(out, "refused: bad flags") {
		t.Fatalf("stderr = %q, want exactly one line naming q-loud and the filing error", out)
	}
}

// A filing that succeeds, or a filer that is not wired, stays silent.
func TestQuarantineQuietOnSuccess(t *testing.T) {
	out := captureStderr(t, func() {
		_ = Quarantine(t.TempDir(), sampleEnvelope("q-ok"), "r", time.Now().UTC(), &fakeFiler{})
		_ = Quarantine(t.TempDir(), sampleEnvelope("q-nil"), "r", time.Now().UTC(), nil)
	})
	if out != "" {
		t.Fatalf("stderr = %q, want nothing", out)
	}
}
