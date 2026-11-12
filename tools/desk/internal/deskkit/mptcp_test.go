package deskkit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// CVE-2026-98432 stopgap (#6213): MPTCP off in every container this repo ships.
//
// Go >= 1.24 opens MPTCP listeners by default, which reaches the vulnerable
// kernel path. GODEBUG=multipathtcp=0 turns that default off. It is a fragile
// stopgap: a dropped env entry, a later override or a new workload re-opens it.
// Patched node kernels are the real fix. These tests hold the stopgap on every
// PR: every pod spec the repo renders or documents carries the setting on every
// container and initContainer, and every Dockerfile final stage sets it, carries
// it from the desk base image, or is a recorded exemption.

const mptcpOff = "multipathtcp=0"

// godebugDisablesMPTCP reports whether a GODEBUG value turns MPTCP off, reading
// the list the way the Go runtime does (internal/godebug parse): entries split at
// ",", the name ends at the first "=", the last entry for a name wins, and
// nothing is trimmed. Net dial turns MPTCP listeners off only when the value is
// exactly "0", so "multipathtcp=0 " (trailing space) and "a=1, multipathtcp=0"
// (the second name is " multipathtcp") both leave them on.
func godebugDisablesMPTCP(value string) bool {
	last := ""
	for _, kv := range strings.Split(value, ",") {
		if name, arg, ok := strings.Cut(kv, "="); ok && name == "multipathtcp" {
			last = arg
		}
	}
	return last == "0"
}

// mptcpSweep counts pod specs and containers and names each container whose
// env does not turn MPTCP off.
type mptcpSweep struct {
	workloads, containers int
	missing               []string
}

func (s *mptcpSweep) add(o mptcpSweep) {
	s.workloads += o.workloads
	s.containers += o.containers
	s.missing = append(s.missing, o.missing...)
}

func (s mptcpSweep) String() string {
	return fmt.Sprintf("%d workloads, %d containers, %d missing GODEBUG=%s",
		s.workloads, s.containers, len(s.missing), mptcpOff)
}

// containerDisablesMPTCP reads a container's env as Kubernetes does: the last
// entry for a name wins, and only a literal value counts.
func containerDisablesMPTCP(box map[string]any) bool {
	env, _ := box["env"].([]any)
	value, set := "", false
	for _, item := range env {
		entry, _ := item.(map[string]any)
		if entry["name"] != "GODEBUG" {
			continue
		}
		v, ok := entry["value"].(string)
		value, set = v, ok
	}
	return set && godebugDisablesMPTCP(value)
}

// sweepPodSpecs walks a decoded document. Any map holding a list of containers
// is a pod spec; its containers and initContainers are each checked.
func sweepPodSpecs(node any, where string) mptcpSweep {
	var s mptcpSweep
	switch v := node.(type) {
	case map[string]any:
		if list, ok := v["containers"].([]any); ok && len(list) > 0 {
			s.workloads++
			for _, field := range []string{"initContainers", "containers"} {
				boxes, _ := v[field].([]any)
				for i, item := range boxes {
					box, _ := item.(map[string]any)
					s.containers++
					if box == nil || !containerDisablesMPTCP(box) {
						name, _ := box["name"].(string)
						s.missing = append(s.missing, fmt.Sprintf("%s: %s[%d] %q", where, field, i, name))
					}
				}
			}
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if k != "containers" && k != "initContainers" {
				s.add(sweepPodSpecs(v[k], where))
			}
		}
	case []any:
		for _, item := range v {
			s.add(sweepPodSpecs(item, where))
		}
	}
	return s
}

// sweepYAMLStream decodes every document in a YAML (or JSON) stream. A stream
// that does not decode is an error: an unreadable manifest is never a pass.
func sweepYAMLStream(raw []byte, where string) (mptcpSweep, error) {
	var s mptcpSweep
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	for {
		var doc any
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			return s, nil
		}
		if err != nil {
			return s, fmt.Errorf("%s: does not decode: %v", where, err)
		}
		s.add(sweepPodSpecs(doc, where))
	}
}

var (
	podSpecLine   = regexp.MustCompile(`(?m)^[\s-]*(initContainers|containers):\s*(#.*)?$|"(initContainers|containers)"\s*:\s*\[`)
	markdownFence = regexp.MustCompile("(?ms)^[ \t]*(```|~~~)[^\n]*\n(.*?)^[ \t]*(```|~~~)[ \t]*$")
)

// sweepMarkdown checks every fenced block that holds a pod spec: a reader
// copies these into a cluster, so a documented workload is held to the rule.
func sweepMarkdown(raw []byte, where string) (mptcpSweep, error) {
	var s mptcpSweep
	for _, m := range markdownFence.FindAllSubmatch(raw, -1) {
		if !podSpecLine.Match(m[2]) {
			continue
		}
		got, err := sweepYAMLStream(m[2], where)
		if err != nil {
			return s, err
		}
		s.add(got)
	}
	return s, nil
}

// kustomizePatchFiles lists the files a kustomization applies as patches. A
// patch carries only what it changes, so it is proven through the rendered
// output, not on its own.
func kustomizePatchFiles(dir string, raw []byte) ([]string, error) {
	var k struct {
		Patches []struct {
			Path string `yaml:"path"`
		} `yaml:"patches"`
		PatchesStrategicMerge []string `yaml:"patchesStrategicMerge"`
	}
	if err := yaml.Unmarshal(raw, &k); err != nil {
		return nil, err
	}
	var out []string
	for _, p := range k.Patches {
		if p.Path != "" {
			out = append(out, filepath.Clean(filepath.Join(dir, p.Path)))
		}
	}
	for _, p := range k.PatchesStrategicMerge {
		out = append(out, filepath.Clean(filepath.Join(dir, p)))
	}
	return out, nil
}

// mptcpDockerfileExempt names Dockerfiles that carry no GODEBUG, and why.
var mptcpDockerfileExempt = map[string]string{
	// Qualification recipes whose recorded property is "FROM scratch, no
	// recipe ENV": the closed image validator fails any environment beyond the
	// engine's PATH, and the helpers listen on AF_UNIX only, with no network.
	"tools/desk/internal/forgeservice/auth/testdata/profile-probe/Dockerfile": "network-none AF_UNIX qualification recipe; no ENV by contract",
	"tools/desk/internal/forgeservice/auth/testdata/runtime/Dockerfile":       "network-none AF_UNIX qualification recipe; no ENV by contract",
}

// deskBaseDockerfile is the image the role Dockerfiles build FROM.
const deskBaseDockerfile = "containers/base/Dockerfile"

// envAssignments returns the KEY=value pairs of one ENV instruction, in both
// the KEY=value and the legacy `ENV KEY value` forms.
func envAssignments(args []string) [][2]string {
	if len(args) == 0 {
		return nil
	}
	unquote := func(s string) string { return strings.Trim(s, `"'`) }
	if !strings.Contains(args[0], "=") {
		return [][2]string{{args[0], unquote(strings.Join(args[1:], " "))}}
	}
	var out [][2]string
	for _, a := range args {
		if k, v, ok := strings.Cut(a, "="); ok {
			out = append(out, [2]string{k, unquote(v)})
		}
	}
	return out
}

// dockerfileMPTCPProblem returns why a Dockerfile's final stage does not run
// with MPTCP off. inheritsDeskBase reports a final stage built FROM the desk
// base image (an ARG BASE_IMAGE defaulting to desk-base), which carries the
// setting unless this stage overrides it.
func dockerfileMPTCPProblem(text string) (problem string, inheritsDeskBase bool) {
	instrs := parseDockerfile(text)
	last := -1
	for i, in := range instrs {
		if in.op == "FROM" {
			last = i
		}
	}
	if last < 0 {
		return "no FROM", false
	}
	if args := instrs[last].args; len(args) > 0 && (args[0] == "${BASE_IMAGE}" || args[0] == "$BASE_IMAGE") {
		for _, in := range instrs[:last] {
			if in.op == "ARG" && len(in.args) == 1 && strings.HasPrefix(in.args[0], "BASE_IMAGE=desk-base") {
				inheritsDeskBase = true
			}
		}
	}
	value, set := "", false
	for _, in := range instrs[last+1:] {
		if in.op != "ENV" {
			continue
		}
		for _, kv := range envAssignments(in.args) {
			if kv[0] == "GODEBUG" {
				value, set = kv[1], true
			}
		}
	}
	switch {
	case set && !godebugDisablesMPTCP(value):
		return fmt.Sprintf("final stage sets GODEBUG=%q, which leaves MPTCP on", value), inheritsDeskBase
	case set, inheritsDeskBase:
		return "", inheritsDeskBase
	default:
		return "final stage has no ENV GODEBUG=" + mptcpOff, false
	}
}

func isDockerfile(name string) bool {
	if strings.HasSuffix(name, ".dockerignore") {
		return false
	}
	return name == "Dockerfile" || strings.HasPrefix(name, "Dockerfile.") ||
		strings.HasSuffix(name, ".Dockerfile") || name == "Containerfile"
}

// repoSweepInputs walks the checkout once and returns the files the sweeps read.
func repoSweepInputs(t *testing.T, root string) (manifests, markdown, dockerfiles, kustomizeRoots []string, patches map[string]bool) {
	t.Helper()
	patches = map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		name := d.Name()
		switch {
		case isDockerfile(name):
			dockerfiles = append(dockerfiles, rel)
		case name == "kustomization.yaml" || name == "kustomization.yml" || name == "Kustomization":
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			list, err := kustomizePatchFiles(filepath.Dir(rel), raw)
			if err != nil {
				return fmt.Errorf("%s: %v", rel, err)
			}
			for _, p := range list {
				patches[filepath.ToSlash(p)] = true
			}
			kustomizeRoots = append(kustomizeRoots, filepath.ToSlash(filepath.Dir(rel)))
		case strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".json"):
			manifests = append(manifests, rel)
		case strings.HasSuffix(name, ".md"):
			markdown = append(markdown, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return
}

// Every committed and documented pod spec, and every kustomize root once
// rendered, runs each container with MPTCP off.
//
// regression: #6213
func TestMPTCPOffInEveryWorkload(t *testing.T) {
	skipIfFixtureAbsent(t, fixtureRepoRoot+"/containers/k8s", "the desk manifests ship only in a full checkout")
	manifests, markdown, _, roots, patches := repoSweepInputs(t, fixtureRepoRoot)

	t.Run("committed", func(t *testing.T) {
		var total mptcpSweep
		for _, rel := range manifests {
			if patches[rel] {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(fixtureRepoRoot, rel))
			if err != nil {
				t.Fatal(err)
			}
			if !podSpecLine.Match(raw) {
				continue
			}
			got, err := sweepYAMLStream(raw, rel)
			if err != nil {
				t.Fatal(err)
			}
			total.add(got)
		}
		for _, rel := range markdown {
			raw, err := os.ReadFile(filepath.Join(fixtureRepoRoot, rel))
			if err != nil {
				t.Fatal(err)
			}
			got, err := sweepMarkdown(raw, rel)
			if err != nil {
				t.Fatal(err)
			}
			total.add(got)
		}
		t.Logf("committed manifests and documented pod specs: %s", total)
		if total.workloads == 0 {
			t.Fatal("found no pod spec at all; the sweep is not reading the tree")
		}
		for _, m := range total.missing {
			t.Errorf("container without GODEBUG=%s: %s", mptcpOff, m)
		}
	})

	t.Run("rendered", func(t *testing.T) {
		kustomize, err := exec.LookPath("kustomize")
		if err != nil {
			t.Skipf("could-not-check: kustomize is not on PATH, so %d kustomize roots were not rendered", len(roots))
		}
		if len(roots) == 0 {
			t.Fatal("found no kustomize root; the sweep is not reading the tree")
		}
		var total mptcpSweep
		for _, dir := range roots {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			cmd := exec.CommandContext(ctx, kustomize, "build", "--load-restrictor=LoadRestrictionsRootOnly",
				filepath.Join(fixtureRepoRoot, dir))
			cmd.Env = append(os.Environ(), "KUBECONFIG=/dev/null")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			cancel()
			if err != nil {
				t.Errorf("%s: kustomize build did not complete: %v: %s", dir, err, strings.TrimSpace(stderr.String()))
				continue
			}
			got, err := sweepYAMLStream(out, dir+" (rendered)")
			if err != nil {
				t.Error(err)
				continue
			}
			t.Logf("%s: %s", dir, got)
			if got.workloads == 0 {
				t.Errorf("%s: rendered no workload", dir)
			}
			total.add(got)
		}
		t.Logf("rendered, %d kustomize roots: %s", len(roots), total)
		for _, m := range total.missing {
			t.Errorf("rendered container without GODEBUG=%s: %s", mptcpOff, m)
		}
	})
}

// Every Dockerfile final stage runs with MPTCP off: set there, carried from
// the desk base image, or a recorded exemption.
//
// regression: #6213
func TestMPTCPOffInEveryDockerfile(t *testing.T) {
	skipIfFixtureAbsent(t, fixtureRepoRoot+"/"+deskBaseDockerfile, "the image recipes ship only in a full checkout")
	_, _, dockerfiles, _, _ := repoSweepInputs(t, fixtureRepoRoot)
	if len(dockerfiles) == 0 {
		t.Fatal("found no Dockerfile; the sweep is not reading the tree")
	}
	seenExempt := map[string]bool{}
	set, inherited := 0, 0
	for _, rel := range dockerfiles {
		raw, err := os.ReadFile(filepath.Join(fixtureRepoRoot, rel))
		if err != nil {
			t.Fatal(err)
		}
		problem, inherits := dockerfileMPTCPProblem(string(raw))
		if why, ok := mptcpDockerfileExempt[rel]; ok {
			seenExempt[rel] = true
			if problem == "" {
				t.Errorf("%s is exempt (%s) but now sets GODEBUG: drop the exemption", rel, why)
			}
			continue
		}
		if problem != "" {
			t.Errorf("%s: %s", rel, problem)
			continue
		}
		if inherits {
			inherited++
		} else {
			set++
		}
	}
	for rel := range mptcpDockerfileExempt {
		if !seenExempt[rel] {
			t.Errorf("exemption %s names no Dockerfile in the tree: drop it", rel)
		}
	}
	if inherited > 0 {
		raw, err := os.ReadFile(filepath.Join(fixtureRepoRoot, deskBaseDockerfile))
		if err != nil {
			t.Fatal(err)
		}
		if problem, _ := dockerfileMPTCPProblem(string(raw)); problem != "" {
			t.Errorf("%d role images inherit the desk base, but %s: %s", inherited, deskBaseDockerfile, problem)
		}
	}
	t.Logf("%d Dockerfiles: %d set GODEBUG=%s in the final stage, %d inherit it from %s, %d exempt",
		len(dockerfiles), set, mptcpOff, inherited, deskBaseDockerfile, len(seenExempt))
}

// The sweeps must catch each way the setting goes missing. Each plant below is
// a shape the checks above would otherwise pass.
//
// regression: #6213
func TestMPTCPSweepRejectsPlants(t *testing.T) {
	pod := func(env string) string {
		return "apiVersion: apps/v1\nkind: Deployment\nspec:\n  template:\n    spec:\n" + env
	}
	ok := "          env:\n            - name: GODEBUG\n              value: multipathtcp=0\n"
	for _, c := range []struct {
		name, manifest string
		missing        int
	}{
		{"clean", pod("      containers:\n        - name: app\n          image: x\n" + ok), 0},
		{"merged-value", pod("      containers:\n        - name: app\n          image: x\n          env:\n            - name: GODEBUG\n              value: http2client=0,multipathtcp=0\n"), 0},
		{"no-env", pod("      containers:\n        - name: app\n          image: x\n"), 1},
		{"init-container", pod("      initContainers:\n        - name: init\n          image: x\n      containers:\n        - name: app\n          image: x\n" + ok), 1},
		{"second-container", pod("      containers:\n        - name: app\n          image: x\n" + ok + "        - name: sidecar\n          image: y\n"), 1},
		{"re-enabled", pod("      containers:\n        - name: app\n          image: x\n          env:\n            - name: GODEBUG\n              value: multipathtcp=0,multipathtcp=1\n"), 1},
		{"space-after-comma", pod("      containers:\n        - name: app\n          image: x\n          env:\n            - name: GODEBUG\n              value: \"multipathtcp=1, multipathtcp=0\"\n"), 1},
		{"trailing-space", pod("      containers:\n        - name: app\n          image: x\n          env:\n            - name: GODEBUG\n              value: \"multipathtcp=0 \"\n"), 1},
		{"later-override", pod("      containers:\n        - name: app\n          image: x\n" + ok + "            - name: GODEBUG\n              value: multipathtcp=2\n"), 1},
		{"from-secret", pod("      containers:\n        - name: app\n          image: x\n          env:\n            - name: GODEBUG\n              valueFrom: {secretKeyRef: {name: s, key: k}}\n"), 1},
		{"cronjob", "kind: CronJob\nspec:\n  jobTemplate:\n    spec:\n      template:\n        spec:\n          containers:\n            - name: tick\n              image: x\n", 1},
	} {
		got, err := sweepYAMLStream([]byte(c.manifest), c.name)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got.workloads != 1 || len(got.missing) != c.missing {
			t.Errorf("%s: got %s %v, want 1 workload and %d missing", c.name, got, got.missing, c.missing)
		}
	}

	if _, err := sweepMarkdown([]byte("```yaml\ncontainers:\n  - name: [unclosed\n```\n"), "plant.md"); err == nil {
		t.Error("an undecodable documented pod spec passed")
	}
	if got, _ := sweepMarkdown([]byte("text\n```\ncontainers:\n  - name: app\n    image: x\n```\n"), "plant.md"); len(got.missing) != 1 {
		t.Errorf("a documented pod spec without GODEBUG passed: %s", got)
	}

	for _, c := range []struct{ name, dockerfile, want string }{
		{"final-stage", "FROM golang AS build\nRUN go build\nFROM scratch\nENV GODEBUG=multipathtcp=0\n", ""},
		{"legacy-form", "FROM alpine\nENV GODEBUG multipathtcp=0\n", ""},
		{"merged", "FROM alpine\nENV A=1 \\\n    GODEBUG=\"x=1,multipathtcp=0\"\n", ""},
		{"builder-only", "FROM golang AS build\nENV GODEBUG=multipathtcp=0\nFROM scratch\n", "no ENV"},
		{"absent", "FROM alpine\nENV A=1\n", "no ENV"},
		{"re-enabled", "FROM alpine\nENV GODEBUG=multipathtcp=0\nENV GODEBUG=multipathtcp=1\n", "leaves MPTCP on"},
		{"base-override", "ARG BASE_IMAGE=desk-base:dev\nFROM ${BASE_IMAGE}\nENV GODEBUG=netdns=go\n", "leaves MPTCP on"},
		{"foreign-base", "ARG BASE_IMAGE=alpine\nFROM ${BASE_IMAGE}\n", "no ENV"},
	} {
		got, _ := dockerfileMPTCPProblem(c.dockerfile)
		if (c.want == "") != (got == "") || !strings.Contains(got, c.want) {
			t.Errorf("%s: problem %q, want %q", c.name, got, c.want)
		}
	}
	if _, inherits := dockerfileMPTCPProblem("ARG BASE_IMAGE=desk-base:dev\nFROM ${BASE_IMAGE}\nENV ASSAY_DESK=x\n"); !inherits {
		t.Error("a role image FROM the desk base was not recognised as inheriting it")
	}
}
