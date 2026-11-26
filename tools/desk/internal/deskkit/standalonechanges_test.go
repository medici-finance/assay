package deskkit

import (
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestStandaloneChangeClientExposesOnlyScopedOperations(t *testing.T) {
	typ := reflect.TypeOf(NewStandaloneChanges).Out(0)
	var methods []string
	for i := 0; i < typ.NumMethod(); i++ {
		methods = append(methods, typ.Method(i).Name)
	}
	want := []string{"Change", "ChangedFiles", "Comments", "OpenChanges", "PublishComment"}
	if typ != reflect.TypeOf((*StandaloneChanges)(nil)) || !reflect.DeepEqual(methods, want) {
		t.Fatalf("standalone constructor exposed %v with methods %v; want only %v", typ, methods, want)
	}
}

type standaloneTestTransport func(*http.Request) (*http.Response, error)

func (f standaloneTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestStandaloneTransportRejectsOtherOrigins(t *testing.T) {
	calls := 0
	client := standaloneBoundHTTP(&http.Client{Transport: standaloneTestTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}, GitHubAPIBase)
	resp, err := client.Get("https://other.example/fixture")
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "left its configured HTTPS origin") || calls != 0 {
		t.Fatalf("off-origin request reached transport %d times: %v", calls, err)
	}
}

func TestStandaloneTransportRejectsRedirectResponse(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "https://api.github.com/fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	transport := standaloneOriginTransport{host: "api.github.com", next: standaloneTestTransport(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusTemporaryRedirect,
			Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	response, err := transport.RoundTrip(request)
	if response != nil {
		_ = response.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "forge returned a redirect") || response != nil {
		t.Fatalf("redirect reached the provider: %v", err)
	}
}
