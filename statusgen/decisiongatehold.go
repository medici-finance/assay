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
//   - no brief after the change matches a `gate: human` base brief by either its
//     board id (<stream>/<NN>) or its permanent frontmatter `id:` (a DROP).
//
// A brief is matched across the base and the change by EITHER key, so a change
// has to alter both to lose the match. A row already at or past its new status at
// the base is left alone, so a pin bump turns nothing red that has already landed.
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

// buildGateSnapshot reads a board from a list of repo-relative paths. read
// returns a file's bytes; an unreadable file contributes what could be read (a
// brief whose file cannot be read has no gate, which the judge treats as not
// `human` — the fail-closed direction for a brief that was `human` at the base).
func buildGateSnapshot(paths []string, read func(string) ([]byte, error)) gateSnapshot {
	type group struct {
		tree, name, readme string
		briefs             []string
	}
	groups := map[string]*group{}
	for _, p := range paths {
		p = filepath.ToSlash(strings.TrimSpace(p))
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

	snap := gateSnapshot{}
	for _, k := range keys {
		g := groups[k]
		local := map[string]*gateBrief{}
		var order []string
		// A brief at the stream root wins over a same-numbered one in done/.
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
			if _, dup := local[key]; dup {
				continue
			}
			gb := &gateBrief{Key: key, Board: g.name + "/" + m[1], Path: p, Readme: g.readme}
			gb.addSpelling(gb.Board)
			if raw, err := read(p); err == nil {
				readGateFrontmatter(gb, raw)
			}
			local[key] = gb
			order = append(order, key)
		}
		if g.readme != "" {
			if raw, err := read(g.readme); err == nil {
				body := string(raw)
				if _, b, ferr := splitFrontmatter(body); ferr == nil {
					body = b
				}
				rows, _ := parseBriefTable(body)
				for _, r := range rows {
					key := g.name + "/" + normBriefNum(r.Num)
					gb := local[key]
					if gb == nil {
						gb = &gateBrief{Key: key, Board: g.name + "/" + r.Num, Readme: g.readme}
						local[key] = gb
						order = append(order, key)
					}
					if gb.Status == "" {
						gb.Status = r.Status
						gb.Board = g.name + "/" + r.Num
					}
					gb.addSpelling(g.name + "/" + r.Num)
				}
			}
		}
		for _, key := range order {
			if _, taken := snap[key]; taken {
				continue
			}
			snap[key] = local[key]
		}
	}
	return snap
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
func gateSnapshotOnDiskWith(root, overrideRel string, override []byte) gateSnapshot {
	var paths []string
	for _, tree := range []string{"streams", "archive"} {
		dir := filepath.Join(root, "docs", tree)
		streams, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, s := range streams {
			if !s.IsDir() {
				continue
			}
			sub := filepath.Join(dir, s.Name())
			_ = filepath.WalkDir(sub, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					if p != sub && d.Name() != archiveDirName {
						return filepath.SkipDir
					}
					return nil
				}
				if rel, rerr := filepath.Rel(root, p); rerr == nil {
					paths = append(paths, filepath.ToSlash(rel))
				}
				return nil
			})
		}
	}
	overrideRel = filepath.ToSlash(overrideRel)
	return buildGateSnapshot(paths, func(rel string) ([]byte, error) {
		if overrideRel != "" && rel == overrideRel {
			return override, nil
		}
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	})
}

// gateSnapshotAtRev reads the board as committed at rev.
func gateSnapshotAtRev(root, rev string) (gateSnapshot, error) {
	out, err := exec.Command("git", "-C", root, "ls-tree", "-r", "-z", "--name-only", rev, "--", "docs/streams", "docs/archive").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-tree %s: %w", rev, err)
	}
	paths := strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")
	return buildGateSnapshot(paths, func(p string) ([]byte, error) { return gitShowObject(root, rev, p) }), nil
}

// gateFault is one change the hold judges: a brief moved, relabelled or dropped
// while in scope.
type gateFault struct {
	Move, Relabel, Drop bool
	Head                *gateBrief   // the brief after the change (nil for a drop)
	Base                []*gateBrief // the base briefs it matches (for a drop, the dropped one)
	From                string       // the lowest matched base status ("" = not on the board)
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
// after the change first, then the base briefs it matches. They are the same
// brief by the matching rule, so a ruling recorded on any of them is that
// brief's ruling.
func (f gateFault) candidates() []*gateBrief {
	var out []*gateBrief
	if f.Head != nil {
		out = append(out, f.Head)
	}
	return append(out, f.Base...)
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
		why := "it has no permanent id:"
		if b.PermID != "" {
			why = "nor its permanent id:"
		}
		parts = append(parts, fmt.Sprintf("drops it — no brief after the change carries its board id %s, %s", b.Board, why))
	}
	return strings.Join(parts, "; and ")
}

// judgeDecisionGate compares two board snapshots and returns every change the
// hold covers, ruled or not. Deterministic order: briefs after the change by
// key, then dropped base briefs by key.
func judgeDecisionGate(base, head gateSnapshot) []gateFault {
	byPerm := map[string][]*gateBrief{}
	for _, k := range sortedGateKeys(base) {
		b := base[k]
		if b.PermID != "" {
			byPerm[b.PermID] = append(byPerm[b.PermID], b)
		}
	}
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
		if h.PermID != "" {
			for _, b := range byPerm[h.PermID] {
				add(b)
			}
		}
		baseHuman := false
		for _, b := range ms {
			matched[b] = true
			if b.Gate == gateHumanValue {
				baseHuman = true
			}
		}
		if h.Gate != gateHumanValue && !baseHuman {
			continue // not in scope on either side
		}
		f := gateFault{Head: h, Base: ms}
		if hr := gateStatusRank(h.Status); hr >= 1 {
			if len(ms) == 0 {
				f.Move = true
			} else {
				low := ms[0]
				for _, b := range ms[1:] {
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
			faults = append(faults, gateFault{Drop: true, Base: []*gateBrief{b}})
		}
	}
	return faults
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
	for _, c := range f.candidates() {
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
func decisionGateHoldProblems(root string) (problems, notices []string) {
	if hasNoGitDir(root) {
		return nil, []string{"decision-gate hold (" + decisionGateSpecRef + ") did not run: .git is absent, so there is no base to judge a status move against"}
	}
	base, resolved := registerLandedBase(root)
	if !resolved {
		return nil, []string{"decision-gate hold (" + decisionGateSpecRef + ") did not run: origin/main could not be resolved, so a gate: human brief's status move cannot be judged against its base. If this is CI, fetch origin/main before the lint step; the network pass (statusgen --decision-gate) fails closed in the same case"}
	}
	baseSnap, err := gateSnapshotAtRev(root, base)
	if err != nil {
		return nil, []string{fmt.Sprintf("decision-gate hold (%s) could not check: the board at the base %s could not be read (%v)", decisionGateSpecRef, base, err)}
	}
	for _, f := range judgeDecisionGate(baseSnap, gateSnapshotOnDisk(root)) {
		if _, ok, missing := offlineRuling(f); !ok {
			problems = append(problems, layerOneRefusal(f, missing))
		}
	}
	return problems, nil
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
