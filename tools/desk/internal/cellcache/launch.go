package cellcache

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Admission never waits behind an inventory for the old multi-minute lock
// allowance. Busy remains an explicit retryable refusal, never uncustodied use.
const admissionWait = 2 * time.Second

// Prepare is bounded storage preparation for cellctl's launch owner. Recursive
// measurement and pressure reclamation remain with explicit Check maintenance.
// It is idempotent for a compatible owned root and policy; no returned report
// is a lease or authority to start a child. Acquire revalidates independently.
func Prepare(p Policy) (Report, error) {
	if r, why := cheapClean(p, false, admitEnv{free: diskFree, now: time.Now}); why == "" {
		return r, nil
	}
	m, err := openWithWait(p, true, admissionWait)
	if err != nil {
		return Report{}, err
	}
	defer m.close()
	return m.launchCheck()
}

func (m *manager) launchCheck() (Report, error) {
	if m.created {
		// Only a directory created by THIS transaction can be known empty without
		// recursively measuring it. Existing empty-looking roots are not adopted.
		for _, name := range []string{"build", "mod", "gopath"} {
			if err := m.r.Mkdir(name, 0700); err != nil {
				return Report{}, err
			}
		}
		return m.emptyAdmission()

	}
	r, why := cheapClean(m.p, false, admitEnv{free: m.free, now: time.Now})
	if why == "" {
		return r, nil
	}
	if strings.Contains(why, "changed after the recorded measurement") {
		// Fresh valid accounting exists, but newly created empty directories
		// are inside the filesystem-clock slack. Prove emptiness with ONE
		// bounded directory read per root; never waive slack on nonempty data.
		if empty, err := m.emptyRoots(); err == nil && empty {
			return m.emptyAdmission()
		}
	}
	// Missing, stale, dirty, partial or corrupt accounting cannot become a zero
	// estimate, and cannot trigger an unbounded scan on the startup path.
	r = Report{Schema: schema, Policy: m.p, Outcome: "could-not-check", Admission: "bounded-refusal", Reason: fmt.Sprintf("%s; run cellctl cache <cell> clean before retrying launch", why)}
	return r, &Deferred{r}
}

func (m *manager) emptyRoots() (bool, error) {
	for _, name := range []string{"build", "mod"} {
		st, err := m.r.Lstat(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		if err = checkEntry(st, m.dev); err != nil || !st.IsDir() {
			return false, fmt.Errorf("invalid cache directory %s", name)
		}
		f, err := m.r.OpenFile(name, os.O_RDONLY|noFollow, 0)
		if err != nil {
			return false, err
		}
		names, readErr := f.Readdirnames(1)
		closeErr := f.Close()
		if closeErr != nil {
			return false, closeErr
		}
		if len(names) > 0 {
			return false, nil
		}
		if !errors.Is(readErr, io.EOF) {
			return false, readErr
		}
	}
	return true, nil
}
func (m *manager) emptyAdmission() (Report, error) {
	free, err := m.free(m.p.Root)
	if err != nil {
		return Report{}, err
	}
	zero := int64(0)
	v := Measurement{At: time.Now().UTC(), LogicalBytes: &zero, AvailableBytes: &free}
	r := Report{Schema: schema, Policy: m.p, Filesystem: filesystemName(m.dev), Outcome: "checked-clean", Admission: "bounded-empty", Before: v, After: v}
	need := m.p.Floor + uint64(m.p.Budget/4)
	if need < m.p.Floor || free < need {
		r.Outcome = "storage-deferred"
		r.Reason = "empty cache has insufficient available space"
	}
	return finish(m, r)
}
