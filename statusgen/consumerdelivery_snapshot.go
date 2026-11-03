package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// Consumer snapshots never extract archive links. Regular entries are written
// exclusively first; every link is resolved against the complete immutable tree
// before any filesystem link is created or any stream is read.
type consumerSnapshot struct {
	modes map[string]string
	dirs  map[string]bool
	links map[string]string
}

func consumerSnapshotPath(p string) bool {
	if p == "." || path.Clean(p) != p || !filepath.IsLocal(filepath.FromSlash(p)) || strings.ContainsAny(p, "\\\x00") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || strings.EqualFold(part, ".git") {
			return false
		}
	}
	return true
}

func consumerBoardLink(p string) bool {
	p = strings.ToLower(p)
	return p == "docs" || p == "docs/streams" || p == "docs/archive" ||
		strings.HasPrefix(p, "docs/streams/") || strings.HasPrefix(p, "docs/archive/")
}

func newConsumerSnapshot(entries []evaluation.TreeEntry, changed map[string]bool) (*consumerSnapshot, error) {
	s := &consumerSnapshot{modes: map[string]string{}, dirs: map[string]bool{".": true}, links: map[string]string{}}
	for _, e := range entries {
		if !consumerSnapshotPath(e.Path) || s.modes[e.Path] != "" {
			return nil, fmt.Errorf("invalid or duplicate delivered path: %q", e.Path)
		}
		switch e.Mode {
		case "100644", "100755":
		case "120000":
			if changed[e.Path] || consumerBoardLink(e.Path) {
				return nil, fmt.Errorf("delivery snapshot link is changed or on a board input: %s", e.Path)
			}
		default:
			return nil, fmt.Errorf("delivery snapshot contains an unsupported entry or submodule: %s", e.Path)
		}
		s.modes[e.Path] = e.Mode
		for p := path.Dir(e.Path); p != "."; p = path.Dir(p) {
			s.dirs[p] = true
		}
	}
	for p := range s.dirs {
		if s.modes[p] != "" {
			return nil, fmt.Errorf("delivered file is also a directory: %s", p)
		}
	}
	return s, nil
}

func materializeConsumerDelivery(archive *exec.Cmd, dest string, entries []evaluation.TreeEntry, changed map[string]bool) error {
	s, err := newConsumerSnapshot(entries, changed)
	if err != nil {
		return err
	}
	var diagnostic bytes.Buffer
	archive.Stderr = &diagnostic
	stream, err := archive.StdoutPipe()
	if err != nil {
		return err
	}
	defer stream.Close()
	if err := archive.Start(); err != nil {
		return err
	}
	materialErr := s.extract(stream, dest)
	if materialErr != nil {
		_ = archive.Process.Kill()
	}
	_ = stream.Close()
	archiveErr := archive.Wait()
	if materialErr != nil {
		return materialErr
	}
	if archiveErr != nil {
		return fmt.Errorf("git archive failed: %s", strings.TrimSpace(diagnostic.String()))
	}
	return nil
}

func (s *consumerSnapshot) extract(input io.Reader, dest string) error {
	initial, err := os.ReadDir(dest)
	if err != nil {
		return err
	}
	if len(initial) != 0 {
		return fmt.Errorf("consumer snapshot destination is not empty")
	}
	reader := tar.NewReader(input)
	seen := map[string]bool{}
	for {
		h, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		} // metadata only; effective paths are checked below
		name := h.Name
		if h.Typeflag == tar.TypeDir {
			name = strings.TrimSuffix(name, "/")
		}
		if !consumerSnapshotPath(name) {
			return fmt.Errorf("unsafe delivery archive path: %q", name)
		}
		if h.Typeflag == tar.TypeDir {
			if !s.dirs[name] {
				return fmt.Errorf("untracked archive directory: %s", name)
			}
			continue
		}
		mode := s.modes[name]
		if mode == "" || seen[name] {
			return fmt.Errorf("untracked or duplicate archive entry: %s", name)
		}
		switch h.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			if mode != "100644" && mode != "100755" {
				return fmt.Errorf("archive type differs from delivered mode: %s", name)
			}
			if err := writeConsumerSnapshotFile(dest, name, mode, reader); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if mode != "120000" || h.Size != 0 {
				return fmt.Errorf("archive link differs from delivered mode: %s", name)
			}
			s.links[name] = h.Linkname
		default:
			return fmt.Errorf("unsupported delivery archive entry: %s", name)
		}
		seen[name] = true
	}
	// Drain Git's tar padding before waiting for the producer, avoiding SIGPIPE.
	if _, err := io.Copy(io.Discard, input); err != nil {
		return err
	}
	for p := range s.modes {
		if !seen[p] {
			return fmt.Errorf("delivery archive omitted a tracked blob (for example export-ignore): %s", p)
		}
	}
	names := make([]string, 0, len(s.links))
	for p := range s.links {
		names = append(names, p)
	}
	sort.Strings(names)
	resolved := map[string]string{}
	for _, name := range names {
		p, err := s.resolve(name)
		if err != nil {
			return err
		}
		resolved[name] = p
	}
	// No more archive writes occur after this point. All targets and chains are
	// already proved in-tree, so creating a link cannot redirect extraction.
	for _, name := range names {
		p := filepath.Join(dest, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.Symlink(s.links[name], p); err != nil {
			return fmt.Errorf("cannot materialize checked link %s: %w", name, err)
		}
	}
	physicalRoot, err := filepath.EvalSymlinks(dest)
	if err != nil {
		return err
	}
	for _, name := range names {
		actual, err := filepath.EvalSymlinks(filepath.Join(dest, filepath.FromSlash(name)))
		want := filepath.Join(physicalRoot, filepath.FromSlash(resolved[name]))
		if err != nil || actual != want {
			return fmt.Errorf("filesystem resolution differs from immutable link target: %s", name)
		}
	}
	return nil
}

func writeConsumerSnapshotFile(dest, name, mode string, source io.Reader) error {
	p := filepath.Join(dest, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	perm := os.FileMode(0o644)
	if mode == "100755" {
		perm = 0o755
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, source)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// Resolve path components in order, expanding links before handling a following
// '..'. Lexically cleaning the target first would miss an in-tree alias followed
// by a parent step that actually escapes. This reads metadata, never host paths.
func (s *consumerSnapshot) resolve(name string) (string, error) {
	pending := strings.Split(name, "/")
	var parts []string
	linksWalked := 0
	for len(pending) != 0 {
		part := pending[0]
		pending = pending[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if len(parts) == 0 {
				return "", fmt.Errorf("delivery link escapes snapshot: %s", name)
			}
			parts = parts[:len(parts)-1]
			continue
		}
		candidate := strings.Join(append(append([]string{}, parts...), part), "/")
		if s.modes[candidate] == "120000" {
			// Match filepath.EvalSymlinks' ceiling. Revisiting a link with
			// fewer pending components can be finite; link identity alone
			// must not be mistaken for a cycle.
			linksWalked++
			if linksWalked > 255 {
				return "", fmt.Errorf("cyclic or excessive-depth delivery link: %s", name)
			}
			target := s.links[candidate]
			if target == "" || strings.HasPrefix(target, "/") || filepath.IsAbs(target) || filepath.VolumeName(target) != "" ||
				strings.ContainsAny(target, "\\\x00") || (len(target) >= 2 && target[1] == ':') {
				return "", fmt.Errorf("absolute or unsupported delivery link target: %s", name)
			}
			pending = append(strings.Split(target, "/"), pending...)
			continue
		}
		if !s.dirs[candidate] && s.modes[candidate] == "" {
			return "", fmt.Errorf("missing delivery link target: %s", name)
		}
		if !s.dirs[candidate] && len(pending) != 0 {
			return "", fmt.Errorf("delivery link traverses a non-directory: %s", name)
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return ".", nil
	}
	return strings.Join(parts, "/"), nil
}
