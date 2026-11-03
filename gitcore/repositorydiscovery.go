package gitcore

import (
	"crypto"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// repositoryDirectory distinguishes a candidate admin directory from a stray
// .git directory without opening configuration, refs or objects. HEAD belongs to
// the candidate; objects/refs belong to its common directory for linked trees.
// Missing layout components mean "keep discovering" for a directory candidate;
// actual observation failures remain errors. Existing pointer parsing is reused.
func repositoryDirectory(gitDir string) (bool, error) {
	head, err := os.Lstat(filepath.Join(gitDir, "HEAD"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if head.IsDir() {
		return false, nil
	}
	validHead, err := discoveryHead(filepath.Join(gitDir, "HEAD"), head)
	if err != nil || !validHead {
		return false, err
	}
	common, err := commonDirOf(gitDir)
	if err != nil {
		return false, err
	}
	if common == "" {
		common = gitDir
	}
	for _, name := range []string{"objects", "refs"} {
		info, err := os.Stat(filepath.Join(common, name))
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !info.IsDir() {
			return false, nil
		}
	}
	return true, nil
}

// discoveryHead validates admin-directory identity only. It never resolves a
// reference, interprets a revision selector or reads an object. A symbolic HEAD
// can name an unborn branch; a detached HEAD can use either Git hash format,
// independently of this go-git build's object-decoding format.
func discoveryHead(path string, info os.FileInfo) (bool, error) {
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		return strings.HasPrefix(target, "refs/"), err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	// Discovery needs only the bounded header prefix, never the whole file.
	var prefix [255]byte
	n, readErr := io.ReadFull(f, prefix[:])
	closeErr := f.Close()
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return false, readErr
	}
	if closeErr != nil {
		return false, closeErr
	}
	b := prefix[:n]
	if rest, ok := strings.CutPrefix(string(b), "ref:"); ok {
		return strings.HasPrefix(strings.TrimLeft(rest, " \t\r\n\v\f"), "refs/"), nil
	}
	for _, algorithm := range []crypto.Hash{crypto.SHA1, crypto.SHA256} {
		width := 2 * algorithm.Size()
		if len(b) >= width {
			if _, err := hex.Decode(make([]byte, algorithm.Size()), b[:width]); err == nil {
				return true, nil
			}
		}
	}
	return false, nil
}
