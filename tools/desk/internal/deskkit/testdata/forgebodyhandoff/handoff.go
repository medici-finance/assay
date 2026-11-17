// Package handoff plants a response body handed to a helper that takes a plain io.Reader, in
// a root with no transport cap. It is never built.
package handoff

import (
	"encoding/json"
	"net/http"

	stdio "io"
)

func readCapped(body stdio.Reader, what string, limit int64) ([]byte, error) {
	return nil, nil
}

func parse(r stdio.Reader, v any) error {
	return json.Unmarshal(nil, v)
}

func handedOff(resp *http.Response, v any) error {
	return parse(resp.Body, v)
}

// cappedHandoff hands the body to the capped reader; the guard must not report it.
func cappedHandoff(resp *http.Response) ([]byte, error) {
	return readCapped(resp.Body, "GET /example", 1<<20)
}

// closeOnly closes the body; the guard must not report it.
func closeOnly(resp *http.Response) error {
	return resp.Body.Close()
}
