// Package planted is the positive control for TestTokenSnapshotPlanted. It is
// never compiled (testdata is ignored by the go tool). It repeats the defect
// shape of the authorization contract: a token map resolved once at start and handed to
// a client constructor that keeps it for the life of the process.
package planted

import "github.com/medici-finance/assay/deskd/internal/adapter"

func buildConsumers(apiBase string, src adapter.TokenSource, owners []string) *adapter.RoutingFetcher {
	byOwner, _ := src.Resolve(owners)
	return adapter.NewGitHubRouter(apiBase, byOwner, nil)
}

// A client built once at package init is the same defect, one level up.
var startClient = adapter.NewGitHub("", "start-time-token", nil)
