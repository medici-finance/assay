package main

// verifysnap.go — retention for verification snapshots.
//
// A verifier run copies the tree it verifies, and the files its rows read and write, into
// "<cache>/assay/verify-snap/<claim key>/", next to the packets and outcome records this tool
// already keeps there. Packets and outcome records are pruned by age (prunePackets,
// pruneOutcomeDirs); snapshots were not, so every verification left a full source tree behind.
// deskdispatch launches every verifier run, so it prunes their snapshots the same way, on the
// same retention, when it dispatches the next one.
//
// Liveness. Nothing records which snapshot a running verifier holds: there is no lease file
// and no process record, and the dispatch claim lives on the forge, which this prune must not
// need. What a live run does leave is filesystem activity: it creates the snapshot, then its
// rows write into it. So a snapshot is in use only if something in it changed recently, and
// "changed" has to mean the inode CHANGE time as well as the modification time — an archive
// extracted into a snapshot (tar, cp -p, rsync -a) keeps the archive's old modification times,
// but the change time of every extracted file is the moment it was written. A snapshot is
// removed only when EVERY entry in it, walked without following a link, has both times older
// than the retention age: a week, against a dispatch claim that goes stale after two hours.
// The snapshot of the claim being dispatched right now is never removed, however old, because
// this very run is about to use it. Anything that cannot be read in full — a walk that fails or
// passes the entry bound — is kept and reported, never removed on a partial view.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// verifySnapDirName is the snapshot directory beside "packets" and "outcomes".
const verifySnapDirName = "verify-snap"

// verifySnapMaxAge is the snapshot retention: the packet retention, for the same reason.
const verifySnapMaxAge = packetMaxAge

// snapVerdict is the prune's decision about one entry directly under verify-snap.
type snapVerdict struct {
	name   string
	remove bool
	reason string
}

// verifySnapBase is "<cache>/assay/verify-snap", or "" when there is no usable cache directory.
func verifySnapBase() string {
	cache, err := packetCacheDir()
	if err != nil || !filepath.IsAbs(cache) {
		return ""
	}
	return filepath.Join(cache, "assay", verifySnapDirName)
}

// planVerifySnaps decides, without changing anything, which snapshots under base go. keep is
// the claim key being dispatched; its snapshot is always kept. limit bounds each snapshot's
// walk. A missing base is no snapshots; a base that is a symlink or not a directory is refused
// (and nothing is pruned through it), as the outcome base is.
func planVerifySnaps(base, keep string, now time.Time, limit int) ([]snapVerdict, error) {
	if base == "" {
		return nil, nil
	}
	if _, err := realOutcomeDir(filepath.Dir(base), false); err != nil {
		return nil, err
	}
	exists, err := realOutcomeDir(base, false)
	if err != nil || !exists {
		return nil, err
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	cutoff := now.Add(-verifySnapMaxAge)
	var out []snapVerdict
	for _, e := range entries {
		v := snapVerdict{name: e.Name()}
		switch {
		case e.Type()&os.ModeSymlink != 0:
			v.reason = "a symlink: never followed, never removed"
		case !e.IsDir():
			v.reason = "not a directory: never removed"
		case e.Name() == keep:
			v.reason = "the snapshot of the claim being dispatched now"
		default:
			newest, werr := newestSnapActivity(root, e.Name(), cutoff, limit)
			switch {
			case werr != nil:
				v.reason = "could not read it in full (" + werr.Error() + "): kept"
			case !newest.Before(cutoff):
				v.reason = fmt.Sprintf("in use or recent: changed %s ago, within the %s retention", snapAge(now, newest), verifySnapMaxAge)
			default:
				v.remove = true
				v.reason = fmt.Sprintf("idle: nothing in it changed for %s, past the %s retention", snapAge(now, newest), verifySnapMaxAge)
			}
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out, nil
}

// errSnapRecent ends a walk early: one recent entry is enough to keep the snapshot.
var errSnapRecent = errors.New("recent entry")

// newestSnapActivity walks one snapshot through root without following a link and returns
// the latest modification or change time of any entry in it, the snapshot directory included.
// It stops at the first entry at or after cutoff and returns that entry's time.
func newestSnapActivity(root *os.Root, name string, cutoff time.Time, limit int) (time.Time, error) {
	var newest time.Time
	err := deskkit.BoundedWalkDir(root.FS(), name, limit, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		st, err := root.Lstat(path)
		if err != nil {
			return err
		}
		t := st.ModTime()
		if c := changeTime(st); c.After(t) {
			t = c
		}
		if t.After(newest) {
			newest = t
		}
		if !t.Before(cutoff) {
			return errSnapRecent
		}
		return nil
	})
	if errors.Is(err, errSnapRecent) {
		return newest, nil
	}
	return newest, err
}

// removeVerifySnap removes one idle snapshot through root. Directories are made owner-writable
// first (a snapshot can hold a module cache, whose directories are read-only), on the
// directories the walk reaches through root, never through a link.
func removeVerifySnap(base, name string, limit int) error {
	root, err := os.OpenRoot(base)
	if err != nil {
		return err
	}
	defer root.Close()
	err = deskkit.BoundedWalkDir(root.FS(), name, limit, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 || !d.IsDir() {
			return nil
		}
		st, err := root.Lstat(path)
		if err != nil || !st.IsDir() {
			return err
		}
		return root.Chmod(path, st.Mode().Perm()|0o700)
	})
	if err != nil {
		return err
	}
	return root.RemoveAll(name)
}

// pruneVerifySnaps applies the plan: it removes each idle snapshot and returns the verdicts
// with any removal failure folded into that snapshot's reason. Best effort, like the sibling
// prunes: a failure is reported, never fatal to the dispatch.
func pruneVerifySnaps(base, keep string, now time.Time, limit int) ([]snapVerdict, error) {
	plan, err := planVerifySnaps(base, keep, now, limit)
	if err != nil {
		return nil, err
	}
	for i, v := range plan {
		if !v.remove {
			continue
		}
		if rerr := removeVerifySnap(base, v.name, limit); rerr != nil {
			plan[i].remove = false
			plan[i].reason += "; removal failed: " + rerr.Error()
		}
	}
	return plan, nil
}

// verifySnapLines renders the verdicts, one line each. dry says "would remove" for a removal.
func verifySnapLines(base string, plan []snapVerdict, dry bool) []string {
	var lines []string
	for _, v := range plan {
		verb := "keep"
		if v.remove {
			verb = "removed"
			if dry {
				verb = "would remove"
			}
		}
		lines = append(lines, fmt.Sprintf("verify-snap: %s %s — %s", verb, filepath.Join(base, v.name), v.reason))
	}
	return lines
}

// snapAge is a whole-minute age, never negative.
func snapAge(now, t time.Time) string {
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return "under a minute"
	}
	return strings.TrimSuffix(d.Truncate(time.Minute).String(), "0s")
}

// reportVerifySnaps runs the snapshot retention for a verifier dispatch. A dry run plans it only
// and lists every snapshot with its verdict and reason; a real dispatch prunes and lists what it
// removed and what it could not judge. Lines go to stderr regardless of --quiet: a removal is a
// durable act the operator must be able to see. It never fails the dispatch; a prune it could
// not run says so.
func reportVerifySnaps(claimKey string, dry bool) {
	base := verifySnapBase()
	if base == "" {
		return
	}
	limit, err := deskkit.MaintenanceEntryLimit()
	var plan []snapVerdict
	if err == nil {
		if dry {
			plan, err = planVerifySnaps(base, claimKey, packetNow(), limit)
		} else {
			plan, err = pruneVerifySnaps(base, claimKey, packetNow(), limit)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "deskdispatch: verify-snap: could-not-check %s — %v; nothing removed\n", base, err)
		return
	}
	if !dry {
		// A real dispatch names only what it removed or could not judge; an ordinary keep is
		// not news on every dispatch.
		shown := plan[:0]
		for _, v := range plan {
			if v.remove || strings.Contains(v.reason, "could not read") || strings.Contains(v.reason, "removal failed") {
				shown = append(shown, v)
			}
		}
		plan = shown
	}
	for _, line := range verifySnapLines(base, plan, dry) {
		fmt.Fprintln(os.Stderr, "deskdispatch: "+line)
	}
}
