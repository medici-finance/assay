package main

// backfill — the one-off status-historian replayer (agentic-metrics/03).
//
// The transition historian (docs/streams/.history.jsonl) was seeded
// 2026-08-15, so before that date the only rows are ~1,069 from:"" snapshot
// seeds and every percentile lead/flow-time is statistically empty. But the
// ground truth for the ~6 weeks before the seed is not lost: brief statuses
// have lived in the git-tracked `docs/streams/*/README.md` status tables since
// July. This subcommand REPLAYS that git history — at each commit that touched
// a stream README it parses the status table and diffs it against the previous
// snapshot through statusgen's OWN diffHistory logic — and PREPENDS the
// reconstructed transitions to the log, turning "insufficient history" into a
// real ~6-week trend.
//
// It is a one-off, not a scheduled path. The live single-writer discipline
// (recordHistory, main CI only) is untouched: this command never runs inside
// --lint/--record and its rows are stamped source:"backfill" so a reader can
// weight commit-date-precision reconstructed dwell differently from a live
// regen-observed transition.
//
// # Ground rules (from the brief)
//
//   - IDEMPOTENT. Re-running must not duplicate. The idempotency key is
//     {brief, to, sha}: a reconstructed transition is tied to the exact commit
//     whose table first shows the new status, so the key is unique per real
//     transition and stable across runs. Any key already present in the log
//     (backfilled on a prior run, OR a live row) is skipped.
//   - PREPEND only. The 2026-08-15+ live rows are preserved BYTE-FOR-BYTE and
//     kept last, so LastRecordedStatus (position: last-write-wins) still yields
//     the current live status and the seed rows are never reordered or
//     rewritten. Reconstructed history goes in front of them.
//   - COMMIT-DATE precision. A replayed transition's ts is the committer date
//     of the commit that first showed the new status. Intermediate stages that
//     flipped and flipped back BETWEEN two commits are unobservable and lost —
//     `to` is recorded as observed, never interpolated (see backfill-notes.md).
//
// # Exit contract (three-state, matches verifyrun/shardcheck in this binary)
//
//	0 = reconstructed and merged (or a clean idempotent no-op — nothing new)
//	1 = a real failure while writing the merged log
//	2 = could-not-check: the root/git/history is unreadable, nothing written

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	backfillOK       = 0
	backfillFailed   = 1
	backfillCouldNot = 2
)

// backfillCommit is one commit that touched a stream README, with its
// committer time (the observation instant for any transition it introduced).
type backfillCommit struct {
	SHA  string
	When time.Time
}

// streamReadmeCommits lists the commits that touched rel (a repo-relative
// README path) in OLDEST-FIRST order — the order a forward replay needs.
// %ct is the committer date, matching gitLastTouch/gitCommitTime elsewhere in
// this binary. A path with no history returns an empty slice (not an error).
func streamReadmeCommits(root, rel string) ([]backfillCommit, error) {
	out, err := exec.Command("git", "-C", root, "log", "--reverse", "--format=%H %ct", "--", rel).Output()
	if err != nil {
		return nil, fmt.Errorf("git log for %s: %w", rel, err)
	}
	var commits []backfillCommit
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		sec, perr := strconv.ParseInt(fields[1], 10, 64)
		if perr != nil {
			continue
		}
		commits = append(commits, backfillCommit{SHA: fields[0], When: time.Unix(sec, 0).UTC()})
	}
	return commits, nil
}

// readmeAtCommit returns the content of rel as it was at sha. A file that did
// not exist at that revision (deleted-then-readded gaps) yields ok=false, not
// an error — the caller skips that snapshot.
func readmeAtCommit(root, sha, rel string) (content string, ok bool) {
	out, err := exec.Command("git", "-C", root, "show", sha+":"+rel).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// parseStreamSnapshot parses a historical README's content into its stream
// name and brief rows, reusing the live parsers (splitFrontmatter +
// parseBriefTable). ok=false when the content is not a parseable stream README
// at that commit (an early stub before the table existed, a schema the current
// parsers reject, malformed frontmatter) — a skip, never a crash. fallbackName
// (the stream directory name) is used when the frontmatter carries no stream:.
func parseStreamSnapshot(content, fallbackName string) (name string, briefs []Brief, ok bool) {
	fmRaw, body, err := splitFrontmatter(content)
	if err != nil {
		return "", nil, false
	}
	var fm frontmatter
	if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
		return "", nil, false
	}
	briefs, err = parseBriefTable(body)
	if err != nil || len(briefs) == 0 {
		return "", nil, false
	}
	name = strings.TrimSpace(fm.Stream)
	if name == "" {
		name = fallbackName
	}
	return name, briefs, true
}

// historyKey is the idempotency key: two rows are the SAME reconstructed
// transition iff their {brief, to, sha} agree.
func historyKey(e HistoryEntry) string {
	return e.Brief + "\x00" + e.To + "\x00" + e.SHA
}

// reconstructBackfill walks every current stream README's git history and
// returns the full reconstructed transition set, source:"backfill" stamped,
// plus the count of commit snapshots that could not be parsed (reported, not
// swallowed). It reuses diffHistory for the per-commit diff, maintaining the
// running last-recorded state itself.
func reconstructBackfill(root string) (entries []HistoryEntry, couldNotParse int, streamsSeen int, err error) {
	streamsDir := filepath.Join(root, "docs", "streams")
	dirEntries, derr := os.ReadDir(streamsDir)
	if derr != nil {
		return nil, 0, 0, fmt.Errorf("reading %s: %w", streamsDir, derr)
	}
	last := map[string]string{}
	for _, de := range dirEntries {
		if !de.IsDir() || reservedRegisterNames[de.Name()] {
			continue
		}
		rel := filepath.ToSlash(filepath.Join("docs", "streams", de.Name(), "README.md"))
		if _, serr := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); serr != nil {
			continue // a stream dir with no README at HEAD — not a stream to replay
		}
		commits, cerr := streamReadmeCommits(root, rel)
		if cerr != nil {
			return nil, 0, 0, cerr
		}
		if len(commits) == 0 {
			continue
		}
		streamsSeen++
		for _, c := range commits {
			content, ok := readmeAtCommit(root, c.SHA, rel)
			if !ok {
				continue
			}
			name, briefs, ok := parseStreamSnapshot(content, de.Name())
			if !ok {
				couldNotParse++
				continue
			}
			snap := &Stream{Name: name, Briefs: briefs}
			// Reuse the live diff logic: transitions since the running state.
			diff := diffHistory([]*Stream{snap}, last, c.SHA, c.When)
			for i := range diff {
				diff[i].Source = "backfill"
			}
			entries = append(entries, diff...)
			// Advance the running state for every brief in this snapshot so the
			// next commit diffs against it (diffHistory itself does not mutate last).
			for _, b := range briefs {
				last[name+"/"+b.Num] = b.Status
			}
		}
	}
	return entries, couldNotParse, streamsSeen, nil
}

// runBackfill is the `statusgen backfill` positional subcommand. It
// reconstructs pre-seed transitions from git history and PREPENDS the new ones
// (idempotently) to docs/streams/.history.jsonl, leaving the live rows
// untouched. --dry-run computes and reports without writing.
func runBackfill(args []string, stdout, stderr *os.File) int {
	flags := flag.NewFlagSet("backfill", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root whose docs/streams/*/README.md history to replay")
	dryRun := flags.Bool("dry-run", false, "compute and report the reconstructed rows without writing the log")
	if err := flags.Parse(args); err != nil {
		return backfillCouldNot
	}
	if hasNoGitDir(*root) {
		fmt.Fprintf(stderr, "could-not-check: %s has no .git — backfill replays git history and has none to read\n", *root)
		return backfillCouldNot
	}

	path := filepath.Join(*root, filepath.FromSlash(historyRelPath))
	existing, err := LoadHistory(path)
	if err != nil {
		fmt.Fprintf(stderr, "could-not-check: reading existing history: %v\n", err)
		return backfillCouldNot
	}
	seen := make(map[string]bool, len(existing))
	for _, e := range existing {
		seen[historyKey(e)] = true
	}

	reconstructed, couldNotParse, streamsSeen, rerr := reconstructBackfill(*root)
	if rerr != nil {
		fmt.Fprintf(stderr, "could-not-check: %v\n", rerr)
		return backfillCouldNot
	}

	// Filter to genuinely-new rows (idempotent against live rows AND a prior
	// backfill run AND intra-batch duplicates).
	var fresh []HistoryEntry
	for _, e := range reconstructed {
		k := historyKey(e)
		if seen[k] {
			continue
		}
		seen[k] = true
		fresh = append(fresh, e)
	}
	// Chronological, then brief, then status — a deterministic, readable
	// prepended block. (File order is cosmetic for the ts-keyed consumers, but
	// determinism keeps re-runs and diffs stable.)
	sort.SliceStable(fresh, func(i, j int) bool {
		if fresh[i].Ts != fresh[j].Ts {
			return fresh[i].Ts < fresh[j].Ts
		}
		if fresh[i].Brief != fresh[j].Brief {
			return fresh[i].Brief < fresh[j].Brief
		}
		return fresh[i].To < fresh[j].To
	})

	fmt.Fprintf(stdout, "backfill: %d stream README(s) replayed, %d reconstructed transition(s), %d new after idempotent filter",
		streamsSeen, len(reconstructed), len(fresh))
	if couldNotParse > 0 {
		fmt.Fprintf(stdout, ", %d snapshot(s) skipped (unparseable table at that commit)", couldNotParse)
	}
	fmt.Fprintln(stdout)
	if len(fresh) > 0 {
		fmt.Fprintf(stdout, "backfill: recovered date range %s .. %s\n", fresh[0].Ts, fresh[len(fresh)-1].Ts)
	}

	if len(fresh) == 0 {
		fmt.Fprintln(stdout, "backfill: nothing new — log unchanged (idempotent no-op)")
		return backfillOK
	}
	if *dryRun {
		fmt.Fprintf(stdout, "backfill: --dry-run — %d row(s) NOT written\n", len(fresh))
		return backfillOK
	}

	// Preserve the live rows byte-for-byte: read the raw file and PREPEND the
	// new block in front of it rather than re-encoding what is already there.
	var rawExisting []byte
	if b, rerr := os.ReadFile(path); rerr == nil {
		rawExisting = b
	} else if !os.IsNotExist(rerr) {
		fmt.Fprintf(stderr, "backfill: reading %s: %v\n", path, rerr)
		return backfillFailed
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // match appendHistory's encoding exactly
	for _, e := range fresh {
		if err := enc.Encode(e); err != nil {
			fmt.Fprintf(stderr, "backfill: encoding row: %v\n", err)
			return backfillFailed
		}
	}
	buf.Write(rawExisting)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(stderr, "backfill: %v\n", err)
		return backfillFailed
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintf(stderr, "backfill: writing %s: %v\n", path, err)
		return backfillFailed
	}
	fmt.Fprintf(stdout, "backfill: prepended %d reconstructed transition(s) to %s\n", len(fresh), path)
	return backfillOK
}
