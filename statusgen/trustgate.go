package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Trust gate for --scan-issues (human:<name>, 2026-07-23; the scanner ships from THIS repo
// via the pinned statusgen release).
//
// The scanner is the surface that CREATES durable desk work items (placeholder
// files) from GitHub issues, so it must enforce the desk trust gate IN CODE, not
// by skill discipline: an open issue authored outside the trusted identities gets
// NO placeholder unless the configured blessing authority has commented on it, and
// the blessing is bless-then-edit aware — content added or edited after their
// latest comment voids it. Quarantined issues surface as NOTICEs (visible, never
// silently dropped); the live quarantine board is `issueboard`'s EXTERNAL /
// UNBLESSED lane.
//
// WHERE THE ROSTER COMES FROM. It used to be a compiled-in
// set naming one organisation's accounts. It is now adopter CONFIGURATION, read
// from outside every ref the tools evaluate — see rosterconfig.go, this tree's
// documented duplicate of the deskkit roster loader. An UNSET roster trusts
// nobody and blesses nobody (P1), which makes --scan-issues refuse rather than
// scan ungated.
//
// KEEP IN SYNC with the desk-tools deskkit trust reader — the desk-tools module
// deliberately does NOT share code with statusgen (the documented-duplicate
// pattern: issueboard's ownedRepos mirrors scanRepos the same way), so the trust
// reader, the GraphQL query, and the blessing rule below are a cross-tree
// duplicate of deskkit's. A change to either copy must be made in both, and a
// behavioural coupling test binds the two READERS over a shared vector file so
// the duplication cannot drift.

// trustedAuthor reports whether a login is a trusted desk identity. Login-only
// (gh issue list JSON carries no numeric id); empty/unknown → untrusted (fail
// closed), and so is EVERY login when the roster is unconfigured. Case-insensitive:
// GitHub logins are case-insensitively unique, so a case variant is the same
// account, never a second one.
//
// For the BOARD surface (planScan), this is the login-only check — the REST API
// (gh issue list --json author) does not return a numeric user id, so recycled-
// login defense for human accounts is deferred to the blessing path
// (evalIssueBlessing), which verifies numeric id via GraphQL. Bot accounts are
// trusted by login alone: GitHub Apps cannot be recycled.
func trustedAuthor(login string) bool {
	if login == "" {
		return false
	}
	c := scanEffectiveConfig()
	if !c.Configured() {
		return false
	}
	return c.Logins[strings.ToLower(login)]
}

// trustedAuthorID reports whether a login+id pair is a trusted desk identity,
// requiring NUMERIC ID match for human accounts as a recycled-login defense.
// Bot accounts (the [bot] suffix or app/ prefix) are trusted by login alone —
// GitHub Apps cannot be recycled. When id is zero (caller has no numeric id),
// human accounts fail closed (not trusted). This is the strict form used in
// the GraphQL blessing path (evalIssueBlessing) where numeric ids ARE available.
//
// A configured human identity with NO pinned id (the id is optional on
// ASSAY_TRUSTED_LOGINS) cannot be id-checked, so on this STRICT surface it is NOT
// trusted — never, and whatever id the caller carries. Login-only would be a match
// with no recycling defence, which is the one thing this surface exists to refuse.
// deskkit's twin agrees on this input: its general-purpose TrustedAuthorID falls
// back to login-only only when the SURFACE supplies no id at all (id == 0, the gh
// CLI list JSON), and once an id is present it answers `want != 0 && want == id`
// too. The BLESSING authority is never in that position: its id is mandatory at
// load.
func trustedAuthorID(login string, id int64) bool {
	if login == "" {
		return false
	}
	c := scanEffectiveConfig()
	if !c.Configured() {
		return false
	}
	lower := strings.ToLower(login)
	// Bot accounts: trusted by login alone (Apps cannot be recycled).
	if strings.HasSuffix(lower, "[bot]") || strings.HasPrefix(lower, "app/") {
		return c.Logins[lower]
	}
	// Human accounts: require numeric id match against the configured pin.
	want, ok := c.Humans[lower]
	if !ok {
		return false
	}
	// want == 0 (configured, never pinned) is NOT a match on this surface. This is
	// the STRICT path — the GraphQL databaseId is always present here — so an
	// unpinned login has no recycling defence and cannot be trusted content.
	//
	// It used to answer `id != 0`: an unpinned human was trusted carrying ANY id,
	// while deskkit's twin (post-review) answers false. That divergence survived
	// because the shared coupling vector listed `unpinned-human` in the roster and
	// then carried no CASE for it. It does now.
	// KEEP IN SYNC with deskkit's TrustedAuthorID / trustedContentAuthor.
	return want != 0 && id == want
}

// authorizedAuthorSet returns the effective AUTHORIZED-AUTHOR set for the
// scan-transcribe lane (R-7 clause 1): the rostered ASSAY_AUTHORIZED_AUTHORS
// humans, SEEDED with the blessing authority. Seeding is what makes an
// UNSET roster value degrade to {the blessing authority} rather than to an empty set — the lane
// always admits at least the blessing authority's own issues, and never crashes
// on absence. With the roster unconfigured the set is empty (fail closed): there
// is no bless identity to seed from and nothing to authorize.
//
// Returns login(lowercased) -> mandatory numeric id. An id of 0 never appears:
// both sources (the parsed set and the bless identity) require a positive id at
// load, so every member is recycled-login-defensible.
func authorizedAuthorSet() map[string]int64 {
	c := scanEffectiveConfig()
	set := map[string]int64{}
	if !c.Configured() {
		return set
	}
	for l, id := range c.AuthorizedAuthors {
		if id != 0 {
			set[strings.ToLower(l)] = id
		}
	}
	// Seed with the blessing authority. The bless id is mandatory at load, so this
	// is always a pinned entry.
	if c.Bless.Login != "" && c.Bless.ID != 0 {
		set[strings.ToLower(c.Bless.Login)] = c.Bless.ID
	}
	return set
}

// authorizedByIdentity is the R-7 clause-1 direct-author predicate: is this
// issue author authorized to BOARD work through the unattended scan-transcribe
// lane? Two disjoint identity classes, each id-pinned to the extent GitHub
// allows:
//
//   - a rostered desk App (type Bot, or a login that renders as one): trusted by
//     login alone — GitHub Apps cannot be recycled — but ONLY when the login is in
//     the rostered bot set (ASSAY_TRUSTED_BOT_SLUGS). The bare slug is never
//     trusted (scanParseConfig only registers the `slug[bot]` and `app/slug`
//     renderings), so a plain user squatting a slug stays untrusted.
//   - a human authorized author: the login MUST be in authorizedAuthorSet AND the
//     numeric id MUST match (recycled-login defense). A human carrying id 0 (the
//     resolver could not read it) fails closed.
//
// With the roster unconfigured every identity is unauthorized (authorizedAuthorSet
// empty, cfg.Logins empty).
func authorizedByIdentity(login string, id int64, typ string) bool {
	lower := strings.ToLower(strings.TrimSpace(login))
	if lower == "" {
		return false
	}
	c := scanEffectiveConfig()
	if !c.Configured() {
		return false
	}
	// Desk App identity: type Bot, or a login rendering as a bot/App. Trusted by
	// login alone, but only if it is a REGISTERED bot rendering.
	if typ == "Bot" || scanLooksLikeBot(lower) {
		return c.Logins[lower]
	}
	// Human authorized author: login in the set AND id-pinned match.
	want, ok := authorizedAuthorSet()[lower]
	if !ok {
		return false
	}
	return id != 0 && id == want
}

// isBlessAuthorityID is the id-aware blessing-authority check: the configured
// bless login carrying any other id is NOT the authority (fail closed). With the
// roster unconfigured there is no authority and this is false for every input.
func isBlessAuthorityID(login string, id int64) bool {
	c := scanEffectiveConfig()
	if !c.Configured() {
		return false
	}
	return strings.EqualFold(login, c.Bless.Login) && id != 0 && id == c.Bless.ID
}

// issueBlessChecker evaluates the blessing for one untrusted-author issue.
// The production implementation is ghIssueBlessChecker; tests inject a fixture.
type issueBlessChecker func(repo string, issue int) (blessed bool, err error)

// scanIssueTrustQuery — duplicate of deskkit.IssueTrustQuery (keep in sync). It
// needs GraphQL because the gate keys on lastEditedAt (CONTENT edits only — REST
// updated_at moves on labels and would re-quarantine spuriously) and databaseId
// (numeric identity, recycled-login defense). first:100 bounded, no cursor walk:
// an overflowing thread fails closed to quarantine.
const scanIssueTrustQuery = `query($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){issue(number:$number){lastEditedAt comments(first:100){pageInfo{hasNextPage} nodes{createdAt lastEditedAt author{login __typename ...on User{databaseId} ...on Bot{databaseId}}}}}}}`

// ghIssueBlessChecker runs the trust query via `gh api graphql` (a READ — the
// query carries no mutation) and applies the bless-then-edit rule.
func ghIssueBlessChecker(repo string, issue int) (bool, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return false, fmt.Errorf("bad repo %q", repo)
	}
	out, err := exec.Command("gh", "api", "graphql",
		"-f", "query="+scanIssueTrustQuery,
		"-f", "owner="+owner, "-f", "name="+name, "-F", "number="+strconv.Itoa(issue)).Output()
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return false, fmt.Errorf("gh api graphql trust query %s#%d: %v %s", repo, issue, err, detail)
	}
	return evalIssueBlessing(out)
}

// evalIssueBlessing parses a scanIssueTrustQuery response and applies the rule
// (duplicate of deskkit.Blessed for the issue shape — keep in sync):
//
//	blessed iff the configured blessing authority (login+databaseId) authored a
//	comment, the issue BODY was not edited after their latest comment, and no
//	untrusted comment was created or edited after it. Any overflow (hasNextPage),
//	parse problem, or UNCONFIGURED roster fails closed.
func evalIssueBlessing(raw []byte) (bool, error) {
	if err := scanRosterUnconfiguredError(); err != nil {
		// P1: no configured blessing authority ⇒ nothing can be blessed. This is a
		// hard error, not a quiet false, so the caller reports WHY the issue stayed
		// quarantined instead of silently treating every issue as unblessed.
		return false, err
	}
	var env struct {
		Data struct {
			Repository struct {
				Issue *struct {
					LastEditedAt string `json:"lastEditedAt"`
					Comments     struct {
						PageInfo struct {
							HasNextPage bool `json:"hasNextPage"`
						} `json:"pageInfo"`
						Nodes []struct {
							CreatedAt    string `json:"createdAt"`
							LastEditedAt string `json:"lastEditedAt"`
							Author       *struct {
								Login      string `json:"login"`
								Typename   string `json:"__typename"`
								DatabaseID int64  `json:"databaseId"`
							} `json:"author"`
						} `json:"nodes"`
					} `json:"comments"`
				} `json:"issue"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return false, fmt.Errorf("parsing trust query response: %w", err)
	}
	if len(env.Errors) > 0 {
		return false, fmt.Errorf("trust query GraphQL error: %s", env.Errors[0].Message)
	}
	iss := env.Data.Repository.Issue
	if iss == nil {
		return false, fmt.Errorf("trust query returned no issue")
	}
	if iss.Comments.PageInfo.HasNextPage {
		return false, nil // >100 comments — fail closed to quarantine
	}
	parse := func(s string) (time.Time, error) {
		if s == "" {
			return time.Time{}, nil
		}
		return time.Parse(time.RFC3339, s)
	}
	bodyEdited, err := parse(iss.LastEditedAt)
	if err != nil {
		return false, fmt.Errorf("bad lastEditedAt: %w", err)
	}

	type ev struct {
		login   string
		id      int64
		created time.Time
		edited  time.Time
	}
	var events []ev
	for _, n := range iss.Comments.Nodes {
		created, cerr := parse(n.CreatedAt)
		if cerr != nil {
			return false, fmt.Errorf("bad comment createdAt: %w", cerr)
		}
		edited, eerr := parse(n.LastEditedAt)
		if eerr != nil {
			return false, fmt.Errorf("bad comment lastEditedAt: %w", eerr)
		}
		e := ev{created: created, edited: edited}
		if n.Author != nil {
			e.login = n.Author.Login
			e.id = n.Author.DatabaseID
			if n.Author.Typename == "Bot" {
				// GraphQL Bot actors carry the BARE slug; re-suffix to the REST
				// rendering so a plain User squatting a slug stays untrusted.
				e.login += "[bot]"
			}
		}
		events = append(events, e)
	}

	var bless time.Time
	for _, e := range events {
		// T10: the selection is STRICT (login + numeric id) — trustedAuthorID (used
		// in the untrusted-content loop below) returns false for a human login with
		// id==0, so an id-less blessing comment accepted here would then void itself
		// as untrusted content. Keeping the selection strict avoids that
		// contradiction. The configured bless id is mandatory at load, so there is
		// never a "configured without a pin" case to fall back to here.
		if isBlessAuthorityID(e.login, e.id) && e.created.After(bless) {
			bless = e.created
		}
	}
	if bless.IsZero() {
		return false, nil
	}
	// T3: use !Before rather than After so a tie (same-second edit) voids
	// the blessing — fail-closed on ties.
	if !bodyEdited.Before(bless) {
		return false, nil // body edited at/after the blessing — void
	}
	for _, e := range events {
		if trustedAuthorID(e.login, e.id) {
			continue // trusted content (numeric-id-verified) is trusted in its own right
		}
		last := e.created
		if e.edited.After(last) {
			last = e.edited
		}
		// T3: tie voids the blessing — fail-closed on same-second edits.
		if !last.Before(bless) {
			return false, nil // untrusted content added/edited at/after the blessing
		}
	}
	return true, nil
}
