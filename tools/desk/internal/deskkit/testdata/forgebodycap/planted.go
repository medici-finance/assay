// Package planted is the forge-body guard's positive control: each function is one shape the
// guard must report (or, for cappedRead, must not). It is never built.
package planted

import (
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
