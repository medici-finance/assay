// Package forgechange exposes repository-scoped PR and MR operations backed by
// the desk-tools forge adapters, with credentials and policy supplied by the caller.
package forgechange

import (
	"errors"
	"net/http"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type (
	Client      = deskkit.StandaloneChanges
	Change      = deskkit.StandaloneChange
	File        = deskkit.StandaloneChangeFile
	Files       = deskkit.StandaloneChangeFiles
	Comment     = deskkit.StandaloneChangeComment
	Publication = deskkit.StandalonePublication
)

// Config describes a trusted project and credential.
type Config struct {
	Forge      string
	ProjectURL string
	Repository string
	Token      string
	HTTPClient *http.Client
	// GitLabLoopback identifies an optional validating TLS proxy on 127.0.0.1.
	GitLabLoopback string
	// PublicationNotice receives sanitized diagnostics. The default is stderr.
	PublicationNotice func(string)
}

// New binds the client to a single configured repository.
func New(cfg Config) (*Client, error) {
	var gitlabInstance string
	switch cfg.Forge {
	case "github":
	case "gitlab":
		gitlabInstance = strings.TrimSuffix(cfg.ProjectURL, "/"+cfg.Repository)
	default:
		return nil, errors.New("forge must be github or gitlab")
	}
	return deskkit.NewStandaloneChanges(deskkit.StandaloneChangeConfig{
		ProjectURL: cfg.ProjectURL, Repository: cfg.Repository, Token: cfg.Token,
		HTTPClient: cfg.HTTPClient, GitLabInstance: gitlabInstance, GitLabLoopback: cfg.GitLabLoopback,
		PublicationNotice: cfg.PublicationNotice,
	})
}
