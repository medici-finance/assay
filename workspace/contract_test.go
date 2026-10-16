package workspace

// contract_test.go — the contract fixtures of testdata/contract/, read through the production
// decoders. Every expected identity (payload digests, repository keys) in expected.json was
// computed outside this package: the digest is the sha256 of the compact fixture's bytes, and
// the repository key is sha256(forge NUL repository) taken by a shell, so a decoder or digest
// regression cannot also rewrite its own expectation.

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

const fixtureDir = "testdata/contract"

type expectations struct {
	Digests  map[string]string `json:"digests"`
	RepoKeys map[string]string `json:"repo_keys"`
	Negative map[string]struct {
		Doc    string `json:"doc"`
		Reason Reason `json:"reason"`
	} `json:"negative"`
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixtureDir, name))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return b
}

func loadExpected(t *testing.T) expectations {
	t.Helper()
	var e expectations
	if err := json.Unmarshal(readFixture(t, "expected.json"), &e); err != nil {
		t.Fatal(err)
	}
	if len(e.Digests) == 0 || len(e.RepoKeys) == 0 || len(e.Negative) == 0 {
		t.Fatal("expected.json carries no expectations: nothing would be checked")
	}
	return e
}

func mustRequest(t *testing.T, name string) Request {
	t.Helper()
	q, err := DecodeRequest(readFixture(t, name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return q
}

func wantReason(t *testing.T, err error, want Reason) {
	t.Helper()
	got, ok := ReasonOf(err)
	if !ok || got != want {
		t.Fatalf("want %s refusal, got %v", want, err)
	}
}

// regression: F-cell-workspace-contract
func TestWorkspaceContractRoundTrip(t *testing.T) {
	exp := loadExpected(t)

	t.Run("identities", func(t *testing.T) {
		for name, digest := range exp.Digests {
			q := mustRequest(t, name)
			if got := q.Digest(); got != digest {
				t.Errorf("%s: digest %s, want %s", name, got, digest)
			}
			if !strings.Contains(name, "-pretty") {
				enc, _ := canonical(q)
				if raw := readFixture(t, name); !bytes.Equal(enc, raw) {
					t.Errorf("%s: canonical re-encoding differs from the fixture:\n got %s\nwant %s", name, enc, raw)
				}
			}
		}
		for name, key := range exp.RepoKeys {
			if got := RepoKey(mustRequest(t, name).Repo); got != key {
				t.Errorf("%s: repo key %s, want %s", name, got, key)
			}
		}
	})

	t.Run("record-replay", func(t *testing.T) {
		raw := readFixture(t, "record-worker.json")
		rec, err := DecodeRecord(raw)
		if err != nil {
			t.Fatal(err)
		}
		if enc, _ := canonical(rec); !bytes.Equal(enc, raw) {
			t.Fatalf("record re-encoding differs:\n got %s\nwant %s", enc, raw)
		}
		if rec.PayloadDigest != exp.Digests["request-worker.json"] {
			t.Fatalf("record digest %s is not the request's", rec.PayloadDigest)
		}
		for _, name := range []string{"request-worker.json", "request-worker-pretty.json"} {
			q := mustRequest(t, name)
			if rec.Key() != q.Key() {
				t.Fatalf("%s: key %+v, record key %+v", name, q.Key(), rec.Key())
			}
			if err := Replay(rec, q); err != nil {
				t.Fatalf("%s: same payload replay refused: %v", name, err)
			}
		}
		changed := mustRequest(t, "replay-changed.json")
		if changed.Key() != rec.Key() {
			t.Fatal("replay-changed.json must reuse the record's key")
		}
		wantReason(t, Replay(rec, changed), ReasonPayloadMismatch)
		other := mustRequest(t, "request-desk.json")
		wantReason(t, Replay(rec, other), ReasonInvalid)
	})

	t.Run("negative", func(t *testing.T) {
		for name, want := range exp.Negative {
			raw := readFixture(t, filepath.Join("negative", name))
			var err error
			switch want.Doc {
			case "request":
				_, err = DecodeRequest(raw)
			case "record":
				_, err = DecodeRecord(raw)
			case "result":
				_, err = DecodeResult(raw)
			default:
				t.Fatalf("%s: unknown doc %q", name, want.Doc)
			}
			got, ok := ReasonOf(err)
			if !ok || got != want.Reason {
				t.Errorf("%s: want %s, got %v", name, want.Reason, err)
				continue
			}
			if res := ResultOf("req-0001", err); res.Outcome.Kind != OutcomeRefused || res.Record != nil || res.Validate() != nil {
				t.Errorf("%s: refusal maps to %+v", name, res)
			}
		}
		entries, _ := os.ReadDir(filepath.Join(fixtureDir, "negative"))
		if len(entries) != len(exp.Negative) {
			t.Errorf("%d negative fixtures, %d expectations: every fixture needs its reason", len(entries), len(exp.Negative))
		}
	})

	t.Run("results", func(t *testing.T) {
		ok, err := DecodeResult(readFixture(t, "result-ok.json"))
		if err != nil {
			t.Fatal(err)
		}
		rec, _ := DecodeRecord(readFixture(t, "record-worker.json"))
		if ok.Record == nil || !reflect.DeepEqual(*ok.Record, rec) {
			t.Fatal("ok result does not carry the record")
		}
		for name, kind := range map[string]OutcomeKind{"result-refused.json": OutcomeRefused, "result-unknown.json": OutcomeUnknown} {
			r, err := DecodeResult(readFixture(t, name))
			if err != nil || r.Outcome.Kind != kind || r.Record != nil {
				t.Fatalf("%s: %+v %v", name, r, err)
			}
		}
		bad := map[string]Result{
			"failure-with-record": {Schema: Schema, RequestID: "r", Outcome: Outcome{Kind: OutcomeRefused, Reason: ReasonInvalid}, Record: &rec},
			"ok-with-reason":      {Schema: Schema, RequestID: "r", Outcome: Outcome{Kind: OutcomeOK, Reason: ReasonBusy}, Record: &rec},
			"ok-without-record":   {Schema: Schema, RequestID: "r", Outcome: Outcome{Kind: OutcomeOK}},
			"reason-wrong-kind":   {Schema: Schema, RequestID: "r", Outcome: Outcome{Kind: OutcomeRefused, Reason: ReasonStaleGeneration}},
			"unknown-reason":      {Schema: Schema, RequestID: "r", Outcome: Outcome{Kind: OutcomeUnknown, Reason: "maybe"}},
		}
		for name, r := range bad {
			wantReason(t, r.Validate(), ReasonInvalid)
			_ = name
		}
		if r := ResultOf("r", errors.New("disk vanished")); r.Outcome.Kind != OutcomeUnknown || r.Validate() != nil {
			t.Fatalf("an untyped error must be unknown, got %+v", r)
		}
	})

	t.Run("record-rules", func(t *testing.T) {
		base, _ := DecodeRecord(readFixture(t, "record-worker.json"))
		cases := map[string]struct {
			edit func(*Record)
			want Reason
		}{
			"active-without-lease": {func(r *Record) { r.Lease = nil }, ReasonInvalid},
			"legacy-ready":         {func(r *Record) { r.Provenance, r.State, r.Lease = ProvenanceLegacy, StateReady, nil }, ReasonInvalid},
			"retained-no-reason":   {func(r *Record) { r.State, r.Lease = StateRetained, nil }, ReasonInvalid},
			"ready-without-git":    {func(r *Record) { r.State, r.Git, r.Lease = StateReady, nil, nil }, ReasonInvalid},
			"repo-key-forged":      {func(r *Record) { r.RepoKey = "widgets-0000000000000000" }, ReasonIdentityMismatch},
			"lease-not-process":    {func(r *Record) { r.Lease = &Ref{Owner: OwnerScratch, ID: "x"} }, ReasonInvalid},
			"time-reversed":        {func(r *Record) { r.TransitionedAt = "2026-09-30T00:00:00Z" }, ReasonInvalid},
			"generation-zero":      {func(r *Record) { r.Generation = 0 }, ReasonInvalid},
			"unknown-schema":       {func(r *Record) { r.Schema = "cell-workspace/v2" }, ReasonUnknownSchema},
		}
		for name, c := range cases {
			r := base
			c.edit(&r)
			if got, ok := ReasonOf(r.Validate()); !ok || got != c.want {
				t.Errorf("%s: want %s, got %v", name, c.want, r.Validate())
			}
		}
		alloc := base
		alloc.State, alloc.Git, alloc.Lease, alloc.Generation = StateAllocating, nil, nil, 1
		if err := alloc.Validate(); err != nil {
			t.Fatalf("an allocating record has no Git identity yet: %v", err)
		}
	})

	t.Run("lifecycle", func(t *testing.T) {
		allowed := map[string]bool{
			"allocating>ready": true, "allocating>retained": true, "allocating>retiring": true,
			"ready>active": true, "ready>retained": true, "ready>retiring": true,
			"active>retained": true,
			"retained>active": true, "retained>retiring": true,
			"retiring>retired": true, "retiring>retained": true,
		}
		at := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
		for _, from := range States() {
			for _, to := range States() {
				r := Record{State: from, Generation: 7}
				next, err := Advance(r, 7, to, at)
				if allowed[string(from)+">"+string(to)] {
					if err != nil || next.State != to || next.Generation != 8 || next.TransitionedAt != "2026-10-02T00:00:00Z" {
						t.Errorf("%s->%s: %+v %v", from, to, next, err)
					}
				} else {
					wantReason(t, err, ReasonIllegalTransition)
				}
			}
		}
		_, err := Advance(Record{State: StateReady, Generation: 7}, 6, StateActive, at)
		wantReason(t, err, ReasonStaleGeneration)
		if k, _ := KindOf(ReasonStaleGeneration); k != OutcomeConflict {
			t.Fatal("a stale generation is a conflict, not a refusal")
		}
	})

	t.Run("capabilities", func(t *testing.T) {
		q := mustRequest(t, "request-worker.json")
		wantReason(t, Admit(q, nil), ReasonUnsupportedCapability)
		if err := Admit(q, []string{"allocate/v1"}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("repo-key-collisions", func(t *testing.T) {
		keys := map[string]RepoRef{}
		for _, r := range []RepoRef{
			{"github.com", "example-org/widgets"}, {"github.com", "other-org/widgets"},
			{"git.example.net", "example-org/widgets"}, {"github.com", "example-org/Widgets"},
		} {
			k := RepoKey(r)
			if prev, dup := keys[k]; dup {
				t.Fatalf("%+v and %+v share key %s", prev, r, k)
			}
			keys[k] = r
		}
	})

	// The canonical encoding escapes only what JSON requires (spec section 4.2). The escape
	// fixtures pin <, >, &, quote, backslash, non-ASCII and U+2028 through expected.json; this
	// pins the one case a fixture cannot show by its digest alone: an identifier whose visible
	// ASCII text spells a backslash-u-2028 escape stays that text, escaped backslash and all.
	t.Run("canonical-escaping", func(t *testing.T) {
		q := mustRequest(t, "request-escape.json")
		lit := `x\` + "u2028y"
		q.Task.ID = lit
		b, err := canonical(q)
		if err != nil {
			t.Fatal(err)
		}
		if want := `"id":"x\\` + `u2028y"`; !bytes.Contains(b, []byte(want)) {
			t.Fatalf("escaped backslash not kept: %s", b)
		}
		back, err := DecodeRequest(b)
		if err != nil || back.Task.ID != lit || back.Digest() != q.Digest() {
			t.Fatalf("canonical bytes do not round-trip: %v %q", err, back.Task.ID)
		}
		for _, r := range []rune{0x2028, 0x2029} {
			q.Cell.Root = "/srv/" + string(r)
			b, _ := canonical(q)
			if !bytes.Contains(b, []byte(q.Cell.Root)) || bytes.HasSuffix(b, []byte("\n")) {
				t.Fatalf("U+%04X not written as UTF-8, or a trailing newline: %q", r, b)
			}
		}
	})

	// Identifiers are 1-256 visible ASCII characters, at every identifier member (spec section 3).
	t.Run("identifier-charset", func(t *testing.T) {
		accept := []string{"!", "~", `<&>"\'` + "`$;|/..", strings.Repeat("a", 256)}
		refuse := []string{"", " ", "a b", "a\tb", "a\n", "\x7f", "a\x00", "caf" + string(rune(0xe9)),
			string(rune(0xa0)), string(rune(0x202e)), string(rune(0x200b)), strings.Repeat("a", 257)}
		set := map[string]func(*Request, string){
			"request_id":      func(q *Request, v string) { q.RequestID = v },
			"attempt_id":      func(q *Request, v string) { q.AttemptID = v },
			"task.id":         func(q *Request, v string) { q.Task.ID = v },
			"session.session": func(q *Request, v string) { q.Session.Session = v },
		}
		for field, edit := range set {
			for _, v := range accept {
				q := mustRequest(t, "request-worker.json")
				edit(&q, v)
				if err := q.Validate(); err != nil {
					t.Errorf("%s %q refused: %v", field, v, err)
				}
			}
			for _, v := range refuse {
				q := mustRequest(t, "request-worker.json")
				edit(&q, v)
				var r *Refusal
				if err := q.Validate(); !errors.As(err, &r) || r.Reason != ReasonInvalid || r.Field != field {
					t.Errorf("%s %q: want invalid at %s, got %v", field, v, field, err)
				}
			}
		}
	})

	// Paths are absolute local paths: a POSIX or drive root, no network (UNC) root, no `..`
	// segment between either separator, no control character, valid UTF-8 (spec section 3).
	t.Run("path-rules", func(t *testing.T) {
		bs := `\`
		accept := []string{"/", "/srv/cells/alpha", "/a/./b", "/a//b", "/...", "/a/..b", "/a/b..",
			"C:/x", "C:" + bs + "x" + bs + "y", "/caf" + string(rune(0xe9)) + string(rune(0x2028)),
			"/a b", "/" + strings.Repeat("a", 4095)}
		refuse := []string{"", "rel", "C:x", "//srv", "//h/share", bs + bs + "srv" + bs + "share",
			bs + bs + "?" + bs + "C:" + bs + "x", "//?/C:/x", "/" + bs + "x", "C:" + bs + bs + "x",
			"/..", "/a/..", "/a/../b", "/a" + bs + ".." + bs + "b", "C:" + bs + "..", "C:/x/../y",
			"/a\tb", "/a\nb", "/a\rb", "/a\x00b", "/a\x01b", "/a\x1fb", "/a\x7fb", "/a\xffb",
			"/" + strings.Repeat("a", 4096)}
		for _, v := range accept {
			if err := checkAbs("cell.root", v); err != nil {
				t.Errorf("%q refused: %v", v, err)
			}
		}
		for _, v := range refuse {
			q := mustRequest(t, "request-worker.json")
			q.Cell.Root = v
			var r *Refusal
			if err := q.Validate(); !errors.As(err, &r) || r.Reason != ReasonInvalid || r.Field != "cell.root" {
				t.Errorf("cell.root %q: want invalid, got %v", v, err)
			}
		}
		rec, _ := DecodeRecord(readFixture(t, "record-worker.json"))
		for field, edit := range map[string]func(*Record){
			"path.logical":   func(r *Record) { r.Path.Logical += "/../x" },
			"path.physical":  func(r *Record) { r.Path.Physical = bs + bs + "host" + bs + "share" },
			"git.common_dir": func(r *Record) { r.Git.CommonDir = "//h/share/.git" },
			"git.admin_dir":  func(r *Record) { r.Git.AdminDir += "\x1b" },
		} {
			r := rec
			g := *rec.Git
			r.Git = &g
			edit(&r)
			var ref *Refusal
			if err := r.Validate(); !errors.As(err, &ref) || ref.Reason != ReasonInvalid || ref.Field != field {
				t.Errorf("%s: want invalid, got %v", field, err)
			}
		}
	})
}

// schemaDefs maps each contract type to its $defs entry in schemas/cell-workspace-v1.json.
var schemaDefs = map[string]any{
	"Request": Request{}, "Record": Record{}, "Result": Result{}, "CellRef": CellRef{},
	"RepoRef": RepoRef{}, "SessionRef": SessionRef{}, "Ref": Ref{}, "Subject": Subject{},
	"Path": Path{}, "GitIdentity": GitIdentity{}, "Outcome": Outcome{},
}

// schemaPatterns maps every identifier and path member of the schema to the Go rule that checks
// it, so the published pattern and the implementation cannot drift apart.
var schemaPatterns = map[string]*regexp.Regexp{
	"SessionRef.session": idRe, "Ref.id": idRe, "Request.request_id": idRe, "Request.attempt_id": idRe,
	"Record.request_id": idRe, "Record.attempt_id": idRe, "Result.request_id": idRe,
	"CellRef.root": absPathRe, "Path.logical": absPathRe, "Path.physical": absPathRe,
	"GitIdentity.common_dir": absPathRe, "GitIdentity.admin_dir": absPathRe,
}

func schemaEnums() map[string][]string {
	str := func(v any) []string {
		rv := reflect.ValueOf(v)
		out := make([]string, rv.Len())
		for i := range out {
			out[i] = rv.Index(i).String()
		}
		sort.Strings(out)
		return out
	}
	return map[string][]string{
		"Operation": str(Operations()), "Kind": str(Kinds()), "State": str(States()), "Mode": str(Modes()),
		"Provenance": str(Provenances()), "RefOwner": str(RefOwners()), "OutcomeKind": str(OutcomeKinds()),
		"Reason": str(Reasons()),
	}
}

// schemaProblems compares the published schema with the Go types: the same properties, the same
// required set (every non-omitempty field), no additional properties, and the same closed sets.
func schemaProblems(doc map[string]any) []string {
	var out []string
	defs, _ := doc["$defs"].(map[string]any)
	for name, v := range schemaDefs {
		def, ok := defs[name].(map[string]any)
		if !ok {
			out = append(out, "$defs."+name+" missing")
			continue
		}
		if def["additionalProperties"] != false {
			out = append(out, "$defs."+name+" admits additional properties")
		}
		props, _ := def["properties"].(map[string]any)
		var req []string
		for _, r := range def["required"].([]any) {
			req = append(req, r.(string))
		}
		var wantProps, wantReq []string
		rt := reflect.TypeOf(v)
		for i := 0; i < rt.NumField(); i++ {
			tag := strings.Split(rt.Field(i).Tag.Get("json"), ",")
			wantProps = append(wantProps, tag[0])
			if len(tag) == 1 {
				wantReq = append(wantReq, tag[0])
			}
		}
		var gotProps []string
		for p := range props {
			gotProps = append(gotProps, p)
		}
		sort.Strings(gotProps)
		sort.Strings(wantProps)
		sort.Strings(req)
		sort.Strings(wantReq)
		if !reflect.DeepEqual(gotProps, wantProps) {
			out = append(out, "$defs."+name+" properties "+strings.Join(gotProps, ",")+" != fields "+strings.Join(wantProps, ","))
		}
		if !reflect.DeepEqual(req, wantReq) {
			out = append(out, "$defs."+name+" required "+strings.Join(req, ",")+" != "+strings.Join(wantReq, ","))
		}
	}
	for member, re := range schemaPatterns {
		def, prop, _ := strings.Cut(member, ".")
		d, _ := defs[def].(map[string]any)
		props, _ := d["properties"].(map[string]any)
		p, _ := props[prop].(map[string]any)
		if got, _ := p["pattern"].(string); got != re.String() {
			out = append(out, "$defs."+member+" pattern "+got+" != Go "+re.String())
		}
	}
	for name, want := range schemaEnums() {
		def, _ := defs[name].(map[string]any)
		var got []string
		if enum, ok := def["enum"].([]any); ok {
			for _, e := range enum {
				got = append(got, e.(string))
			}
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			out = append(out, "$defs."+name+" enum "+strings.Join(got, ",")+" != "+strings.Join(want, ","))
		}
	}
	return out
}

// regression: F-cell-workspace-contract
func TestWorkspaceSchemaCoverage(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "schemas", "cell-workspace-v1.json"))
	if err != nil {
		t.Skipf("could-not-check: published schema unreadable: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, p := range schemaProblems(doc) {
		t.Error(p)
	}
	for _, name := range []string{"Request", "Record", "Result"} {
		props := doc["$defs"].(map[string]any)[name].(map[string]any)["properties"].(map[string]any)
		if c, _ := props["schema"].(map[string]any)["const"].(string); c != Schema {
			t.Errorf("$defs.%s.schema const %q, want %q", name, c, Schema)
		}
	}
	// The comparison must be able to fail: a schema missing a field, a widened enum, or a widened
	// identifier or path pattern, is caught.
	var doctored map[string]any
	_ = json.Unmarshal(raw, &doctored)
	delete(doctored["$defs"].(map[string]any)["Record"].(map[string]any)["properties"].(map[string]any), "generation")
	k := doctored["$defs"].(map[string]any)["Kind"].(map[string]any)
	k["enum"] = append(k["enum"].([]any), "sandbox")
	ref := doctored["$defs"].(map[string]any)["Ref"].(map[string]any)["properties"].(map[string]any)
	ref["id"].(map[string]any)["pattern"] = "^[ -~]{1,256}$"
	root := doctored["$defs"].(map[string]any)["CellRef"].(map[string]any)["properties"].(map[string]any)
	root["root"].(map[string]any)["pattern"] = `^(/|[A-Za-z]:[\\/]|\\\\)`
	if n := len(schemaProblems(doctored)); n != 4 {
		t.Errorf("doctored schema: want 4 problems, got %d", n)
	}
}

var goDirective = regexp.MustCompile(`(?m)^go ([0-9]+)\.([0-9]+)(?:\.([0-9]+))?\s*$`)

func goVersion(t *testing.T, path string) [3]int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("could-not-check: %v", err)
	}
	m := goDirective.FindSubmatch(b)
	if m == nil {
		t.Fatalf("%s: no go directive", path)
	}
	var v [3]int
	for i := 0; i < 3; i++ {
		for _, c := range m[i+1] {
			v[i] = v[i]*10 + int(c-'0')
		}
	}
	return v
}

// TestWorkspaceModuleBoundary holds the module to what both Go consumers can require: the
// standard library only (so no consumer internals and no credential code), and a go directive no
// newer than either consumer's.
func TestWorkspaceModuleBoundary(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", ".")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("could-not-check: go list: %v", err)
	}
	if deps := strings.Fields(string(out)); len(deps) != 1 || deps[0] != "github.com/medici-finance/assay/workspace" {
		t.Fatalf("non-standard dependencies: %v", deps)
	}
	own := goVersion(t, "go.mod")
	for _, consumer := range []string{"../tools/desk/go.mod", "../statusgen/go.mod"} {
		c := goVersion(t, consumer)
		for i := 0; i < 3; i++ {
			if own[i] != c[i] {
				if own[i] > c[i] {
					t.Errorf("go directive %v is newer than %s's %v", own, consumer, c)
				}
				break
			}
		}
	}
}
