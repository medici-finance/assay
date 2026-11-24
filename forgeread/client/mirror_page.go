package client

import (
	"context"
	"time"

	"github.com/medici-finance/assay/forgeread/service"
)

// DoMirrorPage makes one attempt using the explicitly configured page endpoint.
// A restart response asks the caller to discard the partial traversal.
func (c *Client) DoMirrorPage(ctx context.Context, q service.MirrorPageRequest) (out service.MirrorPageResponse, report Report, err error) {
	start := time.Now()
	report.LogicalCalls = 1
	defer func() { report.Duration = time.Since(start) }()
	if c == nil || ctx == nil {
		return out, report, service.ErrInvalid
	}
	if !service.SameScope(q.Query.Scope, c.scope) {
		return out, report, service.ErrScope
	}
	data, err := service.EncodeMirrorPageRequest(q)
	if err != nil {
		return out, report, err
	}
	q, err = service.DecodeMirrorPageRequest(data)
	if err != nil {
		return out, report, err
	}
	if !service.SameScope(q.Query.Scope, c.scope) {
		return out, report, service.ErrScope
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	raw, _, err := c.roundTrip(ctx, data, &report)
	if err != nil {
		return out, report, err
	}
	got, err := service.DecodeMirrorPageResponse(raw, q)
	if err != nil {
		return out, report, err
	}
	if err := ctx.Err(); err != nil {
		return out, report, err
	}
	if got.Metadata.State == service.Permission {
		return got, report, service.ErrScope
	}
	if got.Restart {
		return got, report, service.ErrConflict
	}
	if got.Metadata.State == service.Unavailable {
		return got, report, service.ErrInvalid
	}
	return got, report, nil
}
