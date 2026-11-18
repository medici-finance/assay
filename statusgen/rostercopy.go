package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// The container lane's roster copy (#6904). A container-netns run has no
// config home, so a Verify row that runs `statusgen --lint` inside it refuses
// its configuration: the write class reads the roster ONLY from
// ~/.config/assay/roster.env, which the container does not have. A host roster
// bind-mounted straight into that path does not help either: the loader's
// owner check (checkFileOwner) refuses a file the container's user does not
// own, and each admitted row runs under a fresh, empty HOME of its own anyway.
//
//  1. host open: with --roster-copy <file>, `verifyrun --open-bracket` reads
//     the host roster through the same owner-checked loader, refuses one that
//     does not parse cleanly, and writes to that new file outside the home
//     only the keys statusgen's configuration reads (scanRosterReadKeys):
//     trusted identities, the repository set, product keys. No other key
//     travels, so nothing the roster file holds beside them (a credential
//     path, a host directory, a callout URL) reaches the container. It records
//     the copy's digest in the receipt and prints the container flag that
//     mounts it read-only at containerRosterTarget;
//  2. container: before any row, the run requires the copy at the target, on
//     a read-only mount, digesting to the receipt's and loading cleanly
//     (containerRosterCopy). Each admitted row already gets a fresh, empty
//     HOME; under a roster receipt that HOME also holds the copy as its
//     config-home roster (directory 0700, file 0600, owned by the row's user;
//     installRowRoster), so the row's statusgen reads it through the
//     unchanged, owner-checked loader. Nothing else is added to the row's
//     HOME, and a receipt with no roster leaves rows as they were.

// containerRosterTarget is where the container run reads the open step's roster
// copy. A var so tests point it at a fixture file.
var containerRosterTarget = "/run/assay-roster/roster.env"

// receiptRoster is the receipt's record of the open step's roster copy.
type receiptRoster struct {
	Digest string   `json:"digest"`
	Keys   []string `json:"keys"`
}

// rosterCopyPathProblem validates --roster-copy the way moduleCachePathProblem
// validates --module-cache: the path is written into a --mount option list.
func rosterCopyPathProblem(home, copyPath string) string {
	if !filepath.IsAbs(copyPath) {
		return fmt.Sprintf("--roster-copy %s is not an absolute path", copyPath)
	}
	if i := strings.IndexFunc(copyPath, func(r rune) bool { return r == ',' || r == '"' || unicode.IsSpace(r) || unicode.IsControl(r) }); i >= 0 {
		return fmt.Sprintf("--roster-copy %q carries %q; the path is written into the container's --mount option list, so it may hold no comma, quote, whitespace or control character", copyPath, copyPath[i:i+1])
	}
	if rel, ok := relWithin(home, copyPath); ok {
		return fmt.Sprintf("--roster-copy %s sits inside the home (%s); the copy must be outside it", copyPath, rel)
	}
	if canonicalPath(copyPath) == canonicalPath(scanConfigHomePath()) {
		return fmt.Sprintf("--roster-copy %s is the host's own roster; the open step writes a new copy and never mounts the original", copyPath)
	}
	if _, err := os.Lstat(copyPath); err == nil {
		return fmt.Sprintf("--roster-copy %s exists; the open step writes a new file", copyPath)
	}
	return ""
}

// renderRosterCopy reads the host roster through the owner-checked loader and
// renders the copy: the keys statusgen reads, sorted, one KEY=value per line.
func renderRosterCopy() ([]byte, []string, error) {
	vals, path, err := scanReadConfigFile()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, fmt.Errorf("could-not-check — --roster-copy: no host roster at %s", path)
		}
		return nil, nil, fmt.Errorf("--roster-copy: refusing the host roster %s: %v", path, err)
	}
	if cfg := scanParseConfig(scanClassWrite, "config file "+path, vals); len(cfg.Problems) > 0 {
		return nil, nil, fmt.Errorf("--roster-copy: the host roster %s does not load (%s); the container would refuse it too", path, strings.Join(cfg.Problems, "; "))
	}
	kept := map[string]string{}
	for _, k := range scanRosterReadKeys() {
		if v, ok := vals[k]; ok {
			kept[k] = v
		}
	}
	keys := slices.Sorted(maps.Keys(kept))
	var b bytes.Buffer
	b.WriteString("# statusgen verifyrun --open-bracket roster copy: the keys statusgen reads, no other.\n")
	for _, k := range keys {
		b.WriteString(k + "=" + kept[k] + "\n")
	}
	// A value the line format cannot carry back unchanged is refused, never
	// silently altered.
	if back := scanParseDotenv(b.String()); !maps.Equal(back, kept) {
		return nil, nil, fmt.Errorf("--roster-copy: a value in %s does not survive the copy's KEY=value format unchanged", path)
	}
	return b.Bytes(), keys, nil
}

func rosterDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// writeRosterCopy writes the copy to a new 0600 file and returns its record.
func writeRosterCopy(copyPath string) (*receiptRoster, error) {
	raw, keys, err := renderRosterCopy()
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(copyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		os.Remove(copyPath)
		return nil, err
	}
	if err := f.Close(); err != nil {
		os.Remove(copyPath)
		return nil, err
	}
	return &receiptRoster{Digest: rosterDigest(raw), Keys: keys}, nil
}

// containerRosterFlags are the container flags the roster copy needs.
func containerRosterFlags(copyPath string) []string {
	return []string{
		"--mount", "type=bind,source=" + copyPath + ",target=" + containerRosterTarget + ",readonly",
	}
}

// receiptRosterProblem validates a receipt's roster record.
func receiptRosterProblem(r *receiptRoster) error {
	if r == nil {
		return nil
	}
	if !sha256DigestRe.MatchString(r.Digest) {
		return fmt.Errorf("the receipt's roster copy carries no sha256 digest")
	}
	read := map[string]bool{}
	for _, k := range scanRosterReadKeys() {
		read[k] = true
	}
	for _, k := range r.Keys {
		if !read[k] {
			return fmt.Errorf("the receipt's roster copy names key %q, which is not one statusgen reads", k)
		}
	}
	return nil
}

// containerRosterCopy is the container half: before any row, the open step's
// copy must be mounted read-only at containerRosterTarget, digest to the
// receipt's and load cleanly. It returns the copy's bytes, which every row's
// fresh HOME then holds as its config-home roster (installRowRoster).
func containerRosterCopy(root string, r *receiptRoster) ([]byte, error) {
	target := containerRosterTarget
	if rel, ok := relWithin(root, target); ok {
		return nil, fmt.Errorf("the roster copy target %s sits inside the root (%s)", target, rel)
	}
	raw, err := os.ReadFile(target)
	if err != nil {
		return nil, fmt.Errorf("the receipt carries a roster copy, but %s is unreadable (%v); mount the open step's copy read-only there as it printed", target, err)
	}
	ro, err := mountReadOnly(target)
	if err != nil {
		return nil, fmt.Errorf("could-not-check whether the roster copy %s is mounted read-only: %v", target, err)
	}
	if !ro {
		return nil, fmt.Errorf("the roster copy %s is mounted read-write; mount it read-only", target)
	}
	if got := rosterDigest(raw); got != r.Digest {
		return nil, fmt.Errorf("the roster copy at %s digests to %s, not the %s the open step wrote; mount the open step's copy, never another", target, got, r.Digest)
	}
	if cfg := scanParseConfig(scanClassWrite, "roster copy "+target, scanParseDotenv(string(raw))); len(cfg.Problems) > 0 {
		return nil, fmt.Errorf("the roster copy at %s does not load (%s)", target, strings.Join(cfg.Problems, "; "))
	}
	return raw, nil
}

// installRowRoster writes the admitted roster copy into a row's fresh HOME as
// its config-home roster: directory 0700, file 0600, owned by the row's user,
// which is what the loader's owner check requires.
func installRowRoster(home string, raw []byte) error {
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "roster.env"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(raw)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	return werr
}
