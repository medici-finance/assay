// Package planted is the forge-client guard's positive control: each function is one client
// construction the guard must report (or, for the capped ones, must not). It is never built.
package planted

import (
	"net/http"

	ghapi "github.com/cli/go-gh/v2/pkg/api"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func restNoCap(tok string, rt http.RoundTripper) (*ghapi.RESTClient, error) {
	return ghapi.NewRESTClient(ghapi.ClientOptions{AuthToken: tok, Transport: rt})
}

func restNoTransport(tok string) (*ghapi.RESTClient, error) {
	return ghapi.NewRESTClient(ghapi.ClientOptions{AuthToken: tok})
}

func restCapped(tok string, rt http.RoundTripper) (*ghapi.RESTClient, error) {
	return ghapi.NewRESTClient(ghapi.ClientOptions{AuthToken: tok, Transport: capForgeBodies(rt)})
}

func glBareClient(tok string, hc *http.Client) (*gitlab.Client, error) {
	return gitlab.NewClient(tok, gitlab.WithHTTPClient(hc))
}

func glNoHTTPClient(tok string) (*gitlab.Client, error) {
	return gitlab.NewClient(tok)
}

func glCapped(tok string, hc *http.Client) (*gitlab.Client, error) {
	return gitlab.NewClient(tok, gitlab.WithHTTPClient(cappedForgeClient(hc)))
}
