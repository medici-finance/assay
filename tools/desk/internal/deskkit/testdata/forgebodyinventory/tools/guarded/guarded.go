// Package guarded stands for a guard root: the inventory must skip it. It is never built.
package guarded

import (
	"io"
	"net/http"
)

func fetch(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}
