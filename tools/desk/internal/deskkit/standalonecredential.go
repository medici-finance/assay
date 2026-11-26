package deskkit

import (
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/credential"
)

var standaloneID = regexp.MustCompile(`^[1-9][0-9]*$`)

// MintStandaloneGitHubToken uses an explicit App identity and repository from
// the standalone library. Desk role credential lookup uses its own custody path.
func MintStandaloneGitHubToken(client *http.Client, appID, installationID, repository string, pem []byte) (string, time.Time, error) {
	if !standaloneID.MatchString(appID) || !standaloneID.MatchString(installationID) {
		return "", time.Time{}, errors.New("GitHub App and installation IDs must be positive integers")
	}
	if !standalonePathSegment.MatchString(repository) || repository == "." || repository == ".." {
		return "", time.Time{}, errors.New("invalid GitHub repository name")
	}
	now := time.Now()
	token, err := credential.MintStandaloneGitHub(
		standaloneBoundHTTP(client, GitHubAPIBase), appID, installationID, repository, pem, now)
	if err != nil {
		return "", time.Time{}, errors.New("GitHub did not grant an installation token for the configured repository")
	}
	expires, err := time.Parse(time.RFC3339, token.ExpiresAt)
	if err != nil || !expires.After(now) {
		return "", time.Time{}, errors.New("GitHub installation token has no valid expiry")
	}
	return token.Token, expires, nil
}
