package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// check.go — the three-state comparison.
//
// THE RULE THIS FILE EXISTS FOR (#127): a value that could
// not be ESTABLISHED is `could-not-check`. Never `checked-ok`, never
// `checked-wrong`. GitHub returns `null` for admin-visibility-gated fields when a
// non-admin token reads them, and that response is byte-identical whether the
// feature is on or off — `security_and_analysis` is null on every repo read here
// with a member token, and a ruleset detail read the same way carries no
// `bypass_actors` key at all. A two-state instrument reads one of those as an
// answer and reproduces #127 exactly.
//
// The forge-gitlab guard-read-custody brief moved the fetcher from a shelled `gh api <endpoint>` onto the
// enumerated deskkit.Forge seam (op 40 RepoHardeningRead, op 22 ReadFile), under the
// dedicated read-only `auditor` identity — see forge.go. The three-state semantics below
// are UNCHANGED; only the SOURCE of a fetch failure's status moved from regexing `gh`'s
// stderr to *deskkit.ForgeAPIError.Status / deskkit.IsForgeNotFound / deskkit.IsForgeForbidden.

// State is the verdict for one row.
type State string

const (
	StateOK           State = "checked-ok"
	StateWrong        State = "checked-wrong"
	StateUnknown      State = "could-not-check"
	StateNotAvailable State = "not available"
)

// Result is one row's verdict plus the sentence explaining it.
type Result struct {
	Row    Row
	State  State
	Detail string
}

// Checker evaluates rows against a resolved Forge, for ONE repo — the repo the guard was
// invoked with (--repo), never a repo read out of a row's Read cell (the new `read <kind>` /
// `read file <path>` grammar names no repo at all).
type Checker struct {
	Repo  deskkit.ForgeRepo
	Forge deskkit.Forge
}

// CheckRow returns the verdict for one row.
func (c Checker) CheckRow(r Row) Result {
	if r.notAvailable() {
		// Recorded divergence: the setting does not exist on this repo's plan or
		// visibility. No request is made — asking would produce an error that
		// looks like a permission wall and muddy the two states that matter.
		return Result{Row: r, State: StateNotAvailable, Detail: r.Required}
	}

	parsed, err := r.ParseRead()
	if err != nil {
		return Result{Row: r, State: StateUnknown, Detail: err.Error()}
	}

	if parsed.File != "" {
		return c.checkFileRow(r, parsed.File)
	}
	return c.checkKindRow(r, parsed.Kind)
}

// checkFileRow evaluates a `read file <path>` row: presence only, via the existing ReadFile
// (op 22). A file row carries no Field selector — the document either exists on the ref or it
// does not, and that IS the check.
func (c Checker) checkFileRow(r Row, path string) Result {
	fc, err := c.Forge.ReadFile(c.Repo, deskkit.ReadFileInput{File: path})
	if err != nil {
		if deskkit.IsForgeNotFound(err) {
			return Result{Row: r, State: StateWrong,
				Detail: fmt.Sprintf("file %s: not found — absent (want %s)", path, r.Required)}
		}
		return Result{Row: r, State: StateUnknown,
			Detail: fmt.Sprintf("file %s: %v%s", path, err, adminHint(r))}
	}
	if !fc.Exists {
		return Result{Row: r, State: StateWrong,
			Detail: fmt.Sprintf("file %s: absent (want %s)", path, r.Required)}
	}
	return Result{Row: r, State: StateOK, Detail: fmt.Sprintf("file %s: present", path)}
}

// checkKindRow evaluates a `read <kind>` row: the closed hardening-read vocabulary (op 40).
// The kind is validated by the SAME deskkit.ValidateHardeningReadKind the backend itself runs
// before any request exists — validating it here too means an unknown kind is reported against
// THIS row (naming its ID and line) rather than as a bare backend error.
func (c Checker) checkKindRow(r Row, kindStr string) Result {
	kind, verr := deskkit.ValidateHardeningReadKind(kindStr)
	if verr != nil {
		return Result{Row: r, State: StateUnknown,
			Detail: fmt.Sprintf("row %q (line %d): %v", r.ID, r.Line, verr)}
	}

	raw, err := c.Forge.RepoHardeningRead(c.Repo, kind)
	if err != nil {
		return c.fetchFailure(r, kindStr, err)
	}
	var doc any
	if uerr := json.Unmarshal(raw, &doc); uerr != nil {
		return Result{Row: r, State: StateUnknown,
			Detail: fmt.Sprintf("read %s: response is not JSON (%v)", kindStr, uerr)}
	}

	field := r.Field
	if name, rest, ok := rulesetSelector(field); ok {
		arr, isArr := doc.([]any)
		if !isArr {
			return Result{Row: r, State: StateUnknown,
				Detail: fmt.Sprintf("read %s: response is not a list — a [name=%s] selector cannot be judged against it", kindStr, name)}
		}
		entry, found := findRuleset(arr, name)
		if !found {
			// The list read succeeded and the named entry is not in it. That is a real absence
			// at any permission level — ruleset NAMES (GitHub) and protected branch/tag names
			// (GitLab) are visible to anyone who can read the list, unlike their bypass lists.
			return Result{Row: r, State: StateWrong,
				Detail: fmt.Sprintf("read %s: no entry named %q in the list (want %s)", kindStr, name, r.Required)}
		}
		doc = entry
		field = rest
	}

	v, present := resolve(doc, field)
	if !present {
		return c.absent(r, field, kindStr)
	}
	return compare(r, field, v)
}

// findRuleset locates the entry in arr whose "name" field equals name — op 40's `rulesets`
// array of detail documents on GitHub, and the `protected-branches` / `protected-tags`
// arrays on GitLab (a protected branch or tag is addressed by its `name` the same way).
func findRuleset(arr []any, name string) (any, bool) {
	for _, el := range arr {
		m, ok := el.(map[string]any)
		if !ok {
			continue
		}
		if n, ok := m["name"].(string); ok && n == name {
			return m, true
		}
	}
	return nil, false
}

// fetchFailure maps a failed read to a state. 403 is always could-not-check: a
// permission wall says nothing about the value behind it. A not-found on an admin row is
// could-not-check too — GitHub hides several admin-only Actions endpoints behind
// a 404 rather than a 403 — while a not-found on a public row, from a repo the preflight
// proved the token CAN read, is a genuine absence.
func (c Checker) fetchFailure(r Row, kindStr string, err error) Result {
	if deskkit.IsForgeNotFound(err) && !r.gatedAdmin() {
		return Result{Row: r, State: StateWrong,
			Detail: fmt.Sprintf("read %s: not found — absent (want %s)", kindStr, r.Required)}
	}
	return Result{Row: r, State: StateUnknown,
		Detail: fmt.Sprintf("read %s: %v%s", kindStr, err, adminHint(r))}
}

// absent maps a field that did not resolve to a state, by the same rule.
func (c Checker) absent(r Row, field, kindStr string) Result {
	if r.gatedAdmin() {
		return Result{Row: r, State: StateUnknown,
			Detail: fmt.Sprintf("read %s: %s is null or absent — indistinguishable from a set value at this permission level%s", kindStr, field, adminHint(r))}
	}
	return Result{Row: r, State: StateWrong,
		Detail: fmt.Sprintf("read %s: %s is absent (want %s)", kindStr, field, r.Required)}
}

func adminHint(r Row) string {
	if r.gatedAdmin() {
		return " — admin-gated row: re-run as a repository admin of " + r.Repo
	}
	return ""
}

// rulesetSelector splits "[name=protect-main].bypass_actors" into the ruleset
// name and the remaining path.
func rulesetSelector(field string) (name, rest string, ok bool) {
	if !strings.HasPrefix(field, "[name=") {
		return "", "", false
	}
	end := strings.Index(field, "]")
	if end < 0 {
		return "", "", false
	}
	name = field[len("[name="):end]
	rest = strings.TrimPrefix(field[end+1:], ".")
	return name, rest, name != "" && rest != ""
}

// resolve walks a dotted path. A nil at any step is an absence, not a value:
// `security_and_analysis: null` must never satisfy a row about the field beneath it.
//
// A segment that is a non-negative integer indexes into a LIST at that step
// (`push_access_levels.0.access_level` — the GitLab protected-branch entries are lists of
// access-level objects, and on Community Edition each carries exactly one, role-level entry).
// An index past the end is an absence, exactly like a missing key; a numeric segment against
// an object is an ordinary key lookup, so a document that happens to use "0" as a key still
// resolves.
func resolve(doc any, path string) (any, bool) {
	cur := doc
	if path == "" || path == "." {
		if cur == nil {
			return nil, false
		}
		return cur, true
	}
	for _, seg := range strings.Split(path, ".") {
		switch t := cur.(type) {
		case map[string]any:
			var ok bool
			cur, ok = t[seg]
			if !ok || cur == nil {
				return nil, false
			}
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(t) || t[i] == nil {
				return nil, false
			}
			cur = t[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

// compare renders the live value and matches it against the Required cell.
func compare(r Row, field string, v any) Result {
	req := r.Required
	ok := func(shown string) Result {
		return Result{Row: r, State: StateOK, Detail: fmt.Sprintf("%s = %s", field, shown)}
	}
	wrong := func(shown string) Result {
		return Result{Row: r, State: StateWrong,
			Detail: fmt.Sprintf("%s = %s, want %s", field, shown, req)}
	}

	switch {
	case req == "[]":
		arr, isArr := v.([]any)
		if !isArr {
			return Result{Row: r, State: StateUnknown,
				Detail: fmt.Sprintf("%s is %s, not a list — an empty-list requirement cannot be judged against it", field, render(v))}
		}
		if len(arr) == 0 {
			return ok("[] (empty)")
		}
		return wrong(fmt.Sprintf("%d entr%s: %s", len(arr), plural(len(arr)), render(v)))

	case strings.HasPrefix(req, "contains:"):
		want := strings.TrimPrefix(req, "contains:")
		arr, isArr := v.([]any)
		if !isArr {
			return Result{Row: r, State: StateUnknown,
				Detail: fmt.Sprintf("%s is %s, not a list — a contains: requirement cannot be judged against it", field, render(v))}
		}
		for _, el := range arr {
			switch t := el.(type) {
			case string:
				if t == want {
					return ok(want + " present")
				}
			case map[string]any:
				if s, isStr := t["type"].(string); isStr && s == want {
					return ok(want + " present")
				}
			}
		}
		return wrong("[" + strings.Join(arrayTokens(arr), " ") + "]")

	default:
		shown := render(v)
		if shown == req {
			return ok(shown)
		}
		return wrong(shown)
	}
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

// arrayTokens renders a rules/bypass_actors array as the tokens a reader
// compares against, rather than as raw JSON.
func arrayTokens(arr []any) []string {
	out := make([]string, 0, len(arr))
	for _, el := range arr {
		switch t := el.(type) {
		case string:
			out = append(out, t)
		case map[string]any:
			if s, ok := t["type"].(string); ok {
				out = append(out, s)
				continue
			}
			out = append(out, render(el))
		default:
			out = append(out, render(el))
		}
	}
	return out
}

func render(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
