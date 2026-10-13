// Package adapters holds what every authenticated read adapter shares: the Access an adapter is
// handed, and the refusal an adapter returns when a provider is still paginating at the walk
// ceiling. It carries no credential and starts no request of its own.
//
// AN ADAPTER NEVER DISCOVERS A CREDENTIAL. It reads through an Access that its trusted
// composition (the desk-tools backend that already resolved the credential, validated the host and
// the repository, and decided that this process may read at all) hands it. There is no
// constructor here that takes a token, a base URL, an environment variable or a file path; an
// adapter that wanted ambient access would have nothing to take it from.
package adapters

import (
	"context"
	"errors"
	"fmt"
	"net/url"
)

// Page is one provider response page: the raw JSON body and the provider's own more-pages signal
// (GitHub's Link rel="next"; GitLab's X-Next-Page).
type Page struct {
	Body []byte
	// More is true when the provider says another page exists.
	More bool
}

// Access is the already-admitted read access an adapter reads through. GetPage performs ONE GET
// beneath repo's own route (the composition builds the repository-scoped prefix itself from repo,
// after checking repo against the credential's scope), appending rel and query. An implementation
// returns an error for anything that is not a complete 2xx answer.
type Access interface {
	GetPage(ctx context.Context, repo, rel string, query url.Values) (Page, error)
}

// TruncatedError is returned when the provider is still paginating after the adapter's page
// ceiling. No partial list accompanies it: a short list read as "these are all the open issues"
// would let a consumer retire work for issues that are still open.
type TruncatedError struct {
	Repo     string
	Pages    int
	PageSize int
}

func (e *TruncatedError) Error() string {
	return fmt.Sprintf("could-not-check: %s still reports more open issues after %d pages of %d (the open-issue "+
		"page ceiling) — refusing to hand back a PARTIAL open-issue set, because a consumer reads an issue's "+
		"absence from this list as evidence about it and would act on issues that are still open",
		e.Repo, e.Pages, e.PageSize)
}

// IsTruncated reports whether err is, or wraps, a TruncatedError.
func IsTruncated(err error) bool {
	var t *TruncatedError
	return errors.As(err, &t)
}
