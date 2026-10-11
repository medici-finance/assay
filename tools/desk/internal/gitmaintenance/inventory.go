package gitmaintenance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	maxWorktrees   = 4096
	maxPackEntries = 4096
)

func inspect(ctx context.Context, repo string) (Inventory, error) {
	var inv Inventory
	if err := ctx.Err(); err != nil {
		return inv, err
	}
	common, err := commonDir(repo)
	if err != nil {
		return inv, err
	}
	inv.CommonDir = common
	if err := checkActive(common); err != nil {
		return inv, err
	}
	inv.Repo, err = filepath.Abs(repo)
	if err != nil {
		return inv, err
	}
	inv.ReflogRecovery, err = effectiveReflogRecovery(ctx, inv.Repo)
	if err != nil {
		return inv, err
	}
	_, err = validateConfig(ctx, common)
	if err != nil {
		return inv, err
	}
	inv.LastSuccess, err = readStamp(common)
	if err != nil {
		return inv, err
	}
	inv.LastAttempt, err = readAttempt(common)
	if err != nil {
		return inv, err
	}
	inv.Alternates, err = inspectAlternates(ctx, filepath.Join(common, "objects"))
	if err != nil {
		return inv, err
	}
	counts, err := native(ctx, common, "objects")
	if err != nil {
		return inv, err
	}
	inv.Objects, err = parseObjects(string(counts))
	if err != nil {
		return inv, err
	}
	refs, err := native(ctx, common, "refs")
	if err != nil {
		return inv, err
	}
	inv.RefNamespaces = make(map[string]int)
	for _, name := range strings.Fields(string(refs)) {
		inv.Refs++
		parts := strings.SplitN(name, "/", 3)
		if len(parts) < 2 || parts[0] != "refs" {
			return inv, errors.New("maintenance: invalid reference inventory")
		}
		inv.RefNamespaces["refs/"+parts[1]+"/"]++
		if strings.HasPrefix(name, "refs/dispatch/") || strings.HasPrefix(name, "refs/heads/dispatch/") {
			inv.DispatchRefs = append(inv.DispatchRefs, name)
		}
	}
	if err := validateWorktreeFiles(ctx, common); err != nil {
		return inv, err
	}
	worktrees, err := native(ctx, common, "worktrees")
	if err != nil {
		return inv, err
	}
	inv.Worktrees, err = parseWorktrees(string(worktrees))
	if err != nil {
		return inv, err
	}
	inv.Recovery = "all object IDs retained; reflogs and worktrees unchanged; dispatch claims held; reverse-alternate borrowers may be unknown"
	inv.TemporaryRefs, err = InspectTempRefs(ctx, repo)
	if err != nil {
		return inv, err
	}
	return inv, nil
}

func validateConfig(ctx context.Context, common string) (map[string]string, error) {
	data, err := native(ctx, common, "config")
	if err != nil {
		return nil, err
	}
	// --git-dir=<common> can also load the main worktree's configuration when
	// extensions.worktreeConfig is set. Inspect that file without expanding any
	// include, rather than treating the shared config alone as the whole scope.
	if _, err := os.Lstat(filepath.Join(common, "config.worktree")); err == nil {
		worktreeData, err := native(ctx, common, "worktree-config")
		if err != nil {
			return nil, err
		}
		data = append(data, worktreeData...)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	retention := map[string]string{"gc.reflogexpire": "90 days (Git default)", "gc.reflogexpireunreachable": "30 days (Git default)"}
	for _, entry := range strings.Split(string(data), "\x00") {
		if entry == "" {
			continue
		}
		key, value, _ := strings.Cut(entry, "\n")
		key = strings.ToLower(key)
		if strings.HasPrefix(key, "gc.") && (strings.HasSuffix(key, ".reflogexpire") || strings.HasSuffix(key, ".reflogexpireunreachable")) {
			retention[key] = value
		}
		if strings.HasPrefix(key, "include.") || strings.HasPrefix(key, "includeif.") ||
			key == "extensions.partialclone" || strings.HasSuffix(key, ".promisor") ||
			key == "repack.filter" || key == "repack.filterto" {
			return nil, fmt.Errorf("maintenance: unsupported configuration %s; offline retention could not be proven", key)
		}
	}
	// Partial clones can also be declared by a .promisor pack marker even if the
	// remote's config has been removed. Never turn a missing object into a fetch.
	entries, err := readPackEntries(ctx, common)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if strings.HasSuffix(entry.Name(), ".promisor") {
			return nil, errors.New("maintenance: promisor packs require an explicit offline-completeness migration")
		}
	}
	return retention, nil
}

func parseObjects(data string) (ObjectCounts, error) {
	var counts ObjectCounts
	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			return counts, errors.New("maintenance: malformed object inventory")
		}
		if key == "alternate" {
			continue
		}
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n < 0 {
			return counts, fmt.Errorf("maintenance: invalid object count %s", key)
		}
		seen[key] = true
		switch key {
		case "count":
			counts.Loose = n
		case "size":
			counts.LooseBytes = n * 1024
		case "in-pack":
			counts.Packed = n
		case "packs":
			counts.Packs = n
		case "size-pack":
			counts.PackBytes = n * 1024
		case "garbage":
			counts.Garbage = n
		case "prune-packable", "size-garbage":
		default:
			return counts, fmt.Errorf("maintenance: unknown object inventory field %s", key)
		}
	}
	for _, key := range []string{"count", "size", "in-pack", "packs", "size-pack", "garbage"} {
		if !seen[key] {
			return counts, fmt.Errorf("maintenance: object inventory lacks %s", key)
		}
	}
	if counts.Garbage != 0 {
		return counts, errors.New("maintenance: garbage in object store; could not prove offline completeness")
	}
	return counts, nil
}

func parseWorktrees(data string) ([]Worktree, error) {
	var result []Worktree
	var current *Worktree
	for _, field := range strings.Split(data, "\x00") {
		if field == "" {
			current = nil
			continue
		}
		key, value, _ := strings.Cut(field, " ")
		if key == "worktree" {
			result = append(result, Worktree{Path: value})
			current = &result[len(result)-1]
			continue
		}
		if current == nil {
			return nil, errors.New("maintenance: malformed worktree inventory")
		}
		switch key {
		case "HEAD":
			current.HEAD = value
		case "branch":
			current.Branch = value
		case "locked":
			current.Locked = true
		case "prunable":
			current.Prunable = true
		case "bare", "detached":
		default:
			return nil, fmt.Errorf("maintenance: unknown worktree field %s", key)
		}
	}
	if len(result) == 0 {
		return nil, errors.New("maintenance: empty worktree inventory")
	}
	return result, nil
}

// Do not let Git's tolerant roster display hide an unreadable administrative
// entry. Missing worktree directories are legitimate prunable holders; unreadable
// HEAD/commondir metadata cannot establish what they hold and refuses mutation.
func validateWorktreeFiles(ctx context.Context, common string) error {
	entries, err := readWorktreeEntries(common)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.IsDir() {
			return errors.New("maintenance: unexpected worktree administrative entry")
		}
		for _, name := range []string{"HEAD", "commondir", "gitdir"} {
			data, err := readMaintenanceFile(filepath.Join(common, "worktrees", entry.Name(), name), 1<<20)
			if err != nil || len(data) == 0 {
				return fmt.Errorf("maintenance: unreadable worktree %s metadata %s", entry.Name(), name)
			}
		}
	}
	return nil
}

// Follow only the alternates the object store explicitly declares. Reverse
// borrowers cannot be enumerated reliably, which is why maintenance preserves
// every object, not only objects reachable from this repository's refs.
func inspectAlternates(ctx context.Context, objects string) ([]string, error) {
	seen := map[string]bool{}
	var resolved []string
	var walk func(string) error
	walk = func(dir string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		dir, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return fmt.Errorf("maintenance: unreadable alternate object directory: %w", err)
		}
		if seen[dir] {
			return nil
		}
		if len(seen) >= 128 {
			return errors.New("maintenance: alternate chain exceeds inventory budget")
		}
		seen[dir] = true
		f, err := openMaintenanceDirectory(dir)
		if err != nil {
			return err
		}
		_, readErr := f.ReadDir(1)
		closeErr := f.Close()
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		data, err := readMaintenanceFile(filepath.Join(dir, "info", "alternates"), 1<<20)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if len(data) > 1<<20 {
			return errors.New("maintenance: alternates file exceeds inventory budget")
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "\"") {
				line, err = strconv.Unquote(line)
				if err != nil {
					return err
				}
			}
			if !filepath.IsAbs(line) {
				line = filepath.Join(dir, line)
			}
			alternate, err := filepath.EvalSymlinks(line)
			if err != nil {
				return fmt.Errorf("maintenance: unreadable alternate: %w", err)
			}
			if !seen[alternate] {
				resolved = append(resolved, alternate)
			}
			if err := walk(alternate); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(objects); err != nil {
		return nil, err
	}
	sort.Strings(resolved)
	return resolved, nil
}

// Every metadata read must be a regular file at the kernel open and remain
// bounded before allocation. Context deadlines cannot interrupt a FIFO open.
func readMaintenanceFile(path string, limit int64) ([]byte, error) {
	f, err := openMaintenanceRegular(path)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(f, limit+1))
	closeErr := f.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("maintenance: metadata file exceeds inventory budget")
	}
	return data, nil
}

func readWorktreeEntries(common string) ([]os.DirEntry, error) {
	f, err := openMaintenanceDirectory(filepath.Join(common, "worktrees"))
	if err != nil {
		return nil, err
	}
	entries, readErr := f.ReadDir(maxWorktrees + 1)
	closeErr := f.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(entries) > maxWorktrees {
		return nil, errors.New("maintenance: worktree roster exceeds inventory budget")
	}
	return entries, nil
}

// Count every entry, including stale or junk files: object-count eligibility
// does not bound directory allocation before Git can classify those files.
func readPackEntries(ctx context.Context, common string) ([]os.DirEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := openMaintenanceDirectory(filepath.Join(common, "objects", "pack"))
	if err != nil {
		return nil, err
	}
	entries, readErr := f.ReadDir(maxPackEntries + 1)
	closeErr := f.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(entries) > maxPackEntries {
		return nil, errors.New("maintenance: pack directory exceeds inventory budget")
	}
	return entries, nil
}
