package main

import (
	"net/http"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func authHTTP(clients []*http.Client) *http.Client {
	if len(clients) > 0 && clients[0] != nil {
		return clients[0]
	}
	return httpClient
}

func measuredAuth(ctx perfrec.Context, s *perfrec.Session) *http.Client {
	observer := deskkit.RequestObserverFor(ctx, s, perfrec.OriginDirect)
	if observer.Sink != nil {
		// All network leaves in this invocation are authHTTP calls. A cache hit,
		// help, local refusal, or --no-rotate therefore has measured zero sends.
		s.ObserveRequests()
	}
	return deskkit.ObserveAuthClient(httpClient, observer)
}
