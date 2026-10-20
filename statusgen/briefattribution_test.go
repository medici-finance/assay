package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// regression: #3343 — strict opt-in shares actor policy, not corpus severity.
func TestBriefAttribution(t *testing.T) {
	for _, tc := range []struct {
		name, author string
		want         int
	}{
		{"verifier", vfMailVerifier, briefInfoExitOK},
		{"human", vfMailHuman, briefInfoExitOK},
		{"worker", vfMailWorker, briefInfoExitInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := vfDefaults()
			o.status = "verified"
			o.verified = vfStamp
			o.author = tc.author
			root, _ := vfFixture(t, o)
			// The baseline is already closed: strict acceptance is independent
			// of corpus lint's historical severity classification.
			vfGit(t, root, tc.author, "update-ref", "refs/remotes/origin/main", "HEAD")
			var out, errout bytes.Buffer
			code := runBriefInfo([]string{"--root", root, "--check-verified", "--check-attribution", "vf/01"}, &out, &errout)
			if code != tc.want || (code != 0 && out.Len() != 0) {
				t.Fatalf("exit=%d out=%s err=%s", code, out.String(), errout.String())
			}
			out.Reset()
			errout.Reset()
			if code := runBriefInfo([]string{"--root", root, "--check-verified", "vf/01"}, &out, &errout); code != 0 {
				t.Fatalf("default structural check changed: %d %s", code, errout.String())
			}
			if tc.name == "verifier" {
				if err := os.RemoveAll(root + "/.git"); err != nil {
					t.Fatal(err)
				}
				out.Reset()
				errout.Reset()
				if code := runBriefInfo([]string{"--root", root, "--check-verified", "--check-attribution", "vf/01"}, &out, &errout); code != briefInfoExitResolve || out.Len() != 0 {
					t.Fatalf("missing history exit=%d out=%s err=%s", code, out.String(), errout.String())
				}
			}
		})
	}
	var out, errout bytes.Buffer
	if code := runBriefInfo([]string{"--check-attribution", "vf/01"}, &out, &errout); code != briefInfoExitResolve || out.Len() != 0 {
		t.Fatalf("flag dependency exit=%d out=%s", code, out.String())
	}
}

// regression: #3343 — unavailable policy and grafted history are not rejections.
func TestBriefAttributionUnavailable(t *testing.T) {
	for _, mode := range []string{"policy", "shallow"} {
		t.Run(mode, func(t *testing.T) {
			o := vfDefaults()
			o.status = "verified"
			o.verified = vfStamp
			o.author = vfMailWorker
			root, _ := vfFixture(t, o)
			if mode == "policy" {
				t.Setenv("HOME", t.TempDir())
				scanReloadConfig()
				t.Cleanup(scanReloadConfig)
			} else {
				vfGit(t, root, vfMailWorker, "commit", "-q", "--allow-empty", "-m", "later")
				dest := t.TempDir()
				cmd := exec.Command("git", "clone", "-q", "--depth=1", "file://"+root, dest)
				if raw, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("shallow fixture: %v %s", err, raw)
				}
				root = dest
				if !isShallowRepository(root) {
					t.Fatal("fixture is not shallow")
				}
			}
			var out, errout bytes.Buffer
			if code := runBriefInfo([]string{"--root", root, "--check-verified", "--check-attribution", "vf/01"}, &out, &errout); code != briefInfoExitResolve || out.Len() != 0 {
				t.Fatalf("exit=%d out=%s err=%s", code, out.String(), errout.String())
			}
		})
	}
}

// regression: attribution-schema — the strict check is defined for every brief
// schema that keeps the `## Evidence` section, not only brief-v1. Before the fix
// every brief-v2 and brief-v3 closure exited 2 ("cannot check attribution
// without brief-v1 schema") whoever authored the Evidence.
func TestBriefAttributionAcrossSchemas(t *testing.T) {
	for _, schema := range []string{"brief-v1", "brief-v2", "brief-v3"} {
		for _, tc := range []struct {
			name, author string
			want         int
		}{
			{"verifier", vfMailVerifier, briefInfoExitOK},
			{"human", vfMailHuman, briefInfoExitOK},
			{"worker", vfMailWorker, briefInfoExitInvalid},
		} {
			t.Run(schema+"/"+tc.name, func(t *testing.T) {
				o := vfDefaults()
				o.status = "verified"
				o.verified = vfStamp
				o.author = tc.author
				o.schema = schema
				root, _ := vfFixture(t, o)
				vfGit(t, root, tc.author, "update-ref", "refs/remotes/origin/main", "HEAD")
				var out, errout bytes.Buffer
				code := runBriefInfo([]string{"--root", root, "--check-verified", "--check-attribution", "vf/01"}, &out, &errout)
				if code != tc.want {
					t.Fatalf("exit=%d want %d out=%s err=%s", code, tc.want, out.String(), errout.String())
				}
				if code != 0 && out.Len() != 0 {
					t.Fatalf("a refusal must print no JSON body: %s", out.String())
				}
				if code == briefInfoExitInvalid && !strings.Contains(errout.String(), "Evidence attribution rejected") {
					t.Fatalf("a bad attribution must be a rejection, not a resolve failure: %s", errout.String())
				}
			})
		}
	}
}

// regression: attribution-schema — a schema outside the supported set still
// fails closed with the resolve exit, through the adapter and end to end.
func TestBriefAttributionUnsupportedSchemaFailsClosed(t *testing.T) {
	for _, schema := range []string{"legacy", "", "brief-v0", "brief-v9", "contract-v1"} {
		t.Run("adapter/"+schema, func(t *testing.T) {
			code, err := checkBriefInfoAttribution(t.TempDir(), &briefInfo{Key: "vf/01", Schema: schema})
			if code != briefInfoExitResolve || err == nil || !strings.Contains(err.Error(), "cannot check attribution for schema") {
				t.Fatalf("schema %q: exit=%d err=%v", schema, code, err)
			}
		})
	}
	// End to end: a verifier-authored closure on a schema this binary does not
	// recognize is refused (exit 2, no JSON), never waved through.
	o := vfDefaults()
	o.status = "verified"
	o.verified = vfStamp
	o.schema = "brief-v9"
	root, _ := vfFixture(t, o)
	vfGit(t, root, vfMailVerifier, "update-ref", "refs/remotes/origin/main", "HEAD")
	var out, errout bytes.Buffer
	if code := runBriefInfo([]string{"--root", root, "--check-verified", "--check-attribution", "vf/01"}, &out, &errout); code != briefInfoExitResolve || out.Len() != 0 {
		t.Fatalf("unknown schema exit=%d out=%s err=%s", code, out.String(), errout.String())
	}
}

// regression: attribution-schema — a newly recognized brief schema must be
// decided here (supported or deliberately not), never inherit silently.
func TestBriefAttributionSchemasCoverRecognized(t *testing.T) {
	for s := range recognizedBriefSchemas {
		if !briefAttributionSchemas[s] {
			t.Errorf("recognized schema %q has no attribution decision: add it to briefAttributionSchemas or document why not", s)
		}
	}
	for s := range briefAttributionSchemas {
		if !recognizedBriefSchemas[s] {
			t.Errorf("briefAttributionSchemas names %q, which is not a recognized brief schema", s)
		}
	}
}
