// snapshot.go — the reducer's INPUT side: the same run's four
// metrics-snapshot-<grouping>-<run-id> artifacts, as downloaded by the
// aggregate job (`needs: harvest`, `if: always()`,
// actions/download-artifact with `pattern: metrics-snapshot-*-<run_id>`).
//
// There is no committed raw tree and no cross-run Actions-API read: the
// aggregate job reads its OWN run's sibling uploads and nothing else, so
// artifact retention cannot silently change what a past day's roll-up was
// computed from. The committed roll-up is the durable record.
//
// The single rule this file exists to enforce: A COULD-NOT-CHECK MUST NEVER
// BECOME A MEASURED VALUE. Every read below is three-state — present-and-
// readable, present-and-failed, absent — and each state stays distinguishable
// all the way to the published figure. The collector already speaks this
// dialect: on a failed `gh` read it writes the field as JSON null and exits
// non-zero while still uploading the snapshot, so a null here means "checked,
// and the check failed", which is NOT the same fact as a missing key.
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Field names, used both as JSON keys of a snapshot repo entry and as the
// keys of its optional `truncated` declaration.
const (
	fieldPRsOpenByState          = "prsOpenByState"
	fieldIssuesOpenByLabel       = "issuesOpenByLabel"
	fieldIssuesOpenUnlabeled     = "issuesOpenUnlabeled"
	fieldPRsOpenByReviewDecision = "prsOpenByReviewDecision"
)

// fieldState is the three-state read of ONE field of ONE repo's snapshot
// entry. It is deliberately not a bool: collapsing these is defect 5,
// and rendering `absent` as a value is defect 1.
type fieldState int

const (
	// fieldOK — the key is present with a usable value.
	fieldOK fieldState = iota
	// fieldFailed — the key is present and null: the collector tried the
	// read and it errored (checked-failed).
	fieldFailed
	// fieldAbsent — the key is not present at all: nothing was ever
	// declared about it (could-not-check).
	fieldAbsent
)

// prsByState is the collector's `prsOpenByState` object. Both members are
// pointers because the collector writes {"draft":null,"ready":null} when
// `gh pr list` failed — present, and declaring failure.
type prsByState struct {
	Draft *int `json:"draft"`
	Ready *int `json:"ready"`
}

// snapshotRepo is one entry of a snapshot's `repos` array. It is decoded via
// a raw-key pass rather than plain struct tags so that "key absent" stays
// distinguishable from "key present and null" — encoding/json alone cannot
// tell those apart for a map or a pointer field.
type snapshotRepo struct {
	Repo string

	present map[string]bool

	prsOpenByState      *prsByState
	issuesOpenByLabel   map[string]int
	issuesOpenUnlabeled *int

	// reviewDecision stays nil until the collector emits it (landing separately). Until
	// then the published section is null — never zero.
	reviewDecision map[string]int

	// truncated is the source's own declaration, per field, of whether its
	// upstream listing hit the API cap (`--limit 1000`). defect 8: the
	// collector today warns about truncation on stderr only, so the fact does
	// not survive the artifact boundary and this map is normally nil. The
	// reducer therefore tracks a third possibility — "truncation status was
	// never declared" — and marks the published figure accordingly rather
	// than pretending a possibly-capped count is complete.
	truncated map[string]bool
}

func (r *snapshotRepo) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	r.present = map[string]bool{}
	for k, v := range raw {
		r.present[k] = !isJSONNull(v)
	}

	if v, ok := raw["repo"]; ok {
		if err := json.Unmarshal(v, &r.Repo); err != nil {
			return fmt.Errorf("repo: %w", err)
		}
	}
	if v, ok := raw[fieldPRsOpenByState]; ok && !isJSONNull(v) {
		var p prsByState
		if err := json.Unmarshal(v, &p); err != nil {
			return fmt.Errorf("%s: %w", fieldPRsOpenByState, err)
		}
		r.prsOpenByState = &p
	}
	if v, ok := raw[fieldIssuesOpenByLabel]; ok && !isJSONNull(v) {
		if err := json.Unmarshal(v, &r.issuesOpenByLabel); err != nil {
			return fmt.Errorf("%s: %w", fieldIssuesOpenByLabel, err)
		}
		if r.issuesOpenByLabel == nil {
			r.issuesOpenByLabel = map[string]int{}
		}
	}
	if v, ok := raw[fieldIssuesOpenUnlabeled]; ok && !isJSONNull(v) {
		if err := json.Unmarshal(v, &r.issuesOpenUnlabeled); err != nil {
			return fmt.Errorf("%s: %w", fieldIssuesOpenUnlabeled, err)
		}
	}
	if v, ok := raw[fieldPRsOpenByReviewDecision]; ok && !isJSONNull(v) {
		if err := json.Unmarshal(v, &r.reviewDecision); err != nil {
			return fmt.Errorf("%s: %w", fieldPRsOpenByReviewDecision, err)
		}
	}
	if v, ok := raw["truncated"]; ok && !isJSONNull(v) {
		if err := json.Unmarshal(v, &r.truncated); err != nil {
			return fmt.Errorf("truncated: %w", err)
		}
	}
	return nil
}

func isJSONNull(v json.RawMessage) bool {
	return string(bytesTrimSpace(v)) == "null"
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

// stateOf reports the three-state read of one field name.
func (r *snapshotRepo) stateOf(field string) fieldState {
	declared, ok := r.present[field]
	if !ok {
		return fieldAbsent
	}
	if !declared {
		return fieldFailed
	}
	switch field {
	case fieldPRsOpenByState:
		// {"draft":null,"ready":null} is the collector's failure shape: the
		// object is present, but it declares no numbers.
		if r.prsOpenByState == nil || r.prsOpenByState.Draft == nil || r.prsOpenByState.Ready == nil {
			return fieldFailed
		}
	case fieldIssuesOpenByLabel:
		if r.issuesOpenByLabel == nil {
			return fieldFailed
		}
	case fieldIssuesOpenUnlabeled:
		if r.issuesOpenUnlabeled == nil {
			return fieldFailed
		}
	case fieldPRsOpenByReviewDecision:
		if r.reviewDecision == nil {
			return fieldFailed
		}
	}
	return fieldOK
}

// truncationOf reports what the source declared about a field's upstream
// truncation: (declared, truncated). declared==false is the defect 8
// state — the reducer could not check, and says so at the number.
func (r *snapshotRepo) truncationOf(field string) (declared, truncated bool) {
	if r.truncated == nil {
		return false, false
	}
	v, ok := r.truncated[field]
	if !ok {
		return false, false
	}
	return true, v
}

// snapshotFile is one metrics-snapshot-<grouping>-<date>.json.
type snapshotFile struct {
	Grouping string         `json:"grouping"`
	Date     string         `json:"date"`
	Repos    []snapshotRepo `json:"repos"`
}

// groupingSource is how a grouping's snapshot arrived — the grouping-level
// half of the same three states.
type groupingSource string

const (
	// sourceSnapshot — the artifact was there and parsed.
	sourceSnapshot groupingSource = "snapshot-artifact"
	// sourceUnreadable — a file for this grouping was there and did NOT
	// parse. Checked-failed: distinct from absent, and never reported as
	// "no snapshot" (defect 5's class, one level up).
	sourceUnreadable groupingSource = "snapshot-unreadable"
	// sourceAbsent — no artifact for this grouping in this run's download.
	sourceAbsent groupingSource = "snapshot-absent"
	// sourceDateMismatch — a snapshot arrived, but for a different date than
	// the one being rolled up. Its numbers are not this day's, so they are
	// not used; saying so is the honest read.
	sourceDateMismatch groupingSource = "snapshot-date-mismatch"
)

// groupingSnapshot is the loaded input for one grouping.
type groupingSnapshot struct {
	Source groupingSource
	Note   string
	File   *snapshotFile
	// byRepo indexes File.Repos on the FULL "owner/repo" spec. Full-spec
	// keying is defect 7's fix: base-name keying collapsed two distinct
	// repos into one directory, double-counting one and never reading the
	// other while reporting no gap at all.
	byRepo map[string]*snapshotRepo
}

var snapshotNameRE = regexp.MustCompile(`^metrics-snapshot-([a-z0-9][a-z0-9-]*)-(\d{4}-\d{2}-\d{2})\.json$`)

// loadSnapshots walks the download directory and returns one groupingSnapshot
// per known grouping. It REFUSES (error) only on ambiguity it cannot reduce
// honestly — two snapshots claiming the same grouping. Everything else is
// recorded as a state, because a missing or broken artifact is exactly the
// day the roll-up has to describe rather than fail on.
func loadSnapshots(dir, dateLabel string) (map[string]*groupingSnapshot, error) {
	out := map[string]*groupingSnapshot{}
	for _, g := range groupingOrder {
		out[g] = &groupingSnapshot{
			Source: sourceAbsent,
			Note:   fmt.Sprintf("no metrics-snapshot-%s-*.json under %s", g, dir),
			byRepo: map[string]*snapshotRepo{},
		}
	}

	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			// Not a refusal: a run in which every harvest leg failed to
			// upload produces no download directory at all, and that day's
			// roll-up must still publish four could-not-checks rather than
			// four zeroes (defect 1).
			for _, g := range groupingOrder {
				out[g].Note = fmt.Sprintf("snapshot directory %s does not exist", dir)
			}
			return out, nil
		}
		return nil, fmt.Errorf("cannot stat snapshot directory %s: %w", dir, err)
	}

	claimed := map[string]string{} // grouping -> path that claimed it
	var walkErr error
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		m := snapshotNameRE.FindStringSubmatch(d.Name())
		if m == nil {
			return nil
		}
		grouping := m[1]
		gs, known := out[grouping]
		if !known {
			// A snapshot for a grouping domains.yaml does not know about is
			// config drift, not data — surfaced, never silently dropped.
			walkErr = fmt.Errorf("snapshot %s claims unknown grouping %q", path, grouping)
			return nil
		}
		if prior, dup := claimed[grouping]; dup {
			walkErr = fmt.Errorf("two snapshots claim grouping %q (%s and %s) — "+
				"refusing to guess which is this run's", grouping, prior, path)
			return nil
		}
		claimed[grouping] = path

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			gs.Source = sourceUnreadable
			gs.Note = fmt.Sprintf("%s exists but could not be read: %v", path, readErr)
			return nil
		}
		var sf snapshotFile
		if jsonErr := json.Unmarshal(data, &sf); jsonErr != nil {
			gs.Source = sourceUnreadable
			gs.Note = fmt.Sprintf("%s exists (%d bytes) but did not parse: %v", path, len(data), jsonErr)
			return nil
		}
		if sf.Date != "" && sf.Date != dateLabel {
			gs.Source = sourceDateMismatch
			gs.Note = fmt.Sprintf("%s declares date %q, not %q — its counts are not this day's",
				path, sf.Date, dateLabel)
			return nil
		}
		gs.Source = sourceSnapshot
		gs.Note = ""
		gs.File = &sf
		for i := range sf.Repos {
			r := &sf.Repos[i]
			gs.byRepo[r.Repo] = r
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", dir, err)
	}
	if walkErr != nil {
		return nil, walkErr
	}
	return out, nil
}

// unexpectedRepos lists snapshot entries that domains.yaml does not configure
// for this grouping — config drift in the other direction. Their numbers are
// never summed (the configured set is what the figure claims to cover), but
// the drift is reported rather than dropped.
func (gs *groupingSnapshot) unexpectedRepos(configured []string) []string {
	if gs.File == nil {
		return nil
	}
	want := map[string]bool{}
	for _, s := range configured {
		want[s] = true
	}
	var out []string
	for spec := range gs.byRepo {
		if !want[spec] {
			out = append(out, spec)
		}
	}
	sort.Strings(out)
	return out
}
