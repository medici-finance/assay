package index

import (
	"time"

	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// Test-only accessors. They live in an _test.go file so the shipped API has
// no raw-SQL escape hatch on it: nothing outside a test can reach the
// handle, which is what keeps the read/ack setters genuinely absent from
// this build rather than merely unused.

// ExecForTest runs a statement against the index.
func (d *DB) ExecForTest(query string, args ...any) error {
	_, err := d.sql.Exec(query, args...)
	return err
}

// QueryIntForTest runs a single-value integer query.
func (d *DB) QueryIntForTest(query string, args ...any) (int, error) {
	var n int
	err := d.sql.QueryRow(query, args...).Scan(&n)
	return n, err
}

// PublishForTest drives the producer half of one cycle directly, so a test
// can hand it the exact snapshot a could-not-check BEFORE read leaves behind
// (nil maps, zero badge) without having to make the index's own SQL fail
// between the before- and after-reads of a real PollOnce.
func (p *Poller) PublishForTest(before map[domain.EscalationID]string, beforeBadge contract.BadgeData, beforeHealth []string, at time.Time) {
	p.publishTransitions(before, beforeBadge, beforeHealth, at)
}
