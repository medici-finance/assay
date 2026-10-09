package main

// decisiongatehold.go — the decision-gate hold on `gate: human` briefs
// (spec/lifecycle-v1.md §4.5).
//
// A `gate: human` brief carries a decision issue; the human decides on it BEFORE
// the irreversible act. This file makes that binding at the brief's STATUS
// TRANSITION, judged per change against the base the change merges onto. It is
// never consulted by a pull request's ready-flip: decision latency is a queue's
// main bottleneck, and a flip-block would wedge it. That boundary is deliberate.
//
// WHAT IS REFUSED. While a brief's decision issue has no ruling, a change is
// refused when, for a brief carrying `gate: human` at the base OR after the
// change:
//
//   - it leaves the brief's status at implemented, verified or done and, at the
//     base, the status was earlier or the brief was not on the board (a MOVE);
//   - it moves the brief's gate away from `human`, or deletes the key (a RELABEL);
//   - no brief after the change is the SAME brief as a `gate: human` base brief
//     (below) still on the board — a DROP. The README table is the board: a
//     base brief that had a row and has none after the change is dropped, even
//     when its brief file stays behind (at the stream root, under done/ or in
//     the archive tree);
//   - it leaves a board id the hold covers resolving to more than one record
//     (an AMBIGUITY): no single transition can be judged, and no recorded
//     ruling can lift it;
//   - it leaves a board record the hold cannot read the way the board reads it
//     (UNREADABLE) — a stream index, brief file, done/ folder, stream directory
//     or board tree that is not a plain file or directory in the tree, or a
//     board file that cannot be read. No recorded ruling can lift it.
//
// ONE READING RULE. The base is read from the git tree and the change from the
// working tree, and both listings are classified by one rule (buildGateSnapshot
// over gateEntry kinds) before any content is read, so a record either side
// cannot read as the board does is unreadable on both — never a brief with no
// gate. At the base an unreadable record reads conservatively: gate: human, no
// ruling.
//
// A brief is matched across the base and the change by EITHER key — its board
// id (<stream>/<NN>) or its permanent frontmatter `id:` — and either key alone
// puts a brief in SCOPE. A row already at or past its new status at the base is
// left alone, so a pin bump turns nothing red that has already landed.
//
// IDENTITY IS ONE-TO-ONE. Matching decides scope; it never lends a ruling, a
// prior status, or presence. A base brief and a brief after the change are the
// SAME brief only by the same board id, or by a RENUMBER (the base brief's board
// id is gone after the change AND the other board id is new in it —
// isGateRenumber) that pairs exactly one base brief with exactly one new board
// id (gateSameBriefPairs). Only the same brief supplies the prior status that
// tells a move from a landed row, lends its recorded ruling, and keeps a base
// brief on the board. So moving a ruled brief's permanent id: onto another brief
// puts that brief in scope but cannot hand it the ruling; parking a removed
// brief's id: on a brief already on the board does not keep the removed brief
// (it is a drop); a NEW row that takes the id: of a brief still on the board is
// a move from "not on the board"; and one base brief cannot stand behind two
// renumbers, nor one renumber absorb two base briefs.
//
// Every README row and brief file read is accounted to exactly one board id
// (buildGateSnapshot keeps them all in Records). Number spellings normalise
// (018 and 18 are one board id) and docs/streams and docs/archive share board
// ids, so a board id with two rows, two brief files, or records in both trees
// is AMBIGUOUS. An ambiguous board id is judged from a conservative merge — the
// lowest row status, gate: human if any file says so, no ruling — and a change
// that leaves one in scope with records different from the base's is refused
// outright. The same colliding records left untouched turn nothing red.
//
// WHAT COUNTS AS RULED. The brief records its decision issue (`decision-issue:`)
// and a link to the ruling comment (`ruling:`) in its frontmatter. Two layers:
//
//   - Layer one (offline, every --lint): both keys are present, the link is a
//     well-formed issue-comment URL, and it points at the recorded decision
//     issue. It cannot tell who wrote the comment, and its refusal says so.
//   - Layer two (network: --decision-gate, and the same lane inside
//     --corroborate and --close-verify): the linked comment is fetched and must
//     pass every condition of the design-decision ruling-link check
//     (spec/registers-v1.md §7.5) — an unedited comment by a login mapped to a
//     human, never a bot, on the recorded decision issue, whose body carries the
//     brief's decision-gate marker, and whose own text names the brief by its
//     board id. resolveRulingOn is that check; only its issue binding differs.
//
// Any ruling lifts the refusal, including one that holds or declines the brief:
// the check confirms THAT a human decided, not what they decided.
//
// Pure verification: every function below is a closed-form comparison of two
// board snapshots plus a read of one comment. Nothing here asserts at runtime
// elsewhere in statusgen.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// decisionGateSpecRef is the citation every refusal carries.
const decisionGateSpecRef = "lifecycle-v1 §4.5"

// decisionGateLayerOneScope is the sentence every layer-one refusal ends with.
// Layer one cannot tell whether layer two runs, so it always says what it did
// not check.
const decisionGateLayerOneScope = "This offline check looked only for a well-formed ruling link; it cannot tell who wrote the linked comment — the network pass (statusgen --decision-gate, also run by --corroborate) checks that."

// gateBrief is one brief as a board snapshot sees it.
type gateBrief struct {
	Key           string   // <stream>/<normalised NN> — the board-id match key
	Board         string   // <stream>/<NN> as the board spells it
	Spellings     []string // every spelling of the board id seen (row and file)
	PermID        string   // frontmatter id: ("" = none)
	BriefID       string   // frontmatter brief: ("" = none)
	Gate          string   // frontmatter gate:, lowercased ("" = absent or unreadable)
	FMErr         bool     // the brief file's frontmatter did not parse
	Status        string   // the README row's status ("" = no row)
	DecisionIssue int      // frontmatter decision-issue: (0 = absent)
	Ruling        string   // frontmatter ruling: ("" = absent)
	Path          string   // repo-relative brief file ("" = a row with no file)
	Readme        string   // repo-relative stream README ("" = none)

	// Identity accounting. Every README row and every brief file the snapshot
	// reads lands in exactly one board id's Records, so nothing read is ever
	// silently discarded. A board id with more than one row, more than one
	// brief file, or records from both docs/streams and docs/archive is
	// AMBIGUOUS: the fields above then hold a conservative merge (the lowest
	// row status, gate: human if any file says so, no ruling), never the first
	// record's word.
	Records   []string // canonical form of every row and brief file read for this board id
	PermIDs   []string // every non-empty permanent id: among those files
	Ambiguous string   // why this board id resolves to more than one record ("" = exactly one)

	// Unreadable: a record of this board id (or, for a SCOPE record — Key
	// beginning gateScopeKeyPrefix — a whole stream index, stream directory,
	// done/ folder or board tree) is not a plain file or directory in the tree,
	// or could not be read, so the hold cannot read it the way the board does.
	// Such a record is never taken as carrying no gate: its fields are the
	// conservative reading (gate: human, no ruling), and a change that leaves
	// one is refused (judgeDecisionGate).
	Unreadable string
}

// gateScopeKeyPrefix keys an unreadable SCOPE record: a board path whose
// records the hold cannot enumerate (a stream index, a stream directory, a
// done/ folder, or docs/streams or docs/archive themselves). No board id
// starts with it.
const gateScopeKeyPrefix = "!"

// gateEntryKind is what a board path is in the tree it was read from. Both
// sides of a change are classified by this one rule: the base from the git tree
// (gitTreeEntryKind), the working tree by lstat (diskEntryKind), so a path
// the board would follow somewhere else reads alike on both sides — as not a
// plain file.
type gateEntryKind int

const (
	gateEntryFile    gateEntryKind = iota // a plain file (git 100644/100755)
	gateEntryDir                          // a plain directory (git tree)
	gateEntrySymlink                      // a symbolic link (git 120000)
	gateEntryNested                       // a submodule / nested repository (git 160000, or a directory holding .git)
	gateEntryOther                        // anything else
)

func (k gateEntryKind) String() string {
	switch k {
	case gateEntryFile:
		return "a plain file"
	case gateEntryDir:
		return "a directory"
	case gateEntrySymlink:
		return "a symbolic link"
	case gateEntryNested:
		return "a submodule or nested repository"
	}
	return "not a plain file"
}

// gateEntry is one path under docs/streams or docs/archive (or docs itself),
// repo-relative with forward slashes, and what it is.
type gateEntry struct {
	Path string
	Kind gateEntryKind
}

// gateTreeRootRe, gateStreamDirRe and gateDoneDirRe select the directories the
// board descends through to reach its files. Each must be a plain directory
// (or absent): the board's own readers follow a link or read into a nested
// repository there, which the base's git tree does not.
var (
	gateTreeRootRe  = regexp.MustCompile(`^docs(/(streams|archive))?$`)
	gateStreamDirRe = regexp.MustCompile(`^docs/(streams|archive)/([^/]+)$`)
	gateDoneDirRe   = regexp.MustCompile(`^docs/(streams|archive)/([^/]+)/done$`)
)

// fingerprint is the board id's full record set, order-independent: two
// snapshots hold the same records for a board id exactly when these match.
func (gb *gateBrief) fingerprint() string {
	rs := append([]string(nil), gb.Records...)
	sort.Strings(rs)
	return strings.Join(rs, "\n")
}

// gateSnapshot is a board, keyed by gateBrief.Key.
type gateSnapshot map[string]*gateBrief

// gateBoardPathRe selects the files a board snapshot reads: each stream's
// README and its brief files, active (docs/streams) or archived (docs/archive),
// including briefs a stream moved into its done/ folder.
var gateBoardPathRe = regexp.MustCompile(`^docs/(streams|archive)/([^/]+)/(README\.md|brief-[^/]+\.md|done/brief-[^/]+\.md)$`)

// gateStatusRank orders the statuses the hold covers. Every other status ranks 0.
func gateStatusRank(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "implemented":
		return 1
	case "verified":
		return 2
	case "done":
		return 3
	}
	return 0
}

// buildGateSnapshot reads a board from a list of repo-relative entries. read
// returns a plain file's bytes. Both sides of a change come through here, so
// one rule decides what a record says: a board file that is not a plain file,
// or that cannot be read, is UNREADABLE — never a brief with no gate — and so
// is a stream index, a stream directory, a done/ folder or a board tree that
// is not a plain directory (gateUnreadableScope). The judge refuses a change
// that leaves an unreadable record, and reads one at the base conservatively.
func buildGateSnapshot(entries []gateEntry, read func(string) ([]byte, error)) gateSnapshot {
	type group struct {
		tree, name, readme string
		briefs             []string
	}
	snap := gateSnapshot{}
	kinds := map[string]gateEntryKind{}
	groups := map[string]*group{}
	for _, e := range entries {
		p := filepath.ToSlash(strings.TrimSpace(e.Path))
		kinds[p] = e.Kind
		if gateTreeRootRe.MatchString(p) {
			if e.Kind != gateEntryDir {
				gateUnreadableScope(snap, p, e.Kind.String()+", not a plain directory, where the board reads its streams")
			}
			continue
		}
		if m := gateStreamDirRe.FindStringSubmatch(p); m != nil {
			// A plain file beside the streams is not a stream; a link or a
			// nested repository can be read as one by the board's own readers.
			if !reservedRegisterNames[m[2]] && (e.Kind == gateEntrySymlink || e.Kind == gateEntryNested) {
				gateUnreadableScope(snap, p, e.Kind.String()+" in place of a stream directory")
			}
			continue
		}
		if m := gateDoneDirRe.FindStringSubmatch(p); m != nil {
			if !reservedRegisterNames[m[2]] && e.Kind != gateEntryDir && e.Kind != gateEntryFile {
				gateUnreadableScope(snap, p, e.Kind.String()+" in place of the stream's done/ folder")
			}
			continue
		}
		m := gateBoardPathRe.FindStringSubmatch(p)
		if m == nil || reservedRegisterNames[m[2]] {
			continue
		}
		k := m[1] + "/" + m[2]
		g := groups[k]
		if g == nil {
			g = &group{tree: m[1], name: m[2]}
			groups[k] = g
		}
		if m[3] == "README.md" {
			g.readme = p
		} else {
			g.briefs = append(g.briefs, p)
		}
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	// Active streams first, so an active stream wins a name it shares with an
	// archived one (a stream mid-move can briefly exist under both trees).
	sort.Slice(keys, func(i, j int) bool {
		ai, aj := groups[keys[i]].tree == "streams", groups[keys[j]].tree == "streams"
		if ai != aj {
			return ai
		}
		return keys[i] < keys[j]
	})

	// Every row and brief file is folded into its board id's record; nothing
	// read is skipped. The first record (active tree, stream root, path order)
	// supplies Board, Path and Readme; a second record for the same board id
	// marks it ambiguous and the gate-relevant fields become a conservative
	// merge (mergeGateRecord).
	type tally struct {
		rows, files int
		trees       map[string]bool
		rowNames    []string
		fileNames   []string
	}
	tallies := map[string]*tally{}
	note := func(key, tree string) *tally {
		t := tallies[key]
		if t == nil {
			t = &tally{trees: map[string]bool{}}
			tallies[key] = t
		}
		t.trees[tree] = true
		return t
	}
	for _, k := range keys {
		g := groups[k]
		// A brief at the stream root is read before a same-numbered one in done/.
		sort.Slice(g.briefs, func(i, j int) bool {
			di, dj := strings.Contains(g.briefs[i], "/done/"), strings.Contains(g.briefs[j], "/done/")
			if di != dj {
				return !di
			}
			return g.briefs[i] < g.briefs[j]
		})
		for _, p := range g.briefs {
			m := briefNameRe.FindStringSubmatch(path.Base(p))
			if m == nil {
				continue
			}
			key := g.name + "/" + normBriefNum(m[1])
			rec := &gateBrief{Key: key, Board: g.name + "/" + m[1], Path: p, Readme: g.readme}
			if k := kinds[p]; k != gateEntryFile {
				rec.Unreadable = fmt.Sprintf("%s is %s", p, k)
			} else if raw, err := read(p); err != nil {
				rec.Unreadable = fmt.Sprintf("%s could not be read (%v)", p, err)
			} else {
				readGateFrontmatter(rec, raw)
			}
			var fileRec string
			if rec.Unreadable != "" {
				// What the board would read there cannot be told: the record
				// reads conservatively (gate: human, no ruling — set below for
				// every board id holding an unreadable record).
				fileRec = "unreadable " + rec.Unreadable
			} else {
				fileRec = fmt.Sprintf("file %s gate=%s id=%s brief=%s decision-issue=%d ruling=%s fmerr=%t",
					p, rec.Gate, rec.PermID, rec.BriefID, rec.DecisionIssue, rec.Ruling, rec.FMErr)
			}
			t := note(key, g.tree)
			t.files++
			t.fileNames = append(t.fileNames, p)
			gb := snap[key]
			if gb == nil {
				gb = rec
				snap[key] = gb
			} else {
				mergeGateRecord(gb, rec)
			}
			gb.addSpelling(g.name + "/" + m[1])
			if rec.PermID != "" {
				gb.addPermID(rec.PermID)
			}
			if rec.Unreadable != "" {
				gb.addUnreadable(rec.Unreadable)
			}
			gb.Records = append(gb.Records, fileRec)
		}
		if g.readme == "" {
			continue
		}
		// An index the hold cannot read leaves every row of the stream unknown:
		// the whole stream is an unreadable scope, not a stream with no rows.
		if k := kinds[g.readme]; k != gateEntryFile {
			gateUnreadableScope(snap, g.readme, k.String()+", not a plain file")
			continue
		}
		raw, err := read(g.readme)
		if err != nil {
			gateUnreadableScope(snap, g.readme, fmt.Sprintf("could not be read (%v)", err))
			continue
		}
		body := string(raw)
		if _, b, ferr := splitFrontmatter(body); ferr == nil {
			body = b
		}
		rows, _ := parseBriefTable(body)
		for _, r := range rows {
			key := g.name + "/" + normBriefNum(r.Num)
			t := note(key, g.tree)
			t.rows++
			t.rowNames = append(t.rowNames, g.name+"/"+r.Num)
			gb := snap[key]
			if gb == nil {
				gb = &gateBrief{Key: key, Board: g.name + "/" + r.Num, Readme: g.readme}
				snap[key] = gb
			}
			switch {
			case t.rows == 1 && gb.Status == "":
				gb.Status = r.Status
				gb.Board = g.name + "/" + r.Num
				if gb.Readme == "" {
					gb.Readme = g.readme
				}
			case gateStatusRank(r.Status) < gateStatusRank(gb.Status):
				// A second row for the board id: judge from the LOWEST status, so
				// no row can make the brief read as already landed.
				gb.Status = r.Status
			}
			gb.addSpelling(g.name + "/" + r.Num)
			gb.Records = append(gb.Records, fmt.Sprintf("row %s %s status=%s", g.readme, r.Num, r.Status))
		}
	}
	for key, t := range tallies {
		var why []string
		if t.rows > 1 {
			why = append(why, fmt.Sprintf("%d README rows (%s)", t.rows, strings.Join(t.rowNames, ", ")))
		}
		if t.files > 1 {
			why = append(why, fmt.Sprintf("%d brief files (%s)", t.files, strings.Join(t.fileNames, ", ")))
		}
		if len(t.trees) > 1 {
			why = append(why, "records under both docs/streams and docs/archive")
		}
		if len(why) > 0 {
			gb := snap[key]
			gb.Ambiguous = strings.Join(why, " and ")
			// No single record's ruling can stand for an ambiguous board id.
			gb.DecisionIssue, gb.Ruling = 0, ""
		}
	}
	for _, gb := range snap {
		if gb.Unreadable != "" {
			// The conservative reading of a record the hold cannot read: it
			// may say gate: human, and no ruling can be read off it.
			gb.Gate, gb.DecisionIssue, gb.Ruling = gateHumanValue, 0, ""
		}
	}
	return snap
}

// gateUnreadableScope records a board path whose records the hold cannot
// enumerate the way the board does, as one SCOPE record: gate: human, no
// ruling, never matched by a board id.
func gateUnreadableScope(snap gateSnapshot, p, why string) {
	key := gateScopeKeyPrefix + p
	if snap[key] != nil {
		return
	}
	u := p + " is " + why
	snap[key] = &gateBrief{Key: key, Board: p, Path: p, Gate: gateHumanValue, Unreadable: u,
		Records: []string{"unreadable " + u}}
}

func (gb *gateBrief) addUnreadable(why string) {
	if gb.Unreadable == "" {
		gb.Unreadable = why
	} else if !strings.Contains(gb.Unreadable, why) {
		gb.Unreadable += "; " + why
	}
}

// mergeGateRecord folds a second brief file for the same board id into gb,
// fail-closed: the gate is human if any file says so, an unparseable file
// counts, and the permanent id: of every file is kept for matching.
func mergeGateRecord(gb, rec *gateBrief) {
	if rec.Gate == gateHumanValue {
		gb.Gate = gateHumanValue
	}
	if rec.FMErr {
		gb.FMErr = true
	}
	if gb.Path == "" {
		gb.Path = rec.Path
		gb.Gate, gb.PermID, gb.BriefID = rec.Gate, rec.PermID, rec.BriefID
	}
	if gb.Readme == "" {
		gb.Readme = rec.Readme
	}
}

func (gb *gateBrief) addPermID(id string) {
	for _, have := range gb.PermIDs {
		if have == id {
			return
		}
	}
	gb.PermIDs = append(gb.PermIDs, id)
}

// hasRow reports whether a README row stands for the board id. The README
// table is the board: a brief file with no row behind it is off the board.
func (gb *gateBrief) hasRow() bool {
	for _, r := range gb.Records {
		if strings.HasPrefix(r, "row ") {
			return true
		}
	}
	return false
}

// gateKeeps reports whether brief h after the change keeps base brief b on the
// board: it must be the SAME brief (gateSameBriefPairs), and when a README row
// stood behind b at the base, a row must still stand behind h. A brief file
// left behind — at the stream root, under done/ or in the archive tree — is not
// a brief on the board.
func gateKeeps(same map[gatePair]bool, b, h *gateBrief) bool {
	return same[gatePair{b, h}] && !(b.hasRow() && !h.hasRow())
}

func (gb *gateBrief) addSpelling(s string) {
	for _, have := range gb.Spellings {
		if have == s {
			return
		}
	}
	gb.Spellings = append(gb.Spellings, s)
}

// readGateFrontmatter reads the keys the hold needs straight from the YAML, for
// any brief schema: a check that depended on the brief validating would go
// quiet exactly when a change breaks the frontmatter.
func readGateFrontmatter(gb *gateBrief, raw []byte) {
	fm, _, err := splitFrontmatter(string(raw))
	if err != nil {
		return // no frontmatter: a legacy brief with no gate
	}
	var data map[string]any
	if err := yaml.Unmarshal([]byte(fm), &data); err != nil {
		gb.FMErr = true
		return
	}
	str := func(k string) string {
		if s, ok := data[k].(string); ok {
			return strings.TrimSpace(s)
		}
		return ""
	}
	gb.Gate = strings.ToLower(str("gate"))
	gb.PermID = str("id")
	gb.BriefID = str("brief")
	gb.Ruling = str("ruling")
	switch n := data["decision-issue"].(type) {
	case int:
		gb.DecisionIssue = n
	case int64:
		gb.DecisionIssue = int(n)
	case string:
		if v, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(n), "#")); err == nil {
			gb.DecisionIssue = v
		}
	}
	if gb.DecisionIssue < 0 {
		gb.DecisionIssue = 0
	}
}

// gateSnapshotOnDisk reads the board in the working tree at root.
func gateSnapshotOnDisk(root string) gateSnapshot {
	return gateSnapshotOnDiskWith(root, "", nil)
}

// gateSnapshotOnDiskWith is gateSnapshotOnDisk with one repo-relative file's
// content replaced — the board a write WOULD leave, before it is written.
//
// The working tree is listed by lstat, never by following a link: each entry
// is classified (diskEntryKind) and buildGateSnapshot applies the same rule to
// it as to the base's git tree.
func gateSnapshotOnDiskWith(root, overrideRel string, override []byte) gateSnapshot {
	var entries []gateEntry
	add := func(rel string) gateEntryKind {
		k := diskEntryKind(filepath.Join(root, filepath.FromSlash(rel)))
		entries = append(entries, gateEntry{Path: rel, Kind: k})
		return k
	}
	list := func(rel string) []string {
		ents, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil
		}
		names := make([]string, 0, len(ents))
		for _, e := range ents {
			names = append(names, e.Name())
		}
		return names
	}
	if _, err := os.Lstat(filepath.Join(root, "docs")); err == nil && add("docs") == gateEntryDir {
		for _, tree := range []string{"streams", "archive"} {
			treeRel := "docs/" + tree
			if _, err := os.Lstat(filepath.Join(root, "docs", tree)); err != nil || add(treeRel) != gateEntryDir {
				continue
			}
			for _, s := range list(treeRel) {
				sub := treeRel + "/" + s
				if add(sub) != gateEntryDir {
					continue
				}
				for _, f := range list(sub) {
					fr := sub + "/" + f
					k := add(fr)
					if f == archiveDirName && k == gateEntryDir {
						for _, d := range list(fr) {
							add(fr + "/" + d)
						}
					}
				}
			}
		}
	}
	overrideRel = filepath.ToSlash(overrideRel)
	return buildGateSnapshot(entries, func(rel string) ([]byte, error) {
		if overrideRel != "" && rel == overrideRel {
			return override, nil
		}
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	})
}

// diskEntryKind classifies a working-tree path without following a link. A
// directory holding .git is a nested repository: the base's git tree records
// it as a submodule and cannot read into it.
func diskEntryKind(p string) gateEntryKind {
	fi, err := os.Lstat(p)
	if err != nil {
		return gateEntryOther
	}
	switch m := fi.Mode(); {
	case m&fs.ModeSymlink != 0:
		return gateEntrySymlink
	case m.IsDir():
		if _, err := os.Lstat(filepath.Join(p, ".git")); err == nil {
			return gateEntryNested
		}
		return gateEntryDir
	case m.IsRegular():
		return gateEntryFile
	}
	return gateEntryOther
}

// gitTreeEntryKind classifies a git tree entry by its mode.
func gitTreeEntryKind(mode string) gateEntryKind {
	switch mode {
	case "100644", "100755":
		return gateEntryFile
	case "040000":
		return gateEntryDir
	case "120000":
		return gateEntrySymlink
	case "160000":
		return gateEntryNested
	}
	return gateEntryOther
}

// gateSnapshotAtRev reads the board as committed at rev. The tree is listed
// with each entry's mode (and the directories on the way, -t), and docs itself
// is classified from its parent, so a link anywhere on the board's path reads
// as one here exactly as it does in the working tree.
func gateSnapshotAtRev(root, rev string) (gateSnapshot, error) {
	var entries []gateEntry
	parse := func(out []byte) {
		for _, line := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
			meta, p, ok := strings.Cut(line, "\t")
			if !ok {
				continue
			}
			mode, _, _ := strings.Cut(meta, " ")
			entries = append(entries, gateEntry{Path: p, Kind: gitTreeEntryKind(mode)})
		}
	}
	top, err := exec.Command("git", "-C", root, "ls-tree", "-z", rev, "--", "docs").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-tree %s: %w", rev, err)
	}
	// docs itself, classified from its parent: only a plain directory there is
	// descended into; a link at docs is one unreadable entry, as on disk.
	docsKind, docsSeen := gateEntryOther, false
	for _, line := range strings.Split(strings.TrimRight(string(top), "\x00"), "\x00") {
		if meta, p, ok := strings.Cut(line, "\t"); ok && p == "docs" {
			mode, _, _ := strings.Cut(meta, " ")
			docsKind, docsSeen = gitTreeEntryKind(mode), true
		}
	}
	switch {
	case !docsSeen:
		// No docs at this revision: no board.
	case docsKind != gateEntryDir:
		entries = append(entries, gateEntry{Path: "docs", Kind: docsKind})
	default:
		out, err := exec.Command("git", "-C", root, "ls-tree", "-r", "-t", "-z", rev, "--", "docs/streams", "docs/archive").Output()
		if err != nil {
			return nil, fmt.Errorf("git ls-tree %s: %w", rev, err)
		}
		parse(out)
	}
	return buildGateSnapshot(entries, func(p string) ([]byte, error) { return gitShowObject(root, rev, p) }), nil
}

// gateFault is one change the hold judges: a brief moved, relabelled or dropped
// while in scope.
type gateFault struct {
	Move, Relabel, Drop bool
	// Ambiguous: the change leaves a board id the hold covers resolving to
	// more than one record (Head.Ambiguous says which), so no single
	// transition can be judged and no recorded ruling can lift it.
	Ambiguous bool
	// Unreadable: a record the change leaves (Head), or a base record no
	// brief after the change stands for (Base[0]), cannot be read the way the
	// board reads it (gateBrief.Unreadable says why), so nothing about it can
	// be judged and no recorded ruling can lift it.
	Unreadable bool
	// RowGone (with Drop): the dropped brief's board id still has a brief file
	// after the change, but no README row stands for it.
	RowGone    bool
	Head       *gateBrief   // the brief after the change (nil for a drop)
	Base       []*gateBrief // the base briefs it matches (for a drop, the dropped one)
	RulingBase []*gateBrief // the matched base briefs that are the SAME brief (isGateRenumber); nil for a drop
	From       string       // the lowest matched base status ("" = not on the board)
}

// primary is the record a refusal names: the brief after the change, or the
// dropped one.
func (f gateFault) primary() *gateBrief {
	if f.Head != nil {
		return f.Head
	}
	return f.Base[0]
}

// candidates are the records whose recorded ruling may lift the hold: the brief
// after the change first, then only those matched base briefs that are the same
// brief (same board id, or a renumber — isGateRenumber), so a ruling recorded on
// any of them is that brief's ruling. A base brief matched only by a permanent
// id: that moved between two existing board ids is never a candidate. For a
// drop, the dropped brief's own record.
func (f gateFault) candidates() []*gateBrief {
	if f.Head == nil {
		return f.Base
	}
	return append([]*gateBrief{f.Head}, f.RulingBase...)
}

// boardNames is every board-id spelling a ruling comment may name: the brief's
// id after the change and before it (a renumber in the same change keeps the
// ruling that named the old id).
func (f gateFault) boardNames() []string {
	var out []string
	seen := map[string]bool{}
	for _, c := range f.candidates() {
		for _, s := range c.Spellings {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

// briefIDs is every frontmatter brief: value among the candidates — the id the
// decision issue's marker carries in the colon form.
func (f gateFault) briefIDs() []string {
	var out []string
	seen := map[string]bool{}
	for _, c := range f.candidates() {
		if c.BriefID != "" && !seen[c.BriefID] {
			seen[c.BriefID] = true
			out = append(out, c.BriefID)
		}
	}
	return out
}

// what describes the change in words a reader can act on.
func (f gateFault) what() string {
	var parts []string
	if f.Unreadable {
		if f.Head != nil {
			parts = append(parts, fmt.Sprintf("leaves a board record the hold cannot read the way the board reads it (%s), so its gate, status and ruling cannot be judged — it is refused, never taken as carrying no gate", f.Head.Unreadable))
		} else {
			parts = append(parts, fmt.Sprintf("is judged against a base board record the hold could not read the way the board reads it (%s), and no brief after the change stands for it, so what the change does to it cannot be judged", f.Base[0].Unreadable))
		}
		return strings.Join(parts, "; and ")
	}
	if f.Ambiguous {
		parts = append(parts, fmt.Sprintf("leaves its board id resolving to more than one record (%s), so no single status move can be judged", f.Head.Ambiguous))
	}
	if f.Move {
		from := "it was not on the board at the base"
		if f.From != "" {
			from = fmt.Sprintf("it was %s at the base", f.From)
		}
		parts = append(parts, fmt.Sprintf("moves it to %s (%s)", f.Head.Status, from))
	}
	if f.Relabel {
		switch {
		case f.Head.FMErr:
			parts = append(parts, "leaves its frontmatter unparseable, so its gate: human can no longer be read")
		case f.Head.Gate == "" && f.Head.Path == "":
			parts = append(parts, "removes its brief file, leaving no gate: human behind the row")
		case f.Head.Gate == "":
			parts = append(parts, "deletes its gate: key (it was human)")
		default:
			parts = append(parts, fmt.Sprintf("relabels its gate from human to %q", f.Head.Gate))
		}
	}
	if f.Drop {
		b := f.Base[0]
		why := "and it has no permanent id:"
		if b.PermID != "" {
			why = "nor its permanent id: as the same brief (only a renumber onto a NEW board id, with no other claimant, keeps a brief — a permanent id: parked on a brief already on the board does not)"
		}
		if f.RowGone {
			parts = append(parts, fmt.Sprintf("drops it — its README row for board id %s is gone, and a brief file left behind (at the stream root, under done/ or in the archive tree) is not a brief on the board", b.Board))
		} else {
			parts = append(parts, fmt.Sprintf("drops it — no brief after the change carries its board id %s, %s", b.Board, why))
		}
	}
	return strings.Join(parts, "; and ")
}

// judgeDecisionGate compares two board snapshots and returns every change the
// hold covers, ruled or not. Deterministic order: briefs after the change by
// key, then dropped base briefs by key.
//
// Identity is one-to-one or the change is refused. A base brief and a brief
// after the change are the SAME brief (sameGateBrief) only by the same board
// id, or by a renumber that pairs exactly one base brief with exactly one new
// board id; only the same brief lends a prior status or a ruling, and only the
// same brief keeps a base brief on the board (a permanent id: parked on another
// brief keeps that brief in SCOPE, never the base brief PRESENT) — and only
// while a README row still stands behind it, if one stood at the base
// (gateKeeps): a brief file without its row is off the board, a drop. A record the
// change leaves that the hold cannot read the way the board reads it
// (gateBrief.Unreadable) is refused outright, changed or not: its content can
// change behind an unchanged entry. At the base such a record reads
// conservatively (gate: human, no ruling), and an unreadable base SCOPE record
// that nothing after the change stands for is refused too. A board id
// the hold covers that resolves to more than one record after the change, and
// whose records differ from the base's, is refused outright: which record is
// the brief cannot be told, so no transition and no ruling can be judged.
func judgeDecisionGate(base, head gateSnapshot) []gateFault {
	byPerm := map[string][]*gateBrief{}
	for _, k := range sortedGateKeys(base) {
		b := base[k]
		for _, id := range gatePermIDs(b) {
			byPerm[id] = append(byPerm[id], b)
		}
	}
	same := gateSameBriefPairs(base, head, byPerm)
	matched := map[*gateBrief]bool{}
	var faults []gateFault
	for _, k := range sortedGateKeys(head) {
		h := head[k]
		var ms []*gateBrief
		add := func(b *gateBrief) {
			for _, have := range ms {
				if have == b {
					return
				}
			}
			ms = append(ms, b)
		}
		if b := base[h.Key]; b != nil {
			add(b)
		}
		for _, id := range gatePermIDs(h) {
			for _, b := range byPerm[id] {
				add(b)
			}
		}
		baseHuman := false
		for _, b := range ms {
			if gateKeeps(same, b, h) {
				matched[b] = true // only the same brief, still on the board, keeps a base brief present
			}
			if b.Gate == gateHumanValue {
				baseHuman = true
			}
		}
		if h.Unreadable != "" {
			if b := base[h.Key]; b != nil {
				matched[b] = true // reported here, not again as a drop
			}
			faults = append(faults, gateFault{Unreadable: true, Head: h, Base: ms})
			continue
		}
		if h.Gate != gateHumanValue && !baseHuman {
			continue // not in scope on either side
		}
		if h.Ambiguous != "" {
			if b := base[h.Key]; b != nil && b.fingerprint() == h.fingerprint() {
				// The same colliding records stood at the base: nothing about
				// this board id changed, so a pin bump turns nothing red. Judged
				// below like any other brief, from the conservative merge.
			} else {
				if b := base[h.Key]; b != nil {
					matched[b] = true // reported here, not again as a drop
				}
				faults = append(faults, gateFault{Ambiguous: true, Head: h, Base: ms})
				continue
			}
		}
		f := gateFault{Head: h, Base: ms}
		for _, b := range ms {
			if same[gatePair{b, h}] {
				f.RulingBase = append(f.RulingBase, b)
			}
		}
		// The prior status comes only from the same brief (RulingBase): its own
		// base counterpart (the same board id) or its one renumber. A
		// permanent-id donor that is neither stays in ms for SCOPE, but a row
		// that takes a donor's id: is a NEW row and a move: it cannot inherit
		// the donor's landed status.
		if hr := gateStatusRank(h.Status); hr >= 1 {
			if len(f.RulingBase) == 0 {
				f.Move = true
			} else {
				low := f.RulingBase[0]
				for _, b := range f.RulingBase[1:] {
					if gateStatusRank(b.Status) < gateStatusRank(low.Status) {
						low = b
					}
				}
				if gateStatusRank(low.Status) < hr {
					f.Move = true
					f.From = low.Status
					if f.From == "" {
						f.From = "not on the board"
					}
				}
			}
		}
		if baseHuman && h.Gate != gateHumanValue {
			f.Relabel = true
		}
		if f.Move || f.Relabel {
			faults = append(faults, f)
		}
	}
	for _, k := range sortedGateKeys(base) {
		b := base[k]
		if b.Gate == gateHumanValue && !matched[b] {
			if strings.HasPrefix(b.Key, gateScopeKeyPrefix) {
				faults = append(faults, gateFault{Unreadable: true, Base: []*gateBrief{b}})
				continue
			}
			f := gateFault{Drop: true, Base: []*gateBrief{b}}
			if h := head[b.Key]; h != nil && b.hasRow() && !h.hasRow() {
				f.RowGone = true
			}
			faults = append(faults, f)
		}
	}
	return faults
}

// gatePair is one (base brief, brief after the change) pairing.
type gatePair struct{ b, h *gateBrief }

// gatePermIDs is every permanent id: a record carries (an ambiguous board id can
// carry several; a record built without the identity accounting carries its
// own PermID).
func gatePermIDs(gb *gateBrief) []string {
	if len(gb.PermIDs) > 0 {
		return gb.PermIDs
	}
	if gb.PermID != "" {
		return []string{gb.PermID}
	}
	return nil
}

// gateSameBriefPairs is every (base, after) pairing that is the SAME brief: the
// same board id, or a renumber (isGateRenumber) in which the base brief has
// exactly one renumber claimant after the change and the new brief exactly one
// renumber donor at the base. One base record stands behind at most one brief
// after the change, and one brief after it is the renumber of at most one base
// brief — a one-to-many or many-to-one "renumber" pairs nothing, so each side is
// judged on its own record (moves from "not on the board", drops of the base
// briefs).
func gateSameBriefPairs(base, head gateSnapshot, byPerm map[string][]*gateBrief) map[gatePair]bool {
	same := map[gatePair]bool{}
	claimants := map[*gateBrief][]*gateBrief{}
	donors := map[*gateBrief][]*gateBrief{}
	for _, k := range sortedGateKeys(head) {
		h := head[k]
		if b := base[h.Key]; b != nil {
			same[gatePair{b, h}] = true
		}
		seen := map[*gateBrief]bool{}
		for _, id := range gatePermIDs(h) {
			for _, b := range byPerm[id] {
				if seen[b] || b.Key == h.Key || !isGateRenumber(base, head, b, h) {
					continue
				}
				seen[b] = true
				claimants[b] = append(claimants[b], h)
				donors[h] = append(donors[h], b)
			}
		}
	}
	for h, bs := range donors {
		if len(bs) == 1 && len(claimants[bs[0]]) == 1 {
			same[gatePair{bs[0], h}] = true
		}
	}
	return same
}

// isGateRenumber reports whether base brief b and brief h after the change can
// be the same brief for the purpose of a recorded ruling: the same board id, or
// a renumber — b's board id is gone after the change and h's board id is new in
// it. Anything else — an id: moved or copied onto a brief whose own board id
// already existed, or taken from a brief still on the board — keeps h in scope
// (the match stands) but leaves h to be ruled on its own record. A renumber
// pairs only one-to-one (gateSameBriefPairs).
func isGateRenumber(base, head gateSnapshot, b, h *gateBrief) bool {
	if b.Key == h.Key {
		return true // the same board id: already matched by key
	}
	return head[b.Key] == nil && base[h.Key] == nil
}

func sortedGateKeys(s gateSnapshot) []string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// gateHumanValue is the gate value the hold covers.
const gateHumanValue = "human"

// gateRecordedRuling is the ruling layer one accepted for a fault.
type gateRecordedRuling struct {
	Rec  *gateBrief
	Link rulingLink
}

// offlineRuling is layer one: the first candidate whose frontmatter records a
// decision issue AND a well-formed ruling link to a comment on that issue. When
// none does, missing says what the PRIMARY record lacks.
func offlineRuling(f gateFault) (r gateRecordedRuling, ok bool, missing string) {
	if f.Unreadable {
		return r, false, "no recorded ruling can stand for a board record the hold cannot read the way the board reads it — make every stream index, brief file and done/ folder under docs/streams and docs/archive a plain file or directory in the tree (no symbolic link, submodule or nested repository), then make any status move in a change of its own"
	}
	if f.Ambiguous {
		return r, false, "no recorded ruling can stand for a board id that resolves to more than one record — give each brief its own board number (and each stream its own name across docs/streams and docs/archive), then make the status move in a change of its own"
	}
	for _, c := range f.candidates() {
		if c.Ambiguous != "" || c.Unreadable != "" {
			continue // an ambiguous or unreadable record's ruling is no single brief's ruling
		}
		if c.DecisionIssue == 0 {
			continue
		}
		link, lok := parseRulingURL(c.Ruling)
		if lok && link.Issue == c.DecisionIssue {
			return gateRecordedRuling{Rec: c, Link: link}, true, ""
		}
	}
	p := f.primary()
	switch {
	case p.Ambiguous != "":
		return r, false, fmt.Sprintf("its board id resolves to more than one record (%s), so no single recorded ruling can stand for it — give each brief its own board number first", p.Ambiguous)
	case p.DecisionIssue == 0:
		return r, false, "no decision issue is recorded (frontmatter decision-issue: is absent) — no decision issue at all is a refusal, not a pass; file the brief's decision issue, record its number in decision-issue:, and record the ruling once given"
	case strings.TrimSpace(p.Ruling) == "":
		return r, false, fmt.Sprintf("decision issue #%d has no recorded ruling (frontmatter ruling: is absent) — once a human rules on #%d, link the ruling comment in ruling:", p.DecisionIssue, p.DecisionIssue)
	}
	link, lok := parseRulingURL(p.Ruling)
	if !lok {
		return r, false, fmt.Sprintf("ruling: %q is not an issue-comment URL of the form %s", p.Ruling, rulingURLGrammar)
	}
	return r, false, fmt.Sprintf("ruling: links a comment on #%d, not on the recorded decision issue #%d", link.Issue, p.DecisionIssue)
}

// gateWhere is the file a refusal points at: the README whose row moved, or
// the brief file whose frontmatter changed or vanished.
func (f gateFault) where() string {
	p := f.primary()
	if f.Move && p.Readme != "" {
		return p.Readme
	}
	if p.Path != "" {
		return p.Path
	}
	return p.Readme
}

// layerOneRefusal renders a fault layer one could not see ruled.
func layerOneRefusal(f gateFault, missing string) string {
	return fmt.Sprintf("%s: decision-gate hold (%s): %s is gate: human and this change %s, but %s. %s",
		f.where(), decisionGateSpecRef, f.primary().Board, f.what(), missing, decisionGateLayerOneScope)
}

// decisionGateHoldProblems is layer one, run by every --lint: the working tree
// judged against the merge-base with origin/main. There is no disarm on main —
// with nothing changed against the base there is nothing to judge — so an
// uncommitted status write on main (a landing tool's staged change) is judged
// too.
//
// In a real checkout whose base cannot be resolved or read, layer one fails
// CLOSED while the board has anything to guard — a gate: human brief at
// implemented or later — the same rule the register field-gutting guard follows
// (guttedRegisterFieldsEntries): without the base, a committed move would be
// compared with itself and pass. Only a tree with no .git at all (a `git
// archive` export, which nothing merges into) is reported as a did-not-run
// NOTICE, and so is a real checkout whose board has nothing to guard. Residual:
// a relabel or drop leaves no gate: human brief behind to count, so with the
// base unreadable only layer two, which fails closed on its own base, sees it.
func decisionGateHoldProblems(root string) (problems, notices []string) {
	if hasNoGitDir(root) {
		return nil, []string{"decision-gate hold (" + decisionGateSpecRef + ") did not run: .git is absent, so there is no base to judge a status move against"}
	}
	base, resolved := registerLandedBase(root)
	if !resolved {
		if guarded := gateGuardedBriefs(gateSnapshotOnDisk(root)); len(guarded) > 0 {
			return []string{fmt.Sprintf("decision-gate hold (%s) refused (fail-closed): the exact ref %s could not be resolved to a merge-base with HEAD, so %s — gate: human at implemented or later — cannot be judged against the board it merges onto, and a committed move would compare with itself and pass. Fetch the base branch into %s (actions/checkout fetch-depth: 0, or `git fetch origin main`) and re-run; a clone with no origin remote cannot pass this check",
				decisionGateSpecRef, remoteMainRef, gateBriefList(guarded), remoteMainRef)}, nil
		}
		return nil, []string{"decision-gate hold (" + decisionGateSpecRef + ") did not run: origin/main could not be resolved, so a gate: human brief's status move cannot be judged against its base; no gate: human brief is at implemented or later, so there is nothing to refuse. If this is CI, fetch origin/main before the lint step"}
	}
	baseSnap, err := gateSnapshotAtRev(root, base)
	if err != nil {
		if guarded := gateGuardedBriefs(gateSnapshotOnDisk(root)); len(guarded) > 0 {
			return []string{fmt.Sprintf("decision-gate hold (%s) refused (fail-closed): the board at the base %s could not be read (%v), so %s — gate: human at implemented or later — cannot be judged against it",
				decisionGateSpecRef, base, err, gateBriefList(guarded))}, nil
		}
		return nil, []string{fmt.Sprintf("decision-gate hold (%s) could not check: the board at the base %s could not be read (%v); no gate: human brief is at implemented or later, so there is nothing to refuse", decisionGateSpecRef, base, err)}
	}
	for _, f := range judgeDecisionGate(baseSnap, gateSnapshotOnDisk(root)) {
		if _, ok, missing := offlineRuling(f); !ok {
			problems = append(problems, layerOneRefusal(f, missing))
		}
	}
	return problems, nil
}

// gateGuardedBriefs is what layer one guards when it cannot read the base: every
// brief on the board that is gate: human at implemented or later, by key.
func gateGuardedBriefs(s gateSnapshot) []*gateBrief {
	var out []*gateBrief
	for _, k := range sortedGateKeys(s) {
		if b := s[k]; b.Gate == gateHumanValue && gateStatusRank(b.Status) >= 1 {
			out = append(out, b)
		}
	}
	return out
}

// gateBriefList names up to five briefs and counts the rest.
func gateBriefList(bs []*gateBrief) string {
	const show = 5
	var names []string
	for i, b := range bs {
		if i == show {
			break
		}
		names = append(names, b.Board)
	}
	s := strings.Join(names, ", ")
	if len(bs) > show {
		s += fmt.Sprintf(" and %d more", len(bs)-show)
	}
	if len(bs) == 1 {
		return "brief " + s
	}
	return fmt.Sprintf("%d briefs (%s)", len(bs), s)
}

// ---- layer two ---------------------------------------------------------------

// decisionGateResult is one judged fault after layer two.
type decisionGateResult struct {
	Board   string
	Refused bool
	Detail  string
}

func (r decisionGateResult) line() string {
	if r.Refused {
		return fmt.Sprintf("%s REFUSED — %s", r.Board, r.Detail)
	}
	return fmt.Sprintf("%s RULED — %s", r.Board, r.Detail)
}

func decisionGateRefused(rs []decisionGateResult) bool {
	for _, r := range rs {
		if r.Refused {
			return true
		}
	}
	return false
}

// Seams: the forge client and the human-login map layer two reads.
var (
	decisionGateClientFn = rulingForgeClient
	decisionGateLoginsFn = func() map[string]string { return scanEffectiveConfig().HumanLogins }
)

// briefIssueRulesOn is condition 6 for a brief: the decision issue's body
// carries a decision-gate marker for THIS brief — its board id (any spelling),
// its frontmatter brief: value verbatim, or a colon-form id whose stream and
// number match the board id and whose repository part, when it has one, names
// repo. The verbatim brief: value is accepted whatever its repository part
// says, because it is the id the brief itself declares (a house may spell its
// own repository with a short alias there).
func briefIssueRulesOn(body, repo string, boardIDs, fmBriefs []string) bool {
	repoName := repo
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		repoName = repo[i+1:]
	}
	for _, m := range decisionGateMarkerRe.FindAllStringSubmatch(body, -1) {
		id := m[1]
		for _, b := range fmBriefs {
			if b != "" && id == b {
				return true
			}
		}
		for _, b := range boardIDs {
			if id == b {
				return true
			}
		}
		stream, num, ok := briefStreamNum(id)
		if !ok {
			continue
		}
		if parts := strings.Split(id, ":"); len(parts) >= 3 && !strings.EqualFold(parts[len(parts)-3], repoName) {
			continue
		}
		for _, b := range boardIDs {
			bs, bn, bok := briefStreamNum(b)
			if bok && bs == stream && normBriefNum(bn) == normBriefNum(num) {
				return true
			}
		}
	}
	return false
}

// resolveDecisionGateFaults is layer two over judged faults. The forge is
// contacted only for a fault layer one accepts; a fault it refuses is refused
// here with the same reason.
func resolveDecisionGateFaults(c *ghClient, repo string, faults []gateFault, logins map[string]string) []decisionGateResult {
	var out []decisionGateResult
	for _, f := range faults {
		board := f.primary().Board
		r, ok, missing := offlineRuling(f)
		if !ok {
			out = append(out, decisionGateResult{Board: board, Refused: true,
				Detail: fmt.Sprintf("this change %s, but %s (%s)", f.what(), missing, decisionGateSpecRef)})
			continue
		}
		if repo == "" {
			out = append(out, decisionGateResult{Board: board, Refused: true,
				Detail: fmt.Sprintf("this change %s; the repository could not be determined from the origin remote, so the ruling link %s cannot be checked against it (%s)", f.what(), r.Rec.Ruling, decisionGateSpecRef)})
			continue
		}
		names := f.boardNames()
		binding := rulingIssueBinding{
			what: "brief " + strings.Join(names, " / "),
			rules: func(iss rulingIssue, _ rulingLink) bool {
				return briefIssueRulesOn(iss.Body, repo, names, f.briefIDs())
			},
		}
		var o rulingOutcome
		for _, id := range names {
			fmt.Fprintf(os.Stderr, "statusgen: decision-gate: resolving ruling link %s for %s\n", r.Rec.Ruling, id)
			o = resolveRulingOn(c, repo, decisionRecordStamp{File: r.Rec.Path, ID: id, Ruling: r.Rec.Ruling}, logins, binding)
			if o.Reason != rulingRecordNotNamed {
				break // only a comment that names another spelling is worth a second read
			}
		}
		if o.Present && o.Reason == rulingOK {
			out = append(out, decisionGateResult{Board: board, Detail: fmt.Sprintf("ruled by %s (%s)", o.Author, o.Detail)})
			continue
		}
		out = append(out, decisionGateResult{Board: board, Refused: true,
			Detail: fmt.Sprintf("this change %s; the recorded ruling %s does not stand as a human ruling on decision issue #%d: %s — %s (%s)",
				f.what(), r.Rec.Ruling, r.Rec.DecisionIssue, o.Reason, o.Detail, decisionGateSpecRef)})
	}
	return out
}

// decisionGateAgainst runs both layers for the working tree at root against the
// board committed at baseRev.
func decisionGateAgainst(root, repo, baseRev string) []decisionGateResult {
	baseSnap, err := gateSnapshotAtRev(root, baseRev)
	if err != nil {
		return []decisionGateResult{{Board: "(board)", Refused: true,
			Detail: fmt.Sprintf("the board at the base %s could not be read (%v), so no gate: human brief's move can be judged — fail-closed (%s)", baseRev, err, decisionGateSpecRef)}}
	}
	return decisionGateJudged(repo, judgeDecisionGate(baseSnap, gateSnapshotOnDisk(root)))
}

// decisionGateJudged resolves judged faults, building the forge client and
// reading the login map only when a fault needs them.
func decisionGateJudged(repo string, faults []gateFault) []decisionGateResult {
	if len(faults) == 0 {
		return nil
	}
	var c *ghClient
	var logins map[string]string
	for _, f := range faults {
		if _, ok, _ := offlineRuling(f); ok {
			c = decisionGateClientFn()
			logins = decisionGateLoginsFn()
			break
		}
	}
	return resolveDecisionGateFaults(c, repo, faults, logins)
}

// decisionGateTouchesBoard reports whether a forge file listing names a board
// file (either side of a rename).
func decisionGateTouchesBoard(files []ghPRFile) bool {
	for _, f := range files {
		for _, p := range []string{f.Filename, f.PreviousFilename} {
			if strings.HasPrefix(p, "docs/streams/") || strings.HasPrefix(p, "docs/archive/") {
				return true
			}
		}
	}
	return false
}

// decisionGatePRLane runs layer two for one pull request: root is the checkout
// (the PR head or merge commit), files the forge's file listing, mergeBase the
// PR merge-base ("" = unresolvable), changedFiles the forge's changed-file count
// (read only when the merge-base is unresolvable and the listing names no board
// file). An unresolvable base fails CLOSED when the PR touches the board, or
// when a listing that names no board file cannot be shown complete.
func decisionGatePRLane(root, repo string, files []ghPRFile, mergeBase string, changedFiles func() (int, error)) []decisionGateResult {
	refuse := func(why string) []decisionGateResult {
		return []decisionGateResult{{Board: "(board)", Refused: true, Detail: why + " — fail-closed (" + decisionGateSpecRef + ")"}}
	}
	if mergeBase == "" {
		if decisionGateTouchesBoard(files) {
			return refuse("the PR merge-base could not be resolved, so the status moves this PR makes to gate: human briefs cannot be judged; the exact ref refs/remotes/origin/<base> must exist (fetch the base branch, fetch-depth: 0)")
		}
		n, err := changedFiles()
		if err != nil {
			return refuse(fmt.Sprintf("the PR merge-base could not be resolved and the forge's changed-file count could not be read (%v), so a listing that names no board file cannot be shown complete", err))
		}
		if n != len(files) {
			return refuse(fmt.Sprintf("the PR merge-base could not be resolved and the forge listed %d of the PR's %d changed files, so a truncated listing cannot show the PR leaves the board untouched", len(files), n))
		}
		return nil
	}
	top, err := repoTopLevel(root)
	if err != nil {
		return refuse(fmt.Sprintf("the repository top level could not be resolved (%v)", err))
	}
	if len(files) > 0 {
		same, err := treeMatchesBase(top, mergeBase)
		if err != nil {
			return refuse(fmt.Sprintf("the working tree could not be compared with the PR merge-base (%v)", err))
		}
		if same {
			return refuse(fmt.Sprintf("the working tree is identical to the resolved merge-base %s while the forge lists %d changed file(s), so that base cannot be the PR's base", mergeBase, len(files)))
		}
	}
	return decisionGateAgainst(top, repo, mergeBase)
}

// repoFromOriginAtFn resolves "owner/repo" from root's origin remote — a seam.
var repoFromOriginAtFn = repoFromOriginAt

// printDecisionGateReport prints the lane's section. It always prints, so a
// clean run and a run that did not happen read differently.
func printDecisionGateReport(rs []decisionGateResult) {
	fmt.Println("# decision-gate hold (" + decisionGateSpecRef + ")")
	fmt.Println("# Scope: a gate: human brief this change moves to implemented, verified or done,")
	fmt.Println("# relabels away from human, or drops needs a recorded ruling: link to an unedited")
	fmt.Println("# comment, on its decision issue, by a login mapped to a human (never a bot),")
	fmt.Println("# whose own text names the brief by its board id. It confirms THAT a human")
	fmt.Println("# decided, not what they decided. It is never applied at a PR's ready-flip.")
	fmt.Println()
	if len(rs) == 0 {
		fmt.Println("no gate: human brief moved, relabelled or dropped — clean")
		return
	}
	for _, r := range rs {
		fmt.Println(r.line())
	}
}

// runDecisionGate is `statusgen --decision-gate`: both layers for one change.
// Exactly one of prsArg (pull requests, judged against each PR's merge-base) or
// baseRef (a local revision, e.g. HEAD for an uncommitted landing) is given.
func runDecisionGate(root, prsArg, baseRef string) int {
	if (strings.TrimSpace(prsArg) == "") == (strings.TrimSpace(baseRef) == "") {
		fmt.Fprintln(os.Stderr, "statusgen: --decision-gate needs exactly one of --pr <N[,N...]> or --decision-gate-base <rev>")
		return 2
	}
	defer beginGitReadSession()()
	repo := repoFromOriginAtFn(root)
	var rs []decisionGateResult
	if baseRef != "" {
		out, err := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", strings.TrimSpace(baseRef)+"^{commit}").Output()
		base := strings.TrimSpace(string(out))
		if err != nil || base == "" {
			rs = []decisionGateResult{{Board: "(board)", Refused: true,
				Detail: fmt.Sprintf("the base %q does not resolve to a commit, so no status move can be judged — fail-closed (%s)", baseRef, decisionGateSpecRef)}}
		} else {
			rs = decisionGateAgainst(root, repo, base)
		}
	} else {
		if repo == "" {
			fmt.Fprintln(os.Stderr, "statusgen: --decision-gate --pr: cannot determine the GitHub repository from the origin remote")
			return 1
		}
		for _, prStr := range strings.Split(prsArg, ",") {
			prStr = strings.TrimSpace(prStr)
			if prStr == "" {
				continue
			}
			pr, err := strconv.Atoi(prStr)
			if err != nil || pr <= 0 {
				fmt.Fprintf(os.Stderr, "statusgen: invalid PR number %q\n", prStr)
				return 2
			}
			files, err := corroborateFilesFn(repo, pr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "statusgen: PR #%d: %v\n", pr, err)
				return 1
			}
			countFn := func() (int, error) { return corroborateChangedFilesFn(repo, pr) }
			rs = append(rs, decisionGatePRLane(root, repo, files, corroborateMergeBaseFn(root, repo, pr), countFn)...)
		}
	}
	printDecisionGateReport(rs)
	if decisionGateRefused(rs) {
		return 1
	}
	return 0
}

// closeVerifyDecisionGateFn is the hold on the close to done — a seam so a
// test can drive it against a fake forge.
var closeVerifyDecisionGateFn = closeVerifyDecisionGate

// closeVerifyDecisionGate runs both layers on the close to done before it is
// written: the base is the tree as it stands, the change is the README the close
// would write. The close lands straight on the default branch with no pull
// request, so no PR check would otherwise see it.
func closeVerifyDecisionGate(root, readme string, updated []byte) error {
	rel := readme
	if filepath.IsAbs(readme) {
		r, err := filepath.Rel(root, readme)
		if err != nil {
			return fmt.Errorf("refusing: decision-gate hold (%s): %s is not under %s (%v)", decisionGateSpecRef, readme, root, err)
		}
		rel = r
	}
	faults := judgeDecisionGate(gateSnapshotOnDisk(root), gateSnapshotOnDiskWith(root, rel, updated))
	rs := decisionGateJudged(repoFromOriginAtFn(root), faults)
	var lines []string
	for _, r := range rs {
		if r.Refused {
			lines = append(lines, r.line())
		}
	}
	if len(lines) > 0 {
		return errors.New("refusing: decision-gate hold — " + strings.Join(lines, "; "))
	}
	return nil
}
