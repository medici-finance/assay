// Package server is preparation for the read-only forge-service handler.
// The wire-only preparation is unqualified. It opens no listener and grants
// no caller authority or forge access.
package server

import (
	"io"
	"mime"
	"net/http"

	"github.com/medici-finance/assay/forgeread/service"
)

// DecodeMirrorQuery bounds and decodes a request before any authority, store or
// source access. The returned scope and observation token are caller claims.
// A serving caller must independently authorize them before using the request.
func DecodeMirrorQuery(r *http.Request) (service.Request, error) {
	if r == nil || r.Method != http.MethodPost || r.Body == nil || r.ContentLength > service.MaxMessageBytes || r.Header.Get("Content-Encoding") != "" {
		return service.Request{}, service.ErrInvalid
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return service.Request{}, service.ErrInvalid
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, service.MaxMessageBytes+1))
	if err != nil || len(body) > service.MaxMessageBytes {
		return service.Request{}, service.ErrInvalid
	}
	query, err := service.DecodeRequest(body)
	if err != nil {
		return service.Request{}, err
	}
	if query.Operation != service.MirrorQuery {
		return service.Request{}, service.ErrSchema
	}
	return query, nil
}
