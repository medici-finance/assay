package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unicode/utf16"
)

// newlink.go — how `cellctl new` makes the links a cell home carries, and how it backs out of a
// half-made cell. Two hazards shape it:
//
//   - A stock Windows host (no Developer Mode, not elevated) cannot create symlinks at all:
//     os.Symlink fails with ERROR_PRIVILEGE_NOT_HELD. A directory can still be linked by a
//     JUNCTION and a regular file by a HARDLINK, neither of which needs the privilege, and
//     `cellctl check` already reads a junction back through os.Readlink.
//   - A scaffold that dies after creating the cell directory leaves a half-cell that blocks every
//     retry ("cellctl new never overwrites a cell"). The scaffold therefore journals each path it
//     creates and, on failure, removes exactly those paths — non-recursively, newest first — and
//     nothing else. It never calls os.RemoveAll: a recursive delete that met a junction could walk
//     into the operator's real config home, which is the one directory this file links TO.

// errPrivilegeNotHeld is Windows' ERROR_PRIVILEGE_NOT_HELD (1314), the errno os.Symlink returns
// on a host without SeCreateSymbolicLinkPrivilege. syscall.Errno exists on every GOOS, so the
// fallback decision is table-tested on any host.
const errPrivilegeNotHeld = syscall.Errno(1314)

// linker is the set of primitives a link is made with, plus the goos whose rules apply. Tests
// substitute the primitives and the goos; production uses hostLinker.
type linker struct {
	goos     string
	symlink  func(oldname, newname string) error
	junction func(target, link string) error
	hardlink func(oldname, newname string) error
}

func hostLinker() linker {
	return linker{goos: runtime.GOOS, symlink: os.Symlink, junction: createJunction, hardlink: os.Link}
}

// newLinker is the linker `cellctl new` uses; a package variable only so a test can make a link
// fail mid-scaffold.
var newLinker = hostLinker

// link makes dst refer to src and reports which kind of link it made.
//
// It never replaces anything: an existing dst — file, directory, symlink or junction — is a
// refusal before any primitive runs, so a link is never written THROUGH an existing link either.
// A symlink is always tried first (it is what every other host gets, and what the shell oracle
// makes). Only on windows, and only when the symlink failed for lack of the privilege, does it
// fall back — and only for a src that is itself a plain directory (junction) or a plain regular
// file (hardlink). A src that is a symlink, junction or other reparse point is refused rather
// than followed.
func (l linker) link(src, dst string) (string, error) {
	if _, err := os.Lstat(dst); err == nil {
		return "", fmt.Errorf("refusing to link %s: something already exists there (cellctl new never replaces an entry)", dst)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	serr := l.symlink(src, dst)
	if serr == nil {
		return "symlink", nil
	}
	if l.goos != "windows" || !errors.Is(serr, errPrivilegeNotHeld) {
		return "", serr
	}
	st, err := os.Lstat(src)
	if err != nil {
		return "", fmt.Errorf("%w; no fallback: %w", serr, err)
	}
	switch t := st.Mode().Type(); {
	case t == fs.ModeDir:
		if err := l.junction(src, dst); err != nil {
			return "", fmt.Errorf("%w; junction fallback: %w", serr, err)
		}
		return "junction", nil
	case st.Mode().IsRegular():
		if err := l.hardlink(src, dst); err != nil {
			return "", fmt.Errorf("%w; hardlink fallback: %w", serr, err)
		}
		return "hardlink", nil
	default:
		return "", fmt.Errorf("%w; no fallback: %s is neither a plain directory nor a regular file (mode %v) — refusing to link through it", serr, src, st.Mode())
	}
}

// linkRemediation is the one-line hint a failed link ends with.
func linkRemediation(goos string, err error) string {
	if goos == "windows" && errors.Is(err, errPrivilegeNotHeld) {
		return "enable Windows Developer Mode (or run cellctl from an elevated shell) so symlinks can be created, then re-run cellctl new"
	}
	return "fix the cause above, then re-run cellctl new"
}

// cellScaffold is the creation journal of ONE `cellctl new` run. It exists only for a cell
// directory this run itself created: beginCellScaffold makes the directory with os.Mkdir, which
// fails on anything already there, so a pre-existing directory never gets a journal and can never
// be rolled back.
type cellScaffold struct {
	dir     string
	goos    string
	created []string // every path this run created under dir, in creation order
}

// beginCellScaffold makes dir's PARENTS the way the oracle's `mkdir -p "$d/…"` makes them, so a
// nested cell name (`cellctl new team/a`) scaffolds as it always has, and then dir itself
// exclusively. The parents sit outside the cell and are not journaled: a rollback never removes
// them (as it never removes the cells root), and an empty one left behind never blocks a retry.
func beginCellScaffold(goos, dir string) (*cellScaffold, error) {
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return nil, fmt.Errorf("cannot create %s: %v", filepath.Dir(dir), err)
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("%s already exists (cellctl new never overwrites a cell — remove it yourself, or pick another name)", dir)
		}
		return nil, fmt.Errorf("cannot create %s: %v", dir, err)
	}
	return &cellScaffold{dir: dir, goos: goos}, nil
}

// owns reports whether p is strictly inside the scaffold's cell directory.
func (s *cellScaffold) owns(p string) bool {
	rel, err := filepath.Rel(s.dir, p)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// mkdir is the oracle's `mkdir -p` INSIDE the cell: every missing directory between the cell and
// p is made parent first and journaled, so the rollback removes them child first. p itself must
// not exist — the journal only ever names what this run created.
func (s *cellScaffold) mkdir(paths ...string) error {
	for _, p := range paths {
		if !s.owns(p) {
			return fmt.Errorf("refusing to create %s: outside the cell %s", p, s.dir)
		}
		missing := []string{p}
		for q := filepath.Dir(p); s.owns(q); q = filepath.Dir(q) {
			if _, err := os.Lstat(q); err == nil {
				break
			} else if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("cannot create %s: %v", p, err)
			}
			missing = append(missing, q)
		}
		for i := len(missing) - 1; i >= 0; i-- {
			if err := os.Mkdir(missing[i], 0o755); err != nil {
				return fmt.Errorf("cannot create %s: %v", missing[i], err)
			}
			s.created = append(s.created, missing[i])
		}
	}
	return nil
}

// link links dst to src with l, journaling dst. what names the link in an error.
func (s *cellScaffold) link(l linker, src, dst, what string) error {
	if !s.owns(dst) {
		return fmt.Errorf("refusing to link %s: outside the cell %s", dst, s.dir)
	}
	how, err := l.link(src, dst)
	if err != nil {
		return fmt.Errorf("cannot link %s: %w", what, err)
	}
	s.created = append(s.created, dst)
	if how != "symlink" {
		fmt.Printf("[new] note: no symlink privilege — linked %s by %s\n", dst, how)
	}
	return nil
}

// linkIfPresent is the oracle's `[[ -e <src> ]] && ln -s <src> <dst>`: an absent src is not an
// error, but a present one that cannot be linked is.
func (s *cellScaffold) linkIfPresent(l linker, src, dst, what string) error {
	if !exists(src) {
		return nil
	}
	return s.link(l, src, dst, what)
}

func (s *cellScaffold) chmod700(paths ...string) error {
	for _, p := range paths {
		if err := os.Chmod(p, 0o700); err != nil {
			return fmt.Errorf("cannot chmod %s: %v", p, err)
		}
	}
	return nil
}

// writeNew creates p (it must not exist) and writes body to it, journaling p the moment it exists.
func (s *cellScaffold) writeNew(p, body string) error {
	if !s.owns(p) {
		return fmt.Errorf("refusing to write %s: outside the cell %s", p, s.dir)
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("cannot write %s: %v", p, err)
	}
	s.created = append(s.created, p)
	_, werr := f.WriteString(body)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return fmt.Errorf("cannot write %s: %v", p, werr)
	}
	return nil
}

// rollback removes what this run created, newest first, then the cell directory itself. Each
// removal is os.Remove — one entry, never a recursive delete — so a link is removed as a link
// and its target is never entered, and a directory something else wrote into is left standing
// (its removal fails and is reported) rather than emptied. A nil scaffold — the cell directory
// pre-existed — refuses and removes nothing.
func (s *cellScaffold) rollback() error {
	if s == nil || s.dir == "" {
		return errors.New("refusing to clean up: this run did not create the cell directory, so nothing in it is its to delete")
	}
	var failed []string
	for i := len(s.created) - 1; i >= 0; i-- {
		p := s.created[i]
		if !s.owns(p) {
			failed = append(failed, p+": outside the cell, not removed")
			continue
		}
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			failed = append(failed, err.Error())
		}
	}
	if err := os.Remove(s.dir); err != nil && !errors.Is(err, fs.ErrNotExist) {
		failed = append(failed, err.Error())
	}
	s.created = nil
	if len(failed) > 0 {
		return fmt.Errorf("partial cell %s NOT fully removed (%s)", s.dir, strings.Join(failed, "; "))
	}
	return nil
}

// must rolls the scaffold back and dies, in one line, when err is non-nil.
func (s *cellScaffold) must(err error) {
	if err == nil {
		return
	}
	if rbErr := s.rollback(); rbErr != nil {
		die("new: %v; %v — inspect it by hand before re-running; %s", err, rbErr, linkRemediation(s.goos, err))
	}
	die("new: %v; removed the partial cell %s — %s", err, s.dir, linkRemediation(s.goos, err))
}

// write is writeNew that rolls back and dies on failure, so its call reads like writeFile.
func (s *cellScaffold) write(p, body string) { s.must(s.writeNew(p, body)) }

// ghLinkSource is the GitHub CLI config directory a house cell links in. Everywhere but windows
// it stays <home>/.config/gh, the oracle's source (the parity harness diffs the tree). On windows
// the gh CLI keeps its config under %APPDATA%\GitHub CLI, so the source follows gh's own
// precedence via ghConfigDirFor, the one resolver (homeresolve.go).
func ghLinkSource(goos string, e *Env, home string) string {
	if goos != "windows" {
		return filepath.Join(home, ghConfigRelPath)
	}
	if p, err := ghConfigDirFor(goos, e); err == nil {
		return p
	}
	return filepath.Join(home, ghConfigRelPath)
}

// ---- junction reparse data (built on every host so it is table-tested everywhere) ----

const (
	reparseTagMountPoint    = 0xA0000003 // IO_REPARSE_TAG_MOUNT_POINT
	maxReparseDataBufSize   = 16 * 1024  // MAXIMUM_REPARSE_DATA_BUFFER_SIZE
	reparseHeaderSize       = 8          // tag(4) + data length(2) + reserved(2)
	mountPointHeaderSize    = 8          // subst offset/length + print offset/length, 2 bytes each
	junctionSubstNamePrefix = `\??\`
)

// junctionReparseData is the REPARSE_DATA_BUFFER that turns an empty directory into a junction
// to target. target must be a drive-absolute windows path (`C:\…` or `C:/…`) with no `.`/`..` or
// empty elements: a junction's substitute name is an NT path the kernel does not normalise, and a
// network or device target is refused outright, as for every other cellctl path.
func junctionReparseData(target string) ([]byte, error) {
	if err := cellPathCheck("windows", target); err != nil {
		return nil, fmt.Errorf("junction target %s: %v", target, err)
	}
	t := strings.TrimRight(strings.ReplaceAll(target, "/", `\`), `\`)
	if len(t) < 2 || t[1] != ':' {
		return nil, fmt.Errorf("junction target %s: must be a drive-absolute path", target)
	}
	if len(t) > 2 {
		for _, el := range strings.Split(t[3:], `\`) {
			if el == "" || el == "." || el == ".." {
				return nil, fmt.Errorf("junction target %s: empty, '.' or '..' path element refused", target)
			}
		}
	} else {
		t += `\`
	}
	subst := utf16.Encode([]rune(junctionSubstNamePrefix + t))
	printName := utf16.Encode([]rune(t))
	substBytes, printBytes := 2*len(subst), 2*len(printName)
	dataLen := mountPointHeaderSize + substBytes + 2 + printBytes + 2
	if reparseHeaderSize+dataLen > maxReparseDataBufSize {
		return nil, fmt.Errorf("junction target %s: too long for a reparse point", target)
	}
	b := make([]byte, reparseHeaderSize+dataLen)
	le := binary.LittleEndian
	le.PutUint32(b[0:], reparseTagMountPoint)
	le.PutUint16(b[4:], uint16(dataLen))
	le.PutUint16(b[8:], 0)                     // SubstituteNameOffset
	le.PutUint16(b[10:], uint16(substBytes))   // SubstituteNameLength (no terminator)
	le.PutUint16(b[12:], uint16(substBytes+2)) // PrintNameOffset
	le.PutUint16(b[14:], uint16(printBytes))   // PrintNameLength
	off := reparseHeaderSize + mountPointHeaderSize
	for _, u := range subst {
		le.PutUint16(b[off:], u)
		off += 2
	}
	off += 2 // NUL
	for _, u := range printName {
		le.PutUint16(b[off:], u)
		off += 2
	}
	return b, nil
}
