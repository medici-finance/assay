package planted

// Positive control for forgetimeout_test.go: one instance of each deadline-free client shape
// the class guard must report — a reference to http.DefaultClient, a package-level http.Get,
// a go-gh ClientOptions literal and an http.Client literal with no Timeout, and a discarded
// io.ReadAll error. This file is never compiled.

import (
	"io"
	"net/http"

	ghapi "github.com/cli/go-gh/v2/pkg/api"
)

func plantedDefault(req *http.Request) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

func plantedGet() (*http.Response, error) {
	return http.Get("https://api.example.invalid/")
}

func plantedOptions(tok string) (*ghapi.RESTClient, error) {
	return ghapi.NewRESTClient(ghapi.ClientOptions{Host: "api.example.invalid", AuthToken: tok})
}

func plantedLiteral() *http.Client {
	return &http.Client{}
}

func plantedSwallow(resp *http.Response) []byte {
	raw, _ := io.ReadAll(resp.Body)
	return raw
}
