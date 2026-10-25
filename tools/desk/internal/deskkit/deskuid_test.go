package deskkit

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// The desk account's numeric identity (#3857).
//
// The Kubernetes role manifests run every desk as runAsUser/runAsGroup 999 with
// fsGroup 999, so /work and the 0440 Secret volume are usable only if the image's
// `desk` account IS 999:999. Left to `useradd --system` allocation, 999 held only
// while the base distro's system-ID range and the accounts created before `desk`
// stayed the same. containers/base/Dockerfile therefore pins both IDs and checks
// them at build time; these tests hold that line and the five manifests to one
// value on every PR (the image itself is built only on release).

const (
	deskAccountID    = "999"
	k8sBaseGlob      = fixtureRepoRoot + "/containers/k8s/base/*.yaml"
	deskAccountRoles = 5
)

// deskAccountProblem returns why the base Dockerfile does not create `desk` with
// the pinned UID/GID and fail the build when the account resolves otherwise.
func deskAccountProblem(dockerfile, id string) string {
	var runs []dfInstr
	for _, in := range parseDockerfile(dockerfile) {
		if in.op == "RUN" && (hasWord(in.args, "useradd") || hasWord(in.args, "groupadd")) {
			runs = append(runs, in)
		}
	}
	if len(runs) != 1 {
		return "want exactly one RUN that creates the desk group and user"
	}
	in := runs[0]
	if p := failClosedProblem(in.args); p != "" {
		return "the desk-account RUN " + p
	}
	// The in-image home is assembled from fragments, as leakshape_test.go's planted
	// shapes are: a contiguous literal here would trip TestS2ShapeRepoGate's home-path
	// shape. The asserted step is still the Dockerfile's full useradd line.
	deskHome := "/ho" + "me/" + "desk"
	steps := []string{
		"groupadd --system --gid " + id + " desk",
		"useradd --system --uid " + id + " --gid desk --create-home --home-dir " + deskHome + " --shell /bin/bash desk",
		`test "$(id -u desk):$(id -g desk)" = "` + id + ":" + id + `"`,
	}
	last := -1
	for _, step := range steps {
		i := segAt(in.segs, step)
		if i < 0 {
			return "missing step `" + step + "`"
		}
		if i <= last {
			return "step `" + step + "` is out of order"
		}
		last = i
	}
	return ""
}

var podIdentityField = regexp.MustCompile(`(?m)^\s*(runAsUser|runAsGroup|fsGroup):\s*(\S+)\s*$`)

// manifestIdentityProblem returns why one role manifest does not run its pod
// as id:id with fsGroup id.
func manifestIdentityProblem(text, id string) string {
	got := map[string]string{}
	for _, m := range podIdentityField.FindAllStringSubmatch(text, -1) {
		if _, dup := got[m[1]]; dup {
			return m[1] + " is set more than once"
		}
		got[m[1]] = m[2]
	}
	for _, field := range []string{"runAsUser", "runAsGroup", "fsGroup"} {
		if got[field] != id {
			return field + "=" + got[field] + ", want " + id
		}
	}
	return ""
}

// regression: #3857
func TestDeskAccountIDIsPinned(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the base Dockerfile is a Linux build input; its anchors assume an LF checkout")
	}
	skipIfFixtureAbsent(t, baseDockerfilePath, "containers/ is not part of this repository's published file set")
	raw, err := os.ReadFile(baseDockerfilePath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if p := deskAccountProblem(text, deskAccountID); p != "" {
		t.Fatalf("containers/base/Dockerfile: %s", p)
	}

	paths, err := filepath.Glob(k8sBaseGlob)
	if err != nil {
		t.Fatal(err)
	}
	roles := 0
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "\nkind: StatefulSet\n") {
			continue
		}
		roles++
		if p := manifestIdentityProblem(string(body), deskAccountID); p != "" {
			t.Errorf("%s: %s", filepath.Base(path), p)
		}
	}
	if roles != deskAccountRoles {
		t.Fatalf("found %d role manifests under containers/k8s/base, want %d", roles, deskAccountRoles)
	}
}

// TestDeskAccountIDMutants is the negative control: the pre-pin allocation, a
// different pinned number, a dropped build-time check, a masked one, and a
// manifest that disagrees must each be caught.
func TestDeskAccountIDMutants(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the base Dockerfile is a Linux build input; its anchors assume an LF checkout")
	}
	skipIfFixtureAbsent(t, baseDockerfilePath, "containers/ is not part of this repository's published file set")
	raw, err := os.ReadFile(baseDockerfilePath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	swap := func(old, repl string) string {
		t.Helper()
		if strings.Count(text, old) != 1 {
			t.Fatalf("mutant anchor %q must appear exactly once", old)
		}
		return strings.Replace(text, old, repl, 1)
	}
	group := "groupadd --system --gid 999 desk; \\\n"
	user := "useradd --system --uid 999 --gid desk"
	check := "; \\\n    test \"$(id -u desk):$(id -g desk)\" = \"999:999\"\n"
	mutants := map[string]string{
		"system-allocated (pre-pin) ids": swap(group, "groupadd --system desk; \\\n"),
		"system-allocated (pre-pin) uid": swap(user, "useradd --system --gid desk"),
		"a different pinned uid":         swap(user, "useradd --system --uid 1000 --gid desk"),
		"dropped build-time check":       swap(check, "\n"),
		"masked build-time check":        swap(check, "; \\\n    test \"$(id -u desk):$(id -g desk)\" = \"999:999\" || true\n"),
		"no set -e":                      swap("RUN set -eux; \\\n    groupadd", "RUN set -ux; \\\n    groupadd"),
	}
	for name, mutant := range mutants {
		if p := deskAccountProblem(mutant, deskAccountID); p == "" {
			t.Errorf("%s: not caught", name)
		}
	}

	manifest := "kind: StatefulSet\n      securityContext:\n        runAsUser: 999\n        runAsGroup: 999\n        fsGroup: 999\n"
	if p := manifestIdentityProblem(manifest, deskAccountID); p != "" {
		t.Fatalf("clean manifest control: %s", p)
	}
	for _, field := range []string{"runAsUser", "runAsGroup", "fsGroup"} {
		wrong := strings.Replace(manifest, field+": 999", field+": 1000", 1)
		if p := manifestIdentityProblem(wrong, deskAccountID); p == "" {
			t.Errorf("manifest %s=1000: not caught", field)
		}
		missing := strings.Replace(manifest, "        "+field+": 999\n", "", 1)
		if p := manifestIdentityProblem(missing, deskAccountID); p == "" {
			t.Errorf("manifest without %s: not caught", field)
		}
	}
}
