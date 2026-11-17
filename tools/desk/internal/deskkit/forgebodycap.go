package deskkit

import (
	"errors"
	"fmt"
	"io"
)

// forgebodycap.go — the byte cap on every raw forge response body the desk tools read.
//
// A request timeout bounds how LONG a forge exchange may take, not how much it may return: a
// forge (or anything answering in its place) that streams a large or endless body inside the
// deadline makes a reader that calls io.ReadAll on it buffer all of it. ReadCappedForgeBody is
// the one read every raw forge body in the forge library, the credential-mint paths and the
// lock-holding write verbs goes through; TestForgeBodyReadsCapped fails the build when a
// shipped file in those trees reads a response body with a bare io.ReadAll instead.
//
// Readers that already carry their own, narrower bound keep it: the mirror read
// (forge_response_bounds.go, adapters.MaxUpdateBytes), the run-log archive reader
// (runLogArchiveCap), the composition-source and hotfix fetches and the GitLab account reader.

// MaxForgeBodyBytes is the largest forge response body, in bytes, a raw forge read accepts.
//
// Why 32 MiB: the largest body the capped readers legitimately take is a pull request's
// unified diff (ChangeDiff). The forge refuses to render a diff past its own size limits
// (a few MiB) and answers that with an error status, not a longer body, and every other
// capped read is a single JSON object or one page of at most 100 items. 32 MiB is far above
// any of those and still bounds the memory one hostile response can take. It matches the
// hotfix forge's existing body bound (hotfix.go).
//
// The credential package cannot import this one, so it carries its own copy
// (credential.MaxResponseBytes); TestForgeBodyCapMatches pins the two equal.
const MaxForgeBodyBytes int64 = 32 << 20

// ForgeBodyCapError is the cause carried by a body that exceeded MaxForgeBodyBytes. It holds
// no response bytes, header, endpoint or credential.
type ForgeBodyCapError struct {
	Limit int64
}

func (e *ForgeBodyCapError) Error() string {
	return fmt.Sprintf("forge response body exceeds the %d-byte cap", e.Limit)
}

// IsForgeBodyOverCap reports whether err is (or wraps) an over-cap body refusal.
func IsForgeBodyOverCap(err error) bool {
	var ce *ForgeBodyCapError
	return errors.As(err, &ce)
}

// ReadCappedForgeBody reads a forge response body to its end, accepting at most
// MaxForgeBodyBytes. what names the request for the error ("GET /repos/...").
//
//   - A body over the cap is refused: Unverifiable (exit 6) wrapping *ForgeBodyCapError, with
//     NO bytes returned. A prefix of an over-cap body is never handed back, because a prefix
//     such as `[]` decodes cleanly into an empty, complete-looking answer.
//   - A read error is returned as it came, together with what arrived, so each caller keeps
//     its own could-not-check wording for a body cut off mid-transfer.
//
// Once the shared body-read helper of the bounded-client change (ReadForgeBody) is on main,
// it reads through this function, so the cap and its empty-body rule compose in one place.
func ReadCappedForgeBody(body io.Reader, what string) ([]byte, error) {
	return readCapped(body, what, forgeBodyCap)
}

// forgeBodyCap is the cap ReadCappedForgeBody applies: MaxForgeBodyBytes in production. It is
// a var for ONE reason — so a test in any package can prove a call site's over-cap path with a
// small body instead of streaming 32 MiB (SetForgeBodyCapForTest). There is no flag or env
// override; TestForgeBodyCapDefault pins the production value.
var forgeBodyCap = MaxForgeBodyBytes

// SetForgeBodyCapForTest lowers the cap ReadCappedForgeBody applies and returns a function
// that restores the previous one. It is a TEST seam in the SetCredentialHTTP mould: tests of
// the call sites in other packages use it; production code never calls it.
func SetForgeBodyCapForTest(limit int64) (restore func()) {
	prev := forgeBodyCap
	forgeBodyCap = limit
	return func() { forgeBodyCap = prev }
}

// ReadForgeBodyUpTo is ReadCappedForgeBody for a reader that carries its own, NARROWER bound
// (a command whose answers are never near MaxForgeBodyBytes). The rules are the same: over
// limit is refused with no bytes returned, never handed back truncated. A limit that is not
// positive, or above MaxForgeBodyBytes, is clamped to MaxForgeBodyBytes — the shared cap is
// the ceiling, never widened by a caller.
func ReadForgeBodyUpTo(body io.Reader, what string, limit int64) ([]byte, error) {
	if limit <= 0 || limit > MaxForgeBodyBytes {
		limit = MaxForgeBodyBytes
	}
	return readCapped(body, what, limit)
}

// readCapped is ReadCappedForgeBody with the limit as a parameter.
func readCapped(body io.Reader, what string, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(body, limit+1))
	if int64(len(raw)) > limit {
		return nil, Unverifiable(fmt.Sprintf("%s: response body exceeds the %d-byte forge body cap — refused, never read as a truncated answer", what, limit), &ForgeBodyCapError{Limit: limit})
	}
	return raw, err
}
