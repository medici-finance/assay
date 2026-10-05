package cellscratch

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Snapshot reads Git objects at the recorded revision, not the working directory.
// No recursive copy, untracked mining history, Git credentials, or output gets imported.
// Unlike git archive, this includes export-ignore files needed by tests/reviews.
// Exceeding the bound fails visibly; no required tracked file is silently omitted.
func (r *Run) Snapshot(ctx context.Context, source string, maxBytes int64) error {
	if maxBytes <= 0 {
		return errors.New("snapshot byte budget must be positive")
	}
	list := exec.CommandContext(ctx, "git", "-C", source, "ls-tree", "-rlz", r.Record.Revision)
	pipe, err := list.StdoutPipe()
	if err != nil {
		return err
	}
	if err = list.Start(); err != nil {
		return err
	}
	// Read the manifest with a bounded per-path buffer, before writing any payload.
	type blob struct {
		mode, sha, path string
		size            int64
	}
	var blobs []blob
	var total int64
	scan := bufio.NewScanner(pipe)
	scan.Split(splitNUL)
	scan.Buffer(make([]byte, 4096), 1024*1024)
	for scan.Scan() {
		fields, path, ok := strings.Cut(scan.Text(), "\t")
		parts := strings.Fields(fields)
		if !ok || len(parts) != 4 || !filepath.IsLocal(path) || strings.Contains(path, "\\") || filepath.Base(path) == ".git" {
			err = errors.New("unsafe Git snapshot path")
			break
		}
		if parts[1] != "blob" {
			err = fmt.Errorf("snapshot requires explicit handling of submodule %s", path)
			break
		}
		size, e := strconv.ParseInt(parts[3], 10, 64)
		if e != nil || size < 0 || size > maxBytes-total {
			err = errors.New("snapshot exceeds byte budget; use source references or an explicit larger budget")
			break
		}
		total += size
		blobs = append(blobs, blob{parts[0], parts[2], path, size})
	}
	if err == nil {
		err = scan.Err()
	}
	if err != nil {
		_ = list.Process.Kill()
	}
	err = errors.Join(err, list.Wait())
	if err != nil {
		return err
	}
	work, err := r.Store.root.OpenRoot(r.Record.ID + "/work")
	if err != nil {
		return err
	}
	defer work.Close()
	// cat-file batch streams each blob exactly once; the destination never participates
	// in the input traversal, even when the source is an ancestor of the scratch root.
	cmd := exec.CommandContext(ctx, "git", "-C", source, "cat-file", "--batch")
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	rd := bufio.NewReader(out)
	for _, b := range blobs {
		if _, err = fmt.Fprintln(in, b.sha); err != nil {
			break
		}
		header, e := rd.ReadString('\n')
		if e != nil {
			err = e
			break
		}
		f := strings.Fields(header)
		if len(f) != 3 || f[0] != b.sha || f[1] != "blob" || f[2] != strconv.FormatInt(b.size, 10) {
			err = errors.New("Git object response differs from manifest")
			break
		}
		if err = work.MkdirAll(filepath.Dir(b.path), 0700); err != nil {
			break
		}
		if b.mode == "120000" {
			if b.size > 4096 {
				err = errors.New("oversized snapshot symlink")
				break
			}
			target := make([]byte, b.size)
			if _, err = io.ReadFull(rd, target); err != nil {
				break
			}
			// Preserve repository symlinks only when their target remains within the snapshot.
			joined := filepath.Clean(filepath.Join(filepath.Dir(b.path), string(target)))
			if filepath.IsAbs(string(target)) || !filepath.IsLocal(joined) {
				err = fmt.Errorf("snapshot symlink escapes: %s", b.path)
				break
			}
			err = work.Symlink(string(target), b.path)
		} else {
			mode := os.FileMode(0600)
			if b.mode == "100755" {
				mode = 0700
			}
			var file *os.File
			file, err = work.OpenFile(b.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err == nil {
				_, err = io.CopyN(file, rd, b.size)
				err = errors.Join(err, file.Close())
			}
		}
		if err != nil {
			break
		}
		newline, e := rd.ReadByte()
		if e != nil || newline != '\n' {
			err = errors.New("invalid Git object terminator")
			break
		}
	}
	in.Close()
	if err != nil {
		_ = cmd.Process.Kill()
	}
	return errors.Join(err, cmd.Wait())
}
func splitNUL(data []byte, atEOF bool) (int, []byte, error) {
	for i, b := range data {
		if b == 0 {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return 0, nil, errors.New("unterminated Git manifest")
	}
	return 0, nil, nil
}
