package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPublicAssayModuleCache(t *testing.T) {
	stubPublicAssay(t, http.StatusOK, publicAssayMetadata)
	fx := newContainerFixture(t)
	withModuleRoots(t, fx)
	plantRoot(t, fx, "module example.com/client\n\ngo 1.27.0\n\nrequire github.com/medici-finance/assay/tools/desk v0.0.0-20260908135545-8217692d48e4\n", "github.com/medici-finance/assay/tools/desk v0.0.0-20260908135545-8217692d48e4 "+publicAssayFixtureSum()+"\ngithub.com/medici-finance/assay/tools/desk v0.0.0-20260908135545-8217692d48e4/go.mod "+publicAssayFixtureSum()+"\n")
	orig := operatorPrivateModulePatterns
	operatorPrivateModulePatterns = func() []string { return nil }
	t.Cleanup(func() { operatorPrivateModulePatterns = orig })
	dl := stubDownload(t)
	stubAmbient(t)
	var stdout bytes.Buffer
	receipt, err := prepareModuleCache(fx.home, filepath.Join(t.TempDir(), "cache"), []string{"medici-finance"}, &stdout)
	if err != nil || len(dl.calls) != 1 {
		t.Fatalf("public Assay dependency: downloads=%d err=%v", len(dl.calls), err)
	}
	if receipt.PublicAssay == nil || len(receipt.PublicAssay.Modules) != 1 {
		t.Fatalf("no exact provenance: %+v", receipt)
	}
	if err := moduleCacheRequirementProblem(fx.home, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.PublicAssay.Modules[0].Version = "v1.0.0"
	if err := moduleCacheRequirementProblem(fx.home, &receipt); err == nil {
		t.Fatal("receipt accepted a different module version")
	}
	if !strings.Contains(stdout.String(), publicAssayAPI) {
		t.Fatal("proof endpoint not announced")
	}
}

const publicAssayMetadata = `{"id":1321833604,"full_name":"medici-finance/assay","private":false,"visibility":"public","html_url":"https://github.com/medici-finance/assay","owner":{"id":298844171,"login":"medici-finance"}}`

type publicAssayDoer func(*http.Request) (*http.Response, error)

func (f publicAssayDoer) Do(r *http.Request) (*http.Response, error) { return f(r) }
func stubPublicAssay(t *testing.T, status int, body string) *int {
	t.Helper()
	count := 0
	orig := publicAssayHTTP
	publicAssayHTTP = publicAssayDoer(func(r *http.Request) (*http.Response, error) {
		count++
		if r.Method != http.MethodGet || r.URL.String() != publicAssayAPI || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Fatalf("unexpected proof request: %s %s", r.Method, r.URL)
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	t.Cleanup(func() { publicAssayHTTP = orig })
	return &count
}

func TestPublicAssayProofFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"private", 200, strings.Replace(publicAssayMetadata, `"private":false`, `"private":true`, 1)},
		{"missing private", 200, strings.Replace(publicAssayMetadata, `"private":false,`, "", 1)},
		{"internal", 200, strings.Replace(publicAssayMetadata, `"visibility":"public"`, `"visibility":"internal"`, 1)},
		{"renamed", 200, strings.ReplaceAll(publicAssayMetadata, "medici-finance/assay", "medici-finance/assay-moved")},
		{"recreated", 200, strings.Replace(publicAssayMetadata, "1321833604", "42", 1)},
		{"different owner", 200, strings.Replace(publicAssayMetadata, "298844171", "42", 1)},
		{"unknown", 404, "not found"}, {"rate limited", 403, "limit"}, {"redirect", 302, "redirect"},
		{"malformed", 200, "{"}, {"oversized", 200, strings.Repeat(" ", 65537)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubPublicAssay(t, tc.status, tc.body)
			_, err := provePublicAssay(nil, io.Discard)
			if err == nil || !strings.Contains(err.Error(), "nothing was sent to the proxy") {
				t.Fatalf("did not fail closed: %v", err)
			}
		})
	}
	orig := publicAssayHTTP
	publicAssayHTTP = publicAssayDoer(func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("network unavailable") })
	t.Cleanup(func() { publicAssayHTTP = orig })
	if _, err := provePublicAssay(nil, io.Discard); err == nil {
		t.Fatal("accepted unavailable proof")
	}
}

func TestPublicAssayScopeAndPrivateDenials(t *testing.T) {
	good := "github.com/medici-finance/assay/tools/desk"
	version := "v0.0.0-20260908135545-8217692d48e4"
	for _, tc := range []struct{ name, path, private, sum string }{
		{name: "private repository", path: "github.com/medici-finance/secret"},
		{name: "unknown repository", path: "github.com/medici-finance/unknown"},
		{name: "repository lookalike", path: "github.com/medici-finance/assay-extra/tools/desk"},
		{name: "case variation", path: "github.com/Medici-Finance/assay/tools/desk"},
		{name: "operator private repository", path: good, private: "github.com/medici-finance/assay"},
		{name: "operator private owner", path: good, private: "github.com/medici-finance/*"},
		{name: "missing checksums", path: good},
		{name: "go mod checksum alone", path: good, sum: good + " " + version + "/go.mod " + publicAssayFixtureSum() + "\n"},
		{name: "wrong version checksums", path: good, sum: good + " v1.0.0 " + publicAssayFixtureSum() + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count := stubPublicAssay(t, 200, publicAssayMetadata)
			orig := operatorPrivateModulePatterns
			operatorPrivateModulePatterns = func() []string {
				if tc.private != "" {
					return []string{tc.private}
				}
				return nil
			}
			t.Cleanup(func() { operatorPrivateModulePatterns = orig })
			fx := newContainerFixture(t)
			withModuleRoots(t, fx)
			sum := tc.sum
			if tc.private != "" {
				sum = good + " " + version + " " + publicAssayFixtureSum() + "\n" + good + " " + version + "/go.mod " + publicAssayFixtureSum() + "\n"
			}
			plantRoot(t, fx, "module example.com/client\n\ngo 1.27.0\n\nrequire "+tc.path+" "+version+"\n", sum)
			dl := stubDownload(t)
			stubAmbient(t)
			cache := filepath.Join(t.TempDir(), "cache")
			_, err := prepareModuleCache(fx.home, cache, []string{"medici-finance"}, io.Discard)
			if err == nil || len(dl.calls) != 0 || *count != 0 {
				t.Fatalf("refusal: err=%v downloads=%v proof calls=%d", err, dl.calls, *count)
			}
			if _, err := os.Stat(cache); !os.IsNotExist(err) {
				t.Fatal("refused input created cache")
			}
		})
	}
	for _, p := range []string{good + "/../secret", good + "//x", good + "/!x", good + "/.", "github.com/medici-finance/assay.evil/x"} {
		if isPublicAssayModule(p) {
			t.Errorf("accepted malformed/lookalike path %q", p)
		}
	}
}

func TestPublicAssayProofBeforeDownloadAndReceipt(t *testing.T) {
	fx := newContainerFixture(t)
	withModuleRoots(t, fx)
	path := "github.com/medici-finance/assay/tools/desk"
	v := "v1.0.0"
	sum := path + " " + v + " " + publicAssayFixtureSum() + "\n" + path + " " + v + "/go.mod " + publicAssayFixtureSum() + "\n"
	plantRoot(t, fx, "module example.com/client\n\ngo 1.27.0\n\nrequire "+path+" "+v+"\n", sum)
	orig := operatorPrivateModulePatterns
	operatorPrivateModulePatterns = func() []string { return nil }
	t.Cleanup(func() { operatorPrivateModulePatterns = orig })
	count := stubPublicAssay(t, 404, "not found")
	dl := stubDownload(t)
	stubAmbient(t)
	var stdout bytes.Buffer
	cache := filepath.Join(t.TempDir(), "cache")
	putFile(t, fx.check, validCheck)
	if code := runOpenBracket(fx.home, fx.brief, fx.receipt, cache, &stdout, io.Discard); code == verifyrunExitPass {
		t.Fatal("opened without proof")
	}
	if *count != 1 || len(dl.calls) != 0 {
		t.Fatalf("proof calls=%d downloads=%v", *count, dl.calls)
	}
	for _, p := range []string{cache, fx.receipt} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("refused proof wrote %s", p)
		}
	}
	count2 := stubPublicAssay(t, 200, publicAssayMetadata)
	inner := goModDownload
	goModDownload = func(dir string, env []string) ([]byte, error) {
		if *count2 != 1 || !strings.Contains(stdout.String(), publicAssayAPI) {
			t.Fatal("download preceded proof/announcement")
		}
		for _, want := range []string{"GOVCS=*:off", "GOPROXY=https://proxy.golang.org", "GOSUMDB=sum.golang.org", "GOFLAGS=-mod=readonly"} {
			found := false
			for _, kv := range env {
				if kv == want {
					found = true
				}
			}
			if !found {
				t.Fatalf("download lacks %s", want)
			}
		}
		return inner(dir, env)
	}
	if code := runOpenBracket(fx.home, fx.brief, fx.receipt, cache, &stdout, io.Discard); code != verifyrunExitPass {
		t.Fatalf("open exit %d: %s", code, stdout.String())
	}
	var receipt bracketReceipt
	if err := json.Unmarshal([]byte(getFile(t, fx.receipt)), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.ModuleCache == nil || receipt.ModuleCache.PublicAssay == nil {
		t.Fatal("no source-bound proof in receipt")
	}
	b := receipt.ModuleCache.PublicAssay.Modules[0]
	if !reflect.DeepEqual(b, publicAssayModule{Root: "tools/desk", Path: path, Version: v, Sum: publicAssayFixtureSum(), GoModSum: publicAssayFixtureSum()}) {
		t.Fatalf("wrong binding %+v", b)
	}
	if err := moduleCacheRequirementProblem(fx.home, receipt.ModuleCache); err != nil {
		t.Fatal(err)
	}
	receipt.ModuleCache.PublicAssay = nil
	if err := moduleCacheRequirementProblem(fx.home, receipt.ModuleCache); err == nil {
		t.Fatal("admitted stripped public proof")
	}
}

func TestPublicAssayHTTPEnvelope(t *testing.T) {
	c, ok := publicAssayHTTP.(*http.Client)
	if !ok || c.Timeout <= 0 || c.Jar != nil || c.CheckRedirect == nil {
		t.Fatal("proof client lost its closed unauthenticated envelope")
	}
	if err := c.CheckRedirect(&http.Request{}, nil); err != http.ErrUseLastResponse {
		t.Fatal("proof client allows redirects")
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok || tr.Proxy != nil || tr.TLSClientConfig != nil {
		t.Fatal("proof client inherits proxy/TLS overrides")
	}
}

// Fixture checksum of 32 zero bytes: structurally valid h1 data, no credential.
func publicAssayFixtureSum() string {
	return "h1:" + base64.StdEncoding.EncodeToString(make([]byte, 32))
}
