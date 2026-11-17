// Package sub proves the forge-body guard scans subdirectories of a root. It is never built.
package sub

import (
	"io"
	"net/http"
)

func nestedRead(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}
