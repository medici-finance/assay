// Package elsewhere plants an uncapped body read in a module beside the desk module, outside
// every directory the inventory used to list. It is never built.
package elsewhere

import (
	"io"
	"net/http"
)

func fetch(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}
