package deskkit

// forgetransport.go — the opt-in transport seam for the GitHub backend ResolveForge builds.
//
// A resolved GitHubForge sends through http.DefaultTransport unless a caller installs a
// wrapper here. The seam exists so ONE tool can add read-side behaviour to every request its
// resolved backends send (deskboard's conditional-request cache and per-sweep cost meter,
// issue #2421) WITHOUT that behaviour reaching any other tool: nothing in this package
// installs a wrapper, so every tool that does not call SetGitHubTransport keeps exactly the
// transport it had. It sits beside SetGitHubCustodyMinter for the same reason that hook
// exists — the resolver stays the single construction site, and the caller's own behaviour
// is handed in rather than this package growing a second, tool-specific code path.
//
// The wrapper sees each request AFTER go-gh's header layer (the Authorization header is
// already set) and must return a response with go-gh's normal contract: a 2xx it can decode,
// or a non-2xx it maps to a ForgeAPIError. It is never applied to the CI workflow-token
// backend (ReadOnlyForgeForCIToken) or to the GitLab backend.

import "net/http"

// GitHubTransportWrapper wraps the base round-tripper a resolved GitHub backend sends
// through and returns the one it should use instead.
type GitHubTransportWrapper func(base http.RoundTripper) http.RoundTripper

// githubTransportWrapper is the installed wrapper; nil (every tool but the one that opts in)
// leaves the backend on its default transport.
var githubTransportWrapper GitHubTransportWrapper

// SetGitHubTransport installs fn as the wrapper ResolveForge applies to every GitHub backend
// it constructs in this process. Passing nil restores the default transport.
func SetGitHubTransport(fn GitHubTransportWrapper) {
	githubTransportWrapper = fn
}

// githubSeamClient is the Client field ResolveForge hands a GitHub backend: nil when no
// wrapper is installed (the backend's own default), else a client whose transport is the
// installed wrapper around http.DefaultTransport.
func githubSeamClient() *http.Client {
	if githubTransportWrapper == nil {
		return nil
	}
	return &http.Client{Transport: countedTransport{next: githubTransportWrapper(countRoundTrips(http.DefaultTransport))}}
}

// countingTransport counts each actual HTTP round-trip for the local perf record
// (perf.go) and delegates. An explicitly supplied invocation observer additionally
// wraps response-body lifetime without reading ahead, buffering or changing its
// bytes/errors. No observer retains the existing count-and-delegate path.
// It lives here, beside the transport seam, so the perf files themselves import no network
// package at all.
// countedTransport marks a composed adapter whose leaf already counts attempts.
// The outer adapter may refuse locally; only the leaf represents HTTP work.
type countedTransport struct{ next http.RoundTripper }

func (c countedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return c.next.RoundTrip(req)
}

type countingTransport struct{ next http.RoundTripper }

func (c countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	perfCountForgeCall()
	return observeRoundTrip(c.next, req)
}

// countRoundTrips wraps rt (http.DefaultTransport when nil) in the counting transport. A
// transport already counting is returned as is, so a double install never double-counts.
func countRoundTrips(rt http.RoundTripper) http.RoundTripper {
	if rt == nil {
		rt = http.DefaultTransport
	}
	switch rt.(type) {
	case countingTransport, countedTransport:
		return rt
	}
	return countingTransport{next: rt}
}

// countingClient returns a COPY of c whose transport counts round-trips (a fresh client on
// http.DefaultTransport when c is nil). The caller's client is never mutated, so a test
// client shared across backends keeps exactly the transport it was built with.
func countingClient(c *http.Client) *http.Client {
	if c == nil {
		return &http.Client{Transport: countRoundTrips(nil)}
	}
	cp := *c
	cp.Transport = countRoundTrips(c.Transport)
	return &cp
}
