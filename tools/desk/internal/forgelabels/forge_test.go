package forgelabels

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// fakeForge is an in-memory GitHub repo or GitLab project behind the Transport interface. No
// test here reaches a network.
type fakeForge struct {
	t     *testing.T
	forge string
	// labels are the target's own labels; group are a GitLab project's inherited ancestor-group
	// labels (listed only with include_ancestor_groups=true).
	labels, group map[string]bool
	// writeStatus answers every create (0 = normal); listStatus every list page (0 = 200);
	// listErr makes a list page a transport failure.
	writeStatus, listStatus int
	listErr                 error
	// fullPages makes every list page full, so a list never ends (truncation).
	fullPages bool
	reqs      []string
}

func newFake(t *testing.T, forge string, have ...string) *fakeForge {
	f := &fakeForge{t: t, forge: forge, labels: map[string]bool{}, group: map[string]bool{}}
	for _, h := range have {
		f.labels[h] = true
	}
	return f
}

func jsonBody(v any) []byte { b, _ := json.Marshal(v); return b }

func (f *fakeForge) Do(method, path string, body any) (Response, error) {
	f.reqs = append(f.reqs, method+" "+path)
	p, rawQuery, _ := strings.Cut(path, "?")
	q, _ := url.ParseQuery(rawQuery)
	wantPath := "/repos/example-org/example-repo/labels"
	if f.forge == "gitlab" {
		wantPath = "/projects/example-group%2Fexample-project/labels"
	}
	if p != wantPath {
		f.t.Errorf("fake %s: unexpected path %s %s", f.forge, method, path)
		return Response{Status: 404}, nil
	}
	switch method {
	case "POST":
		if f.writeStatus != 0 {
			return Response{Status: f.writeStatus, Body: jsonBody(map[string]any{"message": "403 Forbidden"})}, nil
		}
		m := body.(map[string]any)
		name := m["name"].(string)
		color := m["color"].(string)
		if (f.forge == "gitlab") != strings.HasPrefix(color, "#") {
			f.t.Errorf("fake %s: label %s color %q (gitlab wants '#', github bare)", f.forge, name, color)
		}
		if f.forge == "github" {
			if f.labels[name] {
				return Response{Status: 422, Body: jsonBody(map[string]any{"message": "Validation Failed",
					"errors": []map[string]any{{"resource": "Label", "code": "already_exists", "field": "name"}}})}, nil
			}
			f.labels[name] = true
			return Response{Status: 201}, nil
		}
		if f.group[name] {
			// GitLab refuses a project label whose title an ancestor group already defines.
			return Response{Status: 400, Body: jsonBody(map[string]any{"message": map[string]any{
				"title": []string{"already exists at group level for example-group. Please choose another one."}}})}, nil
		}
		if f.labels[name] {
			return Response{Status: 409, Body: jsonBody(map[string]any{"message": "Label already exists"})}, nil
		}
		f.labels[name] = true
		return Response{Status: 201}, nil
	case "GET":
		if f.listErr != nil {
			return Response{}, f.listErr
		}
		if f.listStatus != 0 {
			return Response{Status: f.listStatus, Body: jsonBody(map[string]any{"message": "boom"})}, nil
		}
		page, _ := strconv.Atoi(q.Get("page"))
		per, _ := strconv.Atoi(q.Get("per_page"))
		if page < 1 || per < 1 {
			f.t.Errorf("fake %s: list without page/per_page: %s", f.forge, path)
		}
		var all []map[string]string
		for n := range f.labels {
			all = append(all, map[string]string{"name": n})
		}
		if f.forge == "gitlab" && q.Get("include_ancestor_groups") == "true" {
			for n := range f.group {
				all = append(all, map[string]string{"name": n})
			}
		}
		if f.fullPages {
			chunk := make([]map[string]string, per)
			for i := range chunk {
				chunk[i] = map[string]string{"name": fmt.Sprintf("filler-%d-%d", page, i)}
			}
			return Response{Status: 200, Body: jsonBody(chunk)}, nil
		}
		// Stable order so pagination is deterministic.
		sortByName(all)
		lo, hi := (page-1)*per, page*per
		if lo > len(all) {
			lo = len(all)
		}
		if hi > len(all) {
			hi = len(all)
		}
		return Response{Status: 200, Body: jsonBody(all[lo:hi])}, nil
	}
	f.t.Errorf("fake %s: unexpected method %s", f.forge, method)
	return Response{Status: 405}, nil
}

func sortByName(s []map[string]string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j]["name"] < s[j-1]["name"]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func backend(f *fakeForge) Backend {
	if f.forge == "gitlab" {
		return GitLab(f, "example-group/example-project")
	}
	return GitHub(f, "example-org/example-repo")
}

var set = Canonical(DocumentedRoles())

// regression: #6127
// TestEnsureCreatesTheFullSetThenNoOps — the install-path ensure on both forges: a bare target
// gets every label of the set, to: rows included; a second run is all no-ops.
func TestEnsureCreatesTheFullSetThenNoOps(t *testing.T) {
	for _, forge := range []string{"github", "gitlab"} {
		f := newFake(t, forge)
		b := backend(f)
		for _, r := range Ensure(b, set) {
			if r.Err != nil || r.Exists {
				t.Errorf("%s first run: %s exists=%t err=%v", forge, r.Label.Name, r.Exists, r.Err)
			}
		}
		for _, want := range []string{"to:worker", "to:reviewer", "to:desk", "raised-by:verifier"} {
			if !f.labels[want] {
				t.Errorf("%s: %s not created", forge, want)
			}
		}
		for _, r := range Ensure(b, set) {
			if r.Err != nil || !r.Exists {
				t.Errorf("%s re-run: %s exists=%t err=%v, want a no-op", forge, r.Label.Name, r.Exists, r.Err)
			}
		}
		if n, _ := Failed(Ensure(b, set)); n != 0 {
			t.Errorf("%s: %d failed on a third run", forge, n)
		}
	}
}

// TestEnsureGitLabInheritedGroupLabelCountsAsPresent — a label the project inherits from its
// group is a no-op, and no project copy is created to shadow it.
func TestEnsureGitLabInheritedGroupLabelCountsAsPresent(t *testing.T) {
	f := newFake(t, "gitlab")
	f.group["to:worker"] = true
	for _, r := range Ensure(backend(f), set) {
		if r.Label.Name == "to:worker" && (r.Err != nil || !r.Exists) {
			t.Errorf("inherited to:worker: exists=%t err=%v, want present", r.Exists, r.Err)
		}
	}
	if f.labels["to:worker"] {
		t.Error("a project copy of an inherited group label was created")
	}
}

// TestEnsureDeniedIsUnfinished — a credential that cannot write labels fails every row and
// is reported as denied (the caller prints the finish command), never as success.
func TestEnsureDeniedIsUnfinished(t *testing.T) {
	for _, forge := range []string{"github", "gitlab"} {
		for _, st := range []int{401, 403} {
			f := newFake(t, forge)
			f.writeStatus = st
			n, denied := Failed(Ensure(backend(f), set))
			if n != len(set) || !denied {
				t.Errorf("%s %d: failed=%d denied=%t, want %d denied", forge, st, n, denied, len(set))
			}
		}
		f := newFake(t, forge)
		f.writeStatus = 500
		if _, denied := Failed(Ensure(backend(f), set)); denied {
			t.Errorf("%s: a 500 read as a denied credential", forge)
		}
	}
}

// regression: #6127
// TestCheckNamesEachMissingLabel — the read-only check names every label of the set the
// target lacks, in set order, on both forges.
func TestCheckNamesEachMissingLabel(t *testing.T) {
	for _, forge := range []string{"github", "gitlab"} {
		var have []string
		for _, l := range set {
			if l.Name != "to:worker" && l.Name != "to:reviewer" {
				have = append(have, l.Name)
			}
		}
		f := newFake(t, forge, have...)
		missing, err := Check(backend(f), set)
		if err != nil {
			t.Fatalf("%s: %v", forge, err)
		}
		if strings.Join(missing, ",") != "to:worker,to:reviewer" {
			t.Errorf("%s: missing = %v, want [to:worker to:reviewer]", forge, missing)
		}
		for _, r := range f.reqs {
			if strings.HasPrefix(r, "POST") {
				t.Errorf("%s: the check wrote: %s", forge, r)
			}
		}
	}
}

// TestCheckCountsInheritedGroupLabels — on GitLab the check lists with
// include_ancestor_groups, so a group label is present, not missing.
func TestCheckCountsInheritedGroupLabels(t *testing.T) {
	f := newFake(t, "gitlab")
	for _, l := range set {
		f.group[l.Name] = true
	}
	missing, err := Check(backend(f), set)
	if err != nil || len(missing) != 0 {
		t.Errorf("all labels inherited: missing=%v err=%v, want none", missing, err)
	}
}

// TestCheckIsCaseInsensitive — both forges treat label names case-insensitively for
// uniqueness, so `To:Worker` satisfies `to:worker`.
func TestCheckIsCaseInsensitive(t *testing.T) {
	var have []string
	for _, l := range set {
		have = append(have, strings.ToUpper(l.Name))
	}
	missing, err := Check(backend(newFake(t, "github", have...)), set)
	if err != nil || len(missing) != 0 {
		t.Errorf("missing=%v err=%v", missing, err)
	}
}

// TestCheckReadFailureIsCouldNotCheck — a failed or truncated read is an error, never an
// empty missing list.
func TestCheckReadFailureIsCouldNotCheck(t *testing.T) {
	for _, forge := range []string{"github", "gitlab"} {
		for name, plant := range map[string]func(*fakeForge){
			"500":       func(f *fakeForge) { f.listStatus = 500 },
			"403":       func(f *fakeForge) { f.listStatus = 403 },
			"transport": func(f *fakeForge) { f.listErr = errors.New("connection reset") },
			"truncated": func(f *fakeForge) { f.fullPages = true },
		} {
			f := newFake(t, forge)
			plant(f)
			missing, err := Check(backend(f), set)
			if err == nil {
				t.Errorf("%s %s: no error (missing=%v) — a read failure must be could-not-check", forge, name, missing)
			}
			if missing != nil {
				t.Errorf("%s %s: a failed read still answered missing=%v", forge, name, missing)
			}
		}
	}
}

// TestCheckPaginates — a target with more labels than one page is read to the end, so a
// label on page two is not reported missing.
func TestCheckPaginates(t *testing.T) {
	have := Names(set)
	for i := 0; i < 250; i++ {
		have = append(have, fmt.Sprintf("a-filler-%03d", i))
	}
	f := newFake(t, "github", have...)
	missing, err := Check(backend(f), set)
	if err != nil || len(missing) != 0 {
		t.Errorf("missing=%v err=%v", missing, err)
	}
	if gets := strings.Count(strings.Join(f.reqs, "\n"), "GET "); gets < 3 {
		t.Errorf("%d list requests for %d labels, want at least 3 pages", gets, len(have))
	}
}

// TestSingleColonNamesArePlainLabels — every name the set sends to GitLab keeps its single
// colon; nothing is rewritten to the `::` scoped form.
func TestSingleColonNamesArePlainLabels(t *testing.T) {
	f := newFake(t, "gitlab")
	Ensure(backend(f), set)
	for n := range f.labels {
		if strings.Contains(n, "::") {
			t.Errorf("sent scoped label %q", n)
		}
	}
	if !f.labels["to:worker"] {
		t.Error("to:worker not sent as a plain single-colon label")
	}
}

func TestCommand(t *testing.T) {
	for _, c := range []struct {
		forge, target, base string
		check               bool
		want                string
	}{
		{"github", "example-org/example-repo", "", false,
			"deskfleet labels --forge github --repo example-org/example-repo --roles desk,worker --token-file " + CredentialPlaceholder},
		{"gitlab", "example-group/example-project", "https://gitlab.example.com/api/v4", true,
			"GITLAB_API_BASE=https://gitlab.example.com/api/v4 deskfleet labels --forge gitlab --project example-group/example-project --roles desk,worker --check --token-file " + CredentialPlaceholder},
		{"github", "o/it's", "", false,
			`deskfleet labels --forge github --repo 'o/it'\''s' --roles desk,worker --token-file ` + CredentialPlaceholder},
	} {
		if got := Command(c.forge, c.target, []string{"desk", "worker"}, c.base, c.check); got != c.want {
			t.Errorf("Command(%s %s) =\n%s\nwant\n%s", c.forge, c.target, got, c.want)
		}
	}
	// A roster role is operator text the printed command carries into a shell: one that is not
	// shell-literal is quoted with the rest, never pasted bare.
	want0 := `deskfleet labels --forge github --repo o/r --roles 'desk,wo$(x)rker' --token-file ` + CredentialPlaceholder
	if got := Command("github", "o/r", []string{"desk", "wo$(x)rker"}, "", false); got != want0 {
		t.Errorf("Command with a shell-active role =\n%s\nwant\n%s", got, want0)
	}
	// No bound role: the command names the placeholder, never a guessed list.
	want := "deskfleet labels --forge github --repo o/r --roles " + RolesPlaceholder + " --token-file " + CredentialPlaceholder
	if got := Command("github", "o/r", nil, "", false); got != want {
		t.Errorf("Command with no roles =\n%s\nwant\n%s", got, want)
	}
}
