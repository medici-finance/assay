package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
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

// ghRun shells out to the real `gh` binary and returns stdout. It is a package
// var so tests can drive the evaluator without a network or a token, mirroring
// deskboard's single-choke-point pattern. Every call here is a GET; nothing in
// this tool mutates a repository, which is the whole point of the brief that
// produced it (settings are the human's to apply).
var ghRun = func(args ...string) ([]byte, error) {
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("gh %s: %v", strings.Join(args, " "), err)
	}
	return out, nil
}

func ghGet(endpoint string) ([]byte, error) { return ghRun("api", endpoint) }

var statusRe = regexp.MustCompile(`\(HTTP (\d{3})\)`)

// httpStatus digs the status code out of gh's error text. gh renders API
// failures as `gh: <message> (HTTP 403)`. A code we cannot parse returns 0,
// which every caller treats as could-not-check — the fail-closed direction.
func httpStatus(err error) int {
	if err == nil {
		return 0
	}
	if m := statusRe.FindStringSubmatch(err.Error()); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	if strings.Contains(err.Error(), "Not Found") {
		return 404
	}
	return 0
}

// Checker evaluates rows against a fetcher.
type Checker struct {
	Get func(endpoint string) ([]byte, error)
}

// CheckRow returns the verdict for one row.
func (c Checker) CheckRow(r Row) Result {
	if r.notAvailable() {
		// Recorded divergence: the setting does not exist on this repo's plan or
		// visibility. No request is made — asking would produce an error that
		// looks like a permission wall and muddy the two states that matter.
		return Result{Row: r, State: StateNotAvailable, Detail: r.Required}
	}

	endpoint, err := r.Endpoint()
	if err != nil {
		return Result{Row: r, State: StateUnknown, Detail: err.Error()}
	}

	field := r.Field
	if name, rest, ok := rulesetSelector(field); ok {
		id, res := c.rulesetID(r, endpoint, name)
		if res != nil {
			return *res
		}
		endpoint = fmt.Sprintf("repos/%s/rulesets/%d", r.Repo, id)
		field = rest
	}

	body, err := c.Get(endpoint)
	if err != nil {
		return c.fetchFailure(r, endpoint, err)
	}
	var doc any
	if uerr := json.Unmarshal(body, &doc); uerr != nil {
		return Result{Row: r, State: StateUnknown,
			Detail: fmt.Sprintf("%s: response is not JSON (%v)", endpoint, uerr)}
	}

	v, present := resolve(doc, field)
	if !present {
		return c.absent(r, field, endpoint)
	}
	return compare(r, field, v)
}

// fetchFailure maps a failed read to a state. 403 is always could-not-check: a
// permission wall says nothing about the value behind it. 404 on an admin row is
// could-not-check too — GitHub hides several admin-only Actions endpoints behind
// 404 rather than 403 — while 404 on a public row, from a repo the preflight
// proved the token CAN read, is a genuine absence.
func (c Checker) fetchFailure(r Row, endpoint string, err error) Result {
	code := httpStatus(err)
	if code == 404 && !r.gatedAdmin() {
		return Result{Row: r, State: StateWrong,
			Detail: fmt.Sprintf("%s: HTTP 404 — absent (want %s)", endpoint, r.Required)}
	}
	return Result{Row: r, State: StateUnknown,
		Detail: fmt.Sprintf("%s: %v%s", endpoint, err, adminHint(r))}
}

// absent maps a field that did not resolve to a state, by the same rule.
func (c Checker) absent(r Row, field, endpoint string) Result {
	if r.gatedAdmin() {
		return Result{Row: r, State: StateUnknown,
			Detail: fmt.Sprintf("%s: %s is null or absent — indistinguishable from a set value at this permission level%s", endpoint, field, adminHint(r))}
	}
	return Result{Row: r, State: StateWrong,
		Detail: fmt.Sprintf("%s: %s is absent (want %s)", endpoint, field, r.Required)}
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

// rulesetID resolves a ruleset NAME to its id via the list endpoint. The list
// response deliberately omits rules and bypass_actors, so every ruleset row has
// to take this second hop to the detail endpoint — which is the read that
// exposes `bypass_actors`, or, for a non-admin, conspicuously does not.
func (c Checker) rulesetID(r Row, listEndpoint, name string) (int64, *Result) {
	body, err := c.Get(listEndpoint)
	if err != nil {
		res := c.fetchFailure(r, listEndpoint, err)
		if res.State == StateWrong {
			res.Detail = fmt.Sprintf("%s: HTTP 404 — no rulesets endpoint, so ruleset %q cannot exist", listEndpoint, name)
		}
		return 0, &res
	}
	var list []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	if uerr := json.Unmarshal(body, &list); uerr != nil {
		return 0, &Result{Row: r, State: StateUnknown,
			Detail: fmt.Sprintf("%s: ruleset list is not a JSON array (%v)", listEndpoint, uerr)}
	}
	for _, rs := range list {
		if rs.Name == name {
			return rs.ID, nil
		}
	}
	// The list read succeeded and the ruleset is not in it. That is a real
	// absence at any permission level — ruleset NAMES are visible to anyone who
	// can read the repo, unlike their bypass lists.
	return 0, &Result{Row: r, State: StateWrong,
		Detail: fmt.Sprintf("%s: no ruleset named %q (want %s)", listEndpoint, name, r.Required)}
}

// resolve walks a dotted path. A nil at any step is an absence, not a value:
// `security_and_analysis: null` must never satisfy a row about the field beneath it.
func resolve(doc any, path string) (any, bool) {
	cur := doc
	if path == "" || path == "." {
		if cur == nil {
			return nil, false
		}
		return cur, true
	}
	for _, seg := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[seg]
		if !ok || cur == nil {
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
