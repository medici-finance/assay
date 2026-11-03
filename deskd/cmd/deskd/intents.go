package main

import (
	"crypto/ed25519"
	"fmt"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/config/intentkeys"
	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/answer"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// The signed-intent write path's opt-in environment (the authorization contract gap 3,
// ruling 2). The token FILE holds the BOT gateway credential a verified
// human-signed intent's write runs under — a GitHub App installation token
// (ghs_…), re-read on EVERY write so the minter that rotates it hourly (a
// sidecar rewriting the file, the --tokens-dir contract) needs no restart;
// deskd never holds one token for its lifetime, and a user-bound token shape is
// refused (answer.CheckGatewayToken). The login is the bot identity it authors
// as. The credential is SEPARATE from every other deskd token by design: the
// read token cannot write, the desk App token files as the desk, and the
// write-issues token files record PRs — none of them is the gateway. Unset, no
// signed intent is honoured.
const (
	intentGatewayTokenFileEnv = "DESKD_INTENT_GATEWAY_TOKEN_FILE"
	intentGatewayLoginEnv     = "DESKD_INTENT_GATEWAY_LOGIN"
)

// intentWiring is what [buildIntentWiring] decided at startup: a verifier and
// the gateway credential to hand the writer, or neither — never one without
// the other — plus the banner line naming which.
type intentWiring struct {
	Verifier *answer.IntentVerifier
	Gateway  *answer.GatewayCredential
	Banner   string
}

// buildIntentWiring loads the reviewed intent-keys file and the gateway
// environment and decides whether the signed-intent path is on. It is on ONLY
// when BOTH hold: the file enrols at least one key (fail-closed load:
// missing / empty / unparsable / invalid ⇒ no key), and the gateway credential
// builds (a bot login + a token file that reads, at startup, as an installation
// token). Either missing ⇒ Verifier nil, so
// every signed intent is refused with 400 exactly as before gap 3, and the
// middleware stand-in admits nothing.
func buildIntentWiring(path string, getenv func(string) string, now func() time.Time) intentWiring {
	ring, reason := intentkeys.LoadOrEmpty(path)
	if ring.Len() == 0 {
		return intentWiring{Banner: fmt.Sprintf("deskd: signed intents OFF — %s; every signed intent is refused (the authorization contract)", reason)}
	}
	tokenFile := strings.TrimSpace(getenv(intentGatewayTokenFileEnv))
	var read func() (string, error)
	if tokenFile != "" {
		read = answer.GatewayTokenFile(tokenFile)
	}
	gw, err := answer.NewGatewayCredential(getenv(intentGatewayLoginEnv), read)
	if err != nil {
		return intentWiring{Banner: fmt.Sprintf("deskd: signed intents OFF — %d key(s) enrolled in %s but no gateway credential (%s/%s: %v); every signed intent is refused (the authorization contract)",
			ring.Len(), path, intentGatewayTokenFileEnv, intentGatewayLoginEnv, err)}
	}
	return intentWiring{
		Verifier: answer.NewIntentVerifier(ring, now),
		Gateway:  gw,
		Banner: fmt.Sprintf("deskd: signed intents ON — %d key(s) enrolled in %s for %s; a verified intent writes as the declared gateway %s with an App installation token re-read from %s on every write (user-bound tokens refused), signed-by recorded (the authorization contract)",
			ring.Len(), path, strings.Join(ring.Logins(), ","), gw.Login(), tokenFile),
	}
}

// pollAnswerVerifiers is the POLL's half of the signed-intent path
// (the authorization contract gap 3, item (5)): for each cell, the enrolled keys of
// the reviewed intent-keys file as loaded and the cell's CURRENT answer_as,
// which adapter.DeriveAnswer re-verifies every bot-authored answer record
// against. It is independent of [buildIntentWiring]'s on/off decision: with
// the write path OFF (no gateway) the ring still verifies — or, empty, refuses
// — what a bot posted. A cell not in cells.yaml gets the zero verifier, which
// trusts no bot-carried record.
func pollAnswerVerifiers(cfg *schema.File, ring *intentkeys.Ring) func(domain.CellName) adapter.AnswerVerifier {
	answerAs := map[domain.CellName]string{}
	if cfg != nil {
		for i := range cfg.Cells {
			answerAs[domain.CellName(cfg.Cells[i].Name)] = cfg.Cells[i].AnswerAsLogin()
		}
	}
	keys := func(login string) []ed25519.PublicKey {
		var out []ed25519.PublicKey
		for _, k := range ring.KeysFor(login) {
			out = append(out, k.Pub)
		}
		return out
	}
	return func(cell domain.CellName) adapter.AnswerVerifier {
		as, ok := answerAs[cell]
		if !ok {
			return adapter.AnswerVerifier{}
		}
		return adapter.AnswerVerifier{Keys: keys, AnswerAs: as}
	}
}
