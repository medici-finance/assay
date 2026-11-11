package deskkit

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// CVE-2026-98432 stopgap (#6213): every container this repository ships or
// documents runs with GODEBUG=multipathtcp=0.
//
// Go 1.24 and later open MPTCP listeners by default, chosen by the go line of
// the main module's go.mod, and the published exploit reaches the kernel bug
// through such a listener. The setting only covers Go programs that honour it,
// a later GODEBUG override re-opens the path, and patched node kernels are the
// real fix. This test is the guard against the two cheap regressions: a
// workload or image added without the setting, and an edit that drops or
// overrides it.
//
// Defect class: a container whose effective GODEBUG leaves multipathtcp enabled.
// The sweeps below close it for every site that can repeat it: each Dockerfile
// found in the tree (an unlisted new one fails until it is classified),
// Kubernetes workloads in any YAML or fenced Markdown block, and Compose files.
// The containers/k8s/check.py checker holds the same rule over the rendered
// kustomize output, with its own hostile fixtures.

const mptcpOff = "multipathtcp=0"

// godebugOff reports whether a GODEBUG value leaves MPTCP off. Go reads the
// comma list left to right and the last setting of a key wins, so a trailing
// multipathtcp=1 after a 0 re-opens the path.
func godebugOff(value string) bool {
	last := ""
	for _, s := range strings.Split(value, ",") {
		if s = strings.TrimSpace(s); strings.HasPrefix(s, "multipathtcp=") {
			last = s
		}
	}
	return last == mptcpOff
}

// ---- Dockerfiles -------------------------------------------------------------

// dockerfileGodebug returns the GODEBUG value the FINAL stage of a Dockerfile
// sets (the last assignment wins) and whether it sets one at all. Variables set
// in an earlier stage do not cross a FROM.
func dockerfileGodebug(text string) (string, bool) {
	instrs := parseDockerfile(text)
	start := 0
	for i, in := range instrs {
		if in.op == "FROM" {
			start = i + 1
		}
	}
	value, set := "", false
	for _, in := range instrs[start:] {
		if in.op != "ENV" || len(in.args) == 0 {
			continue
		}
		if !strings.Contains(in.args[0], "=") { // legacy `ENV KEY value words`
			if in.args[0] == "GODEBUG" {
				value, set = strings.Trim(strings.Join(in.args[1:], " "), `"'`), true
			}
			continue
		}
		for _, a := range in.args {
			if k, v, ok := strings.Cut(a, "="); ok && k == "GODEBUG" {
				value, set = strings.Trim(v, `"'`), true
			}
		}
	}
	return value, set
}

// dockerfileMptcpProblem returns why a Dockerfile's final stage does not leave
// MPTCP off, or "". carries is true for an image that ships a Go program: it
// must set the value. Any other Dockerfile may leave GODEBUG unset (it inherits
// or ships no program) but must not set it to something that turns MPTCP on.
func dockerfileMptcpProblem(text string, carries bool) string {
	value, set := dockerfileGodebug(text)
	switch {
	case set && !godebugOff(value):
		return "final stage sets GODEBUG=" + value + ", which does not leave " + mptcpOff
	case carries && !set:
		return "final stage does not set ENV GODEBUG=" + mptcpOff
	}
	return ""
}

// mptcpDockerfiles classifies every Dockerfile in the tree. A Dockerfile that is
// in none of the three sets fails the test, so a new image has to be decided on.
var (
	// carries: the final stage ships a Go program, so it sets the value itself.
	mptcpCarries = []string{
		"Dockerfile",
		"containers/base/Dockerfile",
		"images/assay-harness/Dockerfile",
		"images/ci-tools/Dockerfile",
		"images/deskd/Dockerfile",
	}
	// inherits: built FROM the desk base image, whose ENV they keep.
	mptcpInherits = []string{
		"containers/intake-desk/Dockerfile",
		"containers/pr-review-desk/Dockerfile",
		"containers/the-desk/Dockerfile",
		"containers/verify-desk/Dockerfile",
		"containers/worker-desk/Dockerfile",
	}
	// exempt: no Go program is shipped to run in the image. The reason is the
	// reviewed claim; a change that makes it false belongs in carries.
	mptcpExempt = map[string]string{
		".oss-scanner/Dockerfile":         "scanner build environment: builds and vets the tree, serves nothing",
		"images/verify-runner/Dockerfile": "verification toolchain image run with no network; ships no Go program of its own",
		"tools/desk/internal/forgeservice/auth/testdata/profile-probe/Dockerfile": "test fixture pinned to a bare scratch recipe",
		"tools/desk/internal/forgeservice/auth/testdata/runtime/Dockerfile":       "test fixture pinned to a recipe with no ENV (its README says so)",
	}
)

func mptcpRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "containers", "base", "Dockerfile")); err != nil {
		t.Fatalf("repo root %s has no containers/base/Dockerfile: %v", root, err)
	}
	return root
}

// mptcpWalk calls visit for each regular file under root, skipping VCS state,
// dependency trees, test fixtures and the work-tracking streams.
func mptcpWalk(t *testing.T, root string, visit func(rel string, raw []byte)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", ".claude", "changelog":
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		visit(rel, raw)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}

// regression: #6213
func TestMptcpOffInEveryDockerfile(t *testing.T) {
	// Positive controls: the matcher must see each planted defect, or the sweep
	// below proves nothing.
	const stage = "FROM golang:1.27 AS build\nRUN go build -o /out/x .\nFROM alpine:3.21\nCOPY --from=build /out/x /x\n"
	controls := []struct {
		name, text string
		carries    bool
		bad        bool
	}{
		{"no ENV at all", stage, true, true},
		{"set only in the build stage", "FROM golang:1.27 AS build\nENV GODEBUG=" + mptcpOff + "\nFROM alpine:3.21\n", true, true},
		{"cleared", stage + "ENV GODEBUG=\n", true, true},
		{"enabled", stage + "ENV GODEBUG=multipathtcp=1\n", true, true},
		{"overridden by a later ENV", stage + "ENV GODEBUG=" + mptcpOff + "\nENV GODEBUG=http2debug=1\n", true, true},
		{"re-enabled after off", stage + "ENV GODEBUG=multipathtcp=0,multipathtcp=1\n", true, true},
		{"inheritor re-enables", stage + "ENV GODEBUG=multipathtcp=1\n", false, true},
		{"plain form", stage + "ENV GODEBUG=" + mptcpOff + "\n", true, false},
		{"legacy form", stage + "ENV GODEBUG " + mptcpOff + "\n", true, false},
		{"merged with another setting", stage + "ENV CGO_ENABLED=0 \\\n    GODEBUG=http2debug=1,multipathtcp=0\n", true, false},
		{"inheritor leaves it unset", "FROM base\nENV ASSAY_DESK=x\n", false, false},
	}
	for _, c := range controls {
		if got := dockerfileMptcpProblem(c.text, c.carries); (got != "") != c.bad {
			t.Errorf("control %q: problem=%q, want defect=%v", c.name, got, c.bad)
		}
	}

	root := mptcpRepoRoot(t)
	carries := map[string]bool{}
	for _, p := range mptcpCarries {
		carries[p] = true
	}
	known := map[string]bool{}
	for _, p := range append(append([]string{}, mptcpCarries...), mptcpInherits...) {
		known[p] = true
	}
	seen := map[string]bool{}
	mptcpWalk(t, root, func(rel string, raw []byte) {
		if filepath.Base(rel) != "Dockerfile" {
			return
		}
		seen[rel] = true
		_, exempt := mptcpExempt[rel]
		switch {
		case exempt:
		case !known[rel]:
			t.Errorf("%s: new Dockerfile; add it to mptcpCarries (ships a Go program: set ENV GODEBUG=%s in its final stage), mptcpInherits, or mptcpExempt with a reason", rel, mptcpOff)
		default:
			if why := dockerfileMptcpProblem(string(raw), carries[rel]); why != "" {
				t.Errorf("%s: %s", rel, why)
			}
		}
	})
	for _, p := range mptcpCarries {
		if !seen[p] {
			t.Errorf("%s is listed in mptcpCarries but was not found", p)
		}
	}
	for _, p := range mptcpInherits {
		if !seen[p] {
			t.Errorf("%s is listed in mptcpInherits but was not found", p)
		}
	}
	for p := range mptcpExempt {
		if !seen[p] {
			t.Errorf("%s is listed in mptcpExempt but was not found", p)
		}
	}
}

// ---- Kubernetes workloads and Compose services ---------------------------------

// k8sContainerMptcpOff mirrors containers/k8s/check.py mptcp_off: exactly one
// inline GODEBUG entry (never valueFrom, never repeated) that leaves MPTCP off.
func k8sContainerMptcpOff(c map[string]any) bool {
	env, _ := c["env"].([]any)
	var entries []map[string]any
	for _, e := range env {
		if m, ok := e.(map[string]any); ok && m["name"] == "GODEBUG" {
			entries = append(entries, m)
		}
	}
	if len(entries) != 1 || len(entries[0]) != 2 {
		return false
	}
	v, ok := entries[0]["value"].(string)
	return ok && godebugOff(v)
}

// podSpecOf returns the pod spec of a workload document, or nil for any other kind.
func podSpecOf(doc map[string]any) map[string]any {
	kind, _ := doc["kind"].(string)
	dig := func(m map[string]any, path ...string) map[string]any {
		for _, k := range path {
			m, _ = m[k].(map[string]any)
			if m == nil {
				return nil
			}
		}
		return m
	}
	switch kind {
	case "Pod":
		return dig(doc, "spec")
	case "CronJob":
		return dig(doc, "spec", "jobTemplate", "spec", "template", "spec")
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "ReplicationController":
		return dig(doc, "spec", "template", "spec")
	}
	return nil
}

// workloadMptcpProblems lists the containers of one document that do not leave
// MPTCP off. A document that is not a workload yields none and counts 0.
func workloadMptcpProblems(doc map[string]any) (problems []string, workloads int) {
	spec := podSpecOf(doc)
	kind, _ := doc["kind"].(string)
	if spec == nil {
		if _, isWorkload := map[string]bool{"Pod": true, "CronJob": true, "Deployment": true, "StatefulSet": true,
			"DaemonSet": true, "ReplicaSet": true, "Job": true, "ReplicationController": true}[kind]; isWorkload {
			return []string{kind + " has no readable pod spec"}, 1
		}
		return nil, 0
	}
	meta, _ := doc["metadata"].(map[string]any)
	name, _ := meta["name"].(string)
	for _, key := range []string{"containers", "initContainers", "ephemeralContainers"} {
		list, _ := spec[key].([]any)
		for _, item := range list {
			c, _ := item.(map[string]any)
			if c == nil || !k8sContainerMptcpOff(c) {
				cn, _ := c["name"].(string)
				problems = append(problems, kind+" "+name+" "+key+" "+cn+": env needs one GODEBUG="+mptcpOff)
			}
		}
	}
	return problems, 1
}

// yamlDocs decodes every document of a YAML stream into a map; a stream that
// does not parse yields what was read and the error.
func yamlDocs(raw []byte) ([]map[string]any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var out []map[string]any
	for {
		var doc map[string]any
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			return out, err
		}
		if len(doc) != 0 {
			out = append(out, doc)
		}
	}
}

var fencedYAML = regexp.MustCompile("(?s)```(?:yaml|yml)[^\n]*\n(.*?)\n```")

// composeMptcpProblems lists the services of a Compose file that do not leave
// MPTCP off, in either the map or the KEY=value list form of `environment`.
func composeMptcpProblems(doc map[string]any) (problems []string, services int) {
	svcs, _ := doc["services"].(map[string]any)
	names := make([]string, 0, len(svcs))
	for n := range svcs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		svc, _ := svcs[n].(map[string]any)
		services++
		off := false
		switch env := svc["environment"].(type) {
		case map[string]any:
			v, ok := env["GODEBUG"].(string)
			off = ok && godebugOff(v)
		case []any:
			count := 0
			for _, e := range env {
				if s, _ := e.(string); strings.HasPrefix(s, "GODEBUG=") {
					count++
					off = godebugOff(strings.TrimPrefix(s, "GODEBUG="))
				}
			}
			off = off && count == 1
		}
		if !off {
			problems = append(problems, "service "+n+": environment needs GODEBUG="+mptcpOff)
		}
	}
	return problems, services
}

// composeExempt lists Compose files whose services run no Go program.
var composeExempt = map[string]string{
	"containers/postgres/compose.yaml": "upstream PostgreSQL image (C server); GODEBUG is read only by Go runtimes",
}

// workloadsExempt lists tracked YAML that is a partial overlay, not a workload.
var workloadsExempt = map[string]string{
	"containers/k8s/components/compatibility-cellctl/workloads.yaml": "strategic-merge patch; env merges by name from the base, and check.py asserts the rendered result",
}

// regression: #6213
func TestMptcpOffOnEveryWorkload(t *testing.T) {
	const bare = "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: x\nspec:\n  template:\n    spec:\n      containers:\n        - name: c\n          image: i\n"
	const good = "          env:\n            - name: GODEBUG\n              value: http2debug=1,multipathtcp=0\n"
	controls := []struct {
		name, text string
		bad        bool
	}{
		{"no env", bare, true},
		{"enabled", bare + "          env:\n            - {name: GODEBUG, value: multipathtcp=1}\n", true},
		{"valueFrom", bare + "          env:\n            - name: GODEBUG\n              valueFrom: {configMapKeyRef: {name: a, key: b}}\n", true},
		{"duplicated", bare + good + "            - {name: GODEBUG, value: multipathtcp=1}\n", true},
		{"init container without", bare + good + "      initContainers:\n        - {name: i, image: i}\n", true},
		{"cronjob template", "kind: CronJob\nmetadata: {name: x}\nspec:\n  jobTemplate:\n    spec:\n      template:\n        spec:\n          containers:\n            - {name: c, image: i}\n", true},
		{"pod", "kind: Pod\nmetadata: {name: x}\nspec:\n  containers:\n    - {name: c, image: i}\n", true},
		{"set", bare + good, false},
	}
	for _, c := range controls {
		docs, err := yamlDocs([]byte(c.text))
		if err != nil || len(docs) != 1 {
			t.Fatalf("control %q does not parse: %v", c.name, err)
		}
		problems, n := workloadMptcpProblems(docs[0])
		if n != 1 || (len(problems) > 0) != c.bad {
			t.Errorf("control %q: workloads=%d problems=%v, want defect=%v", c.name, n, problems, c.bad)
		}
	}
	// Compose controls, both environment forms.
	for _, c := range []struct {
		text string
		bad  bool
	}{
		{"services:\n  a:\n    image: i\n", true},
		{"services:\n  a:\n    environment:\n      GODEBUG: multipathtcp=1\n", true},
		{"services:\n  a:\n    environment: [\"GODEBUG=multipathtcp=0\", \"GODEBUG=multipathtcp=1\"]\n", true},
		{"services:\n  a:\n    environment:\n      GODEBUG: multipathtcp=0\n", false},
		{"services:\n  a:\n    environment: [\"GODEBUG=multipathtcp=0\"]\n", false},
	} {
		docs, err := yamlDocs([]byte(c.text))
		if err != nil || len(docs) != 1 {
			t.Fatalf("compose control does not parse: %v", err)
		}
		if problems, _ := composeMptcpProblems(docs[0]); (len(problems) > 0) != c.bad {
			t.Errorf("compose control %q: problems=%v, want defect=%v", c.text, problems, c.bad)
		}
	}

	root := mptcpRepoRoot(t)
	workloads, services, unparsed := 0, 0, 0
	byPath := map[string]int{}
	mptcpWalk(t, root, func(rel string, raw []byte) {
		if strings.Contains(rel, "/testdata/") || strings.HasPrefix(rel, "docs/streams/") {
			return
		}
		if _, skip := workloadsExempt[rel]; skip {
			return
		}
		var blocks [][]byte
		switch strings.ToLower(filepath.Ext(rel)) {
		case ".yaml", ".yml":
			blocks = [][]byte{raw}
		case ".md":
			for _, m := range fencedYAML.FindAllSubmatch(raw, -1) {
				blocks = append(blocks, m[1])
			}
		}
		compose := regexp.MustCompile(`(^|/)(docker-)?compose[^/]*\.ya?ml$`).MatchString(rel)
		for _, b := range blocks {
			docs, err := yamlDocs(b)
			if err != nil {
				// Not evidence either way: a templated or partial block. Counted, not hidden.
				unparsed++
				t.Logf("could-not-check: %s has a YAML block that does not parse as strict YAML (%v)", rel, err)
			}
			for _, doc := range docs {
				problems, n := workloadMptcpProblems(doc)
				workloads += n
				byPath[rel] += n
				for _, p := range problems {
					t.Errorf("%s: %s", rel, p)
				}
				if _, skip := composeExempt[rel]; compose && !skip {
					cp, sn := composeMptcpProblems(doc)
					services += sn
					for _, p := range cp {
						t.Errorf("%s: %s", rel, p)
					}
				}
			}
		}
	})
	// The sweep must have reached the places it exists for, or a moved file or a
	// broken walk would let it pass over nothing.
	for _, want := range []string{
		"containers/k8s/base/worker-desk.yaml",
		"plugins/assay/skills/pdfingest/SETUP.md",
	} {
		if byPath[want] == 0 {
			t.Errorf("sweep found no workload in %s", want)
		}
	}
	if services == 0 {
		t.Errorf("sweep found no Compose services (containers/compose.yaml)")
	}
	t.Logf("checked %d workloads and %d Compose services; %d YAML blocks could not be parsed", workloads, services, unparsed)
}
