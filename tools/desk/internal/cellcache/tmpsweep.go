package cellcache

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// tmpDir is the managed GOTMPDIR, beside build/, mod/ and gopath/.
const tmpDir = "tmp"

// goBuildPrefix is the name cmd/go gives each build's work directory
// (os.MkdirTemp(GOTMPDIR, "go-build")). Nothing else under tmp/ is selected.
const goBuildPrefix = "go-build"

// consumerDirs are the directories a consumer's environment names and
// enrollment creates: tmp/ only for a policy that exports GOTMPDIR, since
// cmd/go refuses a GOTMPDIR that does not exist.
func (p Policy) consumerDirs() []string {
	if p.GoTmp {
		return []string{"build", "mod", "gopath", tmpDir}
	}
	return []string{"build", "mod", "gopath"}
}

// TmpSweep is what one GOTMPDIR sweep did. Removed names the work
// directories removed (a dry run: those it would remove) and RemovedBytes
// their logical size. Kept counts the go-build directories left in place.
// Reason is set when the sweep could not run or refused a directory: it is
// could-not-check for what it names, never a clean result.
type TmpSweep struct {
	Removed      []string `json:"removed,omitempty"`
	RemovedBytes int64    `json:"removed_bytes"`
	Kept         int      `json:"kept"`
	Reason       string   `json:"reason,omitempty"`
}

// sweepTmp removes the go-build work directories under tmp/ that no live
// consumer can hold. It runs under the root's custody lock, which is also the
// lock every consumer enrolls under, so the set of active records it reads
// cannot change while it decides.
//
// Liveness, proved rather than guessed. cmd/go creates a build's work
// directory after its consumer enrolled (enrollment is the last step before a
// supervised child starts), and a directory's modification time is never
// earlier than its creation. So a work directory whose own modification time
// is before the EARLIEST active record's enrollment (less AdmitMtimeSlack)
// was created before every live consumer existed: none of them can hold it.
// No PID or age is taken as proof a consumer exited — the records are the
// only liveness signal, exactly as for mod/ and gopath/.
//
// On top of that proof, a directory is removed only when nothing in it — the
// directory and every member, walked through the verified root — changed
// within the policy's build trim age. That covers a GOTMPDIR user the records
// cannot see (a shell that copied the variables), and keeps a live build's
// directory well clear of the cut.
//
// Custody is the trim's: every member must pass checkEntry before anything is
// removed, a bad member refuses that one directory (named in Reason), and
// removal goes through removeTree. A refusal never fails the pass: tmp/ is
// outside the byte budget and must not hold admission hostage.
func (m *manager) sweepTmp(now time.Time, dry bool) *TmpSweep {
	if !m.p.GoTmp {
		return nil
	}
	if m.lock == nil {
		return &TmpSweep{Reason: "could-not-check: cache custody unavailable"}
	}
	st, err := m.r.Lstat(tmpDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return &TmpSweep{Reason: "could-not-check: " + err.Error()}
	}
	if err = checkEntry(st, m.dev); err != nil || !st.IsDir() {
		return &TmpSweep{Reason: "could-not-check: invalid managed GOTMPDIR"}
	}
	cut, err := m.tmpCut(now)
	if err != nil {
		return &TmpSweep{Reason: "could-not-check: consumer records: " + err.Error()}
	}
	names, err := m.subdirNames(tmpDir)
	if err != nil {
		return &TmpSweep{Reason: "could-not-check: " + err.Error()}
	}
	res := &TmpSweep{}
	var refused []string
	for _, name := range names {
		if !strings.HasPrefix(name, goBuildPrefix) {
			continue
		}
		path := tmpDir + "/" + name
		size, removable, err := m.idleWorkDir(path, cut)
		switch {
		case err != nil:
			res.Kept++
			refused = append(refused, fmt.Sprintf("%s: %v", name, err))
			continue
		case !removable:
			res.Kept++
			continue
		}
		if !dry {
			if err := m.removeTree(path); err != nil {
				res.Kept++
				refused = append(refused, fmt.Sprintf("%s: remove: %v", name, err))
				continue
			}
		}
		res.Removed = append(res.Removed, name)
		res.RemovedBytes += size
	}
	if len(refused) > 0 {
		res.Reason = "could-not-check: refused " + strings.Join(refused, "; ")
	}
	return res
}

// tmpCut is the time before which a work directory may go: the build trim
// age, or the earliest active record's enrollment less AdmitMtimeSlack when
// that is earlier. A record's enrollment is the earlier of its recorded
// timestamp and its modification time; one that cannot be read refuses the
// sweep.
func (m *manager) tmpCut(now time.Time) (time.Time, error) {
	cut := now.Add(-m.p.BuildAge())
	active, err := m.active()
	if err != nil {
		return time.Time{}, err
	}
	for _, name := range active {
		st, err := m.r.Lstat(name)
		if err != nil {
			return time.Time{}, err
		}
		b, err := readMetaBounded(m.r, m.dev, name, 128)
		if err != nil {
			return time.Time{}, err
		}
		at, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(b)))
		if err != nil {
			return time.Time{}, fmt.Errorf("unreadable enrollment time in %s", name)
		}
		if st.ModTime().Before(at) {
			at = st.ModTime()
		}
		if held := at.Add(-AdmitMtimeSlack); held.Before(cut) {
			cut = held
		}
	}
	return cut, nil
}

// idleWorkDir reports whether the work directory at path is removable: a
// directory whose own modification time and every member's are before cut,
// every member passing checkEntry (verifiedTreeSize, the trim's custody
// walk). It returns the directory's logical size.
func (m *manager) idleWorkDir(path string, cut time.Time) (int64, bool, error) {
	st, err := m.r.Lstat(path)
	if err != nil {
		return 0, false, err
	}
	if err = checkEntry(st, m.dev); err != nil || !st.IsDir() {
		return 0, false, fmt.Errorf("not an owned directory")
	}
	if !st.ModTime().Before(cut) {
		return 0, false, nil
	}
	total, newest, err := m.verifiedTreeSize(path)
	if err != nil {
		return 0, false, err
	}
	return total, newest.Before(cut), nil
}

// sweepTmpLocked is sweepTmp for the lock-free maintenance path: it takes
// custody only when the policy exports GOTMPDIR and tmp/ exists, waits for
// the lock no longer than admission does, and reports a busy lock as
// could-not-check rather than waiting out a sibling's inventory.
func sweepTmpLocked(p Policy, now time.Time) *TmpSweep {
	if !p.GoTmp {
		return nil
	}
	if _, err := os.Lstat(p.Root + "/" + tmpDir); os.IsNotExist(err) {
		return nil
	}
	m, err := openWithWait(p, false, admissionWait)
	if err != nil {
		return &TmpSweep{Reason: "could-not-check: " + err.Error()}
	}
	defer m.close()
	if m.scanLimit == 0 {
		limit, err := deskkit.MaintenanceEntryLimit()
		if err != nil {
			return &TmpSweep{Reason: "could-not-check: " + err.Error()}
		}
		m.scanLimit = limit
	}
	return m.sweepTmp(now, false)
}
