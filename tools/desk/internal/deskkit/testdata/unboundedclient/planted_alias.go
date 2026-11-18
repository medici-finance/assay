package planted

// Positive control: the same deadline-free shapes as planted.go, written through aliased
// imports, which the class guard must resolve. This file is never compiled.

import (
	stdio "io"
	nethttp "net/http"
)

func plantedAliasDefault(req *nethttp.Request) (*nethttp.Response, error) {
	return nethttp.DefaultClient.Do(req)
}

func plantedAliasLiteral() *nethttp.Client {
	return &nethttp.Client{}
}

func plantedAliasSwallow(resp *nethttp.Response) []byte {
	raw, _ := stdio.ReadAll(resp.Body)
	return raw
}
