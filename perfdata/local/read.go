package local

// read.go — the offline reader. It never writes, never locks out writers and never
// opens a socket. It distinguishes a missing store (unknown), a clean empty store
// (empty-complete), undecodable records (corrupt), an incomplete last line (partial
// tail) and sequence gaps that are explained by recorded retirement or failed writes
// from gaps nothing explains.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/medici-finance/assay/perfdata"
)

// ReadResult is everything a reader found in one store.
type ReadResult struct {
	Present  bool                      `json:"present"`
	Coverage []perfdata.SourceCoverage `json:"coverage"`
	Events   []perfdata.Event          `json:"-"`
	Lines    [][]byte                  `json:"-"`
	Rollups  []perfdata.DailyRollup    `json:"-"`
	// Duplicates counts identical re-reads of one identity; Conflicts counts one
	// identity with two different records (both are excluded from Events after the first).
	Duplicates int `json:"duplicates"`
	Conflicts  int `json:"conflicts"`
}

// Read reads the store under stateDir.
func Read(stateDir string) (ReadResult, error) {
	root := Root(stateDir)
	var rr ReadResult
	if st, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		rr.Coverage = []perfdata.SourceCoverage{{State: perfdata.CoverageMissing, Epochs: []uint32{},
			ExplainedGaps: []perfdata.SeqRange{}, UnexplainedGaps: []perfdata.SeqRange{}, Retired: []perfdata.Retirement{}, EpochResets: []perfdata.EpochReset{}}}
		return rr, nil
	} else if err != nil {
		return rr, err
	} else if !st.IsDir() {
		return rr, errors.New("perfdata/local: store path is not a directory")
	}
	rr.Present = true
	corruptMeta := 0
	if _, err := os.Stat(filepath.Join(root, "manifest.json")); err != nil {
		corruptMeta++
	}
	cov, ok := readCoverage(root)
	if !ok {
		corruptMeta++
	}
	sf, serr := readSource(root)
	if serr != nil && !errors.Is(serr, os.ErrNotExist) {
		corruptMeta++
	}

	type perSource struct {
		c     perfdata.SourceCoverage
		seqs  map[uint64]bool
		epoch map[uint32]bool
	}
	sources := map[string]*perSource{}
	get := func(src string) *perSource {
		ps := sources[src]
		if ps == nil {
			ps = &perSource{c: perfdata.SourceCoverage{Source: src}, seqs: map[uint64]bool{}, epoch: map[uint32]bool{}}
			sources[src] = ps
		}
		return ps
	}
	if serr == nil {
		get(sf.Source).c.LastSeq = sf.LastSeq
	}
	seen := map[string]string{}
	orphanCorrupt, orphanPartial := 0, 0
	for _, sg := range listSegments(root) {
		b, err := os.ReadFile(filepath.Join(root, "segments", sg.name))
		if err != nil {
			orphanCorrupt++
			continue
		}
		pieces := bytes.Split(b, []byte("\n"))
		partial := len(pieces[len(pieces)-1]) > 0
		pieces = pieces[:len(pieces)-1]
		segSrc := ""
		segCorrupt := 0
		var segEvents []perfdata.Event
		var segLines [][]byte
		for _, line := range pieces {
			e, err := perfdata.DecodeEvent(line)
			if err != nil {
				segCorrupt++
				continue
			}
			segSrc = e.Source
			segEvents = append(segEvents, e)
			segLines = append(segLines, line)
		}
		if segSrc != "" && cov.retiredSegment(segSrc, sg) {
			continue // retirement recorded; removal pending
		}
		if segSrc == "" {
			orphanCorrupt += segCorrupt
			if partial {
				orphanPartial++
			}
			continue
		}
		ps := get(segSrc)
		ps.c.CorruptLines += segCorrupt
		if partial {
			ps.c.PartialTails++
		}
		for i, e := range segEvents {
			d := perfdata.Digest(segLines[i])
			if prev, ok := seen[e.ID]; ok {
				if prev == d {
					rr.Duplicates++
				} else {
					rr.Conflicts++
				}
				continue
			}
			seen[e.ID] = d
			ps.seqs[e.Seq] = true
			ps.epoch[e.Epoch] = true
			ps.c.Events++
			ps.c.LastSeq = max(ps.c.LastSeq, e.Seq)
			if ps.c.FirstRecordedAt == "" || e.RecordedAt < ps.c.FirstRecordedAt {
				ps.c.FirstRecordedAt = e.RecordedAt
			}
			if e.RecordedAt > ps.c.LastRecordedAt {
				ps.c.LastRecordedAt = e.RecordedAt
			}
			if e.Kind == perfdata.KindHealth {
				if v, err := perfdata.DecodePayload(e.Kind, e.Payload); err == nil {
					h := v.(*perfdata.Health)
					ps.c.Drops.Add(perfdata.Drops{QueueFull: h.QueueFull, WriteFailed: h.WriteFailed, FlushTimeout: h.FlushTimeout, Refused: h.Refused})
				}
			}
			rr.Events = append(rr.Events, e)
			rr.Lines = append(rr.Lines, segLines[i])
		}
	}
	for _, r := range cov.Retired {
		if r.Source != "" {
			ps := get(r.Source)
			ps.c.Retired = append(ps.c.Retired, r.Retirement)
			if r.Reason != RetireDistribution {
				ps.c.LastSeq = max(ps.c.LastSeq, r.ToSeq)
			}
		}
	}
	for _, l := range cov.Lost {
		ps := get(l.Source)
		ps.c.LastSeq = max(ps.c.LastSeq, l.Range.To)
	}
	// A live marker whose lock nobody holds belongs to a writer that ended without a
	// clean close (a crash or an abandoned flush) and has not been reaped yet. The probe
	// is non-blocking, so a running writer is never delayed.
	live, orphaned := 0, 0
	if ents, err := os.ReadDir(filepath.Join(root, "run")); err == nil {
		for _, e := range ents {
			if !strings.HasPrefix(e.Name(), "live-") {
				continue
			}
			f, err := os.OpenFile(filepath.Join(root, "run", e.Name()), os.O_RDWR, 0)
			switch {
			case err != nil || !liveMarkersSupported:
				live++
			case tryLockFile(f):
				unlockFile(f)
				orphaned++
			default:
				live++
			}
			if f != nil {
				f.Close()
			}
		}
	}
	if len(sources) == 0 {
		get("")
	}
	// The current source (or the only one) carries store-level facts.
	primary := sf.Source
	if _, ok := sources[primary]; !ok || primary == "" {
		for k := range sources {
			primary = k
			break
		}
	}
	keys := make([]string, 0, len(sources))
	for k := range sources {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ps := sources[k]
		c := &ps.c
		if k == primary {
			c.CorruptMetadata = corruptMeta
			c.CorruptLines += orphanCorrupt
			c.PartialTails += orphanPartial
			c.Drops.Add(cov.Drops)
			c.UncleanTerminations = cov.Unclean + orphaned
			c.LiveWriters = live
			c.EpochResets = append(c.EpochResets, cov.EpochResets...)
		}
		for e := range ps.epoch {
			c.Epochs = append(c.Epochs, e)
		}
		sort.Slice(c.Epochs, func(i, j int) bool { return c.Epochs[i] < c.Epochs[j] })
		c.ExplainedGaps, c.UnexplainedGaps = gaps(k, ps.seqs, c.LastSeq, cov)
		c.State = state(*c)
		if c.Epochs == nil {
			c.Epochs = []uint32{}
		}
		if c.Retired == nil {
			c.Retired = []perfdata.Retirement{}
		}
		if c.EpochResets == nil {
			c.EpochResets = []perfdata.EpochReset{}
		}
		rr.Coverage = append(rr.Coverage, *c)
	}
	ents, _ := os.ReadDir(filepath.Join(root, "distributions"))
	for _, e := range ents {
		if _, ok := rollupDay(e.Name()); !ok {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, "distributions", e.Name()))
		var r perfdata.DailyRollup
		if err == nil {
			r, err = perfdata.DecodeRollup(b)
		}
		if err != nil {
			for i := range rr.Coverage {
				if rr.Coverage[i].Source == primary {
					rr.Coverage[i].CorruptMetadata++
					rr.Coverage[i].State = perfdata.CoverageCorrupt
				}
			}
			continue
		}
		rr.Rollups = append(rr.Rollups, r)
	}
	return rr, nil
}

// gaps splits the missing sequences 1..last into those explained by a recorded
// retirement or failed write and those nothing explains. It walks holes between present
// sequences, so its cost follows the number of holes and ranges, not of sequences.
func gaps(src string, seqs map[uint64]bool, last uint64, cov coverageFile) (explained, unexplained []perfdata.SeqRange) {
	explained, unexplained = []perfdata.SeqRange{}, []perfdata.SeqRange{}
	var known []perfdata.SeqRange
	for _, r := range cov.Retired {
		if r.Source == src && r.Reason != RetireDistribution {
			known = append(known, perfdata.SeqRange{From: r.FromSeq, To: r.ToSeq, Reason: "retired-" + r.Reason})
		}
	}
	for _, l := range cov.Lost {
		if l.Source == src {
			known = append(known, l.Range)
		}
	}
	sort.Slice(known, func(i, j int) bool { return known[i].From < known[j].From })
	present := make([]uint64, 0, len(seqs))
	for q := range seqs {
		present = append(present, q)
	}
	sort.Slice(present, func(i, j int) bool { return present[i] < present[j] })
	emit := func(r perfdata.SeqRange) {
		list := &explained
		if r.Reason == perfdata.GapUnexplained {
			list = &unexplained
		}
		if n := len(*list); n > 0 && (*list)[n-1].Reason == r.Reason && (*list)[n-1].To+1 == r.From {
			(*list)[n-1].To = r.To
			return
		}
		*list = append(*list, r)
	}
	hole := func(a, b uint64) {
		q := a
		for _, k := range known {
			if k.To < q || k.From > b {
				continue
			}
			if k.From > q {
				emit(perfdata.SeqRange{From: q, To: k.From - 1, Reason: perfdata.GapUnexplained})
				q = k.From
			}
			end := min(k.To, b)
			emit(perfdata.SeqRange{From: q, To: end, Reason: k.Reason})
			q = end + 1
			if q > b {
				return
			}
		}
		emit(perfdata.SeqRange{From: q, To: b, Reason: perfdata.GapUnexplained})
	}
	next := uint64(1)
	for _, p := range present {
		if p > last {
			break
		}
		if p > next {
			hole(next, p-1)
		}
		next = p + 1
	}
	if next <= last {
		hole(next, last)
	}
	return explained, unexplained
}

func state(c perfdata.SourceCoverage) string {
	switch {
	case c.CorruptLines > 0 || c.CorruptMetadata > 0:
		return perfdata.CoverageCorrupt
	case len(c.UnexplainedGaps) > 0 || c.PartialTails > 0 || c.UncleanTerminations > 0 ||
		c.Drops != (perfdata.Drops{}) || len(c.EpochResets) > 0:
		return perfdata.CoveragePartial
	case c.Events == 0:
		return perfdata.CoverageEmptyComplete
	}
	return perfdata.CoverageComplete
}
