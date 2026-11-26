// Package forgechange exposes repository-scoped PR and MR operations backed by
// the desk-tools forge adapters, with credentials and policy supplied by the caller.
package forgechange

import "github.com/medici-finance/assay/tools/desk/internal/deskkit"

type (
	Config      = deskkit.StandaloneChangeConfig
	Client      = deskkit.StandaloneChanges
	Change      = deskkit.StandaloneChange
	File        = deskkit.StandaloneChangeFile
	Files       = deskkit.StandaloneChangeFiles
	Comment     = deskkit.StandaloneChangeComment
	Publication = deskkit.StandalonePublication
)

// New binds the client to a single configured repository.
func New(cfg Config) (*Client, error) {
	return deskkit.NewStandaloneChanges(cfg)
}
