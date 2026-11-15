package deskkit

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// CVE-2026-98432 stopgap (#6213), the two sites mptcp_test.go does not reach:
// Compose services (they hold no pod spec) and kustomize patches (they carry
// only what they change, and the rendered check needs a kustomize binary this
// test cannot assume). The value rule is the runtime's, godebugDisablesMPTCP.
//
// Declared residual, not swept by the tests that run in the ordinary CI job (each
// is reachable only through a reviewed commit to this repository; the rendered
// subtest of TestMPTCPOffInEveryWorkload sees the first group, not the wrapper,
// and only where a kustomize binary is on PATH, which that job does not install):
//   - kustomize transformers, generators and replacements, which can rewrite or
//     drop an env entry without a patch;
//   - a patch that sets command or args on a container to a wrapper that resets
//     GODEBUG for the process it starts;
//   - Compose files reached through include:, services in a *.compose.yaml file,
//     and a service command or entrypoint that wraps the process.

var composeFile = regexp.MustCompile(`(^|/)(docker-)?compose[^/]*\.ya?ml$`)

// composeMptcpExempt lists Compose files whose services run no Go program.
var composeMptcpExempt = map[string]string{
	"containers/postgres/compose.yaml": "upstream PostgreSQL image (a C server); GODEBUG is read only by Go runtimes",
	"workflow/testdata/compose.yaml":   "disposable PostgreSQL fixture; GODEBUG is read only by Go runtimes",
}

// yamlDocs decodes every non-empty document of a YAML (or JSON) stream into a map.
// A stream that does not decode is an error: an unreadable file is never a pass.
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

// yamlNodes is yamlDocs for streams whose documents may be lists (a JSON6902
// operation list), and keeps nothing but the decoded value.
func yamlNodes(raw []byte) ([]any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var out []any
	for {
		var doc any
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			return out, err
		}
		if doc != nil {
			out = append(out, doc)
		}
	}
}

// patchProblems sweeps one decoded kustomize patch document: a JSON6902
// operation list (a top-level list) or a strategic-merge patch (a map).
func patchProblems(node any, where string) []string {
	ops, isOps := node.([]any)
	if !isOps {
		return patchMptcpProblems(node, where)
	}
	var out []string
	for i, item := range ops {
		out = append(out, jsonPatchOpProblems(item, where+" op "+strconv.Itoa(i+1))...)
	}
	return out
}

// podSpecSegments are the path segments that reach a pod spec or its
// containers, for every workload kind (StatefulSet and Deployment: template;
// CronJob: jobTemplate; Pod: containers directly under /spec).
var podSpecSegments = map[string]bool{"template": true, "jobTemplate": true, "containers": true, "initContainers": true, "env": true}

// reachesPodSpec reports whether a JSON6902 pointer is, is inside, or is an
// ancestor of a pod spec or its env: such an op can drop or flip the base entry.
func reachesPodSpec(ptr string) bool {
	if ptr == "" || ptr == "/" || ptr == "/spec" {
		// The whole document, or the whole of a Pod's spec.
		return true
	}
	for _, seg := range strings.Split(ptr, "/") {
		if podSpecSegments[seg] {
			return true
		}
	}
	return false
}

// jsonPatchOpProblems lists what one JSON6902 operation does to the pod spec.
// The only op allowed to reach it appends or inserts one env entry (add at
// .../env/- or .../env/<n>), and that entry may carry only the literal setting.
// Any other op that reaches a pod spec, as path or from, could remove or
// replace the base entry unseen.
func jsonPatchOpProblems(item any, where string) []string {
	op, _ := item.(map[string]any)
	kind, _ := op["op"].(string)
	path, pathOK := op["path"].(string)
	if kind == "" || !pathOK {
		return []string{where + ": an operation without a string op and path cannot be read"}
	}
	from, _ := op["from"].(string)
	if kind == "test" || (!reachesPodSpec(path) && (from == "" || !reachesPodSpec(from))) {
		return nil
	}
	segs := strings.Split(path, "/")
	last := segs[len(segs)-1]
	_, atoiErr := strconv.Atoi(last)
	insertsEnvEntry := kind == "add" && from == "" && len(segs) >= 2 && segs[len(segs)-2] == "env" && (last == "-" || atoiErr == nil)
	if !insertsEnvEntry {
		return []string{where + ": " + kind + " " + path + " can drop or reshape the base GODEBUG entry unseen"}
	}
	entry, isMap := op["value"].(map[string]any)
	if !isMap {
		return []string{where + ": an env entry added by a patch must be a map"}
	}
	return patchMptcpProblems(entry, where)
}

// composeMptcpProblems lists the services of one Compose document that do not
// leave MPTCP off, in either the map or the KEY=value list form of `environment`,
// and counts the services it read.
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
		var values []string
		switch env := svc["environment"].(type) {
		case map[string]any:
			if v, ok := env["GODEBUG"]; ok {
				s, isString := v.(string)
				if !isString {
					s = "\x00not a string"
				}
				values = append(values, s)
			}
		case []any:
			for _, e := range env {
				s, _ := e.(string)
				switch {
				case strings.HasPrefix(s, "GODEBUG="):
					values = append(values, strings.TrimPrefix(s, "GODEBUG="))
				case s == "GODEBUG":
					// Compose reads a bare name from the host environment.
					values = append(values, "\x00from the host environment")
				}
			}
		}
		if len(values) != 1 || !godebugDisablesMPTCP(values[0]) {
			problems = append(problems, "service "+n+": environment needs exactly one GODEBUG="+mptcpOff)
		}
	}
	return problems, services
}

// patchMptcpProblems lists what a strategic-merge patch does to GODEBUG. A patch
// may leave the base entry alone; it may not carry one that is anything but the
// literal setting, and it may not carry a strategic-merge directive ($patch,
// $retainKeys, ...) or a null (which clears the field it names) that could
// delete or replace the base entry unseen. Every entry of a containers or
// initContainers list must carry the literal entry itself: the list merges by
// name, so an entry whose name is not in the base is a container the patch
// adds, and it would have none (the merged one is harmless: same name, same
// value). The sweep cannot tell the two apart without rendering, so it asks
// for the entry on both.
func patchMptcpProblems(node any, where string) []string {
	var out []string
	switch v := node.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if strings.HasPrefix(k, "$") {
				out = append(out, where+": patch directive "+k+" can drop the base GODEBUG entry unseen")
			}
			if v[k] == nil {
				// A null clears the field it names in the target.
				out = append(out, where+": null "+k+" can drop base fields, GODEBUG among them, unseen")
			}
			out = append(out, patchMptcpProblems(v[k], where)...)
			if k == "containers" || k == "initContainers" {
				out = append(out, containerListProblems(k, v[k], where)...)
			}
		}
		if v["name"] == "GODEBUG" {
			if s, ok := v["value"].(string); !ok || s != mptcpOff || len(v) != 2 {
				out = append(out, where+": a GODEBUG entry in a patch must be exactly name+value "+mptcpOff)
			}
		}
	case []any:
		for _, item := range v {
			if item == nil {
				out = append(out, where+": a null list item can drop base entries unseen")
			}
			out = append(out, patchMptcpProblems(item, where)...)
		}
	}
	return out
}

// containerListProblems lists the entries of a patch's containers or
// initContainers list that carry no GODEBUG env entry. Whether
// that entry is the literal setting is patchMptcpProblems' own check.
func containerListProblems(key string, list any, where string) []string {
	items, _ := list.([]any)
	var out []string
	for i, item := range items {
		c, _ := item.(map[string]any)
		env, _ := c["env"].([]any)
		n := 0
		for _, e := range env {
			if m, ok := e.(map[string]any); ok && m["name"] == "GODEBUG" {
				n++
			}
		}
		if n == 0 {
			out = append(out, where+": "+key+" entry "+strconv.Itoa(i+1)+" must carry a GODEBUG="+mptcpOff+" env entry: a container the patch adds would have none")
		}
	}
	return out
}

// regression: #6213
func TestMptcpOffOnEveryComposeSvc(t *testing.T) {
	const svc = "services:\n  a:\n    image: i\n"
	for _, c := range []struct {
		name, text string
		bad        bool
	}{
		{"no environment", svc, true},
		{"enabled", svc + "    environment:\n      GODEBUG: multipathtcp=1\n", true},
		{"other setting only", svc + "    environment:\n      GODEBUG: http2debug=1\n", true},
		{"list form twice", svc + "    environment: [\"GODEBUG=multipathtcp=0\", \"GODEBUG=multipathtcp=1\"]\n", true},
		{"space after comma", svc + "    environment:\n      GODEBUG: \"multipathtcp=1, multipathtcp=0\"\n", true},
		{"trailing space", svc + "    environment:\n      GODEBUG: \"multipathtcp=0 \"\n", true},
		{"bare name after the setting", svc + "    environment: [\"GODEBUG=multipathtcp=0\", \"GODEBUG\"]\n", true},
		{"not a string", svc + "    environment:\n      GODEBUG: 0\n", true},
		{"own environment replaces the shared one", "x-c: &c\n  environment:\n    GODEBUG: multipathtcp=0\nservices:\n  a:\n    <<: *c\n    environment:\n      OTHER: x\n", true},
		{"map form", svc + "    environment:\n      GODEBUG: multipathtcp=0\n", false},
		{"list form", svc + "    environment: [\"GODEBUG=multipathtcp=0\"]\n", false},
		{"merged setting", svc + "    environment:\n      GODEBUG: http2debug=1,multipathtcp=0\n", false},
		{"inherited through a merge key", "x-c: &c\n  environment:\n    GODEBUG: multipathtcp=0\nservices:\n  a:\n    <<: *c\n", false},
	} {
		docs, err := yamlDocs([]byte(c.text))
		if err != nil || len(docs) != 1 {
			t.Fatalf("control %q does not parse: %v", c.name, err)
		}
		if problems, n := composeMptcpProblems(docs[0]); n != 1 || (len(problems) > 0) != c.bad {
			t.Errorf("control %q: services=%d problems=%v, want defect=%v", c.name, n, problems, c.bad)
		}
	}

	skipIfFixtureAbsent(t, fixtureRepoRoot+"/containers/compose.yaml", "the Compose files ship only in a full checkout")
	manifests, _, _, _, _ := repoSweepInputs(t, fixtureRepoRoot)
	services, files := 0, map[string]bool{}
	seenExempt := map[string]bool{}
	for _, rel := range manifests {
		if !composeFile.MatchString(rel) || strings.HasSuffix(rel, ".json") {
			continue
		}
		if _, ok := composeMptcpExempt[rel]; ok {
			seenExempt[rel] = true
			continue
		}
		raw, err := os.ReadFile(filepath.Join(fixtureRepoRoot, rel))
		if err != nil {
			t.Fatal(err)
		}
		docs, err := yamlDocs(raw)
		if err != nil {
			t.Errorf("%s: does not decode: %v", rel, err)
			continue
		}
		files[rel] = true
		for _, doc := range docs {
			problems, n := composeMptcpProblems(doc)
			services += n
			for _, p := range problems {
				t.Errorf("%s: %s", rel, p)
			}
		}
	}
	for rel := range composeMptcpExempt {
		if !seenExempt[rel] {
			t.Errorf("exemption %s names no Compose file in the tree: drop it", rel)
		}
	}
	if !files["containers/compose.yaml"] || services == 0 {
		t.Fatalf("the sweep did not reach containers/compose.yaml (files %v, services %d)", files, services)
	}
	t.Logf("%d Compose services in %d files, %d exempt", services, len(files), len(seenExempt))
}

// regression: #6213
func TestMptcpPatchesKeepBaseEntry(t *testing.T) {
	for _, c := range []struct {
		name, text string
		bad        bool
	}{
		{"patch without containers", `{"spec":{"replicas":0}}`, false},
		{"container with the setting", `{"spec":{"containers":[{"name":"c","env":[{"name":"X","value":"1"},{"name":"GODEBUG","value":"multipathtcp=0"}]}]}}`, false},
		{"initContainer with the setting", `{"spec":{"initContainers":[{"name":"i","env":[{"name":"GODEBUG","value":"multipathtcp=0"}]}]}}`, false},
		{"container without GODEBUG", `{"spec":{"containers":[{"name":"c","env":[{"name":"X","value":"1"}]}]}}`, true},
		{"added container without env", `{"spec":{"template":{"spec":{"containers":[{"name":"sidecar","image":"i"}]}}}}`, true},
		{"added initContainer without GODEBUG", `{"spec":{"template":{"spec":{"initContainers":[{"name":"init","image":"i","env":[{"name":"X","value":"1"}]}]}}}}`, true},
		{"second container without GODEBUG", `{"spec":{"containers":[{"name":"a","env":[{"name":"GODEBUG","value":"multipathtcp=0"}]},{"name":"b"}]}}`, true},
		{"literal setting", `{"env":[{"name":"GODEBUG","value":"multipathtcp=0"}]}`, false},
		{"enabled", `{"env":[{"name":"GODEBUG","value":"multipathtcp=1"}]}`, true},
		{"merged setting", `{"env":[{"name":"GODEBUG","value":"http2debug=1,multipathtcp=0"}]}`, true},
		{"valueFrom", `{"env":[{"name":"GODEBUG","valueFrom":{"secretKeyRef":{"name":"s","key":"k"}}}]}`, true},
		{"delete directive", `{"env":[{"name":"GODEBUG","$patch":"delete"}]}`, true},
		{"replace directive", `{"containers":[{"name":"c","$patch":"replace"}]}`, true},
		{"retainKeys", `{"env":[{"$retainKeys":["name"],"name":"X"}]}`, true},
		{"null env", `{"spec":{"template":{"spec":{"containers":[{"name":"the-desk","env":null}]}}}}`, true},
		{"null containers", `{"spec":{"template":{"spec":{"containers":null}}}}`, true},
		{"null value", `{"env":[{"name":"GODEBUG","value":null}]}`, true},
		{"null list item", `{"env":[null]}`, true},
		{"json6902 storage class", `[{"op":"add","path":"/spec/volumeClaimTemplates/0/spec/storageClassName","value":"x"}]`, false},
		{"json6902 add env entry", `[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"X","value":"1"}}]`, false},
		{"json6902 add literal setting", `[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"GODEBUG","value":"multipathtcp=0"}}]`, false},
		{"json6902 add other setting", `[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"GODEBUG","value":"multipathtcp=1"}}]`, true},
		{"json6902 remove env entry", `[{"op":"remove","path":"/spec/template/spec/containers/0/env/2"}]`, true},
		{"json6902 replace env entry", `[{"op":"replace","path":"/spec/template/spec/containers/0/env/2/value","value":"multipathtcp=1"}]`, true},
		// The only op that may touch an env entry is an add; a replace of a whole
		// entry with a harmless one drops the base GODEBUG entry (the `kind == "add"` condition).
		{"json6902 replace whole env entry", `[{"op":"replace","path":"/spec/template/spec/containers/0/env/2","value":{"name":"X","value":"1"}}]`, true},
		{"json6902 add replaces env list", `[{"op":"add","path":"/spec/template/spec/containers/0/env","value":[]}]`, true},
		{"json6902 add replaces pod spec", `[{"op":"add","path":"/spec/template/spec","value":{}}]`, true},
		{"json6902 remove containers", `[{"op":"remove","path":"/spec/template/spec/containers"}]`, true},
		{"json6902 add container", `[{"op":"add","path":"/spec/template/spec/containers/-","value":{"name":"x"}}]`, true},
		{"json6902 remove template", `[{"op":"remove","path":"/spec/template"}]`, true},
		{"json6902 remove spec", `[{"op":"remove","path":"/spec"}]`, true},
		{"json6902 copy from env", `[{"op":"copy","from":"/spec/template/spec/containers/0/env","path":"/metadata/annotations"}]`, true},
		{"json6902 move from env", `[{"op":"move","from":"/spec/template/spec/containers/0/env/0","path":"/metadata/x"}]`, true},
		{"json6902 cronjob env", `[{"op":"remove","path":"/spec/jobTemplate/spec/template/spec/containers/0/env/0"}]`, true},
		{"json6902 pod containers", `[{"op":"remove","path":"/spec/containers/0/env"}]`, true},
		{"json6902 add null env entry", `[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":null}]`, true},
		{"json6902 unreadable op", `[{"path":"/spec/template"}]`, true},
	} {
		docs, err := yamlNodes([]byte(c.text))
		if err != nil || len(docs) != 1 {
			t.Fatalf("control %q does not parse: %v", c.name, err)
		}
		if got := patchProblems(docs[0], c.name); (len(got) > 0) != c.bad {
			t.Errorf("control %q: problems=%v, want defect=%v", c.name, got, c.bad)
		}
	}

	skipIfFixtureAbsent(t, fixtureRepoRoot+"/containers/k8s", "the desk manifests ship only in a full checkout")
	_, _, _, _, patches := repoSweepInputs(t, fixtureRepoRoot)
	const compat = "containers/k8s/components/compatibility-cellctl/workloads.yaml"
	if !patches[compat] {
		t.Fatalf("the sweep did not find %s among the kustomize patches %v", compat, patches)
	}
	paths := make([]string, 0, len(patches))
	for p := range patches {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		raw, err := os.ReadFile(filepath.Join(fixtureRepoRoot, rel))
		if err != nil {
			t.Errorf("%s: patch listed by a kustomization is unreadable: %v", rel, err)
			continue
		}
		docs, err := yamlNodes(raw)
		if err != nil {
			t.Errorf("%s: does not decode: %v", rel, err)
			continue
		}
		for _, doc := range docs {
			for _, p := range patchProblems(doc, rel) {
				t.Error(p)
			}
		}
	}
	t.Logf("%d kustomize patches swept", len(paths))
}

// regression: #6213
func TestMptcpInlineKustomizePatches(t *testing.T) {
	const head = "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n"
	for _, c := range []struct {
		name, text string
		files      []string
		bad        bool
	}{
		{"storage class op", head + "patches:\n  - target: {kind: StatefulSet}\n    patch: |-\n      - op: add\n        path: /spec/volumeClaimTemplates/0/spec/storageClassName\n        value: x\n", nil, false},
		{"inline null env", head + "patches:\n  - patch: |-\n      {\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"the-desk\",\"env\":null}]}}}}\n", nil, true},
		{"inline op removes env", head + "patches:\n  - target: {kind: StatefulSet}\n    patch: |-\n      - op: remove\n        path: /spec/template/spec/containers/0/env/2\n", nil, true},
		{"inline op flips setting", head + "patches:\n  - patch: |-\n      - op: replace\n        path: /spec/template/spec/containers/0/env/2/value\n        value: multipathtcp=1\n", nil, true},
		{"json6902 inline", head + "patchesJson6902:\n  - target: {kind: StatefulSet, name: x}\n    patch: |-\n      - op: remove\n        path: /spec/template/spec/containers/0/env\n", nil, true},
		{"inline adds a container", head + "patches:\n  - patch: |-\n      {\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"sidecar\",\"image\":\"i\"}]}}}}\n", nil, true},
		{"json6902 file", head + "patchesJson6902:\n  - target: {kind: StatefulSet, name: x}\n    path: ops.yaml\n", []string{"k/ops.yaml"}, false},
		{"patch file", head + "patches:\n  - path: p.yaml\n", []string{"k/p.yaml"}, false},
	} {
		files, err := kustomizePatchFiles("k", []byte(c.text))
		if err != nil || strings.Join(files, ",") != strings.Join(c.files, ",") {
			t.Errorf("control %q: files=%v err=%v, want %v", c.name, files, err, c.files)
		}
		bodies, err := kustomizeInlinePatches([]byte(c.text))
		if err != nil {
			t.Fatalf("control %q does not parse: %v", c.name, err)
		}
		var got []string
		for _, b := range bodies {
			docs, err := yamlNodes([]byte(b))
			if err != nil {
				got = append(got, c.name+": inline patch does not decode")
			}
			for _, d := range docs {
				got = append(got, patchProblems(d, c.name)...)
			}
		}
		if (len(got) > 0) != c.bad {
			t.Errorf("control %q: problems=%v, want defect=%v", c.name, got, c.bad)
		}
	}

	skipIfFixtureAbsent(t, fixtureRepoRoot+"/containers/k8s", "the desk manifests ship only in a full checkout")
	_, _, _, roots, _ := repoSweepInputs(t, fixtureRepoRoot)
	const example = "containers/k8s/overlays/compatibility-example"
	bodies, seenExample := 0, false
	for _, dir := range roots {
		for _, name := range []string{"kustomization.yaml", "kustomization.yml", "Kustomization"} {
			raw, err := os.ReadFile(filepath.Join(fixtureRepoRoot, dir, name))
			if err != nil {
				continue
			}
			inline, err := kustomizeInlinePatches(raw)
			if err != nil {
				t.Errorf("%s/%s: does not decode: %v", dir, name, err)
				continue
			}
			for i, b := range inline {
				bodies++
				seenExample = seenExample || dir == example
				where := dir + "/" + name + " inline patch " + strconv.Itoa(i+1)
				docs, err := yamlNodes([]byte(b))
				if err != nil {
					t.Errorf("%s: does not decode: %v", where, err)
				}
				for _, doc := range docs {
					for _, p := range patchProblems(doc, where) {
						t.Error(p)
					}
				}
			}
		}
	}
	if !seenExample {
		t.Fatalf("the sweep did not reach the inline patch in %s (%d inline patches read)", example, bodies)
	}
	t.Logf("%d inline kustomize patches swept", bodies)
}
