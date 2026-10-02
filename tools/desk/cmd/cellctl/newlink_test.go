package main

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unicode/utf16"
)

// newlink_test.go — the Windows link fallback and the scaffold rollback, table-tested on any host
// by passing the goos and the link primitives explicitly. A real junction / real
// ERROR_PRIVILEGE_NOT_HELD needs a Windows host; these tests drive the same decision code with
// the errno and primitives injected.

func privErr(o, n string) error {
	return &os.LinkError{Op: "symlink", Old: o, New: n, Err: errPrivilegeNotHeld}
}

// recLinker records which primitives ran. symlinkErr, when set, makes every symlink fail with it.
type recLinker struct {
	calls      []string
	symlinkErr func(o, n string) error
}

func (r *recLinker) linker(goos string) linker {
	return linker{
		goos: goos,
		symlink: func(o, n string) error {
			r.calls = append(r.calls, "symlink")
			if r.symlinkErr != nil {
				return r.symlinkErr(o, n)
			}
			return os.Symlink(o, n)
		},
		// A junction is stood in for by a symlink: what is under test is the DECISION to fall back.
		junction: func(t, l string) error { r.calls = append(r.calls, "junction"); return os.Symlink(t, l) },
		hardlink: func(o, n string) error { r.calls = append(r.calls, "hardlink"); return os.Link(o, n) },
	}
}

func TestLinkFallbackTable(t *testing.T) {
	accessDenied := func(o, n string) error { return &os.LinkError{Op: "symlink", Old: o, New: n, Err: syscall.Errno(5)} }
	cases := []struct {
		name    string
		goos    string
		fail    func(o, n string) error
		src     string // "dir" | "file" | "symlink"
		want    string // link kind, or "" for a refusal
		wantRun string // primitives that ran, joined
	}{
		{"unix symlink", "linux", nil, "dir", "symlink", "symlink"},
		{"windows symlink ok", "windows", nil, "file", "symlink", "symlink"},
		{"windows no-priv dir", "windows", privErr, "dir", "junction", "symlink,junction"},
		{"windows no-priv file", "windows", privErr, "file", "hardlink", "symlink,hardlink"},
		{"windows no-priv symlink src", "windows", privErr, "symlink", "", "symlink"},
		{"windows other errno", "windows", accessDenied, "dir", "", "symlink"},
		{"errno 1314 off windows", "linux", privErr, "dir", "", "symlink"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmp := t.TempDir()
			src := filepath.Join(tmp, "src")
			switch c.src {
			case "dir":
				must(t, os.Mkdir(src, 0o755))
			case "file":
				must(t, os.WriteFile(src, []byte("x"), 0o644))
			case "symlink":
				must(t, os.Mkdir(filepath.Join(tmp, "real"), 0o755))
				must(t, os.Symlink(filepath.Join(tmp, "real"), src))
			}
			r := &recLinker{symlinkErr: c.fail}
			got, err := r.linker(c.goos).link(src, filepath.Join(tmp, "dst"))
			if c.want == "" && err == nil {
				t.Fatalf("want a refusal, got %q", got)
			}
			if c.want != "" && (err != nil || got != c.want) {
				t.Fatalf("got %q, %v; want %q", got, err, c.want)
			}
			if run := strings.Join(r.calls, ","); run != c.wantRun {
				t.Fatalf("primitives run = %s, want %s", run, c.wantRun)
			}
			if c.want == "hardlink" {
				a, _ := os.Stat(src)
				b, _ := os.Stat(filepath.Join(tmp, "dst"))
				if !os.SameFile(a, b) {
					t.Fatal("hardlink fallback did not link the same file")
				}
			}
		})
	}
}

// An existing dst — a file, or a link into a directory — is refused before ANY primitive runs,
// so nothing is replaced and nothing is written through the existing link.
func TestLinkNeverReplacesDst(t *testing.T) {
	for _, kind := range []string{"file", "link"} {
		t.Run(kind, func(t *testing.T) {
			tmp := t.TempDir()
			src, dst, target := filepath.Join(tmp, "src"), filepath.Join(tmp, "dst"), filepath.Join(tmp, "target")
			must(t, os.Mkdir(src, 0o755))
			must(t, os.Mkdir(target, 0o755))
			if kind == "file" {
				must(t, os.WriteFile(dst, []byte("sentinel"), 0o644))
			} else {
				must(t, os.Symlink(target, dst))
			}
			for _, goos := range []string{"linux", "windows"} {
				r := &recLinker{symlinkErr: privErr}
				if _, err := r.linker(goos).link(src, dst); err == nil {
					t.Fatalf("%s: an existing dst must be refused", goos)
				}
				if len(r.calls) != 0 {
					t.Fatalf("%s: primitives ran against an existing dst: %v", goos, r.calls)
				}
			}
			if kind == "file" {
				if b, _ := os.ReadFile(dst); string(b) != "sentinel" {
					t.Fatal("existing dst was modified")
				}
			} else if ents, _ := os.ReadDir(target); len(ents) != 0 {
				t.Fatalf("something was written through the existing link: %v", ents)
			}
		})
	}
}

// The safety property the rollback rests on: a cell directory this run did not create is never
// the subject of a cleanup. Pre-existing with a sentinel => error returned, nothing deleted.
func TestScaffoldPreexistingRefused(t *testing.T) {
	d := filepath.Join(t.TempDir(), "cell")
	must(t, os.Mkdir(d, 0o755))
	sentinel := filepath.Join(d, "sentinel")
	must(t, os.WriteFile(sentinel, []byte("keep"), 0o644))

	s, err := beginCellScaffold("windows", d)
	if err == nil || s != nil {
		t.Fatalf("beginCellScaffold on a pre-existing dir = %v, %v; want a refusal and no journal", s, err)
	}
	if err := s.rollback(); err == nil {
		t.Fatal("rollback without a journal must refuse")
	}
	if b, err := os.ReadFile(sentinel); err != nil || string(b) != "keep" {
		t.Fatalf("sentinel lost: %v", err)
	}
	if ents, _ := os.ReadDir(d); len(ents) != 1 {
		t.Fatalf("pre-existing cell dir changed: %v", ents)
	}
}

// rollback removes exactly the journal and never follows a link into its target.
func TestScaffoldRollbackExact(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "operator-config")
	must(t, os.Mkdir(target, 0o755))
	must(t, os.WriteFile(filepath.Join(target, "roster.env"), []byte("keep"), 0o600))
	d := filepath.Join(tmp, "cells", "c")
	must(t, os.Mkdir(filepath.Dir(d), 0o755))

	s, err := beginCellScaffold("linux", d)
	must(t, err)
	must(t, s.mkdir(filepath.Join(d, "home"), filepath.Join(d, "home", ".config")))
	must(t, s.link(hostLinker(), target, filepath.Join(d, "home", ".config", "assay"), "the config home"))
	must(t, s.writeNew(filepath.Join(d, "cell.env"), "CELL=c\n"))
	if err := s.mkdir(filepath.Join(tmp, "outside")); err == nil {
		t.Fatal("a create outside the cell must be refused")
	}
	must(t, s.rollback())

	if exists(d) {
		t.Fatal("cell dir survived rollback")
	}
	if !isDir(filepath.Dir(d)) {
		t.Fatal("rollback removed the cells root, which this run did not create")
	}
	if b, err := os.ReadFile(filepath.Join(target, "roster.env")); err != nil || string(b) != "keep" {
		t.Fatalf("rollback reached through the link into its target: %v", err)
	}
}

// Something this run did not create, inside a directory it did, is left standing: the
// rollback reports the leftover instead of emptying the directory.
func TestRollbackKeepsForeignFiles(t *testing.T) {
	d := filepath.Join(t.TempDir(), "c")
	s, err := beginCellScaffold("linux", d)
	must(t, err)
	must(t, s.mkdir(filepath.Join(d, "home")))
	foreign := filepath.Join(d, "home", "not-ours")
	must(t, os.WriteFile(foreign, []byte("x"), 0o644))
	if err := s.rollback(); err == nil || !strings.Contains(err.Error(), "NOT fully removed") {
		t.Fatalf("rollback over a foreign file = %v; want a NOT-fully-removed error", err)
	}
	if !exists(foreign) {
		t.Fatal("rollback deleted a file this run did not create")
	}
}

// End to end: a house scaffold whose link fails (a Windows host with no symlink privilege and
// a junction that also fails) dies with the remediation, leaves NO cell dir, and a retry works.
func TestNewHouseLinkFailRollback(t *testing.T) {
	tmp := t.TempDir()
	home, cfg, repo, root := filepath.Join(tmp, "home"), filepath.Join(tmp, "cfg"), filepath.Join(tmp, "repo"), filepath.Join(tmp, "cells")
	for _, p := range []string{filepath.Join(home, ghConfigRelPath), cfg, filepath.Join(repo, "docs", "streams")} {
		must(t, os.MkdirAll(p, 0o755))
	}
	must(t, os.WriteFile(filepath.Join(cfg, "roster.env"), []byte("keep"), 0o600))
	must(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n"), 0o644))
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Skipf("git init: %v %s", err, out)
	}
	e := &Env{vals: map[string]string{}, set: map[string]bool{}}
	e.Put("HOME", home)
	e.Put("ASSAY_CONFIG_HOME", cfg)
	roots := "o/r=" + repo

	defer func(prev func() linker) { newLinker = prev }(newLinker)
	r := &recLinker{symlinkErr: func(o, n string) error {
		if filepath.Base(n) == "gh" {
			return privErr(o, n)
		}
		return os.Symlink(o, n)
	}}
	l := r.linker("windows")
	l.junction = func(string, string) error { return errors.New("junction refused") }
	newLinker = func() linker { return l }

	stderr := captureStderr(t, func() {
		assertDies(t, "link failure", func() { newHouse(e, root, "c", repo, roots, "the-desk", "8787") })
	})
	if !strings.Contains(stderr, "removed the partial cell") || !strings.Contains(stderr, "Developer Mode") {
		t.Fatalf("refusal lacks the cleanup note or the remediation: %q", stderr)
	}
	if exists(filepath.Join(root, "c")) {
		t.Fatal("a failed scaffold left the partial cell behind")
	}
	if b, err := os.ReadFile(filepath.Join(cfg, "roster.env")); err != nil || string(b) != "keep" {
		t.Fatalf("the operator config home was touched: %v", err)
	}

	newLinker = hostLinker
	captureStdout(t, func() { newHouse(e, root, "c", repo, roots, "the-desk", "8787") })
	if link, _ := os.Readlink(filepath.Join(root, "c", "home", ".config", "assay")); link != cfg {
		t.Fatalf("retry did not scaffold the cell: config link %q", link)
	}

	// A pre-existing cell dir is refused through newHouse too, and its contents survive.
	sentinel := filepath.Join(root, "c", "sentinel")
	must(t, os.WriteFile(sentinel, []byte("keep"), 0o644))
	assertDies(t, "pre-existing", func() { newHouse(e, root, "c", repo, roots, "the-desk", "8787") })
	if !exists(sentinel) || !exists(filepath.Join(root, "c", "cell.env")) {
		t.Fatal("a refusal over a pre-existing cell deleted something")
	}
}

func TestGhLinkSource(t *testing.T) {
	e := &Env{vals: map[string]string{}, set: map[string]bool{}}
	e.Put("APPDATA", "/appdata")
	if got := ghLinkSource("linux", e, "/h"); got != filepath.Join("/h", ghConfigRelPath) {
		t.Errorf("linux: %s", got)
	}
	if got := ghLinkSource("windows", e, "/h"); got != filepath.Join("/appdata", "GitHub CLI") {
		t.Errorf("windows: %s", got)
	}
	e.Put("GH_CONFIG_DIR", "/explicit")
	if got := ghLinkSource("windows", e, "/h"); got != "/explicit" {
		t.Errorf("windows GH_CONFIG_DIR: %s", got)
	}
}

func TestJunctionReparseData(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`C:\Users\op\.config\assay`, `C:\Users\op\.config\assay`},
		{`C:/Users/op/.config/assay/`, `C:\Users\op\.config\assay`},
		{`d:\`, `d:\`},
	} {
		b, err := junctionReparseData(tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}
		le := binary.LittleEndian
		if le.Uint32(b) != reparseTagMountPoint || int(le.Uint16(b[4:]))+8 != len(b) {
			t.Fatalf("%s: bad header", tc.in)
		}
		str := func(off, n int) string {
			u := make([]uint16, n/2)
			for i := range u {
				u[i] = le.Uint16(b[16+off+2*i:])
			}
			return string(utf16.Decode(u))
		}
		so, sn, po, pn := int(le.Uint16(b[8:])), int(le.Uint16(b[10:])), int(le.Uint16(b[12:])), int(le.Uint16(b[14:]))
		if got := str(so, sn); got != `\??\`+tc.want {
			t.Errorf("%s: substitute name %q", tc.in, got)
		}
		if got := str(po, pn); got != tc.want {
			t.Errorf("%s: print name %q", tc.in, got)
		}
	}
	for _, bad := range []string{`relative\dir`, `\\server\share\x`, `//?/C:/x`, `C:\a\..\b`, `C:\a\\b`, `C:\` + strings.Repeat("a", 9000)} {
		if _, err := junctionReparseData(bad); err == nil {
			t.Errorf("junction target %q must be refused", bad)
		}
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// captureStderr is plan_test.go's captureStdout for stderr, where die() writes its refusal.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	rd, wr, err := os.Pipe()
	must(t, err)
	prev := os.Stderr
	os.Stderr = wr
	done := make(chan string)
	go func() { b, _ := io.ReadAll(rd); done <- string(b) }()
	defer func() { os.Stderr = prev }()
	fn()
	wr.Close()
	return <-done
}
