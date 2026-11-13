package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// carriers.go — a fresh scan writes only the placeholders no OPEN scan carrier already carries.
//
// THE CLASS THIS CLOSES. An open scan carrier past the coalesce window is sealed, and the next pass
// cuts a fresh branch from the default branch. `statusgen --scan-issues` then writes every
// placeholder the default branch lacks — including the ones already sitting on the sealed, unmerged
// carriers. Each fresh carrier was a superset of the last, and once one merged the others conflicted
// add/add on the same files. Nothing about this is forge-specific: it happened the same way on a
// GitHub pull request and a GitLab merge request.
//
// THE RULE. Before the scan delta is committed, every path under the scan directory that an open
// scan carrier by this App already carries — other than the carrier being written to — is put back
// to the base the worktree was cut from. Each carrier stays an independent delta that merges cleanly
// in any order. The fresh branch is NOT stacked on the sealed head: a sealed carrier closed unmerged
// would then take the fresh one's base with it.
//
// SELF-HEAL. The exclusion reads the forge on every pass. A sealed carrier closed unmerged is no
// longer open, so the next pass does not see it and writes its placeholders again.
//
// UNREADABLE IS NOT NONE. If the open carriers, or the files one of them carries, cannot be read,
// the scan dispatch is refused as could-not-check. Writing the full delta on that reading is the
// superset this file exists to prevent, so there is no fallback to it.

// listChangedFilesFn reads the files one change carries. A seam so a test can stand in for the
// forge; production resolves the forge through the same minted-App-token path as the open-change read.
var listChangedFilesFn = func(repo string, number int) ([]deskkit.ChangedFile, error) {
	f, fr, err := forgeFor(repo)
	if err != nil {
		return nil, err
	}
	return f.ListChangedFiles(fr, number)
}

// scanCarriersIn is every open scan carrier by this App in the read, in any state — a flipped
// carrier is still unmerged and still carries its placeholders. A scan-shaped branch by another
// account is not ours: letting it suppress placeholders would hand that account a way to keep
// issues out of intake.
//
// A list cut at the forge's cap may hold more carriers than it returned, so the answer is
// could-not-check: an unseen carrier's placeholders would be written again.
func scanCarriersIn(r *scanRead) ([]OpenScanPR, error) {
	if r == nil || r.oc == nil {
		return nil, deskkit.Unverifiable("scanloop: the open-change read returned nothing at all — not an empty list", nil)
	}
	if r.oc.TruncatedAtCap {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"scanloop: the open-change read stopped at its cap of %d — an open scan carrier may be among "+
				"the changes not returned, and its placeholders would be written again", r.oc.Cap), nil)
	}
	var out []OpenScanPR
	for _, c := range r.oc.Changes {
		if !strings.HasPrefix(c.HeadRef, scanBranchPrefix) || !strings.EqualFold(c.Author.Login, r.login) {
			continue
		}
		out = append(out, OpenScanPR{Number: c.Number, Branch: c.HeadRef})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, nil
}

// forgeScanReads wires the pass's forge reads of the scan target: the open scan PR the coalesce
// decision is made against, and the open carriers the exclusion reads. Both come from ONE
// open-change read.
func forgeScanReads(target string, now time.Time, window time.Duration) (
	openPR func() (*OpenScanPR, error), carriers func() ([]OpenScanPR, error)) {
	read := onePerPass(func() (*scanRead, error) { return readScanChanges(target) })
	openPR = func() (*OpenScanPR, error) {
		r, err := read()
		if err != nil {
			return nil, err
		}
		return selectOpenScanPR(r.oc, r.login, now, window)
	}
	carriers = func() ([]OpenScanPR, error) {
		r, err := read()
		if err != nil {
			return nil, err
		}
		return scanCarriersIn(r)
	}
	return openPR, carriers
}

// carrierFilesOn reads the files a carrier on target carries.
func carrierFilesOn(target string) func(int) ([]deskkit.ChangedFile, error) {
	return func(n int) ([]deskkit.ChangedFile, error) { return listChangedFilesFn(target, n) }
}

// carriedPaths maps every scan-directory path an open carrier carries to that carrier's number,
// skipping the carrier on the branch being written (a coalesce writes into it). nil Carriers means
// none is known; an unreadable answer is an error, never an empty map.
func (s *ScanLoop) carriedPaths(writing string) (map[string]int, error) {
	if s.Carriers == nil {
		return nil, nil
	}
	cs, err := s.Carriers()
	if err != nil {
		return nil, deskkit.Unverifiable("scanloop: cannot read the open scan carriers on "+s.ScanTarget+
			" — which placeholders they already carry is could-not-check, and a fresh scan written on that "+
			"reading would duplicate them", err)
	}
	carried := map[string]int{}
	for _, c := range cs {
		if c.Branch == writing {
			continue
		}
		if s.CarrierFiles == nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf("scanloop: open scan carrier #%d is known but no reader "+
				"of its files is wired — what it carries is could-not-check", c.Number), nil)
		}
		files, ferr := s.CarrierFiles(c.Number)
		if ferr != nil {
			return nil, deskkit.Unverifiable(fmt.Sprintf("scanloop: cannot read the files open scan carrier #%d "+
				"carries — writing the scan without excluding them would duplicate its placeholders", c.Number), ferr)
		}
		for _, f := range files {
			for _, p := range []string{f.Filename, f.PreviousFilename} {
				if !strings.HasPrefix(p, deskkit.ScanDir+"/") {
					continue
				}
				if _, seen := carried[p]; !seen {
					carried[p] = c.Number
				}
			}
		}
	}
	return carried, nil
}

// statusEntry is one `git status --porcelain=v1 -z` entry.
type statusEntry struct {
	path      string
	untracked bool
}

// parseStatusZ reads `git status --porcelain=v1 -z` output. A rename or copy entry carries its
// source path as the next field; both halves are kept.
func parseStatusZ(raw string) []statusEntry {
	var out []statusEntry
	fields := strings.Split(raw, "\x00")
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 4 {
			continue
		}
		xy, p := f[:2], f[3:]
		out = append(out, statusEntry{path: p, untracked: xy == "??"})
		if (xy[0] == 'R' || xy[0] == 'C') && i+1 < len(fields) {
			i++
			out = append(out, statusEntry{path: fields[i]})
		}
	}
	return out
}

// carriersNamed renders the carriers numbers for a step note, ascending.
func carriersNamed(set map[int]bool) string {
	var ns []int
	for n := range set {
		ns = append(ns, n)
	}
	sort.Ints(ns)
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = fmt.Sprintf("#%d", n)
	}
	return strings.Join(parts, ", ")
}
