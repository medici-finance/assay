package main

import (
	"bytes"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/gitquiet/goquiet"
)

// The verb's exit contract, stated here rather than read from rowread.go, so the
// tests also run (and fail) against a build without the verb.
const (
	rowreadWantUsage    = 5
	rowreadWantCouldNot = 6
)

// rowreadFakeToken is a token-shaped placeholder. It is assembled at run time so
// no token-shaped literal sits in the source; no forge ever sees it.
func rowreadFakeToken(tag string) string {
	return "ghs" + "_" + tag + strings.Repeat("Q", 36-len(tag))
}

// rowreadTokenShape is the shape scan run over everything a row could see.
var rowreadTokenShape = regexp.MustCompile(`(ghs|ghp|gho|ghu|ghr)_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}`)

var rowreadBin struct {
	once sync.Once
	path string
	err  error
	out  []byte
}

// rowreadStatusgen builds this package once, so a Verify row can run the real
// `statusgen rowread` from PATH.
func rowreadStatusgen(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain unavailable")
	}
	rowreadBin.once.Do(func() {
		dir, err := os.MkdirTemp("", "rowread-bin-")
		if err != nil {
			rowreadBin.err = err
			return
		}
		rowreadBin.path = filepath.Join(dir, "statusgen")
		rowreadBin.out, rowreadBin.err = goquiet.Cmd(exec.Command("go", "build", "-o", rowreadBin.path, ".")).CombinedOutput()
	})
	if rowreadBin.err != nil {
		t.Fatalf("build statusgen: %v\n%s", rowreadBin.err, rowreadBin.out)
	}
	return rowreadBin.path
}

// rowreadForge is a fake forge plus a stub `deskread` that stands in for the
// custody verb: the stub holds its credential in its own process (here, in its
// own script, outside every directory the row can name), sends it to the fake
// forge, and answers deskread's per-issue envelope. It records its argv and
// environment under rec, which is also outside the row's HOME and TMPDIR.
type rowreadForge struct {
	minted string
	rec    string
	mu     sync.Mutex
	auth   []string
}

func newRowreadForge(t *testing.T, body string) *rowreadForge {
	t.Helper()
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl unavailable for the stub forge client")
	}
	f := &rowreadForge{minted: rowreadFakeToken("MINTED"), rec: t.TempDir()}
	env := `{"schema":1,"kind":"comments","items":[{"repo":"example-org/one","number":7,"comments":[` +
		`{"authorLogin":"example-bot","authorId":1,"createdAt":"2026-11-03T00:00:00Z","body":` + body + `}]}],"partial":[]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		f.mu.Unlock()
		w.Write([]byte(env))
	}))
	t.Cleanup(srv.Close)
	bin := t.TempDir()
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" >> '" + filepath.Join(f.rec, "argv") + "'\n" +
		"env >> '" + filepath.Join(f.rec, "env") + "'\n" +
		"exec curl -sf -H 'Authorization: token " + f.minted + "' '" + srv.URL + "/comments'\n"
	if err := os.WriteFile(filepath.Join(bin, "deskread"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return f
}

func (f *rowreadForge) started() bool {
	_, err := os.Stat(filepath.Join(f.rec, "argv"))
	return err == nil
}

// rowreadAssertClean fails on any planted or minted token, or any token-shaped
// value, in the bytes given or in any file under the directories given.
func rowreadAssertClean(t *testing.T, secrets []string, what string, data []byte) {
	t.Helper()
	for _, s := range secrets {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("%s carries a token", what)
		}
	}
	if rowreadTokenShape.Match(data) {
		t.Errorf("%s carries a token-shaped value", what)
	}
}

func rowreadAssertTree(t *testing.T, secrets []string, dir string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !d.Type().IsRegular() {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rowreadAssertClean(t, secrets, "file "+p, b)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
}

// regression: #4700
// TestRowreadHoldsNoToken: an admitted Verify row reads forge comments through
// `statusgen rowread`, and the credential the read needs never reaches the row:
// not its environment, not the reader's argv or environment, not a file under
// its HOME or TMPDIR, not its output. The fake forge proves the read used one.
func TestRowreadHoldsNoToken(t *testing.T) {
	sg := rowreadStatusgen(t)
	root, _ := admittedFixture(t, true)
	f := newRowreadForge(t, `"triage done (fixture triage)"`)
	t.Setenv("PATH", filepath.Dir(sg)+string(os.PathListSeparator)+os.Getenv("PATH"))
	// The invoking environment may carry credential-shaped variables that
	// admittedRowEnv leaves in place by design (its out-of-scope note: only the
	// four forge variables are removed). Clear them, so the scan below asserts
	// what this lane adds and nothing the caller brought.
	for _, kv := range os.Environ() {
		if k, v, _ := strings.Cut(kv, "="); rowreadTokenShape.MatchString(v) {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}
	}
	secrets := []string{f.minted}
	for _, name := range forgeTokenVars {
		v := rowreadFakeToken(strings.ReplaceAll(name, "_", ""))
		secrets = append(secrets, v)
		t.Setenv(name, v)
	}
	rowHome, rowTmp := t.TempDir(), t.TempDir()
	t.Setenv("HOME", rowHome)
	t.Setenv("TMPDIR", rowTmp)
	row := `statusgen rowread comments --issue example-org/one#7 --body-contains '(fixture triage)' > "$TMPDIR/out" && env > "$TMPDIR/rowenv" && test -s "$TMPDIR/out"`
	if code := verifyrunRow(t, root, row); code != verifyrunExitPass {
		t.Fatalf("admitted rowread row exit %d, want pass", code)
	}
	f.mu.Lock()
	auth := append([]string(nil), f.auth...)
	f.mu.Unlock()
	if len(auth) == 0 {
		t.Fatal("the fake forge received no request")
	}
	for _, a := range auth {
		if a != "token "+f.minted {
			t.Fatalf("the fake forge did not receive the reader's own credential on every request")
		}
	}
	rowenv, err := os.ReadFile(filepath.Join(rowTmp, "rowenv"))
	if err != nil || len(rowenv) == 0 {
		t.Fatalf("the row did not record its environment: %v", err)
	}
	out, _ := os.ReadFile(filepath.Join(rowTmp, "out"))
	if !strings.Contains(string(out), `"number":7`) || strings.Contains(string(out), "fixture triage") {
		t.Errorf("rowread output = %q, want the matching comment named, its body withheld", out)
	}
	for _, name := range []string{"argv", "env"} {
		b, err := os.ReadFile(filepath.Join(f.rec, name))
		if err != nil {
			t.Fatalf("the reader did not record its %s: %v", name, err)
		}
		rowreadAssertClean(t, secrets, "reader "+name, b)
	}
	rowreadAssertClean(t, secrets, "row environment", rowenv)
	rowreadAssertTree(t, secrets, rowHome)
	rowreadAssertTree(t, secrets, rowTmp)
}

// regression: #4700
// TestRowreadRefusesNonRead: a write verb, another read kind, a CI-token flag
// and a flag smuggled into the target are refused before any reader starts.
func TestRowreadRefusesNonRead(t *testing.T) {
	sg := rowreadStatusgen(t)
	f := newRowreadForge(t, `"x"`)
	for _, args := range [][]string{
		{"post", "--issue", "example-org/one#7"},
		{"close", "--issue", "example-org/one#7"},
		{"issue-list", "--repo", "example-org/one"},
		{"trust", "--issue", "example-org/one#7"},
		{"comments", "--issue", "example-org/one#7", "--ci-workflow-token"},
		{"comments", "--issue", "--repo=example-org/one#7"},
		{"comments", "--issue", "example-org/one#0"},
		{"comments"},
		{},
	} {
		var stderr bytes.Buffer
		cmd := exec.Command(sg, append([]string{"rowread"}, args...)...)
		cmd.Stderr = &stderr
		err := cmd.Run()
		ee, ok := err.(*exec.ExitError)
		if !ok || ee.ExitCode() != rowreadWantUsage {
			t.Errorf("rowread %q: %v, want exit %d (refused)", args, err, rowreadWantUsage)
		}
	}
	if f.started() {
		t.Fatal("a refused rowread started the reader")
	}
}

// regression: #4700
// TestRowreadUnreadFailsClosed: an issue the reader did not read is a
// could-not-check exit, never an empty answer a row would read as "absent".
func TestRowreadUnreadFailsClosed(t *testing.T) {
	sg := rowreadStatusgen(t)
	bin := t.TempDir()
	script := "#!/bin/sh\necho '{\"schema\":1,\"kind\":\"comments\",\"items\":[],\"partial\":[{\"repo\":\"example-org/one\",\"number\":7,\"reason\":\"unreadable\"}]}'\n"
	if err := os.WriteFile(filepath.Join(bin, "deskread"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := exec.Command(sg, "rowread", "comments", "--issue", "example-org/one#7").Output()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != rowreadWantCouldNot || len(out) != 0 {
		t.Fatalf("partial read: %v, stdout %q; want exit %d and nothing printed", err, out, rowreadWantCouldNot)
	}
}
