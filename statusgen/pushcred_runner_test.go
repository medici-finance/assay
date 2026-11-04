package main

import (
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The runner-checkout class (#5004, after #5012). #5012 reset every http
// extraheader for the board push, and the push was still sent as the CI job
// token. A GitLab runner's checkout has two other ways to hand git the job
// token for the instance host, and these tests plant each one in a workspace
// shaped like the runner leaves it, then push through pushcred_push to the
// loopback forge of newHdrRig, which records which credential each request
// carried:
//
//   - insteadof: url.<scheme>://gitlab-ci-token:<job>@<host>/.insteadOf =
//     <scheme>://<host>/ — the runner's rewrite so other repos on the same
//     instance (submodules, includes) clone with the job token. git rewrites
//     the board remote, which pushcred builds without userinfo, to a URL that
//     carries the job token, and URL userinfo is used before any helper is
//     asked.
//   - credhelper: credential.<scheme>://<host>.helper (and .username) — the
//     runner's form that keeps the token out of URLs. A URL-scoped helper key
//     joins the same helper list as the plain key.

// runnerShapes is every checkout shape planted below.
var runnerShapes = []string{"insteadof", "credhelper", "envhelper", "originurl", "all"}

// plantRunner writes one runner checkout shape into the workspace's repo
// config, aimed at the rig's host.
func (r *hdrRig) plantRunner(shape string) {
	r.t.Helper()
	u, err := url.Parse(r.url)
	if err != nil {
		r.t.Fatal(err)
	}
	base := u.Scheme + "://" + u.Host
	withTok := u.Scheme + "://gitlab-ci-token:" + jobHeaderSentinel + "@" + u.Host
	helper := `!f() { test "$1" = get || return 0; echo username=gitlab-ci-token; echo password=` + jobHeaderSentinel + `; }; f`
	switch shape {
	case "insteadof":
		r.git(r.ws, "config", "url."+withTok+"/.insteadOf", base+"/")
	case "credhelper":
		r.git(r.ws, "config", "credential."+base+".username", "gitlab-ci-token")
		r.git(r.ws, "config", "credential."+base+".helper", helper)
	case "envhelper":
		// The same URL-scoped helper at command scope, through the
		// GIT_CONFIG_COUNT environment: read in the same layer as -c.
		r.env = append(r.env, "GIT_CONFIG_COUNT=2",
			"GIT_CONFIG_KEY_0=credential."+base+".username", "GIT_CONFIG_VALUE_0=gitlab-ci-token",
			"GIT_CONFIG_KEY_1=credential."+base+".helper", "GIT_CONFIG_VALUE_1="+helper)
	case "originurl":
		r.git(r.ws, "remote", "set-url", "origin", withTok+u.Path)
	case "all":
		for _, s := range []string{"insteadof", "credhelper", "envhelper", "originurl"} {
			r.plantRunner(s)
		}
		r.git(r.ws, "config", "http."+base+"/.extraheader", jobHeader)
	default:
		r.t.Fatalf("unknown runner shape %q", shape)
	}
}

// TestPushcredRunnerShapes: with each runner checkout shape planted,
// pushcred_push with PUSHCRED_TOKEN_VAR set sends the board credential and
// never the job token, the push lands, and no form of either value reaches
// the output.
//
// regression: #5004
func TestPushcredRunnerShapes(t *testing.T) {
	for _, sh := range []string{"bash", "sh"} {
		for _, shape := range runnerShapes {
			t.Run(sh+"/"+shape, func(t *testing.T) {
				r := newHdrRig(t)
				r.plantRunner(shape)
				rc, out := r.push(sh, "set -x; PUSHCRED_TOKEN_VAR="+boardTokenVar+"; ",
					boardTokenVar+"="+boardTokenSentinel)
				seen := r.seenSet()
				if rc != 0 || seen["job"] || !seen["board"] {
					t.Fatalf("%s: rc=%d, forge saw %v; want rc=0, the board credential, never the job token:\n%s",
						shape, rc, r.seen, out)
				}
				if got, want := r.remoteMain(), r.git(r.ws, "rev-parse", "HEAD"); got != want {
					t.Errorf("remote main = %q, want the pushed %q", got, want)
				}
				assertNoHeaderValue(t, sh+" "+shape, out)
			})
		}
	}
}

// TestPushcredRunnerRemoteName: a destination given as a remote NAME whose
// push URL carries a credential (here through the planted rewrite) cannot be
// pinned, so pushcred_push says so as a NOTICE without printing the URL.
//
// regression: #5004
func TestPushcredRunnerRemoteName(t *testing.T) {
	for _, sh := range []string{"bash", "sh"} {
		t.Run(sh, func(t *testing.T) {
			r := newHdrRig(t)
			r.plantRunner("insteadof")
			src, err := filepath.Abs("pushcred.sh")
			if err != nil {
				t.Fatal(err)
			}
			c := exec.Command(sh, "-c", `set -x; . "$1"; PUSHCRED_TOKEN_VAR=`+boardTokenVar+`; pushcred_push origin HEAD:main; echo "rc=$?"`, sh, src)
			c.Dir = r.ws
			c.Env = append(append([]string{}, r.env...), boardTokenVar+"="+boardTokenSentinel)
			out, _ := c.CombinedOutput()
			if !strings.Contains(string(out), "NOTICE: pushcred: the push URL of remote origin carries a URL credential") {
				t.Errorf("no NOTICE for a credential on a named remote's push URL:\n%s", out)
			}
			assertNoHeaderValue(t, sh, string(out))
		})
	}
}

// TestPushcredRunnerPreflight: the regen job's first board call is
// pushcred_preflight, whose dry-run push goes through pushcred_push --dry-run;
// with the rewrite planted it authenticates as the board credential too.
// PUSHCRED_FORGE is unset, so no self-introspection call is made.
//
// regression: #5004
func TestPushcredRunnerPreflight(t *testing.T) {
	for _, sh := range []string{"bash", "sh"} {
		t.Run(sh, func(t *testing.T) {
			r := newHdrRig(t)
			r.plantRunner("all")
			src, err := filepath.Abs("pushcred.sh")
			if err != nil {
				t.Fatal(err)
			}
			c := exec.Command(sh, "-c", `. "$1"; PUSHCRED_TOKEN_VAR=`+boardTokenVar+`; pushcred_preflight board "$2" HEAD:main`, sh, src, r.url)
			c.Dir = r.ws
			c.Env = append(append([]string{}, r.env...), boardTokenVar+"="+boardTokenSentinel)
			out, err := c.CombinedOutput()
			seen := r.seenSet()
			if err != nil || seen["job"] || !seen["board"] || !strings.Contains(string(out), "credential preflight passed") {
				t.Fatalf("preflight err=%v, forge saw %v; want a pass on the board credential, never the job token:\n%s", err, r.seen, out)
			}
			assertNoHeaderValue(t, sh, string(out))
		})
	}
}

// TestPushcredRunnerInstrument: the rig reproduces each shape. A plain push
// to the board remote (no pushcred) authenticates as the job token, so the
// forge sees it; if git ever stopped using a shape, this goes red and the
// test above proves nothing for it.
func TestPushcredRunnerInstrument(t *testing.T) {
	for _, shape := range []string{"insteadof", "credhelper", "envhelper"} {
		t.Run(shape, func(t *testing.T) {
			r := newHdrRig(t)
			r.plantRunner(shape)
			cmd := r.cmd("push", r.url, "HEAD:main")
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("a job-token push was accepted; the rig no longer refuses it:\n%s", out)
			}
			if !r.seenSet()["job"] {
				t.Errorf("forge saw %v, want the job token from the planted %s", r.seen, shape)
			}
		})
	}
}

// cmd is a git command in the workspace with the rig's env and no prompt.
func (r *hdrRig) cmd(args ...string) *exec.Cmd {
	c := exec.Command("git", args...)
	c.Dir = r.ws
	c.Env = append(append([]string{}, r.env...), "GIT_TERMINAL_PROMPT=0")
	return c
}
