package main

import (
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
type Beacon struct {
	Session  string      `json:"session"`
	Role     string      `json:"role,omitempty"`
	Updated  string      `json:"updated"`
	OpenWork []WorkEntry `json:"open_work,omitempty"`
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

func loadBeacon(session string) (*Beacon, error) {
	path, err := beaconPath(session)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Beacon{Session: session}, nil
		}
		return nil, fmt.Errorf("cannot read beacon %s: %w", path, err)
	}
	var b Beacon
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("malformed beacon %s: %w", path, err)
	}
	if b.OpenWork == nil {
		b.OpenWork = []WorkEntry{}
	}
	return &b, nil
}

func saveBeacon(b *Beacon) error {
	dir, err := rosterDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cannot create roster dir: %w", err)
	}
	b.Updated = time.Now().UTC().Format(time.RFC3339)
	path, err := beaconPath(b.Session)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal beacon: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("cannot write beacon: %w", err)
	}
	return nil
}

// removeBeaconFile removes the beacon file if it has no work and no role (clean up empty beacons).
func removeBeaconFile(session string) error {
	path, err := beaconPath(session)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove beacon %s: %w", path, err)
	}
	return nil
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

// ---- gh helpers ----

// ghViewPR calls `gh pr view` for a given PR and returns its state info.
// Returns nil if gh fails (e.g. PR not found, network error).
func ghViewPR(fullRepo string, pr int) *PRInfo {
	prStr := strconv.Itoa(pr)
	cmd := exec.Command("gh", "pr", "view", prStr, "--repo", fullRepo,
		"--json", "state,isDraft,title")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var info PRInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil
	}
	info.Number = pr
	return &info
}

// ghListOpenPRs calls `gh pr list` for a repo and returns open PRs.
// Returns nil on failure (gh not available, network error, etc.).
func ghListOpenPRs(fullRepo string) []PRInfo {
	cmd := exec.Command("gh", "pr", "list", "--repo", fullRepo,
		"--state", "open", "--json", "number,title,isDraft", "--limit", "50")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var prs []PRInfo
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil
	}
	return prs
}

// ---- commands ----

func cmdSet(args []string) error {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	var (
		repo    string
		pr      int
		what    string
		role    string
		session string
	)
	fs.StringVar(&repo, "repo", "", "Short repo name (e.g. tracker)")
	fs.IntVar(&pr, "pr", 0, "PR number")
	fs.StringVar(&what, "what", "", "Description of the work")
	fs.StringVar(&role, "role", "", "Optional role label for this session")
	fs.StringVar(&session, "session", "", "Session name (env: $DESK_SESSION or $CLAUDE_SESSION_ID)")

	if err := fs.Parse(args); err != nil {
		return deskkit.Refused(fmt.Sprintf("set: %v", err))
	}

	// Role-only set: --role without --repo/--pr/--what updates just the role.
	roleOnly := (role != "" && repo == "" && pr == 0 && what == "")
	// Work-entry set: --repo, --pr, and --what are all required.
	workSet := (repo != "" && pr != 0 && what != "")

	if !roleOnly && !workSet {
		return deskkit.Refused("set requires either (--repo, --pr, --what) for a work entry, or --role alone for a standing session")
	}

	sess, err := resolveSession(session)
	if err != nil {
		return err
	}

	b, err := loadBeacon(sess)
	if err != nil {
		return deskkit.Unverifiable("cannot load beacon", err)
	}

	// Set role if provided.
	if role != "" {
		b.Role = role
	}

	// Upsert the work entry (idempotent on (repo,pr)).
	if workSet {
		found := false
		for i, w := range b.OpenWork {
			if w.Repo == repo && w.PR == pr {
				b.OpenWork[i].What = what
				found = true
				break
			}
		}
		if !found {
			b.OpenWork = append(b.OpenWork, WorkEntry{Repo: repo, PR: pr, What: what})
		}
	}

	if err := saveBeacon(b); err != nil {
		return deskkit.Unverifiable("cannot save beacon", err)
	}

	if workSet {
		fmt.Printf("deskroster: %s registered PR #%d (%s) as %q\n", sess, pr, repo, what)
	} else {
		fmt.Printf("deskroster: %s set role %q\n", sess, role)
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

	b, err := loadBeacon(sess)
	if err != nil {
		return deskkit.Unverifiable("cannot load beacon", err)
	}

	// Remove the entry.
	filtered := b.OpenWork[:0]
	found := false
	for _, w := range b.OpenWork {
		if w.Repo == repo && w.PR == pr {
			found = true
			continue
		}
		filtered = append(filtered, w)
	}
	b.OpenWork = filtered

	if !found {
		fmt.Printf("deskroster: %s has no entry for PR #%d (%s) — nothing to drop\n", sess, pr, repo)
		return nil
	}

	// If no work left and no role, remove the file entirely.
	if len(b.OpenWork) == 0 && b.Role == "" {
		if err := removeBeaconFile(sess); err != nil {
			return deskkit.Unverifiable("cannot remove beacon file", err)
		}
		fmt.Printf("deskroster: dropped PR #%d (%s) from %s (beacon removed — no remaining work)\n", pr, repo, sess)
		return nil
	}

	if err := saveBeacon(b); err != nil {
		return deskkit.Unverifiable("cannot save beacon", err)
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
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var b Beacon
			if err := json.Unmarshal(data, &b); err != nil {
				continue
			}
			if b.OpenWork == nil {
				b.OpenWork = []WorkEntry{}
			}
			beacons = append(beacons, b)
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
	modifiedBeacons := make(map[string]*Beacon) // session -> potentially modified beacon

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
				continue
			}
			surviving = append(surviving, w)
			coveredByBeacon[repoPR{repo: w.Repo, pr: w.PR}] = b.Session
		}

		if changed {
			b.OpenWork = surviving
			modifiedBeacons[b.Session] = b
		}
	}

	// Flush modified beacons (auto-pruned from merged/closed).
	for _, b := range modifiedBeacons {
		if len(b.OpenWork) == 0 && b.Role == "" {
			_ = removeBeaconFile(b.Session)
		} else {
			_ = saveBeacon(b)
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
