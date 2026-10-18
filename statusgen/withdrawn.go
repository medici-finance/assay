package main

// withdrawn.go — the terminal `withdrawn` brief status.
//
// A brief retired by a later decision, or merged into another brief, is not
// open work, not blocked work, and not done work. It is WITHDRAWN: the README
// Status cell carries the bare token `withdrawn`, and the brief file's
// frontmatter — schema brief-v3, which is brief-v2 plus exactly these three
// keys — records when, why, and where the work went:
//
//	schema: brief-v3
//	withdrawn-date: "2026-10-18"
//	superseded-by: ["stream/28", "cell:alias:stream:30"]   # [] = won't-fix
//	withdrawn-reason: "One sentence: why, and what survives where."
//
// Semantics (spec/brief-v1.md §3.6, spec/lifecycle-v1.md §2.0a): terminal; excluded from the
// done-ratio denominator, the Next-up/dispatch queue, the stalled/bottleneck/
// blocked views and the todo count; shown in its own Withdrawn count.
//
// Lint (checkBriefFiles → checkWithdrawalFile / withdrawnRowProblems):
//   - status `withdrawn` and the three keys require schema brief-v3;
//   - status `withdrawn` ⇔ withdrawn-date AND a non-empty withdrawn-reason;
//   - any of the three keys on a non-withdrawn brief is a PROBLEM;
//   - a same-repo `<stream>/<NN>` successor must resolve to an existing brief;
//   - a global `<cell>:<alias>:<stream>:<NN>` successor is syntax-checked, and
//     resolved only when the root it names is configured in this run;
//   - a brief whose `depends:` names a withdrawn brief gets a NOTICE naming the
//     successor(s), so the dependency is re-pointed rather than held forever.

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const statusWithdrawn = "withdrawn"

// Withdrawal is the parsed withdrawn-status record of one brief file. Has*
// distinguish an ABSENT key from a present-but-empty one, which the pairing
// rule needs (a present key on a live brief is a PROBLEM even when empty).
type Withdrawal struct {
	Date            string
	HasDate         bool
	Reason          string
	HasReason       bool
	SupersededBy    []string
	HasSupersededBy bool
}

var (
	withdrawnDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	// globalBriefIDRe is the global brief id — the hierarchical form the
	// brief-v2 `brief:` key uses: <cell>:<repo-alias>:<stream>:<NN>.
	globalBriefIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:[0-9]+[a-z]?$`)
)

// isWithdrawn reports whether a README status cell is the withdrawn token.
func isWithdrawn(status string) bool { return status == statusWithdrawn }

// liveBriefCount is a stream's brief count WITHOUT withdrawn briefs — the
// denominator of every "Briefs done x/y" ratio.
func liveBriefCount(s *Stream) int {
	n := 0
	for _, b := range s.Briefs {
		if !isWithdrawn(b.Status) {
			n++
		}
	}
	return n
}

// withdrawnCount is a stream's withdrawn-brief count (its own Withdrawn count).
func withdrawnCount(s *Stream) int {
	return len(s.Briefs) - liveBriefCount(s)
}

// parseWithdrawalKeys reads the three withdrawn keys off the raw frontmatter.
// Only TYPE errors are raised (via addBad, which fails the parse).
func parseWithdrawalKeys(data map[string]any, bf *BriefFile, addBad func(string, ...any)) {
	var w Withdrawal
	if v, ok := data["withdrawn-date"]; ok {
		w.HasDate = true
		switch d := v.(type) {
		case string:
			w.Date = strings.TrimSpace(d)
		case time.Time:
			// An unquoted YAML date decodes to a timestamp; keep its calendar day.
			w.Date = d.Format("2006-01-02")
		case nil:
			w.Date = ""
		default:
			addBad("withdrawn-date must be a YYYY-MM-DD string")
		}
	}
	if v, ok := data["withdrawn-reason"]; ok {
		w.HasReason = true
		switch r := v.(type) {
		case string:
			w.Reason = strings.TrimSpace(r)
		case nil:
			w.Reason = ""
		default:
			addBad("withdrawn-reason must be a string")
		}
	}
	if v, ok := data["superseded-by"]; ok {
		w.HasSupersededBy = true
		if v != nil {
			list, err := stringList(v)
			if err != nil {
				addBad("superseded-by: %v", err)
			} else {
				w.SupersededBy = list
			}
		}
	}
	if w.HasDate || w.HasReason || w.HasSupersededBy {
		bf.Withdrawal = &w
	}
}

// withdrawalResolver resolves successor refs against the streams of this run.
type withdrawalResolver struct {
	all      []*Stream
	byRoot   map[string]map[string]*Stream
	regCache map[string]*graphRepos
	regSeen  map[string]bool
	selfID   map[string]string
}

func newWithdrawalResolver(all []*Stream) *withdrawalResolver {
	r := &withdrawalResolver{all: all, byRoot: map[string]map[string]*Stream{}, regCache: map[string]*graphRepos{}, regSeen: map[string]bool{}, selfID: map[string]string{}}
	for _, s := range all {
		m := r.byRoot[s.Root]
		if m == nil {
			m = map[string]*Stream{}
			r.byRoot[s.Root] = m
		}
		if _, dup := m[s.Name]; !dup {
			m[s.Name] = s
		}
	}
	return r
}

func (r *withdrawalResolver) registry(root string) *graphRepos {
	if !r.regSeen[root] {
		r.regSeen[root] = true
		reg, _, _ := loadGraphRepos(root) // a broken registry is reported by its own check
		r.regCache[root] = reg
	}
	return r.regCache[root]
}

// findBrief returns the README row for stream/num under root, or nil.
func (r *withdrawalResolver) findBrief(root, stream, num string) *Brief {
	s := r.byRoot[root][stream]
	if s == nil {
		return nil
	}
	for i := range s.Briefs {
		if s.Briefs[i].Num == num {
			return &s.Briefs[i]
		}
	}
	return nil
}

// rootIdentity returns the "<cell>:<alias>" a root identifies itself as: the
// registry's `self:` key when it declares one, else the cell:alias prefix of the
// first brief-v2-family `brief:` id found in that root's streams. "" = unknown.
func (r *withdrawalResolver) rootIdentity(root string) string {
	if id, ok := r.selfID[root]; ok {
		return id
	}
	id := ""
	if reg := r.registry(root); reg != nil && reg.Self != "" {
		id = reg.Cell + ":" + reg.Self
	} else {
	scan:
		for _, s := range r.byRoot[root] {
			for _, path := range briefFilePaths(s) {
				bf, ok, err := parseBriefFile(path)
				if err != nil || !ok || !isBriefV2Family(bf.Schema) {
					continue
				}
				if cell, alias, _, _, ok := parseBriefV2ID(bf.Brief); ok {
					id = cell + ":" + alias
					break scan
				}
			}
		}
	}
	r.selfID[root] = id
	return id
}

// rootForGlobal returns the configured root that identifies itself as
// cell:alias, and whether one was found. ownID is the declaring brief's own
// cell:alias (from its brief-v2-family id), which identifies its own root even
// when the registry declares no `self:`.
func (r *withdrawalResolver) rootForGlobal(ownRoot, ownID, cell, alias string) (string, bool) {
	want := cell + ":" + alias
	if ownID == want || r.rootIdentity(ownRoot) == want {
		return ownRoot, true
	}
	for root := range r.byRoot {
		if root != ownRoot && r.rootIdentity(root) == want {
			return root, true
		}
	}
	return "", false
}

// checkWithdrawalFile runs the per-file withdrawn rules for one parsed brief
// whose README row is in hand.
func checkWithdrawalFile(add, notice func(string, ...any), path, selfID string, bf *BriefFile, row *Brief, s *Stream, res *withdrawalResolver) {
	checkWithdrawalKeys(add, notice, path, selfID, bf, row, s, res)
	checkDependsOnWithdrawn(notice, path, bf, s, res)
}

// checkWithdrawalKeys is the status ⇔ keys pairing for one brief file.
func checkWithdrawalKeys(add, notice func(string, ...any), path, selfID string, bf *BriefFile, row *Brief, s *Stream, res *withdrawalResolver) {
	w := bf.Withdrawal
	// The withdrawn keys and status exist only in brief-v3 (driver ruling
	// 2026-10-18: a new key set is a schema version, not a silent extension of
	// brief-v1/brief-v2).
	if bf.Schema != briefSchemaV3 {
		if isWithdrawn(row.Status) {
			add("%s: brief %s is withdrawn but is schema %s — a withdrawn brief must be schema brief-v3", path, selfID, bf.Schema)
		} else if w != nil {
			add("%s: brief %s carries withdrawn keys but is schema %s — withdrawn-date / withdrawn-reason / superseded-by exist only in schema brief-v3", path, selfID, bf.Schema)
		}
		return
	}
	if !isWithdrawn(row.Status) {
		if w != nil {
			var keys []string
			if w.HasDate {
				keys = append(keys, "withdrawn-date")
			}
			if w.HasReason {
				keys = append(keys, "withdrawn-reason")
			}
			if w.HasSupersededBy {
				keys = append(keys, "superseded-by")
			}
			add("%s: brief %s carries %s but its README status is %q — these keys belong only to a `withdrawn` brief; set the Status cell to withdrawn or remove them", path, selfID, strings.Join(keys, ", "), row.Status)
		}
		return
	}
	if w == nil || !w.HasDate || w.Date == "" {
		add("%s: brief %s is withdrawn but has no withdrawn-date — add `withdrawn-date: \"YYYY-MM-DD\"`", path, selfID)
	} else if !withdrawnDateRe.MatchString(w.Date) {
		add("%s: brief %s withdrawn-date %q is not YYYY-MM-DD", path, selfID, w.Date)
	} else if _, err := time.Parse("2006-01-02", w.Date); err != nil {
		add("%s: brief %s withdrawn-date %q is not a calendar date", path, selfID, w.Date)
	}
	if w == nil || w.Reason == "" {
		add("%s: brief %s is withdrawn but has no non-empty withdrawn-reason — say why, and what survives where", path, selfID)
	}
	if w != nil {
		for _, ref := range w.SupersededBy {
			checkSuccessorRef(add, path, selfID, bf.Brief, ref, s, res) // briefid:raw only its cell:alias prefix is read, to identify this brief's own root for a global successor id
		}
	}
	if row.Verified != "" || row.Reviewed != "" {
		notice("%s: brief %s is withdrawn but its README Verified/Reviewed cells are set — a withdrawn brief was never verified; leave them `—`", path, selfID)
	}
}

// checkDependsOnWithdrawn NOTICEs every depends edge into a withdrawn brief.
func checkDependsOnWithdrawn(notice func(string, ...any), path string, bf *BriefFile, s *Stream, res *withdrawalResolver) {
	// depends-on-withdrawn: the edge can never be satisfied. Name the successor
	// so the author re-points the dependency instead of waiting on it.
	for _, dep := range bf.Depends {
		parts := strings.SplitN(dep, "/", 2)
		if len(parts) != 2 {
			continue // malformed refs are checkRef's PROBLEM
		}
		target := res.findBrief(s.Root, parts[0], parts[1])
		if target == nil || !isWithdrawn(target.Status) {
			continue
		}
		succ := "no successor (won't-fix)"
		if len(target.SupersededBy) > 0 {
			succ = "successor " + strings.Join(target.SupersededBy, ", ")
		}
		notice("%s: depends on %s, which is withdrawn — %s; re-point the dependency (a withdrawn brief never satisfies a depends edge)", path, dep, succ)
	}
}

// checkSuccessorRef validates one superseded-by entry.
func checkSuccessorRef(add func(string, ...any), path, selfID, selfFull, ref string, s *Stream, res *withdrawalResolver) {
	ref = strings.TrimSpace(ref)
	switch {
	case ref == "":
		add("%s: superseded-by has an empty entry", path)
	case refInRepoBriefRe.MatchString(ref):
		if ref == selfID {
			add("%s: superseded-by %q is the brief itself — a brief cannot supersede itself", path, ref)
			return
		}
		parts := strings.SplitN(ref, "/", 2)
		if res.findBrief(s.Root, parts[0], parts[1]) == nil {
			add("%s: superseded-by %q does not resolve to a brief in this repo (no README row %s in stream %s)", path, ref, parts[1], parts[0])
		}
	case globalBriefIDRe.MatchString(ref):
		seg := strings.Split(ref, ":")
		ownID := ""
		if c, a, _, _, ok := parseBriefV2ID(selfFull); ok {
			ownID = c + ":" + a
		}
		root, configured := res.rootForGlobal(s.Root, ownID, seg[0], seg[1])
		if !configured {
			return // syntax only: the root it names is not configured in this run
		}
		if root == s.Root && seg[2]+"/"+seg[3] == selfID {
			add("%s: superseded-by %q is the brief itself — a brief cannot supersede itself", path, ref)
			return
		}
		if res.findBrief(root, seg[2], seg[3]) == nil {
			add("%s: superseded-by %q does not resolve — no README row %s in stream %s of the configured root it names", path, ref, seg[3], seg[2])
		}
	default:
		add("%s: superseded-by %q is not a brief id (want <stream>/<NN> for this repo, or the global <cell>:<repo-alias>:<stream>:<NN>)", path, ref)
	}
}

// withdrawnRowProblems flags a README row at `withdrawn` that has no typed
// (brief-v1/brief-v2) brief file to carry the required keys — a legacy or
// placeholder row cannot record why or where the work went.
func withdrawnRowProblems(streams []*Stream) []string {
	var out []string
	for _, s := range streams {
		for _, b := range s.Briefs {
			if !isWithdrawn(b.Status) {
				continue
			}
			if b.Schema != briefSchemaCurrent && !isBriefV2Family(b.Schema) {
				out = append(out, fmt.Sprintf("%s/brief-%s: status withdrawn requires a schema brief-v3 brief file carrying withdrawn-date and withdrawn-reason (this row has none)", s.Name, b.Num))
			}
		}
	}
	return out
}
