package deskkit

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// WithForgeReadContext returns a private backend copy with request cancellation.
// It does not mutate a resolver-shared client or change identity/transport policy.
// Callers must use this only for their existing read surface.
func WithForgeReadContext(f Forge, ctx context.Context) Forge {
	copyClient := func(base *http.Client) *http.Client {
		c := &http.Client{}
		if base != nil {
			*c = *base
		}
		rt := c.Transport
		if rt == nil {
			rt = http.DefaultTransport
		}
		c.Transport = forgeContextTransport{ctx, rt}
		return c
	}
	switch g := f.(type) {
	case *outboundForge:
		cp := *g
		cp.Forge = WithForgeReadContext(g.Forge, ctx)
		return &cp
	case *GitHubForge:
		cp := *g
		cp.rc = nil
		cp.Client = copyClient(g.Client)
		return &cp
	case *GitLabForge:
		cp := *g
		cp.cl = nil
		cp.Client = copyClient(g.Client)
		return &cp
	default:
		return f
	}
}

type forgeContextTransport struct {
	ctx  context.Context
	next http.RoundTripper
}

func (t forgeContextTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(r.Context())
	stop := context.AfterFunc(t.ctx, cancel)
	if t.ctx.Err() != nil {
		cancel()
	}
	resp, err := t.next.RoundTrip(r.WithContext(ctx))
	if err != nil {
		stop()
		cancel()
		return resp, err
	}
	resp.Body = &forgeContextBody{ReadCloser: resp.Body, done: func() { stop(); cancel() }}
	return resp, nil
}

type forgeContextBody struct {
	io.ReadCloser
	done func()
}

func (b *forgeContextBody) Close() error { defer b.done(); return b.ReadCloser.Close() }

// ForgeReadIdentity reports the already-resolved backend's instance without a
// new custody resolution, credentials or provider request.
func ForgeReadIdentity(f Forge, repo ForgeRepo) (ForgeKind, string, error) {
	switch g := backendOf(f).(type) {
	case *GitHubForge:
		host, e := gitHostForKind(ForgeGitHub, g.baseURL(), repo)
		return ForgeGitHub, host, e
	case *GitLabForge:
		host, e := gitHostForKind(ForgeGitLab, g.baseURL(), repo)
		return ForgeGitLab, host, e
	default:
		return "", "", fmt.Errorf("read backend has no qualified identity")
	}
}
