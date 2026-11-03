package index

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"

	_ "modernc.org/sqlite" // pure-Go driver: no cgo, so the CI image needs no toolchain
)

// DB is the derived index plus the local attention store.
type DB struct {
	sql *sql.DB
}

// Open opens (creating if needed) the index at path.
func Open(path string) (*DB, error) {
	h, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := h.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		h.Close()
		return nil, err
	}
	if err := applySchema(h); err != nil {
		h.Close()
		return nil, err
	}
	return &DB{sql: h}, nil
}

// Close closes the underlying handle.
func (d *DB) Close() error { return d.sql.Close() }

// DropDerived implements "delete the derived index cache" (the read surface Verify 7).
//
// It iterates [derivedTables] and nothing else, so it CANNOT drop a local
// table — the pure-cache invariant is enforced by what this function is able
// to reach, not by a comment asking callers to be careful. After it runs, a
// poll must reproduce every derived row; the local read/ack rows are still
// there, which is the proof that the index is a cache and not a database.
func (d *DB) DropDerived() error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range derivedTables {
		if strings.HasPrefix(t, "local_") {
			// Unreachable by construction; kept as a tripwire in case the
			// list is ever edited carelessly.
			return fmt.Errorf("refusing to drop %q: DropDerived may not touch local state (ruling D7)", t)
		}
		if _, err := tx.Exec("DROP TABLE IF EXISTS " + t); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return applySchema(d.sql)
}

// crockford is the ULID alphabet (Crockford base32, no I/L/O/U).
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// newULID mints a lexicographically sortable id: 48-bit big-endian
// millisecond timestamp followed by 80 bits of randomness, base32-encoded.
func newULID(t time.Time) (domain.EscalationID, error) {
	var b [16]byte
	ms := uint64(t.UTC().UnixMilli())
	for i := range 6 {
		b[i] = byte(ms >> (8 * (5 - i)))
	}
	if _, err := rand.Read(b[6:]); err != nil {
		return "", err
	}
	var out [26]byte
	// 128 bits into 26 base32 chars (130 bits of space; the top 2 are 0).
	var acc, bits uint32
	pos := 25
	for i := 15; i >= 0; i-- {
		acc |= uint32(b[i]) << bits
		bits += 8
		for bits >= 5 {
			out[pos] = crockford[acc&31]
			pos--
			acc >>= 5
			bits -= 5
		}
	}
	for pos >= 0 {
		out[pos] = crockford[acc&31]
		pos--
		acc >>= 5
	}
	return domain.EscalationID(out[:]), nil
}

// MintID resolves a platform ref to a stable domain id, minting on first
// ingest (spec §1.5). The mapping lives in `local_id_map`, so re-polls and
// warm restarts return the same id.
func (d *DB) MintID(repo adapter.Repo, key int) domain.EscalationID {
	var id string
	err := d.sql.QueryRow(`SELECT escalation_id FROM local_id_map WHERE repo = ? AND platform_key = ?`,
		repo.String(), key).Scan(&id)
	if err == nil {
		return domain.EscalationID(id)
	}
	minted, mErr := newULID(time.Now())
	if mErr != nil {
		// A failure of crypto/rand is not something to paper over with a
		// predictable id; surface it as an id that is obviously not one.
		return domain.EscalationID("")
	}
	_, _ = d.sql.Exec(`INSERT OR IGNORE INTO local_id_map (repo, platform_key, escalation_id, minted_at) VALUES (?,?,?,?)`,
		repo.String(), key, string(minted), time.Now().UTC().Format(time.RFC3339Nano))
	// Re-read: another goroutine may have won the insert.
	if err := d.sql.QueryRow(`SELECT escalation_id FROM local_id_map WHERE repo = ? AND platform_key = ?`,
		repo.String(), key).Scan(&id); err == nil {
		return domain.EscalationID(id)
	}
	return minted
}

// Row is a stored escalation plus the serving-edge presentation data the
// domain deliberately does not carry.
type Row struct {
	domain.Escalation
	// Links is presentation data attached at the SERVING EDGE (spec §1.5).
	// The platform deep-link is never a domain field, which is why it lives
	// on this wrapper and not on domain.Escalation.
	Links map[string]string `json:"links"`
	// AgeSeconds derives from FiledAt at serve time.
	AgeSeconds int64 `json:"age_seconds"`
	// Attention is the row's derived attention state (spec §1.2), rendered as
	// glyph + word so a consumer never encodes it as colour alone (§2.6). It
	// is nil (JSON null) for an unattributable row, whose automation — and so
	// whose state — is underivable, exactly as its automation renders null.
	Attention *domain.AttentionRender `json:"attention"`
	// Closed is the platform CLOSED state (the service contract increment 3): a
	// serving-edge fact, not a domain one — domain.Escalation deliberately
	// carries no such field (v1's read surface has historically read the OPEN
	// inbox only, treating "closed" as absence). It exists on Row so a caller
	// that DOES read with includeClosed=true (the /v1/escalations/{id} item
	// view, task 6) can tell a served closed row apart from an open one and
	// decide whether to attach its derived Outcome.
	Closed bool `json:"closed"`
}

// MarshalJSON merges the domain escalation's own encoding with the
// serving-edge fields.
//
// It has to be written by hand: domain.Escalation defines MarshalJSON (to
// render underivable fields as explicit nulls), and Go promotes an embedded
// type's marshaller to the outer struct — so without this method, Row would
// silently serialise as a bare Escalation and DROP `links` and
// `age_seconds`. That is not a hypothetical: the end-to-end test caught it
// doing exactly that, which is why the assertion "every served row carries a
// platform deep-link" is in the suite rather than assumed.
func (r Row) MarshalJSON() ([]byte, error) {
	inner, err := r.Escalation.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(inner, &merged); err != nil {
		return nil, err
	}
	links, err := json.Marshal(r.Links)
	if err != nil {
		return nil, err
	}
	merged["links"] = links
	merged["age_seconds"] = json.RawMessage(strconv.FormatInt(r.AgeSeconds, 10))
	attention, err := json.Marshal(r.Attention)
	if err != nil {
		return nil, err
	}
	merged["attention"] = attention
	closed, err := json.Marshal(r.Closed)
	if err != nil {
		return nil, err
	}
	merged["closed"] = closed
	return json.Marshal(merged)
}

// UnmarshalJSON restores a Row, including the serving-edge fields the
// embedded type does not know about.
func (r *Row) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &r.Escalation); err != nil {
		return err
	}
	var edge struct {
		Links      map[string]string       `json:"links"`
		AgeSeconds int64                   `json:"age_seconds"`
		Attention  *domain.AttentionRender `json:"attention"`
		Closed     bool                    `json:"closed"`
	}
	if err := json.Unmarshal(b, &edge); err != nil {
		return err
	}
	r.Links, r.AgeSeconds, r.Attention, r.Closed = edge.Links, edge.AgeSeconds, edge.Attention, edge.Closed
	return nil
}

// UpsertEscalations replaces the derived rows for one repo.
//
// Replacement is scoped to the repo so an unreachable repo's rows are LEFT
// ALONE rather than deleted. Deleting them would turn "we could not reach
// this repo" into "this repo has no escalations" — the exact zero-for-unknown
// substitution this index exists to avoid. The repo's health row records the
// could-not-check, and the CLI/API say the rows are stale.
func (d *DB) UpsertEscalations(repo adapter.Repo, escs []domain.Escalation, raw map[domain.EscalationID]adapter.RawIssue) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM derived_options WHERE escalation_id IN
	    (SELECT id FROM derived_escalations WHERE repo = ?)`, repo.String()); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM derived_escalations WHERE repo = ?`, repo.String()); err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, e := range escs {
		fc, err := json.Marshal(e.Fields)
		if err != nil {
			return err
		}
		var acell, arole any
		if !e.Automation.Zero() {
			acell, arole = string(e.Automation.Cell), string(e.Automation.Role)
		}
		var updated any
		if e.Updated != nil {
			updated = e.Updated.UTC().Format(time.RFC3339Nano)
		}
		ri := raw[e.ID]
		optionsPresent := 0
		if e.Options != nil {
			optionsPresent = 1
		}
		aLogin, aIsBot, aOption, aRev, aAt := answerColumns(e)
		if _, err := tx.Exec(`INSERT INTO derived_escalations
		    (id, cell, repo, platform_key, platform_url, automation_cell, automation_role, kind, provenance,
		     urgency, title, situation, rationale, raw_body, options_rev, options_present, collapse_key,
		     repeat, filed_at, updated_at, closed, answer_login, answer_is_bot, answer_option, answer_options_rev, answer_at,
		     answer_unverified, field_confidence, first_seen_at, last_seen_at)
		    VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			string(e.ID), string(e.Cell), repo.String(), ri.Number, ri.URL, acell, arole,
			string(e.Kind), string(e.Provenance), int(e.Urgency), e.Title, e.Situation, e.Rationale,
			e.RawBody, e.Rev, optionsPresent, e.Key().String(), e.Repeat,
			e.FiledAt.UTC().Format(time.RFC3339Nano), updated, boolInt(ri.Closed),
			aLogin, aIsBot, aOption, aRev, aAt, boolInt(e.Answer != nil && e.Answer.Unverified), string(fc), now, now,
		); err != nil {
			return err
		}
		for i, o := range e.Options {
			if _, err := tx.Exec(`INSERT INTO derived_options (escalation_id, position, option_id, label, consequence)
			    VALUES (?,?,?,?,?)`, string(e.ID), i, string(o.ID), o.Label, o.Consequence); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// answerColumns projects a derived [domain.Answer] onto the five answer_*
// storage columns. answer_at is nil (SQL NULL) whenever e.Answer is nil — that
// NULL is the "no derived answer" flag [Inbox] reads back, never a zero time
// that would misread as an answer at the epoch.
func answerColumns(e domain.Escalation) (login any, isBot int, option any, optionsRev string, at any) {
	if e.Answer == nil {
		return nil, 0, nil, "", nil
	}
	a := e.Answer
	if a.By.Name != "" {
		login = a.By.Name
	}
	if a.By.IsBot {
		isBot = 1
	}
	if a.Option != nil && *a.Option != "" {
		option = string(*a.Option)
	}
	optionsRev = a.OptionsRev
	if !a.At.IsZero() {
		at = a.At.UTC().Format(time.RFC3339Nano)
	}
	return login, isBot, option, optionsRev, at
}

// UpsertOne updates or inserts a SINGLE escalation row without touching any
// other row of its repo (the service contract increment 3: the storage side of the
// refresh nudge, POST /v1/escalations/{id}/refresh). [UpsertEscalations]
// replaces a whole repo's slice on every poll, which is correct for a full
// cycle but wrong for a targeted one-issue re-derive — that would wipe every
// sibling escalation of the same repo. This is an INSERT ... ON CONFLICT
// UPDATE keyed on id, exactly as if this one row had been the only member of
// its repo's slice this cycle.
func (d *DB) UpsertOne(repo adapter.Repo, e domain.Escalation, ri adapter.RawIssue) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	fc, err := json.Marshal(e.Fields)
	if err != nil {
		return err
	}
	var acell, arole any
	if !e.Automation.Zero() {
		acell, arole = string(e.Automation.Cell), string(e.Automation.Role)
	}
	var updated any
	if e.Updated != nil {
		updated = e.Updated.UTC().Format(time.RFC3339Nano)
	}
	optionsPresent := 0
	if e.Options != nil {
		optionsPresent = 1
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	aLogin, aIsBot, aOption, aRev, aAt := answerColumns(e)

	if _, err := tx.Exec(`INSERT INTO derived_escalations
	    (id, cell, repo, platform_key, platform_url, automation_cell, automation_role, kind, provenance,
	     urgency, title, situation, rationale, raw_body, options_rev, options_present, collapse_key,
	     repeat, filed_at, updated_at, closed, answer_login, answer_is_bot, answer_option, answer_options_rev, answer_at,
	     answer_unverified, field_confidence, first_seen_at, last_seen_at)
	    VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	    ON CONFLICT(id) DO UPDATE SET
	        cell=excluded.cell, repo=excluded.repo, platform_key=excluded.platform_key, platform_url=excluded.platform_url,
	        automation_cell=excluded.automation_cell, automation_role=excluded.automation_role, kind=excluded.kind,
	        provenance=excluded.provenance, urgency=excluded.urgency, title=excluded.title, situation=excluded.situation,
	        rationale=excluded.rationale, raw_body=excluded.raw_body, options_rev=excluded.options_rev,
	        options_present=excluded.options_present, collapse_key=excluded.collapse_key, repeat=excluded.repeat,
	        filed_at=excluded.filed_at, updated_at=excluded.updated_at, closed=excluded.closed,
	        answer_login=excluded.answer_login, answer_is_bot=excluded.answer_is_bot, answer_option=excluded.answer_option,
	        answer_options_rev=excluded.answer_options_rev, answer_at=excluded.answer_at,
	        answer_unverified=excluded.answer_unverified,
	        field_confidence=excluded.field_confidence, last_seen_at=excluded.last_seen_at`,
		string(e.ID), string(e.Cell), repo.String(), ri.Number, ri.URL, acell, arole,
		string(e.Kind), string(e.Provenance), int(e.Urgency), e.Title, e.Situation, e.Rationale,
		e.RawBody, e.Rev, optionsPresent, e.Key().String(), e.Repeat,
		e.FiledAt.UTC().Format(time.RFC3339Nano), updated, boolInt(ri.Closed),
		aLogin, aIsBot, aOption, aRev, aAt, boolInt(e.Answer != nil && e.Answer.Unverified), string(fc), now, now,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM derived_options WHERE escalation_id = ?`, string(e.ID)); err != nil {
		return err
	}
	for i, o := range e.Options {
		if _, err := tx.Exec(`INSERT INTO derived_options (escalation_id, position, option_id, label, consequence)
		    VALUES (?,?,?,?,?)`, string(e.ID), i, string(o.ID), o.Label, o.Consequence); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RecordHealth stores one repo's three-state verdict.
func (d *DB) RecordHealth(h adapter.RepoHealth) error {
	probe, err := json.Marshal(h.Assessment.Probe)
	if err != nil {
		return err
	}
	_, err = d.sql.Exec(`INSERT INTO derived_repo_health (repo, confidence, reason, probe, not_modified, checked_at)
	    VALUES (?,?,?,?,?,?)
	    ON CONFLICT(repo) DO UPDATE SET confidence=excluded.confidence, reason=excluded.reason,
	        probe=excluded.probe, not_modified=excluded.not_modified, checked_at=excluded.checked_at`,
		h.Repo, string(h.Assessment.Confidence), h.Assessment.Reason, string(probe),
		boolInt(h.NotModified), h.CheckedAt.UTC().Format(time.RFC3339Nano))
	return err
}

// Health returns every repo's verdict.
func (d *DB) Health() ([]adapter.RepoHealth, error) {
	rows, err := d.sql.Query(`SELECT repo, confidence, reason, probe, not_modified, checked_at
	    FROM derived_repo_health ORDER BY repo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []adapter.RepoHealth
	for rows.Next() {
		var h adapter.RepoHealth
		var conf, reason, probe, checked string
		var nm int
		if err := rows.Scan(&h.Repo, &conf, &reason, &probe, &nm, &checked); err != nil {
			return nil, err
		}
		h.Assessment.Confidence = domain.Confidence(conf)
		h.Assessment.Reason = reason
		_ = json.Unmarshal([]byte(probe), &h.Assessment.Probe)
		h.NotModified = nm == 1
		h.CheckedAt, _ = time.Parse(time.RFC3339Nano, checked)
		out = append(out, h)
	}
	return out, rows.Err()
}

// MarkPoll records the start/end of a poll cycle. IndexAge reads it.
func (d *DB) MarkPoll(started, ended time.Time, outcome string) error {
	_, err := d.sql.Exec(`INSERT INTO derived_poll (k, started_at, ended_at, outcome) VALUES ('last',?,?,?)
	    ON CONFLICT(k) DO UPDATE SET started_at=excluded.started_at, ended_at=excluded.ended_at, outcome=excluded.outcome`,
		started.UTC().Format(time.RFC3339Nano), ended.UTC().Format(time.RFC3339Nano), outcome)
	return err
}

// IndexAge returns how old the index is, and whether that is even knowable.
//
// A COLD index — one that has never completed a poll — returns
// could-not-check, NOT a zero age. Zero age on a cold index reads as "fresh",
// which is the most dangerous thing an empty board can say.
func (d *DB) IndexAge(now time.Time) (time.Duration, domain.Assessment) {
	p := domain.Probe{
		Source: "derived_poll.ended_at — the completion time of the last poll cycle",
		Method: domain.MethodLocalTable,
		Window: "the most recent completed poll",
		Limit:  0,
	}
	var ended, outcome string
	err := d.sql.QueryRow(`SELECT ended_at, outcome FROM derived_poll WHERE k = 'last'`).Scan(&ended, &outcome)
	if err != nil {
		return 0, domain.Unchecked(p, "the index has never completed a poll — its age is unknown, which is NOT the same as zero")
	}
	t, perr := time.Parse(time.RFC3339Nano, ended)
	if perr != nil {
		return 0, domain.Unchecked(p, "the recorded poll time did not parse: "+perr.Error())
	}
	if outcome != "ok" {
		return now.Sub(t), domain.Failed(p, "the last poll cycle ended with: "+outcome+
			" — the index is as old as its last SUCCESSFUL slice, per repo; see repo health")
	}
	return now.Sub(t), domain.Clean(p)
}

// Inbox returns the escalation rows for a cell, urgency-then-age.
//
// It reads ONLY the derived tables and the local attention table. It does
// not call GitHub — a serving path that can reach the network can be slow,
// can be rate-limited, and can return a different answer than the index it
// claims to serve.
func (d *DB) Inbox(cell string, includeClosed bool) ([]Row, error) {
	q := `SELECT e.id, e.cell, e.platform_url, e.automation_cell, e.automation_role, e.kind, e.provenance,
	         e.urgency, e.title, e.situation, e.rationale, e.raw_body, e.options_rev, e.options_present,
	         e.repeat, e.filed_at, e.updated_at, e.closed,
	         e.answer_login, e.answer_is_bot, e.answer_option, e.answer_options_rev, e.answer_at, e.answer_unverified,
	         e.field_confidence,
	         (SELECT MIN(read_at) FROM local_attention a WHERE a.escalation_id = e.id),
	         (SELECT MIN(acked_at) FROM local_attention a WHERE a.escalation_id = e.id)
	      FROM derived_escalations e WHERE (? = '' OR e.cell = ?)`
	if !includeClosed {
		q += ` AND e.closed = 0`
	}
	rows, err := d.sql.Query(q, cell, cell)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Row
	now := time.Now().UTC()
	for rows.Next() {
		var (
			r                              Row
			id, cellName, url, kind, prov  string
			acell, arole, updated          sql.NullString
			readAt, ackedAt                sql.NullString
			title, situation, rat, rawBody string
			rev                            string
			optionsPresent, urg, repeat    int
			filed, fc                      string
			closedInt, answerIsBot         int
			answerUnverified               int
			answerLogin, answerOption      sql.NullString
			answerRev                      string
			answerAt                       sql.NullString
		)
		if err := rows.Scan(&id, &cellName, &url, &acell, &arole, &kind, &prov, &urg, &title,
			&situation, &rat, &rawBody, &rev, &optionsPresent, &repeat, &filed, &updated, &closedInt,
			&answerLogin, &answerIsBot, &answerOption, &answerRev, &answerAt, &answerUnverified,
			&fc, &readAt, &ackedAt); err != nil {
			return nil, err
		}
		r.ID = domain.EscalationID(id)
		r.Cell = domain.CellName(cellName)
		if acell.Valid && arole.Valid {
			r.Automation = domain.AutomationRef{Cell: domain.CellName(acell.String), Role: domain.Role(arole.String)}
		}
		r.Kind = domain.EscalationKind(kind)
		r.Provenance = domain.Provenance(prov)
		r.Urgency = domain.Urgency(urg)
		r.Title, r.Situation, r.Rationale, r.RawBody, r.Rev = title, situation, rat, rawBody, rev
		r.Repeat = repeat
		r.FiledAt, _ = time.Parse(time.RFC3339Nano, filed)
		r.Updated = parseNullTime(updated)
		r.Read = parseNullTime(readAt)
		r.Acked = parseNullTime(ackedAt)
		r.Closed = closedInt != 0
		// answer_at NULL is the "no derived answer" flag (the service contract
		// increment 3): a present-but-empty answer would otherwise be
		// indistinguishable from "never answered" once round-tripped through
		// nullable text columns.
		if at := parseNullTime(answerAt); at != nil {
			ans := &domain.Answer{OptionsRev: answerRev, At: *at, By: domain.Actor{IsBot: answerIsBot != 0}, Unverified: answerUnverified != 0}
			if answerLogin.Valid {
				ans.By.Name = answerLogin.String
			}
			if answerOption.Valid && answerOption.String != "" {
				opt := domain.OptionID(answerOption.String)
				ans.Option = &opt
			}
			r.Answer = ans
		}
		if err := json.Unmarshal([]byte(fc), &r.Fields); err != nil {
			return nil, fmt.Errorf("stored field_confidence for %s did not decode: %w", id, err)
		}
		if optionsPresent == 1 {
			opts, err := d.options(id)
			if err != nil {
				return nil, err
			}
			r.Options = opts
		}
		r.Links = map[string]string{"platform": url}
		r.AgeSeconds = int64(now.Sub(r.FiledAt).Seconds())
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Collapse fold (spec §1.1.3): fold same-key same-guard OPEN filings into
	// one row with a repeat count before anything else sees them. A changed
	// option set is a different decision and stays separate.
	out = foldCollapse(out, now)

	// Per-automation open-escalation counts, over the FOLDED set, feed the
	// waiting predicate (spec §1.2). A folded group is one open escalation for
	// its automation; counting pre-fold would double-count a retrying loop.
	openByAutomation := map[string]int{}
	for _, r := range out {
		// A closed row is decided, not open — it must not inflate the
		// "≥1 open escalation" waiting predicate for its automation. This only
		// matters once a caller reads with includeClosed=true (the service contract
		// increment 3): before Row carried Closed, every row reaching this
		// loop WAS open by construction (the default query already filtered
		// closed=0), so the check was unreachable dead weight; it is live now.
		if r.Attributable() && !r.Closed {
			openByAutomation[r.Automation.String()]++
		}
	}
	for i := range out {
		out[i].Attention = attentionFor(out[i], openByAutomation, now)
	}

	escs := make([]domain.Escalation, len(out))
	for i := range out {
		escs[i] = out[i].Escalation
	}
	domain.SortInbox(escs)
	byID := map[domain.EscalationID]Row{}
	for _, r := range out {
		byID[r.ID] = r
	}
	sorted := make([]Row, 0, len(out))
	for _, e := range escs {
		sorted = append(sorted, byID[e.ID])
	}
	return sorted, nil
}

func (d *DB) options(escID string) ([]domain.Option, error) {
	rows, err := d.sql.Query(`SELECT option_id, label, consequence FROM derived_options
	    WHERE escalation_id = ? ORDER BY position`, escID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// Non-nil zero-length: an escalation flagged options_present had a
	// parsed v2 block, so an empty list here is a storage bug, not the
	// prose-v1 nil case. Keeping the two distinguishable matters — nil
	// means "no typed options exist", [] would mean "there are zero of them".
	opts := []domain.Option{}
	for rows.Next() {
		var o domain.Option
		var id string
		if err := rows.Scan(&id, &o.Label, &o.Consequence); err != nil {
			return nil, err
		}
		o.ID = domain.OptionID(id)
		opts = append(opts, o)
	}
	return opts, rows.Err()
}

func parseNullTime(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, s.String)
	if err != nil {
		return nil
	}
	return &t
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
