// Package planted is the forge-body guard's positive control: each function is one shape the
// guard must report (or, for cappedRead, must not). It is never built.
package planted

import (
	"bytes"
	"encoding/json"
	"net/http"

	stdio "io"

	"example.invalid/deskkit"
)

func truncatingRead(resp *http.Response) ([]byte, error) {
	return stdio.ReadAll(stdio.LimitReader(resp.Body, 4<<20))
}

func aliasedRead(resp *http.Response) []byte {
	b, err := stdio.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	return b
}

func discardedRead(resp *http.Response) []byte {
	b, _ := stdio.ReadAll(resp.Body)
	return b
}

func discardedCapped(resp *http.Response) []byte {
	b, _ := deskkit.ReadCappedForgeBody(resp.Body, "GET /example")
	return b
}

func cappedRead(resp *http.Response) ([]byte, error) {
	return deskkit.ReadCappedForgeBody(resp.Body, "GET /example")
}

func decodeRead(resp *http.Response, v any) error {
	return json.NewDecoder(resp.Body).Decode(v)
}

func copyRead(resp *http.Response) ([]byte, error) {
	var buf bytes.Buffer
	_, err := stdio.Copy(&buf, resp.Body)
	return buf.Bytes(), err
}

func bodyAliasRead(resp *http.Response) ([]byte, error) {
	r := resp.Body
	return stdio.ReadAll(r)
}

// permitThenRead drains a status probe (the read a permit may cover) and then reads the body
// whole — a second read the drain's permit must not cover.
func permitThenRead(resp *http.Response) ([]byte, error) {
	_, _ = stdio.Copy(stdio.Discard, resp.Body)
	return stdio.ReadAll(resp.Body)
}

func truncatedCapped(resp *http.Response) ([]byte, error) {
	return deskkit.ReadForgeBodyUpTo(stdio.LimitReader(resp.Body, 1<<20), "GET /example", 1<<20)
}

func truncatedAliasCapped(resp *http.Response) ([]byte, error) {
	lr := stdio.LimitReader(resp.Body, 1<<20)
	return deskkit.ReadCappedForgeBody(lr, "GET /example")
}

func truncatedLimitedCapped(resp *http.Response) ([]byte, error) {
	return deskkit.ReadCappedForgeBody(&stdio.LimitedReader{R: resp.Body, N: 1 << 20}, "GET /example")
}
