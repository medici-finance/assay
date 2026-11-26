package forgechange

import (
	"errors"
	"net/http"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type InstallationToken struct {
	Value     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// MintGitHubToken exchanges the caller's App key for a token scoped to a single
// repository in the named installation. The caller owns credential storage.
func MintGitHubToken(client *http.Client, appID, installationID, repository string, pem []byte) (InstallationToken, error) {
	if !idPattern.MatchString(appID) || !idPattern.MatchString(installationID) {
		return InstallationToken{}, errors.New("GitHub App and installation IDs must be positive integers")
	}
	if !repoPattern.MatchString("owner/"+repository) || repository == "." || repository == ".." {
		return InstallationToken{}, errors.New("invalid GitHub repository name")
	}
	token, expires, err := deskkit.MintStandaloneGitHubToken(
		boundHTTP(client, deskkit.GitHubAPIBase), appID, installationID, repository, pem)
	if err != nil {
		return InstallationToken{}, err
	}
	return InstallationToken{Value: token, ExpiresAt: expires}, nil
}
