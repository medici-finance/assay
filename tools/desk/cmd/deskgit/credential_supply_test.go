package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// credential_supply_test.go — sec-1587-S1, round 3: the two answer-point layers, each on its own.
//
//   - Layer A (git's scoping): the helper is installed only under credential.https://github.com.helper,
//     after `credential.helper=` has cleared every helper from every config scope, so git never
//     ASKS it about another host.
//   - Layer B (the helper's own check): asked anyway, it answers only protocol=https for
//     github.com / github.com:443.
//
// asrole_answerpoint_test.go shows the two together on real transports; these pin each alone.

// runHelper runs the ephemeral helper script directly, as git would, with request on stdin.
func runHelper(t *testing.T, script, action, request string) string {
	t.Helper()
	cmd := exec.Command("/bin/sh", script, action)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "DESKGIT_TOKEN=" + fixtureToken}
	cmd.Stdin = strings.NewReader(request)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper %s exited %v: %s", action, err, out)
	}
	return string(out)
}

// Layer B alone: the script refuses every request that is not https to exactly github.com.
func TestCredentialHelperScript_AnswersOnlyGitHubHTTPS(t *testing.T) {
	_, prefix, cleanup, err := credentialSupply(fixtureToken)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	script := strings.TrimSuffix(strings.TrimPrefix(prefix[3], credentialHelperKey+"=!'"), "'")

	answered := []string{
		"protocol=https\nhost=github.com\n\n",
		"protocol=https\nhost=github.com:443\n\n",
		"protocol=https\nhost=github.com\npath=example-org/tracker.git\nusername=corpuser\n\n",
	}
	for _, req := range answered {
		out := runHelper(t, script, "get", req)
		if !strings.Contains(out, "username=x-access-token\n") || !strings.Contains(out, "password="+fixtureToken+"\n") {
			t.Fatalf("helper did not answer the github.com https request %q: %q", req, out)
		}
	}
	refused := []string{
		"protocol=http\nhost=github.com\n\n",            // cleartext
		"protocol=http\nhost=127.0.0.1:8080\n\n",        // a proxy prompt
		"protocol=https\nhost=127.0.0.1:8443\n\n",       // a loopback / submodule host
		"protocol=https\nhost=gitlab.example.com\n\n",   // another forge
		"protocol=https\nhost=github.com.evil.test\n\n", // lookalike
		"protocol=https\nhost=github.com.\n\n",          // trailing dot
		"protocol=https\nhost=api.github.com\n\n",       // subdomain
		"protocol=https\nhost=github.com:8443\n\n",      // another port
		"protocol=https\nhost=evil.test\nhost=x\n\n",    // no github host at all
		"host=github.com\n\n",                           // no protocol
		"protocol=https\n\n",                            // no host
		"protocol=https\nhost=GitHub.com.evil.test\n\n", // mixed-case lookalike
	}
	for _, req := range refused {
		if out := runHelper(t, script, "get", req); out != "" {
			t.Fatalf("helper answered a request that is not https to github.com (%q): %q", req, out)
		}
	}
	for _, action := range []string{"store", "erase"} {
		if out := runHelper(t, script, action, answered[0]); out != "" {
			t.Fatalf("helper printed on %q: %q", action, out)
		}
	}
}

// Layer A alone: an ANSWER-EVERYTHING helper installed under the same key credentialSupply uses
// is asked only about https://github.com. The ambient helpers — unscoped AND host-scoped, in
// repo config — are cleared and never fire.
func TestCredentialHelperKey_GitAsksOnlyForGitHub(t *testing.T) {
	_, prefix, cleanup, err := credentialSupply(fixtureToken)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if len(prefix) != 4 || prefix[0] != "-c" || prefix[1] != "credential.helper=" || prefix[2] != "-c" ||
		!strings.HasPrefix(prefix[3], credentialHelperKey+"=") || credentialHelperKey != "credential.https://github.com.helper" {
		t.Fatalf("credential argv prefix %q is not: clear every helper, then add one under the github.com https key", prefix)
	}

	dir := t.TempDir()
	mustGit(t, "", "init", "-q", filepath.Join(dir, "r"))
	repo := filepath.Join(dir, "r")
	canary := filepath.Join(dir, "AMBIENT_FIRED")
	ambient := filepath.Join(dir, "ambient.sh")
	if err := os.WriteFile(ambient, []byte("#!/bin/sh\ntouch "+canary+"\necho password=ambient\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "config", "credential.helper", ambient)
	mustGit(t, repo, "config", "credential.https://github.com.helper", ambient)

	everything := filepath.Join(dir, "everything.sh")
	if err := os.WriteFile(everything, []byte("#!/bin/sh\necho username=x-access-token\necho password=$DESKGIT_TOKEN\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	gitArgs := []string{"-c", "credential.helper=", "-c", credentialHelperKey + "=!'" + everything + "'", "credential", "fill"}

	fill := func(url string) (string, error) {
		cmd := exec.Command("git", gitArgs...)
		cmd.Dir = repo
		cmd.Env = append(scrubbedEnv(os.Environ()), "HOME="+dir, "DESKGIT_TOKEN="+fixtureToken)
		cmd.Stdin = strings.NewReader("url=" + url + "\n\n")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	for _, url := range []string{"https://github.com/example-org/tracker.git", "https://github.com:443/example-org/tracker.git"} {
		out, err := fill(url)
		if err != nil || !strings.Contains(out, "password="+fixtureToken) {
			t.Fatalf("git did not ask the scoped helper for %s: err=%v out=%q", url, err, out)
		}
	}
	for _, url := range []string{
		"http://github.com/example-org/tracker.git",
		"http://corpuser@127.0.0.1:8080",
		"https://127.0.0.1:8443/other-org/sub.git",
		"https://gitlab.example.com/example-org/tracker.git",
		"https://github.com.evil.test/example-org/tracker.git",
		"https://api.github.com/",
		"https://github.com:8443/example-org/tracker.git",
	} {
		out, err := fill(url)
		if err == nil || strings.Contains(out, fixtureToken) {
			t.Fatalf("git asked the scoped helper about %s (err=%v): %q", url, err, out)
		}
	}
	if _, err := os.Stat(canary); err == nil {
		t.Fatal("an ambient credential helper from repo config was consulted")
	}
}
