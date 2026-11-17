// Package gaps plants the forge-body guard's former blind spots: each function, or each
// package-level function value, is one shape the guard must report. It is never built.
package gaps

import (
	"bytes"
	"net/http"

	stdio "io"

	"example.invalid/deskkit"
)

// viaLimitBody returns a call of limitBody, a helper that returns a truncating reader. It is
// declared first, so one pass in source order cannot see it.
func viaLimitBody(resp *http.Response) stdio.Reader {
	return limitBody(resp)
}

func limitBody(resp *http.Response) stdio.Reader {
	return stdio.LimitReader(resp.Body, 1<<20)
}

func helperTruncCapped(resp *http.Response) ([]byte, error) {
	return deskkit.ReadCappedForgeBody(limitBody(resp), "GET /example")
}

func helper2TruncCapped(resp *http.Response) ([]byte, error) {
	return deskkit.ReadCappedForgeBody(viaLimitBody(resp), "GET /example")
}

func twoHopCapped(resp *http.Response) ([]byte, error) {
	lr := stdio.LimitReader(resp.Body, 1<<20)
	r2 := lr
	return deskkit.ReadCappedForgeBody(r2, "GET /example")
}

// loopHopCapped assigns r2 from lr before lr is assigned in source order; one pass in source
// order cannot see it.
func loopHopCapped(resp *http.Response) ([]byte, error) {
	var lr, r2 stdio.Reader
	for i := 0; ; i++ {
		if i == 2 {
			return deskkit.ReadCappedForgeBody(r2, "GET /example")
		}
		r2 = lr
		lr = stdio.LimitReader(resp.Body, 1<<20)
	}
}

func bodyReassignCapped(resp *http.Response) ([]byte, error) {
	resp.Body = stdio.NopCloser(stdio.LimitReader(resp.Body, 1<<20))
	return deskkit.ReadCappedForgeBody(resp.Body, "GET /example")
}

// pkgLitRead and pkgTable are function literals outside every function declaration.
var pkgLitRead = func(resp *http.Response) ([]byte, error) {
	return stdio.ReadAll(resp.Body)
}

var pkgTable = map[string]func(*http.Response) ([]byte, error){
	"read": func(resp *http.Response) ([]byte, error) {
		return stdio.ReadAll(resp.Body)
	},
}

// a and b each have a drain method: the two reads must be two distinct keys, so one permit
// can never cover the other.
type a struct{}

type b struct{}

func (a) drain(resp *http.Response) error {
	_, err := stdio.Copy(stdio.Discard, resp.Body)
	return err
}

func (*b) drain(resp *http.Response) error {
	_, err := stdio.Copy(stdio.Discard, resp.Body)
	return err
}

// aliasByName reads a byte slice, not the body: the name r was bound to the body in another
// scope. Matching by name over-reports it, and that over-report is kept by decision.
func aliasByName(resp *http.Response, buf []byte) ([]byte, error) {
	if resp != nil {
		r := resp.Body
		_ = r
	}
	r := bytes.NewReader(buf)
	return stdio.ReadAll(r)
}

// helperDirectCapped hands the body itself to the capped reader; the guard must not report it.
func helperDirectCapped(resp *http.Response) ([]byte, error) {
	return deskkit.ReadCappedForgeBody(resp.Body, "GET /example")
}
