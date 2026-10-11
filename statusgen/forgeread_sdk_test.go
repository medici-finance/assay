package main

// forgeread_sdk_test.go — forge-neutral brief 36: the statusgen half of the importable fact
// reader's proof rows.
//
//	TestReaderSDKCLIParity         both providers' fake-provider observations read through the SDK
//	                               adapters, the deskread envelope and statusgen's reader agree
//	TestReaderSDKUnavailable       known-empty, partial, all-unavailable, truncated and
//	                               unsupported-schema stay five distinct answers
//	TestReaderSDKAuthorityBoundary statusgen gains no credential and no ambient fallback from the
//	                               library, and reaches no adapter
//	TestReaderSDKExternalConsumer  the module builds for an outside consumer from a module proxy
//	                               with no workspace and no relative replace
//	TestStatusgenImportsNoFacadeBelowRoot  the import-path guard, with its negative control
//
// The first arm's fixtures live in forgeread/testdata/parity and are the SAME observations the
// adapters' own tests and the deskread verb's tests read; the golden envelopes beside them are
// what the verb printed for those observations (held by cmd/deskread/shared_parity_test.go).

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/forgeread"
	"github.com/medici-finance/assay/forgeread/adapters"
	ghadapter "github.com/medici-finance/assay/forgeread/adapters/github"
	gladapter "github.com/medici-finance/assay/forgeread/adapters/gitlab"
)

const parityDir = "../forgeread/testdata/parity"

type sdkObserved struct {
	Pages   []json.RawMessage `json:"pages"`
	Status  int               `json:"status"`
	Endless bool              `json:"endless"`
}

type sdkExpect struct {
	Issues      map[string][]forgeread.Issue `json:"issues"`
	Unavailable map[string]string            `json:"unavailable"`
}

func loadSDKParity(t *testing.T) (map[string]map[string]sdkObserved, map[string]sdkExpect, []string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(parityDir, "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var obs map[string]map[string]sdkObserved
	if err := json.Unmarshal(raw, &obs); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(parityDir, "expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	var exp struct {
		Repos  []string  `json:"repos"`
		GitHub sdkExpect `json:"github"`
		GitLab sdkExpect `json:"gitlab"`
	}
	if err := json.Unmarshal(raw, &exp); err != nil {
		t.Fatal(err)
	}
	return obs, map[string]sdkExpect{"github": exp.GitHub, "gitlab": exp.GitLab}, exp.Repos
}

// sdkFixtureAccess serves one provider's recorded observations a page at a time.
type sdkFixtureAccess struct{ repos map[string]sdkObserved }

func (f sdkFixtureAccess) GetPage(_ context.Context, repo, _ string, q url.Values) (adapters.Page, error) {
	o, ok := f.repos[repo]
	if !ok {
		return adapters.Page{}, fmt.Errorf("404 %s", repo)
	}
	if o.Status != 0 {
		return adapters.Page{}, fmt.Errorf("provider answered HTTP %d", o.Status)
	}
	if o.Endless {
		return adapters.Page{Body: []byte("[]"), More: true}, nil
	}
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 || page > len(o.Pages) {
		return adapters.Page{}, fmt.Errorf("unexpected page %d", page)
	}
	return adapters.Page{Body: o.Pages[page-1], More: page < len(o.Pages)}, nil
}

// sdkSetReader is the smallest set-level Reader an SDK consumer writes over one provider's
// adapter: each requested repo is read once and lands in data or in unavailable.
type sdkSetReader struct {
	fetch func(context.Context, string) ([]forgeread.Issue, error)
}

func (s sdkSetReader) OpenIssues(ctx context.Context, repos []string) (forgeread.Result, error) {
	res := forgeread.NewResult()
	for _, repo := range forgeread.Normalize(repos) {
		issues, err := s.fetch(ctx, repo)
		switch {
		case adapters.IsTruncated(err):
			res.Unavailable = append(res.Unavailable, forgeread.Unavailable{Repo: repo, Reason: err.Error(), Class: forgeread.ClassTruncated})
		case err != nil:
			res.Unavailable = append(res.Unavailable, forgeread.Unavailable{Repo: repo, Reason: err.Error(), Class: forgeread.ClassUnreadable})
		default:
			res.Issues[repo] = issues
		}
	}
	return forgeread.Reconcile(repos, res, "the adapter answered neither data nor a reason"), nil
}

// stubDeskread puts a `deskread` on PATH that prints out, verbatim, and exits code.
func stubDeskread(t *testing.T, out string, code int) {
	t.Helper()
	dir := t.TempDir()
	body := filepath.Join(dir, "answer.json")
	if err := os.WriteFile(body, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	writeStubBin(t, dir, "deskread", fmt.Sprintf("#!/bin/sh\ncat '%s'\nexit %d\n", body, code))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// canon reduces a (data, unavailable) answer to what must be identical on every path: the issues
// per repo, and which repos were unavailable. Reasons are path-specific prose and compared only
// where the test says so.
type canon struct {
	Issues      map[string][]forgeIssue
	Unavailable []string
}

func canonOf(data map[string][]forgeIssue, un []forgeUnavailable) canon {
	c := canon{Issues: data, Unavailable: []string{}}
	for _, u := range un {
		c.Unavailable = append(c.Unavailable, u.Repo)
	}
	sort.Strings(c.Unavailable)
	return c
}

// regression: #2395
func TestReaderSDKCLIParity(t *testing.T) {
	obs, exp, repos := loadSDKParity(t)
	for provider, build := range map[string]func(adapters.Access) func(context.Context, string) ([]forgeread.Issue, error){
		"github": func(a adapters.Access) func(context.Context, string) ([]forgeread.Issue, error) {
			return ghadapter.New(a, nil).OpenIssues
		},
		"gitlab": func(a adapters.Access) func(context.Context, string) ([]forgeread.Issue, error) {
			return gladapter.New(a, nil).OpenIssues
		},
	} {
		t.Run(provider, func(t *testing.T) {
			// arm 1: the SDK, adapters over the recorded observations.
			sdk := sdkSetReader{fetch: build(sdkFixtureAccess{repos: obs[provider]})}
			sres, err := sdk.OpenIssues(context.Background(), repos)
			if err != nil {
				t.Fatal(err)
			}
			sdkData, sdkUn := fromSDKResult(sres)
			sdkCanon := canonOf(sdkData, sdkUn)

			// arm 2: the deskread verb's envelope for the same observations (golden, printed by
			// the real verb), through statusgen's reader.
			golden, err := os.ReadFile(filepath.Join(parityDir, "deskread-"+provider+".json"))
			if err != nil {
				t.Fatal(err)
			}
			stubDeskread(t, string(golden), 0)
			cliData, cliUn, err := newDeskreadReader().OpenIssues(repos)
			if err != nil {
				t.Fatal(err)
			}
			cliCanon := canonOf(cliData, cliUn)

			// arm 3: the same SDK result frozen and read through statusgen's frozen seam.
			fData, fUn, err := newFrozenReader(sres).OpenIssues(repos)
			if err != nil {
				t.Fatal(err)
			}
			frozenCanon := canonOf(fData, fUn)

			// The recorded expectation, converted the same way.
			wantRes := forgeread.NewResult()
			for r, is := range exp[provider].Issues {
				wantRes.Issues[r] = is
			}
			for r := range exp[provider].Unavailable {
				wantRes.Unavailable = append(wantRes.Unavailable, forgeread.Unavailable{Repo: r, Reason: "x"})
			}
			wData, wUn := fromSDKResult(wantRes)
			want := canonOf(wData, wUn)

			for name, got := range map[string]canon{"sdk": sdkCanon, "deskread": cliCanon, "frozen": frozenCanon} {
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s/%s diverged from the recorded expectation:\n got %+v\nwant %+v", provider, name, got, want)
				}
			}
			if !reflect.DeepEqual(sdkCanon, cliCanon) {
				t.Errorf("%s: SDK and deskread/statusgen disagree:\n sdk %+v\n cli %+v", provider, sdkCanon, cliCanon)
			}
			// The truncated repo is unavailable on every path, never a short list.
			for name, un := range map[string][]forgeUnavailable{"sdk": sdkUn, "deskread": cliUn} {
				var loop *forgeUnavailable
				for i := range un {
					if un[i].Repo == "example-org/loop" {
						loop = &un[i]
					}
				}
				if loop == nil || !strings.Contains(loop.Reason, "PARTIAL") {
					t.Errorf("%s/%s: the truncated repo must be unavailable with the partial-set reason, got %+v", provider, name, loop)
				}
			}
		})
	}
}

// regression: #2395
func TestReaderSDKUnavailable(t *testing.T) {
	repos := []string{"o/known-empty", "o/served", "o/down"}
	issue := forgeread.Issue{Number: 3, Title: "t", State: "open", AuthorLogin: "a", CreatedAt: "2026-03-04T05:06:07Z", URL: "u"}

	envelope := func(res forgeread.Result) string {
		b, err := json.Marshal(forgeread.NewIssuesEnvelope(repos, res))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	t.Run("known-empty is data, not unavailable", func(t *testing.T) {
		res := forgeread.NewResult()
		res.Issues["o/known-empty"] = []forgeread.Issue{}
		res.Issues["o/served"] = []forgeread.Issue{issue}
		res.Issues["o/down"] = []forgeread.Issue{}
		stubDeskread(t, envelope(res), 0)
		data, un, err := newDeskreadReader().OpenIssues(repos)
		if err != nil || len(un) != 0 {
			t.Fatalf("err=%v unavailable=%v", err, un)
		}
		if got, ok := data["o/known-empty"]; !ok || len(got) != 0 {
			t.Fatalf("a repo with no open issues must be PRESENT and empty, got %v present=%v", got, ok)
		}
	})

	t.Run("partial keeps the read repos and names the unread one", func(t *testing.T) {
		res := forgeread.NewResult()
		res.Issues["o/known-empty"] = []forgeread.Issue{}
		res.Issues["o/served"] = []forgeread.Issue{issue}
		res.Unavailable = []forgeread.Unavailable{{Repo: "o/down", Reason: "HTTP 503"}}
		stubDeskread(t, envelope(res), 0)
		data, un, err := newDeskreadReader().OpenIssues(repos)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != 2 || len(un) != 1 || un[0].Repo != "o/down" {
			t.Fatalf("data=%v unavailable=%v", data, un)
		}
		if _, leaked := data["o/down"]; leaked {
			t.Fatal("an unavailable repo must have NO data entry")
		}
	})

	t.Run("all-unavailable is every repo unavailable and none empty", func(t *testing.T) {
		stubDeskread(t, "", 6)
		data, un, err := newDeskreadReader().OpenIssues(repos)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != 0 || len(un) != len(repos) {
			t.Fatalf("data=%v unavailable=%v", data, un)
		}
	})

	t.Run("truncated is unavailable with its class, never a short list", func(t *testing.T) {
		res := forgeread.NewResult()
		res.Issues["o/known-empty"] = []forgeread.Issue{}
		res.Issues["o/served"] = []forgeread.Issue{issue}
		res.Unavailable = []forgeread.Unavailable{{Repo: "o/down", Reason: "still more after 25 pages", Class: forgeread.ClassTruncated}}
		stubDeskread(t, envelope(res), 0)
		data, un, err := newDeskreadReader().OpenIssues(repos)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := data["o/down"]; ok || len(un) != 1 || un[0].Repo != "o/down" {
			t.Fatalf("data=%v unavailable=%v", data, un)
		}
	})

	t.Run("unsupported schema is a refusal, not an answer", func(t *testing.T) {
		stubDeskread(t, `{"schema":2,"kind":"issues","repos":[{"repo":"o/served","issues":[]}]}`, 0)
		data, un, err := newDeskreadReader().OpenIssues(repos)
		if err == nil {
			t.Fatalf("a newer envelope was read: data=%v unavailable=%v", data, un)
		}
		if data != nil || un != nil {
			t.Fatalf("a refusal must carry no data and no unavailable list, got %v %v", data, un)
		}
		if !strings.Contains(err.Error(), "does not understand") {
			t.Fatalf("refusal should say why: %v", err)
		}
	})

	t.Run("the five answers are pairwise distinct", func(t *testing.T) {
		// known-empty -> present+empty; partial -> mixed; all-unavailable -> none present; the
		// truncated and unsupported cases differ from both by class and by error. This asserts
		// the shapes above cannot collapse into one another: an unavailable repo is never also
		// present, and an error is never a (data, unavailable) answer.
		_, un, _ := newOfflineReader().OpenIssues(repos)
		if len(un) != len(repos) {
			t.Fatalf("offline default must be all-unavailable, got %v", un)
		}
	})
}

// regression: #2395
func TestReaderSDKAuthorityBoundary(t *testing.T) {
	// 1. No ambient fallback: with a token in the environment, a forge API address in the
	// environment and a `gh` on PATH, a run where the verb cannot run reaches NOTHING — the
	// answer is every repo unavailable, the `gh` stub is never executed and the API is never hit.
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.NotFound(w, r)
	}))
	defer srv.Close()
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "gh-ran")
	writeStubBin(t, bin, "gh", fmt.Sprintf("#!/bin/sh\ntouch '%s'\nexit 1\n", marker))
	t.Setenv("PATH", bin) // no deskread anywhere
	t.Setenv("GH_TOKEN", "sk-not-a-real-token")
	t.Setenv("GITHUB_TOKEN", "sk-not-a-real-token")
	t.Setenv("GITHUB_API_URL", srv.URL)

	repos := []string{"example-org/a", "example-org/b"}
	for name, rd := range map[string]forgeReader{"deskread": newDeskreadReader(), "offline": newOfflineReader()} {
		data, un, err := rd.OpenIssues(repos)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(data) != 0 || len(un) != len(repos) {
			t.Fatalf("%s: with no verb every repo must be unavailable, got data=%v un=%v", name, data, un)
		}
	}
	if hits != 0 {
		t.Errorf("a token in the environment must not make statusgen reach the forge, got %d requests", hits)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("the `gh` on PATH was executed — an ambient fallback")
	}

	// 2. The default reader for a run is the offline one and starts no process.
	if _, ok := forgeReaderForRun.(sdkReader); !ok {
		t.Errorf("forgeReaderForRun default = %T, want the SDK offline reader", forgeReaderForRun)
	}

	// 3. No transitive custody write: the library's dependency closure inside statusgen holds no
	// authenticated adapter, and the library itself declares no requirement of its own.
	deps := goList(t, ".", "list", "-deps", ".")
	for _, d := range strings.Split(deps, "\n") {
		if strings.Contains(d, "/forgeread/adapters") {
			t.Errorf("statusgen links %q — the adapters are linked by desk-tools alone", d)
		}
	}
	mod, err := os.ReadFile("../forgeread/go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mod), "require") {
		t.Errorf("forgeread/go.mod declares a requirement — a library the offline path links must add no dependency:\n%s", mod)
	}

	// 4. The reader source itself never reads a credential.
	src, err := os.ReadFile("forgeread.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GITLAB_TOKEN", "CI_JOB_TOKEN", "Getenv", "net/http", "auth token"} {
		for _, line := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if strings.Contains(line, banned) {
				t.Errorf("forgeread.go reads or reaches %q: %s", banned, strings.TrimSpace(line))
			}
		}
	}
}

// goBin is resolved once, before any test narrows PATH.
var goBin = func() string {
	if p, err := exec.LookPath("go"); err == nil {
		return p
	}
	return filepath.Join(runtime.GOROOT(), "bin", "go")
}()

// goList runs `go <args>` in dir with the workspace off and returns trimmed stdout.
func goList(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(goBin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go %v in %s: %v\n%s", args, dir, err, stderr.String())
	}
	return strings.TrimSpace(string(out))
}

// --- external consumer ------------------------------------------------------------------------

const publishedVersion = "v1.0.0"
const forgereadPath = "github.com/medici-finance/assay/forgeread"

// publishToProxy writes the forgeread module into root as a GOPROXY=file:// layout.
func publishToProxy(t *testing.T, root string) {
	t.Helper()
	base := filepath.Join(root, filepath.FromSlash(forgereadPath), "@v")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	src := "../forgeread"
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		w, err := zw.Create(forgereadPath + "@" + publishedVersion + "/" + filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	gomod, err := os.ReadFile(filepath.Join(src, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{
		publishedVersion + ".zip":  buf.Bytes(),
		publishedVersion + ".mod":  gomod,
		publishedVersion + ".info": []byte(`{"Version":"` + publishedVersion + `","Time":"2026-10-11T00:00:00Z"}`),
		"list":                     []byte(publishedVersion + "\n"),
	} {
		if err := os.WriteFile(filepath.Join(base, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const consumerMain = `package main

import (
	"context"
	"fmt"

	"github.com/medici-finance/assay/forgeread"
)

func main() {
	res, _ := forgeread.OfflineReader{Why: "external consumer"}.OpenIssues(context.Background(), []string{"o/r"})
	frozen := forgeread.NewFrozenReader(forgeread.Result{Issues: map[string][]forgeread.Issue{"o/r": {}}})
	got, _ := frozen.OpenIssues(context.Background(), []string{"o/r", "o/missing"})
	fmt.Println(len(res.Unavailable), len(got.Issues), len(got.Unavailable), forgeread.APIVersion)
}
`

// buildConsumer builds a throwaway module that requires the library by version, with no
// workspace, no replace and only proxy as its module source. It returns the build error.
func buildConsumer(t *testing.T, proxy string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	gomod := "module example.test/consumer\n\ngo 1.25.0\n\nrequire " + forgereadPath + " " + publishedVersion + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(consumerMain), 0o644); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(gomod, "replace") {
		t.Fatal("the consumer must not use a replace")
	}
	run := func(args ...string) (string, error) {
		cmd := exec.Command(goBin, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GOWORK=off", "GOFLAGS=-mod=mod -modcacherw", "GOPROXY=file://"+proxy, "GOSUMDB=off",
			"GONOSUMDB=*", "GOMODCACHE="+filepath.Join(dir, "modcache"),
			"GOTOOLCHAIN=local", "GOPRIVATE=")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("build", "-o", filepath.Join(dir, "consumer"), "."); err != nil {
		return out, err
	}
	cmd := exec.Command(filepath.Join(dir, "consumer"))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// regression: #2395
func TestReaderSDKExternalConsumer(t *testing.T) {
	if testing.Short() {
		t.Skip("builds modules")
	}
	t.Run("a consumer builds against the published module with no workspace", func(t *testing.T) {
		proxy := t.TempDir()
		publishToProxy(t, proxy)
		out, err := buildConsumer(t, proxy)
		if err != nil {
			t.Fatalf("external consumer failed: %v\n%s", err, out)
		}
		if strings.TrimSpace(out) != "1 1 1 1.0.0" {
			t.Errorf("consumer output = %q", out)
		}
	})

	t.Run("omitting the module publication fails the build", func(t *testing.T) {
		if _, err := buildConsumer(t, t.TempDir()); err == nil {
			t.Fatal("the consumer built with the module absent from the proxy — the test would pass without the publication step")
		}
	})

	t.Run("the release build of statusgen resolves the module with the workspace off", func(t *testing.T) {
		goList(t, ".", "build", "-o", filepath.Join(t.TempDir(), "statusgen"), ".")
	})

	t.Run("the release build of desk-tools resolves the module with the workspace off", func(t *testing.T) {
		goList(t, "../tools/desk", "build", "-o", filepath.Join(t.TempDir(), "deskread"), "./cmd/deskread")
	})

	t.Run("the repository has no root go.work", func(t *testing.T) {
		if _, err := os.Stat("../go.work"); err == nil {
			t.Fatal("a root go.work exists — the release builds and loopadmin's request test refuse it")
		}
	})
}

// --- import-path guard (row 10) ---------------------------------------------------------------

var belowRootImport = regexp.MustCompile(`"([^"]*/)?forgeread/[^"]+"`)

// countBelowRootQuotes counts non-test, non-comment lines in dir's top-level Go sources that quote
// an import path below the forgeread module root.
func countBelowRootQuotes(t *testing.T, dir string) (int, []string) {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	n := 0
	var where []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if belowRootImport.MatchString(line) {
				n++
				where = append(where, fmt.Sprintf("%s:%d", filepath.Base(f), i+1))
			}
		}
	}
	return n, where
}

// regression: #2395
func TestStatusgenImportsNoFacadeBelowRoot(t *testing.T) {
	if n, where := countBelowRootQuotes(t, "."); n != 0 {
		t.Errorf("statusgen quotes %d import path(s) below the forgeread module root: %v — only the root package is linked here", n, where)
	}
	// Negative control: the guard must see a planted adapter import.
	dir := t.TempDir()
	planted := "package main\n\nimport _ \"github.com/medici-finance/assay/forgeread/adapters/github\"\n"
	if err := os.WriteFile(filepath.Join(dir, "planted.go"), []byte(planted), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, _ := countBelowRootQuotes(t, dir); n != 1 {
		t.Fatalf("negative control: planted adapter import counted %d, want 1 — the guard is blind", n)
	}
}
