package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ---- types ----

// WorkEntry is one piece of open work (a PR) registered by a session.
type WorkEntry struct {
	Repo string `json:"repo"`
	PR   int    `json:"pr"`
	What string `json:"what"`
}

// Beacon is a session's self-declared roster entry stored at
// ~/.config/assay/roster/<session>.json.
//
// Acks is carried as an OPAQUE RawMessage, not a typed field, deliberately: the `deskack`
// verb appends receipt records under the `acks` key of this SAME
// file, and deskroster owns none of that shape. Round-tripping it verbatim keeps
// deskroster's own rewrites (set / auto-prune) from dropping another writer's data on the
// floor — the same reason AppendAck (deskkit/ackbeacon.go) preserves deskroster's fields.
type Beacon struct {
	Session  string          `json:"session"`
	Role     string          `json:"role,omitempty"`
	Updated  string          `json:"updated"`
	OpenWork []WorkEntry     `json:"open_work,omitempty"`
	Acks     json.RawMessage `json:"acks,omitempty"`
	// Resource is example-stream/13's self-reported vitals block, owned by
	// deskkit.MergeResourceVitals — carried here as an opaque RawMessage for the SAME
	// reason Acks is: display readers can inspect it without owning its schema.
	// Writes preserve every non-roster raw field through the shared transaction.
	Resource json.RawMessage `json:"resource,omitempty"`
}

// hasAcks reports whether a displayed beacon carries at least one receipt record.
// Mutation deletion is more conservative: every foreign field preserves the file.
func (b *Beacon) hasAcks() bool {
	s := strings.TrimSpace(string(b.Acks))
	return s != "" && s != "[]" && s != "null"
}

// Claim is the LEGACY roster/bash write shape for a per-brief dispatch claim stored at
// ~/.config/assay/claims/*.claim. It is retained so this package's tests can WRITE
// that historical shape; the READ path (loadClaims) now goes through the canonical
// deskkit.Claim tolerant reader, so `deskroster list` surfaces every
// claim regardless of which writer wrote it (loopengine's {itemID,runner,claimed}, the
// roster/bash {brief,session,ts}, or the canonical {kind,item,owner,branch,ts}).
type Claim struct {
	Brief   string `json:"brief"`
	Session string `json:"session"`
	TS      string `json:"ts"`
}

// PRInfo is the live state of a PR fetched from GitHub.
type PRInfo struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"`
	IsDraft bool   `json:"isDraft"`
}

// ---- path helpers ----

func rosterDir() (string, error) {
	base, err := deskkit.StateDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve state dir: %w", err)
	}
	return filepath.Join(base, "roster"), nil
}

func beaconPath(session string) (string, error) {
	dir, err := rosterDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, session+".json"), nil
}

func claimsDir() (string, error) {
	base, err := deskkit.StateDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve state dir: %w", err)
	}
	return filepath.Join(base, "claims"), nil
}

// ---- session resolution ----

// resolveSession returns the session name from env or flag.
// Order: $DESK_SESSION → $CLAUDE_SESSION_ID → flag → exit 6.
func resolveSession(sessionFlag string) (string, error) {
	if s := os.Getenv("DESK_SESSION"); s != "" {
		return s, nil
	}
	if s := os.Getenv("CLAUDE_SESSION_ID"); s != "" {
		return s, nil
	}
	if sessionFlag != "" {
		return sessionFlag, nil
	}
	return "", deskkit.Unverifiable(
		"cannot resolve session identity: set $DESK_SESSION, $CLAUDE_SESSION_ID, or pass --session",
		nil)
}

// ---- beacon I/O ----

func decodeBeacon(session string, fields map[string]json.RawMessage) (*Beacon, error) {
	data, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	var b Beacon
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("malformed beacon: %w", err)
	}
	// The filename selects the session; a stored field never redirects writes.
	b.Session = session
	return &b, nil
}

func loadBeacon(session string) (*Beacon, error) {
	fields, err := deskkit.ReadRosterBeacon(session)
	if err != nil {
		return nil, err
	}
	return decodeBeacon(session, fields)
}

// mutateBeacon decodes and changes only deskroster's fields under the shared
// beacon lock. Unknown fields belong to other writers and survive every update.
// Returning false leaves the current file untouched, including a missing file.
func mutateBeacon(session string, update func(*Beacon) bool) (deleted bool, err error) {
	_, err = deskkit.MutateRosterBeacon(session, func(fields map[string]json.RawMessage) (deskkit.BeaconAction, error) {
		b, err := decodeBeacon(session, fields)
		if err != nil {
			return deskkit.BeaconKeep, err
		}
		if !update(b) {
			return deskkit.BeaconKeep, nil
		}
		// Delete only a beacon wholly owned by deskroster. Even empty foreign
		// fields are not ours to discard (acks, resource, and future extensions).
		onlyRoster := true
		for key := range fields {
			switch key {
			case "session", "role", "updated", "open_work":
			default:
				onlyRoster = false
			}
		}
		if len(b.OpenWork) == 0 && b.Role == "" && onlyRoster {
			deleted = true
			return deskkit.BeaconDelete, nil
		}
		b.Updated = time.Now().UTC().Format(time.RFC3339)
		data, err := json.Marshal(b)
		if err != nil {
			return deskkit.BeaconKeep, err
		}
		var owned map[string]json.RawMessage
		if err := json.Unmarshal(data, &owned); err != nil {
			return deskkit.BeaconKeep, err
		}
		for _, key := range []string{"session", "role", "updated", "open_work"} {
			if value, ok := owned[key]; ok {
				fields[key] = value
			} else {
				delete(fields, key)
			}
		}
		return deskkit.BeaconWrite, nil
	})
	return deleted, err
}

// pruneBeacon removes only exact entries proven closed before acquiring the lock.
// A concurrent upsert with a different description is new work, not that evidence.
func pruneBeacon(session string, closed []WorkEntry) error {
	candidates := make(map[WorkEntry]bool, len(closed))
	for _, entry := range closed {
		candidates[entry] = true
	}
	_, err := mutateBeacon(session, func(b *Beacon) bool {
		changed := false
		kept := b.OpenWork[:0]
		for _, entry := range b.OpenWork {
			if candidates[entry] {
				changed = true
			} else {
				kept = append(kept, entry)
			}
		}
		b.OpenWork = kept
		return changed
	})
	return err
}

// ---- claims loading ----

// loadClaims reads every claim through the canonical deskkit tolerant reader, so a claim
// written in ANY of the three historical shapes (item 2) appears in the
// roster. It preserves the prior best-effort semantics: a missing dir is empty, and an
// individual unreadable/malformed file is skipped rather than blanking the whole list.
func loadClaims() ([]deskkit.Claim, error) {
	dir, err := claimsDir()
	if err != nil {
		return nil, err
	}
	return deskkit.List(deskkit.ClaimConfig{ClaimsDir: dir})
}

// ---- repo name mapping ----

// shortToFull builds a map from short repo name (e.g. "tracker") to
// full owner/repo (e.g. "example-org/tracker").
func shortToFull() map[string]string {
	m := make(map[string]string)
	for _, full := range deskkit.AllowedRepos() {
		parts := strings.SplitN(full, "/", 2)
		if len(parts) == 2 {
			m[parts[1]] = full
		}
	}
	return m
}

// resolveFullRepo returns the full owner/repo for a short repo name.
// If the short name isn't in the mapping, returns it as-is (for robustness).
func resolveFullRepo(short string) string {
	m := shortToFull()
	if full, ok := m[short]; ok {
		return full
	}
	return short
}

// resolveShortRepo returns the short repo name from a full owner/repo.
func resolveShortRepo(full string) string {
	parts := strings.SplitN(full, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return full
}

// ---- forge PR reads ----
//
// These two reads are DISPLAY enrichment for the roster listing — they read a
// change's state/draft/title so `deskroster list` can annotate a beacon's open
// work and surface unclaimed open PRs. They route through the enumerated Forge
// seam (forge.go), never a forge CLI: the closed-surface brief (example-stream/08)
// bans invoking a forge CLI (gh/glab) anywhere in tools/desk, and both fields
// these reads need already exist on the interface — GetPullRequest carries state,
// draft and title; ListOpenChanges carries number, title and draft — so the
// migration needs no new op (spec §6 freeze rule) and no ambient credential.
// Both fail SOFT: a resolver or read error returns nil/empty and the caller
// renders "?"/omits the row, exactly as the former gh shell-out did on failure.

// ghViewPR reads a single change's state/draft/title via the Forge seam and
// maps it to the roster's PRInfo. Returns nil if the forge cannot be resolved
// or the read fails (e.g. PR not found, no readable identity).
func ghViewPR(fullRepo string, pr int) *PRInfo {
	f, fr, err := forgeFor(fullRepo)
	if err != nil {
		return nil
	}
	p, err := f.GetPullRequest(fr, pr)
	if err != nil || p == nil {
		return nil
	}
	// GetPullRequest reports State as "open"/"closed" with a separate merged
	// flag; the roster display expects the uppercase OPEN/MERGED/CLOSED the old
	// `gh pr view --json state` returned, so derive it here.
	state := strings.ToUpper(p.State)
	if p.Merged || p.MergedAt != "" {
		state = "MERGED"
	}
	return &PRInfo{Number: pr, Title: p.Title, State: state, IsDraft: p.Draft}
}

// ghListOpenPRs reads a repo's OPEN changes via the Forge seam and maps them to
// PRInfo. Returns nil on failure (forge unresolvable, read error).
func ghListOpenPRs(fullRepo string) []PRInfo {
	f, fr, err := forgeFor(fullRepo)
	if err != nil {
		return nil
	}
	oc, err := f.ListOpenChanges(fr)
	if err != nil || oc == nil {
		return nil
	}
	out := make([]PRInfo, 0, len(oc.Changes))
	for _, c := range oc.Changes {
		out = append(out, PRInfo{Number: c.Number, Title: c.Title, State: c.State, IsDraft: c.Draft})
	}
	return out
}

// ---- commands ----

// parseVitalNumeric parses a numeric vitals flag's raw value: "" means the flag was not
// passed (caller skips it entirely — the field stays unset/null), the sentinel "unknown"
// means the source read failed (could-not-check), and anything else must parse as the
// requested numeric kind or the whole `set` call is refused — a numeric flag that silently
// became could-not-check on a typo would hide a real reading behind a false blind.
func parseVitalNumericInt(flagName, raw string) (*deskkit.VitalField, error) {
	switch raw {
	case "":
		return nil, nil
	case "unknown":
		return deskkit.CouldNotCheckVital(), nil
	default:
		n, perr := strconv.ParseInt(raw, 10, 64)
		if perr != nil {
			return nil, deskkit.Refused(fmt.Sprintf(
				"refused: --%s must be an integer or the sentinel \"unknown\" (got %q)", flagName, raw))
		}
		return deskkit.MeasuredInt(n), nil
	}
}

func parseVitalNumericFloat(flagName, raw string) (*deskkit.VitalField, error) {
	switch raw {
	case "":
		return nil, nil
	case "unknown":
		return deskkit.CouldNotCheckVital(), nil
	default:
		f, perr := strconv.ParseFloat(raw, 64)
		if perr != nil {
			return nil, deskkit.Refused(fmt.Sprintf(
				"refused: --%s must be a number or the sentinel \"unknown\" (got %q)", flagName, raw))
		}
		return deskkit.MeasuredFloat(f), nil
	}
}

// parseVitalString parses a string vitals flag (--model): "" ⇒ unset, "unknown" ⇒
// could-not-check, anything else is the measured value verbatim.
func parseVitalString(raw string) *deskkit.VitalField {
	switch raw {
	case "":
		return nil
	case "unknown":
		return deskkit.CouldNotCheckVital()
	default:
		return deskkit.MeasuredString(raw)
	}
}

func cmdSet(args []string) error {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	var (
		repo    string
		pr      int
		what    string
		role    string
		session string
		width   int
		// Vitals flags (example-stream/13) — the per-tick self-report the desk session
		// makes about ITSELF, not about the work. Each is a raw string so the sentinel
		// "unknown" is distinguishable from "not passed" (flag.StringVar's zero value, "").
		tokens    string
		ctxPct    string
		ageSecs   string
		subagents string
		model     string
	)
	fs.StringVar(&repo, "repo", "", "Short repo name (e.g. tracker)")
	fs.IntVar(&pr, "pr", 0, "PR number")
	fs.StringVar(&what, "what", "", "Description of the work")
	fs.StringVar(&role, "role", "", "Optional role label for this session")
	fs.StringVar(&session, "session", "", "Session name (env: $DESK_SESSION or $CLAUDE_SESSION_ID)")
	fs.IntVar(&width, "width", 0, "With --role: set that LOOP's agent-pool width (see `deskroster width --role`)")
	fs.StringVar(&tokens, "tokens", "", "Cumulative tokens this session has consumed, or \"unknown\" (could-not-check); omit to leave unset")
	fs.StringVar(&ctxPct, "context-pct", "", "Percent of the context window in use (0-100), or \"unknown\"; omit to leave unset")
	fs.StringVar(&ageSecs, "session-age-seconds", "", "Wall seconds since this session booted, or \"unknown\"; omit to leave unset")
	fs.StringVar(&subagents, "subagents", "", "Count of subagents this session has launched, or \"unknown\"; omit to leave unset")
	fs.StringVar(&model, "model", "", "The model id this session runs, or \"unknown\"; omit to leave unset")

	if err := fs.Parse(args); err != nil {
		return deskkit.Refused(fmt.Sprintf("set: %v", err))
	}

	// Role-only set: --role without --repo/--pr/--what updates just the role.
	roleOnly := (role != "" && repo == "" && pr == 0 && what == "")
	// Work-entry set: --repo, --pr, and --what are all required.
	workSet := (repo != "" && pr != 0 && what != "")
	// Width set: --role names the LOOP whose pool is being resized. It is a distinct shape
	// from the two above, and deliberately so — see the next block.
	widthSet := width != 0
	// Vitals set: any one of the five resource flags — orthogonal to role/work/width, and
	// combinable with --role (the per-tick self-declare call this brief's facts describe).
	vitalsSet := tokens != "" || ctxPct != "" || ageSecs != "" || subagents != "" || model != ""

	if widthSet {
		// --width addresses ANOTHER window's pool; it does not describe this session. Writing
		// the role onto this session's beacon here would rename the SETTER (the coordinator
		// steering a bottlenecked desk would relabel itself as that desk), so the width path
		// never touches the beacon at all — see width.go's header for the full argument.
		if role == "" {
			return deskkit.Refused("set --width requires --role naming the loop whose pool to size " +
				"(e.g. --role pr-review-desk --width 8); a width with no loop is a number with no pool")
		}
		if repo != "" || pr != 0 || what != "" {
			return deskkit.Refused("set --width is a pool-size directive for a loop, not a work entry: " +
				"pass it with --role alone, and register work in a separate `deskroster set --repo … --pr … --what …`")
		}
		if vitalsSet {
			return deskkit.Refused("set --width is a pool-size directive for a loop, not a vitals self-report: " +
				"pass it with --role alone, and report vitals in a separate `deskroster set --tokens … [...]`")
		}
		setBy, serr := resolveSession(session)
		if serr != nil {
			return serr
		}
		canonical, werr := setWidth(role, width, setBy, time.Now())
		if werr != nil {
			return werr
		}
		fmt.Printf("deskroster: %s set %s pool width to %d (in force for %s; %s reads it each tick)\n",
			setBy, canonical, width, deskkit.WidthTTL, canonical)
		return nil
	}

	// role/work-entry require exactly the shapes above; a vitals flag with none of those
	// is its own valid shape (the per-tick "just report vitals" call), so it is checked
	// separately rather than folded into roleOnly/workSet.
	bareVitalsOnly := !roleOnly && !workSet && vitalsSet && repo == "" && pr == 0 && what == ""
	if !roleOnly && !workSet && !bareVitalsOnly {
		return deskkit.Refused("set requires either (--repo, --pr, --what) for a work entry, --role alone for a " +
			"standing session, or a vitals flag (--tokens/--context-pct/--session-age-seconds/--subagents/--model) " +
			"to report resource vitals")
	}

	sess, err := resolveSession(session)
	if err != nil {
		return err
	}
	// Security review finding S-1 (example-stream/13): a session name that does not
	// resolve to a single path segment must never be silently joined into a beacon path —
	// once a beacon write can arm an involuntary recycle (example-stream/14), that join
	// is a control-bearing surface, not just a display quirk.
	if !deskkit.ValidSessionSegment(sess) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: session name %q does not resolve to a single path segment (no \"/\", no \"..\", "+
				"non-empty) — refusing to join it into the beacon path", sess))
	}

	if role != "" || workSet {
		_, err := mutateBeacon(sess, func(b *Beacon) bool {
			if role != "" {
				b.Role = role
			}
			if workSet {
				for i, w := range b.OpenWork {
					if w.Repo == repo && w.PR == pr {
						b.OpenWork[i].What = what
						return true
					}
				}
				b.OpenWork = append(b.OpenWork, WorkEntry{Repo: repo, PR: pr, What: what})
			}
			return true
		})
		if err != nil {
			return deskkit.Unverifiable("cannot update beacon", err)
		}
	}

	// Vitals: an independent field-preserving raw-key merge (deskkit.MergeResourceVitals),
	// using the same beacon transaction as role/work mutations — see vitals.go for why a
	// second, dedicated writer is the safer property than one struct trusted to remember
	// every co-owned field forever.
	if vitalsSet {
		rv := deskkit.ResourceVitals{}
		if rv.Tokens, err = parseVitalNumericInt("tokens", tokens); err != nil {
			return err
		}
		if rv.ContextPctUsed, err = parseVitalNumericFloat("context-pct", ctxPct); err != nil {
			return err
		}
		if rv.SessionAgeSeconds, err = parseVitalNumericInt("session-age-seconds", ageSecs); err != nil {
			return err
		}
		if rv.SubagentsSpawned, err = parseVitalNumericInt("subagents", subagents); err != nil {
			return err
		}
		rv.Model = parseVitalString(model)
		if _, err := deskkit.MergeResourceVitals(sess, rv); err != nil {
			return err
		}
	}

	switch {
	case workSet:
		fmt.Printf("deskroster: %s registered PR #%d (%s) as %q\n", sess, pr, repo, what)
	case role != "":
		fmt.Printf("deskroster: %s set role %q\n", sess, role)
	default:
		fmt.Printf("deskroster: %s reported resource vitals\n", sess)
	}
	if vitalsSet && (workSet || role != "") {
		fmt.Printf("deskroster: %s also reported resource vitals\n", sess)
	}
	return nil
}

func cmdDrop(args []string) error {
	fs := flag.NewFlagSet("drop", flag.ContinueOnError)
	var (
		repo    string
		pr      int
		session string
	)
	fs.StringVar(&repo, "repo", "", "Short repo name (e.g. tracker)")
	fs.IntVar(&pr, "pr", 0, "PR number")
	fs.StringVar(&session, "session", "", "Session name (env: $DESK_SESSION or $CLAUDE_SESSION_ID)")

	if err := fs.Parse(args); err != nil {
		return deskkit.Refused(fmt.Sprintf("drop: %v", err))
	}

	if repo == "" || pr == 0 {
		return deskkit.Refused("drop requires --repo and --pr")
	}

	sess, err := resolveSession(session)
	if err != nil {
		return err
	}

	found := false
	deleted, err := mutateBeacon(sess, func(b *Beacon) bool {
		filtered := b.OpenWork[:0]
		for _, w := range b.OpenWork {
			if w.Repo == repo && w.PR == pr {
				found = true
				continue
			}
			filtered = append(filtered, w)
		}
		b.OpenWork = filtered
		return found
	})
	if err != nil {
		return deskkit.Unverifiable("cannot update beacon", err)
	}
	if !found {
		fmt.Printf("deskroster: %s has no entry for PR #%d (%s) — nothing to drop\n", sess, pr, repo)
		return nil
	}
	if deleted {
		fmt.Printf("deskroster: dropped PR #%d (%s) from %s (beacon removed — no remaining work)\n", pr, repo, sess)
		return nil
	}

	fmt.Printf("deskroster: dropped PR #%d (%s) from %s\n", pr, repo, sess)
	return nil
}

// lineEntry is one display row for the list command.
type lineEntry struct {
	session string
	pr      string // "#N" or "-"
	state   string
	what    string
}

func cmdList() error {
	// 0. Scope guard (#494, same shape as #489 in deskboard). `list` sweeps
	// deskkit.AllowedRepos() twice — once via shortToFull() to resolve beacon repo
	// names, once directly for the unclaimed-PR scan — and an empty repo set makes
	// both loops iterate zero times. Before this guard that read nothing and reported
	// it as a clean, silent result: no unclaimed section, no beacon repos resolved,
	// exit 0. That is could-not-check, not "nothing to report", so it is checked
	// BEFORE any read is attempted, exactly like deskboard's dispatch-level guard.
	if err := deskkit.BoardScopeError(); err != nil {
		return err
	}

	// 1. Load all beacons.
	dir, err := rosterDir()
	if err != nil {
		return deskkit.Unverifiable("cannot resolve roster dir", err)
	}
	entries, readErr := os.ReadDir(dir)
	var beacons []Beacon
	if readErr == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			b, err := loadBeacon(strings.TrimSuffix(e.Name(), ".json"))
			if err != nil {
				continue
			}
			beacons = append(beacons, *b)
		}
	}
	// If dir doesn't exist, beacons stays empty.

	// 2. Load claims.
	claims, err := loadClaims()
	if err != nil {
		return deskkit.Unverifiable("cannot load claims", err)
	}

	// Build a set of (repo,pr) covered by beacons for the unclaimed scan.
	type repoPR struct {
		repo string
		pr   int
	}
	coveredByBeacon := make(map[repoPR]string) // (repo,pr) -> session name

	// 3. Query live PR state for each beacon entry and auto-prune merged/closed.
	type beaconRow struct {
		session string
		repo    string
		pr      int
		what    string
		state   string // LIVE, MERGED, CLOSED, or "?" on gh failure
	}
	var rows []beaconRow
	closedEntries := make(map[string][]WorkEntry) // exact entries checked outside the lock

	for i := range beacons {
		b := &beacons[i]
		var surviving []WorkEntry
		changed := false
		for _, w := range b.OpenWork {
			fullRepo := resolveFullRepo(w.Repo)
			info := ghViewPR(fullRepo, w.PR)
			state := "?"
			if info != nil {
				if info.State == "OPEN" {
					if info.IsDraft {
						state = "draft"
					} else {
						state = "READY"
					}
				} else {
					state = info.State // MERGED or CLOSED
				}
			}

			rows = append(rows, beaconRow{
				session: b.Session,
				repo:    w.Repo,
				pr:      w.PR,
				what:    w.What,
				state:   state,
			})

			// Auto-prune merged/closed entries (self-healing).
			if info != nil && (info.State == "MERGED" || info.State == "CLOSED") {
				changed = true
				closedEntries[b.Session] = append(closedEntries[b.Session], w)
				continue
			}
			surviving = append(surviving, w)
			coveredByBeacon[repoPR{repo: w.Repo, pr: w.PR}] = b.Session
		}

		if changed {
			b.OpenWork = surviving
		}
	}

	// Apply checked removals to the latest beacon, preserving concurrent writers.
	for session, closed := range closedEntries {
		if err := pruneBeacon(session, closed); err != nil {
			return deskkit.Unverifiable("cannot auto-prune beacon", err)
		}
	}

	// 4. Print main table.
	if len(rows) == 0 {
		fmt.Println("(no registered work)")
	} else {
		fmt.Printf("%-12s %-6s %-8s %s\n", "SESSION", "PR", "STATE", "WHAT")
		fmt.Printf("%-12s %-6s %-8s %s\n", "-------", "----", "------", "----")
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].session != rows[j].session {
				return rows[i].session < rows[j].session
			}
			if rows[i].repo != rows[j].repo {
				return rows[i].repo < rows[j].repo
			}
			return rows[i].pr < rows[j].pr
		})
		for _, r := range rows {
			fmt.Printf("%-12s #%-5d %-8s %s\n", r.session, r.pr, r.state, r.what)
		}
	}

	// 4b. Standing sessions (role-only, no open work).
	var standing []Beacon
	for _, b := range beacons {
		if b.Role != "" && len(b.OpenWork) == 0 {
			standing = append(standing, b)
		}
	}
	if len(standing) > 0 {
		if len(rows) > 0 {
			fmt.Println()
		}
		fmt.Println("--- standing sessions ---")
		fmt.Printf("%-12s %-6s %-8s %s\n", "SESSION", "PR", "STATE", "ROLE")
		fmt.Printf("%-12s %-6s %-8s %s\n", "-------", "----", "------", "----")
		sort.Slice(standing, func(i, j int) bool {
			return standing[i].Session < standing[j].Session
		})
		for _, s := range standing {
			fmt.Printf("%-12s %-6s %-8s %s\n", s.Session, "-", "active", s.Role)
		}
	}

	// 5. Dispatch claims section.
	if len(claims) > 0 {
		fmt.Println()
		fmt.Println("--- dispatch claims ---")
		fmt.Printf("%-12s %-6s %-9s %s\n", "SESSION", "PR", "STATE", "BRIEF")
		fmt.Printf("%-12s %-6s %-9s %s\n", "-------", "----", "------", "-----")
		sort.Slice(claims, func(i, j int) bool {
			if claims[i].Owner != claims[j].Owner {
				return claims[i].Owner < claims[j].Owner
			}
			return claims[i].Item < claims[j].Item
		})
		for _, c := range claims {
			fmt.Printf("%-12s %-6s %-9s %s\n", c.Owner, "-", "claimed", c.Item)
		}
	}

	// 6. Unclaimed open PRs section.
	// Scan all allowed repos for open PRs not covered by any beacon entry.
	var unclaimed []PRInfo
	for _, fullRepo := range deskkit.AllowedRepos() {
		shortRepo := resolveShortRepo(fullRepo)
		openPRs := ghListOpenPRs(fullRepo)
		for _, pr := range openPRs {
			if _, covered := coveredByBeacon[repoPR{repo: shortRepo, pr: pr.Number}]; covered {
				continue
			}
			unclaimed = append(unclaimed, pr)
		}
	}

	if len(unclaimed) > 0 {
		fmt.Println()
		fmt.Println("--- unclaimed (no session registered) ---")
		fmt.Printf("%-12s %-6s %-8s %s\n", "SESSION", "PR", "STATE", "TITLE (repo)")
		fmt.Printf("%-12s %-6s %-8s %s\n", "-------", "----", "------", "------------")
		// Sort by PR number (they will all be unclaimed, ordering doesn't matter as much).
		// Actually, let's sort by repo then pr.
		sort.Slice(unclaimed, func(i, j int) bool {
			return unclaimed[i].Number < unclaimed[j].Number
		})
		for _, pr := range unclaimed {
			state := "READY"
			if pr.IsDraft {
				state = "draft"
			}
			fmt.Printf("%-12s #%-5d %-8s %s\n", "(none)", pr.Number, state, pr.Title)
		}
	}

	return nil
}

func cmdMine(args []string) error {
	fs := flag.NewFlagSet("mine", flag.ContinueOnError)
	var session string
	fs.StringVar(&session, "session", "", "Session name (env: $DESK_SESSION or $CLAUDE_SESSION_ID)")

	if err := fs.Parse(args); err != nil {
		return deskkit.Refused(fmt.Sprintf("mine: %v", err))
	}

	sess, err := resolveSession(session)
	if err != nil {
		return err
	}

	b, err := loadBeacon(sess)
	if err != nil {
		return deskkit.Unverifiable("cannot load beacon", err)
	}

	if len(b.OpenWork) == 0 {
		fmt.Printf("session %s: no registered work\n", sess)
		if b.Role != "" {
			fmt.Printf("  role: %s\n", b.Role)
		}
		return nil
	}

	fmt.Printf("session %s", sess)
	if b.Role != "" {
		fmt.Printf(" (%s)", b.Role)
	}
	fmt.Printf(" — %d item(s), updated %s\n\n", len(b.OpenWork), b.Updated)

	fmt.Printf("%-6s %-8s %s\n", "PR", "STATE", "WHAT")
	fmt.Printf("%-6s %-8s %s\n", "----", "------", "----")

	for _, w := range b.OpenWork {
		fullRepo := resolveFullRepo(w.Repo)
		info := ghViewPR(fullRepo, w.PR)
		state := "?"
		if info != nil {
			if info.State == "OPEN" {
				if info.IsDraft {
					state = "draft"
				} else {
					state = "READY"
				}
			} else {
				state = info.State
			}
		}
		fmt.Printf("#%-5d %-8s %s (%s)\n", w.PR, state, w.What, w.Repo)
	}

	return nil
}
