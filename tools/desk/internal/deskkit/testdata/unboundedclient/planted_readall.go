package planted

// Positive control for the discarded-io.ReadAll-error shapes beyond a literal `x, _ :=`.
// Each function holds one shape the class guard must report; the function name says which.
// This file is never compiled.

import (
	"io"
	"io/ioutil"
	"net/http"
)

func plantedBlankAssign(resp *http.Response) {
	_, _ = io.ReadAll(resp.Body)
}

var plantedRaw, _ = io.ReadAll(nil)

func plantedVarDecl(resp *http.Response) []byte {
	var raw, _ = io.ReadAll(resp.Body)
	return raw
}

func plantedIgnored(resp *http.Response) {
	io.ReadAll(resp.Body)
}

func plantedUnchecked(resp *http.Response) []byte {
	raw, err := io.ReadAll(resp.Body)
	return raw
}

func plantedBlanked(resp *http.Response) []byte {
	raw, err := io.ReadAll(resp.Body)
	_ = err
	return raw
}

func plantedOverwritten(resp *http.Response) ([]byte, error) {
	var err error
	raw, err := io.ReadAll(resp.Body)
	err = resp.Body.Close()
	return raw, err
}

func plantedIoutil(resp *http.Response) []byte {
	raw, _ := ioutil.ReadAll(resp.Body)
	return raw
}

func readBody(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

func plantedWrapper(resp *http.Response) []byte {
	raw, _ := readBody(resp.Body)
	return raw
}
