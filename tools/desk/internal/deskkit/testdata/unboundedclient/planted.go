package planted

// Positive control for forgetimeout_test.go: one instance of each deadline-free client shape
// the class guard must report — a reference to http.DefaultClient, a package-level http.Get,
// and a go-gh ClientOptions literal with no Timeout. This file is never compiled.

import (
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
