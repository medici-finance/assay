package adapter

import (
	"crypto/ed25519"
	"strings"

	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// AnswerVerifier is the poll-side context an answer record is re-verified
// against (the authorization contract gap 3, item (5)). It is supplied by the caller
// per cell — never read off the record being checked.
//
// The zero value trusts no signature: every BOT-authored record then derives
// `answered-unverified`. That is the correct shape when no key is enrolled
// (the intent path is off, so no genuine bot-carried record can exist).
type AnswerVerifier struct {
	// Keys returns the ed25519 keys enrolled for a login in the reviewed
	// intent-keys file AS LOADED. Revoking a key there un-derives every record
	// it signed (on the next poll after the file is reloaded) — intended.
	Keys func(login string) []ed25519.PublicKey
	// AnswerAs is the cell's CURRENT answer_as. An answer_as handover
	// re-derives earlier intent-path records as answered-unverified.
	AnswerAs string
}

// verify runs [contract.VerifyAnswerRecord] for a bot-authored record.
func (v AnswerVerifier) verify(rec contract.AnswerRecord, iss RawIssue, c RawComment) error {
	var keys []ed25519.PublicKey
	if v.Keys != nil {
		keys = v.Keys(rec.Login)
	}
	return contract.VerifyAnswerRecord(rec, keys, contract.AnswerBinding{
		AnswerAs: v.AnswerAs,
		Issue:    iss.URL,
		PostedAt: c.CreatedAt,
	})
}

// DeriveAnswer scans a CLOSED issue's polled comment tail for the answer-record grammar
// answer-record marker (`<!-- assay:answer v1 login=… option=… options_rev=…
// -->`) and derives the domain outcome (the service contract increment 3, task 6).
// This is [contract.ParseAnswerRecord]'s first non-test caller: the audited
// gap was that the parser existed with nothing in internal/adapter invoking it
// ON POLL, so nothing derived {answered_by, option, options_rev} from a real
// comment.
//
// It is meaningful ONLY for a closed issue: an escalation still open is still
// being decided, so deriving an Answer for it would assert an outcome nobody
// has committed. A caller passes an open iss and gets (nil, false) back
// unconditionally — the closed check is inside this function, not left to
// every call site to remember.
//
// Comments are scanned MOST-RECENT FIRST. GraphQL's `comments(last: N)`
// returns the requested tail oldest-first, so this walks iss.Comments in
// reverse. That matters because the marker's own grammar carries no sequence
// number: if a decision were corrected by a follow-up comment, the most
// recent marker is the one that should win, and scanning newest-first is the
// only way this function can prefer it over an earlier, superseded one.
//
// ok=false means no marker was found anywhere in the window: the caller falls
// back to whatever it derives for a closed, unmarked issue (prose or
// otherwise) — unchanged from before this marker grammar existed, and not
// this function's concern. ok=true covers BOTH [contract.Answered] and
// [contract.AnsweredUnparsed] — a present-but-malformed marker is still
// reported as answered, per ParseAnswerRecord's own three-way contract
// (design §10's malformed rule: render what is there, never drop it back to
// "unanswered").
//
// # Who the answer is attributed to (gap-3 item (5))
//
// The marker's `login=` is self-asserted text, so it is never trusted on its
// own:
//
//   - a BOT-authored record (the intent gateway carries the human's signed
//     record) derives as that login's answer ONLY when
//     [contract.VerifyAnswerRecord] passes under v — signature under an
//     enrolled key, action=answer, login= is the cell's current answer_as,
//     issue= is THIS issue, posted inside [iat, exp+120s] (lower edge at whole-second created_at resolution);
//   - a HUMAN-authored record derives as that login's answer ONLY when the
//     comment author IS that login;
//   - anything else — including a present-but-unparsed marker — derives
//     `answered-unverified` ([domain.Answer.Unverified]) attributed to the
//     comment's real AUTHOR, with no option: an anomaly to show, never a
//     human decision any agent may act on.
func DeriveAnswer(iss RawIssue, v AnswerVerifier) (*domain.Answer, bool) {
	if !iss.Closed {
		return nil, false
	}
	for i := len(iss.Comments) - 1; i >= 0; i-- {
		c := iss.Comments[i]
		rec, status := contract.ParseAnswerRecord(c.Body)
		if status == contract.NoMarker {
			continue
		}
		verified := false
		if status == contract.Answered {
			if c.AuthorBot {
				verified = v.verify(rec, iss, c) == nil
			} else {
				verified = c.Login != "" && strings.EqualFold(c.Login, rec.Login)
			}
		}
		if !verified {
			return &domain.Answer{
				By:         domain.Actor{Name: c.Login, IsBot: c.AuthorBot},
				At:         c.CreatedAt,
				Unverified: true,
			}, true
		}
		ans := &domain.Answer{
			OptionsRev: rec.OptionsRev,
			// The verified answerer is the HUMAN the record names: on the
			// intent path the bot only carried the signed record.
			By: domain.Actor{Name: rec.Login},
			At: c.CreatedAt,
		}
		if rec.OptionID != "" {
			opt := rec.OptionID
			ans.Option = &opt
		}
		return ans, true
	}
	return nil, false
}
