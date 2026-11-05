// Package plantedverb is the positive control for TestCredentialImportConfinementSeesPlant:
// a verb that imports the provider and runs an operation itself. It is never built.
package plantedverb

import "github.com/medici-finance/assay/tools/desk/internal/credential"

func plantedMint() {
	_, _ = credential.GitHub(credential.GitHubRequest{}, credential.Adapters{})
}
