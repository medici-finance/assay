package credential

import (
	"fmt"
	"io"
)

// MaxResponseBytes is the largest response body, in bytes, a credential-path forge read
// accepts. It is deskkit.MaxForgeBodyBytes: this package cannot import deskkit (deskkit
// imports it), so it carries its own copy, and deskkit's TestForgeBodyCapMatches pins the two
// equal. Every answer this package reads (a token mint, a rotation, one REST object) is a
// small JSON document far below it.
const MaxResponseBytes int64 = 32 << 20

// readCappedResponse reads a response body to its end, accepting at most MaxResponseBytes.
// Over the cap it returns NO bytes and an error, so a prefix is never handed on as the answer.
// Callers turn the error into could-not-check (credential.Error → deskkit.CredentialError,
// exit 6), never a success.
func readCappedResponse(body io.Reader) ([]byte, error) {
	return readCappedN(body, MaxResponseBytes)
}

// readCappedN is readCappedResponse with the limit as a parameter (so a test proves the
// over-cap path without a 32 MiB body).
func readCappedN(body io.Reader, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(body, limit+1))
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("response body exceeds the %d-byte cap — refused, never read as a truncated answer", limit)
	}
	return raw, err
}
