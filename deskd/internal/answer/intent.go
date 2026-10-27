package answer

import (
	"context"
	"errors"
	"fmt"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/deskd/config/intentkeys"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// This file is the SIGNED-INTENT half of the write path (the authorization contract,
// gap 3). The three recorded rulings it implements, and nothing wider:
//
//  1. Trust anchor. A signed intent is honoured only when it verifies against a
//     key enrolled for its login in the reviewed intent-keys file
//     (config/intentkeys). Unknown key, bad signature, expired intent, or a
//     replayed nonce is refused BEFORE any write.
//  2. Stand-in. A verified human-signed intent stands in for the per-request
//     credential: the write runs under the bot GATEWAY credential, and the
//     human's signature is the gate:human act. A bot write with no verified
//     signature still fails — the writer reaches the gateway credential ONLY
//     through a [SignedAct] this file minted (github.go, token()).
//  3. Cookie OR intent. The /v1 handler takes this path when the request
//     carries an intent, and the unchanged live-session path otherwise.
//
// # The layers, lowest first
//
//   - The writer (github.go) hands out the gateway token only for a context
//     carrying a verified SignedAct. No act → the per-request human path,
//     unchanged, which has no credential for a bot → [ErrNoCredential].
//   - The Service ([Service.AnswerSigned] / [Service.AckSigned]) re-checks the
//     act: minted here, unexpired, signed by the cell's answerer, for THIS
//     action on THIS issue, with THIS option — and refuses unsigned free text.
//   - The verifier ([IntentVerifier.Accept]) checks the trust anchor, the
//     signature, the time window, the lifetime cap, the process-start floor
//     and the nonce, and is the only constructor of a verified SignedAct.
//   - Above all three, the capability middleware admits a session-less
//     request only when [IntentVerifier.Check] passes (serve, deskd routes).
//
// A SignedAct's `verified` bit is unexported and set only in [IntentVerifier.Accept],
// exactly as a Session's `live` bit is set only by the session store: "a human
// signed this" is a fact the type system carries, not a boolean a caller sets.

// MaxIntentLifetime caps exp-iat. A longer-lived intent is refused outright:
// the cap bounds both the replay window and the nonce memory.
const MaxIntentLifetime = 10 * time.Minute

// nonceRE bounds a nonce: long enough to be unique in practice, short and
// plain enough to record on the audit trail verbatim.
//
// # Derivation (the authorization contract)
//
// WHO MINTS. The client mints the nonce: it is a field of the
// [contract.SignedIntent] the client signs. No production minter exists in
// this repo or elsewhere in the org (searched for #242). The only mints that
// reach this verifier are test fixtures: nonce-%016d (22 chars) here,
// serve-nonce-%08d (20) in internal/serve, and the literal
// identity-nonce-00000001 (23) in internal/identity. Other fixtures never get
// here (n-0001 in internal/contract and n1 in internal/serve only exercise
// contract-level Verify or a refusal path with no verifier wired). So the bounds below rest on a stated rationale, not on a
// minter's length. TestNonceDerivation pins both the rationale and the
// fixture mint.
//
// WHAT THE NONCE MUST DO. It has to be UNIQUE per login for [MaxIntentLifetime]:
// the replay memory keys on login+nonce. It does not have to be secret. The
// signature covers the nonce, so an attacker who guesses one still cannot sign
// an intent around it.
//
//   - Floor 16: the hex encoding of 64 random bits (8 bytes → 16 chars), the
//     shortest common encoding of a 64-bit random value. Two intents from one
//     human inside 10 minutes collide with probability about n²/2⁶⁵, which is
//     negligible. Shorter forms are counters and timestamps ("n-0001"), and
//     they are refused. A 64-bit value in unpadded base64url is 11 chars and is
//     refused too: a client must send at least 12 random bytes (16 chars) in
//     that encoding.
//   - Ceiling 128: the hex encoding of 512 bits (64 bytes, e.g. a SHA-512
//     digest), the longest common digest encoding. It bounds the `nonce=` field
//     of the answer record, recorded verbatim in an issue comment, and each
//     entry of the in-memory nonce memory.
//   - Alphabet: unpadded base64url ∪ hex. It has no space and no '=', so the
//     space-separated key=value audit line parses back unambiguously.
var nonceRE = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

// Refusal sentinels for the intent path. Every one maps to 403
// ([StatusForError]) and every one is returned before any write.
var (
	// ErrIntentRefused wraps every refusal below so a handler maps them once.
	ErrIntentRefused = errors.New("signed intent refused")
	// ErrIntentUnknownKey — no key is enrolled for the intent's login.
	ErrIntentUnknownKey = fmt.Errorf("%w: no key is enrolled for this login in the reviewed intent-keys file", ErrIntentRefused)
	// ErrIntentReplayed — this login+nonce was already accepted.
	ErrIntentReplayed = fmt.Errorf("%w: nonce already used (replay)", ErrIntentRefused)
	// ErrIntentLifetime — exp-iat is not in (0, MaxIntentLifetime].
	ErrIntentLifetime = fmt.Errorf("%w: lifetime must be positive and at most %s", ErrIntentRefused, MaxIntentLifetime)
	// ErrIntentPredates — issued before this process started, so its nonce
	// cannot be proven fresh (the nonce memory does not survive a restart).
	ErrIntentPredates = fmt.Errorf("%w: issued before the service started — re-sign it", ErrIntentRefused)
	// ErrIntentNonce — the nonce is missing or malformed.
	ErrIntentNonce = fmt.Errorf("%w: nonce must match %s", ErrIntentRefused, nonceRE)
	// ErrIntentUnverified — a SignedAct that the verifier did not mint.
	ErrIntentUnverified = fmt.Errorf("%w: the act was not verified by the intent verifier", ErrIntentRefused)
	// ErrIntentNotAnswerer — the signer is not the cell's authorized answerer.
	ErrIntentNotAnswerer = fmt.Errorf("%w: the signer is not the cell's authorized answerer", ErrIntentRefused)
	// ErrIntentMismatch — the request does not match what was signed.
	ErrIntentMismatch = fmt.Errorf("%w: the request does not match the signed record", ErrIntentRefused)
	// ErrUnsignedText — free text is not covered by the signature.
	ErrUnsignedText = fmt.Errorf("%w: free text is not covered by the signature, so it cannot ride a signed answer", ErrIntentRefused)
	// ErrNoGateway — no gateway credential is configured to carry the write.
	ErrNoGateway = fmt.Errorf("%w: no gateway credential is configured for signed-intent writes", ErrIntentRefused)
)

// SignedAct is a signed intent the verifier ACCEPTED: trust anchor, signature,
// window and nonce all checked, nonce consumed. Its fields are unexported and
// `verified` is set only by [IntentVerifier.Accept]; a SignedAct built any
// other way (including the zero value) is refused by the Service and the
// writer alike.
type SignedAct struct {
	login       string
	keyID       string
	fingerprint string
	nonce       string
	action      contract.Action
	issue       string
	optionID    domain.OptionID
	optionsRev  string
	expires     time.Time
	// signed is the intent exactly as the client signed it (login case,
	// times, signature): the answer record carries it so the POLL can rebuild
	// the signed bytes and re-verify them without the service (gap-3 item (4)).
	signed   contract.SignedIntent
	verified bool
}

// Login is the human who signed the act.
func (a *SignedAct) Login() string { return a.login }

// KeyID is the enrolled key id the signature verified against.
func (a *SignedAct) KeyID() string { return a.keyID }

// Fingerprint is the SHA256 fingerprint of that key.
func (a *SignedAct) Fingerprint() string { return a.fingerprint }

// Nonce is the act's consumed nonce.
func (a *SignedAct) Nonce() string { return a.nonce }

func (a *SignedAct) ok() bool { return a != nil && a.verified }

// IntentVerifier checks signed intents against the enrolled keys and remembers
// accepted nonces until their intent expires.
//
// The nonce memory is per process. Replay across a restart is closed by the
// start floor instead (an intent issued before this verifier existed is
// refused), so nothing needs persisting. A deployment that runs more than one
// deskd replica behind one address would need a shared nonce store; deskd
// runs one.
type IntentVerifier struct {
	keys  *intentkeys.Ring
	now   func() time.Time
	start time.Time

	mu   sync.Mutex
	seen map[string]time.Time // login\x00nonce -> intent exp
}

// NewIntentVerifier builds a verifier over the enrolled ring. A nil or empty
// ring trusts nothing: every intent is [ErrIntentUnknownKey].
func NewIntentVerifier(keys *intentkeys.Ring, now func() time.Time) *IntentVerifier {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &IntentVerifier{keys: keys, now: now, start: now(), seen: map[string]time.Time{}}
}

// Check verifies si for action on issue WITHOUT consuming its nonce. It is the
// capability middleware's admission test; the handler still calls [Accept],
// which re-verifies everything and consumes the nonce.
func (v *IntentVerifier) Check(si contract.SignedIntent, action contract.Action, issue string) error {
	_, err := v.verify(si, action, issue, false)
	return err
}

// Accept verifies si for action on issue and, only if every check passes,
// consumes its nonce and returns the verified act. Any error means nothing was
// recorded and nothing may be written.
func (v *IntentVerifier) Accept(si contract.SignedIntent, action contract.Action, issue string) (*SignedAct, error) {
	key, err := v.verify(si, action, issue, true)
	if err != nil {
		return nil, err
	}
	return &SignedAct{
		login:       strings.ToLower(si.Login),
		keyID:       key.ID,
		fingerprint: key.Fingerprint(),
		nonce:       si.Nonce,
		action:      si.Action,
		issue:       si.Issue,
		optionID:    si.OptionID,
		optionsRev:  si.OptionsRev,
		expires:     si.ExpiresAt,
		signed:      si,
		verified:    true,
	}, nil
}

func (v *IntentVerifier) verify(si contract.SignedIntent, action contract.Action, issue string, consume bool) (intentkeys.Key, error) {
	if v == nil {
		return intentkeys.Key{}, ErrIntentUnknownKey
	}
	now := v.now()
	keys := v.keys.KeysFor(si.Login)
	if len(keys) == 0 {
		return intentkeys.Key{}, ErrIntentUnknownKey
	}
	if !nonceRE.MatchString(si.Nonce) {
		return intentkeys.Key{}, ErrIntentNonce
	}
	if life := si.ExpiresAt.Sub(si.IssuedAt); life <= 0 || life > MaxIntentLifetime {
		return intentkeys.Key{}, ErrIntentLifetime
	}
	if si.IssuedAt.Before(v.start) {
		return intentkeys.Key{}, ErrIntentPredates
	}
	var matched *intentkeys.Key
	var lastErr error
	for i := range keys {
		err := si.Verify(keys[i].Pub, now, action, issue)
		if err == nil {
			matched = &keys[i]
			break
		}
		lastErr = err
		if !errors.Is(err, contract.ErrIntentBadSignature) {
			// Action, resource and time checks run before the signature and do
			// not depend on the key: another key cannot change the answer.
			break
		}
	}
	if matched == nil {
		// Both are wrapped (%w twice): errors.Is reaches ErrIntentRefused for
		// the status mapping AND the specific contract cause
		// (ErrIntentBadSignature, ErrIntentExpired, ...) for the record.
		return intentkeys.Key{}, fmt.Errorf("%w: %w", ErrIntentRefused, lastErr)
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	for k, exp := range v.seen {
		if now.After(exp) {
			delete(v.seen, k)
		}
	}
	nk := strings.ToLower(si.Login) + "\x00" + si.Nonce
	if _, used := v.seen[nk]; used {
		return intentkeys.Key{}, ErrIntentReplayed
	}
	if consume {
		v.seen[nk] = si.ExpiresAt
	}
	return *matched, nil
}

// signedActKey is the context key a verified act travels under. It is
// unexported: only [withSignedAct], called by the Service's signed methods,
// can put one there, and it is the ONLY thing that lets the writer reach the
// gateway credential.
type signedActKey struct{}

func withSignedAct(ctx context.Context, act *SignedAct) context.Context {
	return context.WithValue(ctx, signedActKey{}, act)
}

func signedActFrom(ctx context.Context) (*SignedAct, bool) {
	if ctx == nil {
		return nil, false
	}
	act, ok := ctx.Value(signedActKey{}).(*SignedAct)
	return act, ok && act != nil
}

// GatewayCredential is the BOT credential a verified human-signed write runs
// under (ruling 2: "the bot is just the gateway"). It is never a human's
// credential, and that is ENFORCED on the token, not taken from a label:
//
//   - the token must have the shape of a GitHub App INSTALLATION token
//     (`ghs_…`, [CheckGatewayToken]). Every user-bound shape — a classic or
//     fine-grained PAT, an OAuth or user-to-server token, a refresh token — is
//     refused by name, so a human's credential cannot be configured into the
//     slot and custody model (b) ("no human credential at rest") stays true;
//   - the token is NOT held for the process lifetime. It is re-read from its
//     source on EVERY use (the --tokens-dir precedent, the service contract): an
//     installation token expires after an hour, and the minter that refreshes
//     it (a sidecar rewriting the file) is picked up with no restart. A read
//     that fails, is empty, or yields a user-bound shape refuses THAT write —
//     nothing falls back to an earlier token;
//   - its login must be a bot login (`…[bot]`) and never the human write
//     identity. The login is still operator-DECLARED (a ghs_ token does not
//     name its App without a forge call); the token-shape check is what keeps
//     a human credential out.
//
// The writer uses it only for a context carrying a verified [SignedAct].
type GatewayCredential struct {
	login string
	read  func() (string, error)
}

// ErrGatewayTokenShape — the gateway source yielded something that is not a
// GitHub App installation token. It wraps [ErrNoGateway]: the write is refused
// exactly as if no gateway were configured.
var ErrGatewayTokenShape = fmt.Errorf("%w: the gateway credential must be a GitHub App installation token (ghs_…)", ErrNoGateway)

// installationTokenRE is the accepted gateway shape: the installation-token
// prefix, then only token characters (no whitespace, so nothing can ride into
// the Authorization header).
var installationTokenRE = regexp.MustCompile(`^ghs_[A-Za-z0-9_]+$`)

// userBoundTokenPrefixes are GitHub's credential prefixes that act for a USER:
// classic PAT, fine-grained PAT, OAuth access, user-to-server, refresh. They
// are refused by name so the operator sees why.
var userBoundTokenPrefixes = []string{"ghp_", "github_pat_", "gho_", "ghu_", "ghr_"}

// CheckGatewayToken accepts only an installation-token shape. It never
// returns or logs the token: the error names at most the refused PREFIX.
func CheckGatewayToken(tok string) error {
	for _, p := range userBoundTokenPrefixes {
		if strings.HasPrefix(tok, p) {
			return fmt.Errorf("%w — refused a user-bound %q token: a human's credential is never held at rest (custody model (b))", ErrGatewayTokenShape, p)
		}
	}
	if !installationTokenRE.MatchString(tok) {
		return ErrGatewayTokenShape
	}
	return nil
}

// GatewayTokenFile is the gateway source deskd uses: it re-reads path on
// EVERY call and caches nothing, so a minter rotating the file is picked up
// on the next write. An error names the path, never a value.
func GatewayTokenFile(path string) func() (string, error) {
	return func() (string, error) {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("gateway token file %s: %w", path, err)
		}
		return string(raw), nil
	}
}

// NewGatewayCredential builds the gateway credential over a token SOURCE,
// never a token. A blank login, a non-bot login, the human write identity or
// a nil source is refused; the source is read once here too, so a missing or
// wrongly-shaped credential turns the path OFF at startup instead of failing
// the first signed write. An unbuilt gateway means every signed intent is
// refused.
func NewGatewayCredential(login string, read func() (string, error)) (*GatewayCredential, error) {
	login = strings.TrimSpace(login)
	if login == "" || read == nil {
		return nil, ErrNoGateway
	}
	if !strings.HasSuffix(login, "[bot]") || strings.EqualFold(login, humanActor()) {
		return nil, fmt.Errorf("%w: gateway login %q must be a bot login (…[bot]) and never the human write identity", ErrNoGateway, login)
	}
	g := &GatewayCredential{login: login, read: read}
	if _, err := g.current(); err != nil {
		return nil, err
	}
	return g, nil
}

// current reads the source afresh and checks the shape. It is unexported: the
// only caller is the writer's token(), behind a verified SignedAct.
func (g *GatewayCredential) current() (string, error) {
	if g == nil || g.read == nil {
		return "", ErrNoGateway
	}
	raw, err := g.read()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrNoGateway, err)
	}
	tok := strings.TrimSpace(raw)
	if tok == "" {
		return "", ErrNoGateway
	}
	if err := CheckGatewayToken(tok); err != nil {
		return "", err
	}
	return tok, nil
}

// Login is the bot identity signed-intent writes are authored as on the
// platform (operator-declared; see [GatewayCredential]).
func (g *GatewayCredential) Login() string {
	if g == nil {
		return ""
	}
	return g.login
}

// requireAct is the Service's own gate for the signed path — the analogue of
// requireLive, independent of the handler and the middleware.
func (s *Service) requireAct(act *SignedAct, action contract.Action, ref Ref) error {
	if !humanid.Enabled() {
		return humanid.ErrDisabled
	}
	if !act.ok() {
		return ErrIntentUnverified
	}
	if s.now().After(act.expires) {
		return fmt.Errorf("%w: %w", ErrIntentRefused, contract.ErrIntentExpired)
	}
	if act.login == "" || s.AnswerAs() == "" || !strings.EqualFold(act.login, s.AnswerAs()) {
		return fmt.Errorf("%w: signer %q, answerer %q", ErrIntentNotAnswerer, act.login, s.AnswerAs())
	}
	if act.action != action {
		return fmt.Errorf("%w: signed action %q, invoked %q", ErrIntentMismatch, act.action, action)
	}
	if !ref.valid() {
		return ErrBadRef
	}
	if act.issue != ref.URL {
		return fmt.Errorf("%w: signed issue %q, acting on %q", ErrIntentMismatch, act.issue, ref.URL)
	}
	return nil
}

// AnswerSigned commits an answer on a verified human-signed intent (rulings 2
// and 3): the same comment + close + race check as [Service.Answer], executed
// under the gateway credential, with the signer recorded in the answer-record
// marker and on the decision line. The decision is the SIGNED one: the
// request's option and options-rev must equal what was signed, and free text
// (not covered by the signature) is refused.
func (s *Service) AnswerSigned(ctx context.Context, act *SignedAct, req AnswerRequest) (AnswerResult, error) {
	if err := s.requireAct(act, contract.ActionAnswer, req.Ref); err != nil {
		return AnswerResult{}, err
	}
	if req.Text != "" {
		return AnswerResult{}, ErrUnsignedText
	}
	if req.Option != act.optionID || req.OptionsRev != act.optionsRev {
		return AnswerResult{}, fmt.Errorf("%w: option/options_rev differ from the signed ones", ErrIntentMismatch)
	}
	if req.Option == "" {
		return AnswerResult{}, ErrEmptyAnswer
	}
	res, err := s.commit(withSignedAct(ctx, act), req, act)
	res.SignedBy = act.login
	res.AuthoredAs = s.gatewayActor()
	return res, err
}

// AckSigned is the ack on a verified human-signed intent: the same 👀 reaction
// as [Service.Ack], under the gateway credential, with the SIGNER recorded as
// the acker.
func (s *Service) AckSigned(ctx context.Context, act *SignedAct, id domain.EscalationID, ref Ref) error {
	if err := s.requireAct(act, contract.ActionAck, ref); err != nil {
		return err
	}
	return s.ack(withSignedAct(ctx, act), id, ref, act.login)
}

// gatewayActor reports the bot login signed writes are authored as, when the
// writer exposes one.
func (s *Service) gatewayActor() string {
	if g, ok := s.writer.(interface{ GatewayActor() string }); ok {
		return g.GatewayActor()
	}
	return ""
}

// signedDecisionBody is the decision comment for a signed answer: the
// answer-record marker naming the SIGNER (so the poll derives the human as the
// answerer), the decision line, and an audit line naming the key and nonce.
//
// The marker is the SIGNED record (gap-3 item (4)): login/option/options_rev
// are the signed ones — never the request body's — followed by the item (4)
// fields, so the poll re-verifies the human's signature itself
// ([contract.VerifyAnswerRecord]) instead of trusting a bot-authored login=.
func signedDecisionBody(req AnswerRequest, act *SignedAct) string {
	rec := contract.AnswerRecord{
		Login:      act.signed.Login,
		OptionID:   act.signed.OptionID,
		OptionsRev: act.signed.OptionsRev,
		Signed:     contract.SignedRecordFor(act.signed, act.fingerprint),
	}
	decision := fmt.Sprintf("Decision: `%s`", req.Option)
	if req.OptionsRev != "" {
		decision += fmt.Sprintf(" (options-rev %s)", domain.ShortRev(req.OptionsRev))
	}
	return rec.Marshal(decision) + "\n\n" +
		fmt.Sprintf("— signed by %s (ed25519 key `%s`, %s, nonce `%s`); the console gateway relays it — the signature is the human act (gate: human), the gateway only transports it (the service contract)",
			act.login, act.keyID, act.fingerprint, act.nonce)
}
