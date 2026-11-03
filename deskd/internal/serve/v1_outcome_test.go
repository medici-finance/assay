package serve

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// TestOutcomeUnverifiedOnWire: an answered-unverified derivation (gap-3 item
// (5)) reaches the wire as `unverified: true`, attributed to the comment's
// real author with no option and no staleness claim — never as a verified
// answer a client or agent could act on. A verified answer omits the field.
func TestOutcomeUnverifiedOnWire(t *testing.T) {
	var row index.Row
	row.Closed = true
	row.Rev = "rev2"
	row.Answer = &domain.Answer{By: domain.Actor{Name: "gateway-app[bot]", IsBot: true}, At: time.Now(), Unverified: true}
	out := v1Outcome(row)
	if out == nil || !out.Unverified || out.AnsweredBy != "gateway-app[bot]" || out.Option != "" || out.Stale {
		t.Fatalf("outcome = %+v, want unverified by the bot, no option, not stale", out)
	}
	b, _ := json.Marshal(out)
	if !strings.Contains(string(b), `"unverified":true`) {
		t.Fatalf("wire outcome %s lacks unverified:true", b)
	}

	opt := domain.OptionID("opt-a")
	row.Answer = &domain.Answer{By: domain.Actor{Name: "operator"}, Option: &opt, OptionsRev: "rev2", At: time.Now()}
	b, _ = json.Marshal(v1Outcome(row))
	if strings.Contains(string(b), "unverified") {
		t.Fatalf("verified outcome %s carries unverified", b)
	}
}
