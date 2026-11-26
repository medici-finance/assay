package deskkit

import (
	"errors"
	"net/http"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/credential"
)

// MintStandaloneGitHubToken uses an explicit App identity and repository from
// the standalone library. Desk role credential lookup uses its own custody path.
func MintStandaloneGitHubToken(client *http.Client, appID, installationID, repository string, pem []byte) (string, time.Time, error) {
	key, err := credential.ParsePrivateKey(pem)
	if err != nil {
		return "", time.Time{}, errors.New("invalid GitHub App private key")
	}
	now := time.Now()
	jwt, err := credential.BuildJWT(appID, now, key)
	if err != nil {
		return "", time.Time{}, errors.New("could not sign GitHub App credential")
	}
	token, err := credential.ExchangeRepositoryJWT(client, jwt, installationID, repository)
	if err != nil {
		return "", time.Time{}, errors.New("GitHub did not grant an installation token for the configured repository")
	}
	expires, err := time.Parse(time.RFC3339, token.ExpiresAt)
	if err != nil || !expires.After(now) {
		return "", time.Time{}, errors.New("GitHub installation token has no valid expiry")
	}
	return token.Token, expires, nil
}
