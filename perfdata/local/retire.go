package local

// retire.go — age and byte-cap retirement. Retired data was never sent (no uploader
// exists in this module), so every retirement is recorded as unsent coverage loss. With
// distribution retention configured, retired tool completions are first folded into
// daily rollups, so pooled Avg/P95 survive the raw age cap; rollups past their own
// window are retired with a recorded window too.
//
// Order is crash-safe against double counting: the retirement record is saved before
// the rollup and before the segment is removed, and a reader ignores any segment whose
// range is already recorded as retired.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/medici-finance/assay/perfdata"
)

// Retirement reasons.
const (
	RetireAge          = "age"
	RetireBytes        = "bytes"
	RetireDistribution = "distribution-age"
)

func (cov *coverageFile) retiredSegment(src string, sg segment) bool {
	for _, r := range cov.Retired {
		if r.Source == src && r.FromSeq == sg.firstSeq && r.Reason != RetireDistribution {
			return true
		}
	}
	return false
}

func (s *Store) retire(cov *coverageFile) error {
	now := s.opts.now()
	keepFrom := now.AddDate(0, 0, -(s.policy.AgeCapDays - 1)).Format("2006-01-02")
	segs := listSegments(s.root)
	total := int64(0)
	sizes := map[string]int64{}
	for _, sg := range segs {
		if st, err := os.Stat(filepath.Join(s.root, "segments", sg.name)); err == nil {
			sizes[sg.name] = st.Size()
			total += st.Size()
		}
	}
	for _, sg := range segs {
		reason := ""
		switch {
		case sg.day < keepFrom:
			reason = RetireAge
		case total > s.policy.ByteCap:
			reason = RetireBytes
		default:
			continue
		}
		if err := s.retireSegment(cov, sg, reason, now); err != nil {
			return err
		}
		total -= sizes[sg.name]
	}
	if s.policy.DistributionDays > 0 {
		dKeep := now.AddDate(0, 0, -(s.policy.DistributionDays - 1)).Format("2006-01-02")
		ents, _ := os.ReadDir(filepath.Join(s.root, "distributions"))
		for _, e := range ents {
			day, ok := rollupDay(e.Name())
			if !ok || day >= dKeep {
				continue
			}
			cov.Retired = append(cov.Retired, retired{Retirement: perfdata.Retirement{
				Reason: RetireDistribution, FromDay: day, ToDay: day, Unsent: true, At: perfdata.FormatTime(now)}})
			if err := s.saveCoverage(*cov); err != nil {
				return err
			}
			_ = os.Remove(filepath.Join(s.root, "distributions", e.Name()))
		}
	}
	return nil
}

func rollupDay(name string) (string, bool) {
	if !strings.HasPrefix(name, "day-") || !strings.HasSuffix(name, ".json") {
		return "", false
	}
	d := strings.TrimSuffix(strings.TrimPrefix(name, "day-"), ".json")
	if _, ok := perfdata.ParseTime(d + "T00:00:00.000Z"); !ok {
		return "", false
	}
	return d, true
}

func (s *Store) retireSegment(cov *coverageFile, sg segment, reason string, now interface{ Format(string) string }) error {
	p := filepath.Join(s.root, "segments", sg.name)
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	var events []perfdata.Event
	src := ""
	last := sg.firstSeq
	for _, line := range bytes.Split(b, []byte("\n")) {
		e, err := perfdata.DecodeEvent(line)
		if err != nil {
			continue
		}
		events = append(events, e)
		src = e.Source
		last = max(last, e.Seq)
	}
	if src == "" {
		if sf, err := readSource(s.root); err == nil {
			src = sf.Source
		}
	}
	if cov.retiredSegment(src, sg) {
		return os.Remove(p) // retirement already recorded; finish the removal
	}
	r := retired{Source: src, Retirement: perfdata.Retirement{
		Reason: reason, FromSeq: sg.firstSeq, ToSeq: last, Events: len(events), Bytes: int64(len(b)),
		FromDay: sg.day, ToDay: sg.day, Unsent: true, At: now.Format(perfdata.TimeLayout),
	}}
	if s.policy.DistributionDays > 0 && len(events) > 0 {
		r.RolledUp = true
	}
	cov.Retired = append(cov.Retired, r)
	if err := s.saveCoverage(*cov); err != nil {
		return err
	}
	if r.RolledUp {
		if err := s.rollUp(events); err != nil {
			return err
		}
	}
	return os.Remove(p)
}

// rollUp folds tool completions into their days' rollup files.
func (s *Store) rollUp(events []perfdata.Event) error {
	days := map[string]*perfdata.DailyRollup{}
	for _, e := range events {
		if e.Kind != perfdata.KindTool {
			continue
		}
		v, err := perfdata.DecodePayload(e.Kind, e.Payload)
		if err != nil {
			continue
		}
		tc := v.(*perfdata.ToolCompletion)
		day := perfdata.DayOf(tc.StartedAt)
		r := days[day]
		if r == nil {
			nr := perfdata.NewRollup(day)
			r = &nr
			days[day] = r
		}
		r.AddEvent(e, *tc)
	}
	keys := make([]string, 0, len(days))
	for d := range days {
		keys = append(keys, d)
	}
	sort.Strings(keys)
	for _, day := range keys {
		r := days[day]
		name := "day-" + day + ".json"
		if b, err := os.ReadFile(filepath.Join(s.root, "distributions", name)); err == nil {
			old, err := perfdata.DecodeRollup(b)
			if err != nil {
				return errors.New("perfdata/local: corrupt rollup")
			}
			if err := old.MergeRollup(*r); err != nil {
				return err
			}
			r = &old
		}
		if err := s.writeJSON(filepath.Join("distributions", name), r); err != nil {
			return err
		}
	}
	return nil
}
