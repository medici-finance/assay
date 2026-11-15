package evaluation

import (
	"encoding/json"
	"reflect"
	"testing"
)

// readset_gorun_test.go — a `go-run` entry's JSON is written and read without
// encoding/json (the purity boundary). The test side may use it as the oracle:
// GoRunEntry writes the bytes encoding/json's Marshal gives the struct, and
// ParseGoRun reads them back.

func TestGoRunEntryMatchesJSON(t *testing.T) {
	for _, g := range []GoRun{
		{},
		{Dir: ".", Args: []string{}},
		{Dir: "tools/x", Args: []string{"go", "test", "./..."}},
		{Dir: "a b", Args: []string{"go", "build", "-ldflags=-X a=<b>&c", "q\"uote", `back\slash`}, GOFLAGS: "-tags=x", GOWORK: "off", GOOS: "linux", GOARCH: "arm64", CGOEnabled: "0"},
		{Dir: "ctl\x00\x01\b\f\n\r\t\x1f\x7f", Args: []string{"  ", "é漢😀", "bad\xffutf8", "\xc3"}},
	} {
		want, err := json.Marshal(g)
		if err != nil {
			t.Fatal(err)
		}
		got := GoRunEntry(g)
		if got != EntryGoRun+" "+string(want) {
			t.Errorf("GoRunEntry(%#v):\n got %s\nwant %s %s", g, got, EntryGoRun, want)
		}
		var back GoRun
		if err := json.Unmarshal(want, &back); err != nil {
			t.Fatal(err)
		}
		parsed, ok := ParseGoRun(got)
		if !ok || !reflect.DeepEqual(parsed, back) {
			t.Errorf("ParseGoRun(%s) = %#v, %v; want %#v", got, parsed, ok, back)
		}
	}
}

// An entry the reader cannot fully read is malformed, never a partial GoRun.
func TestParseGoRunRefusesUnknown(t *testing.T) {
	for _, body := range []string{
		`[]`, `"x"`, `null`, `{"dir":"."`, `{"dir":".","extra":"x"}`, `{"Dir":"."}`,
		`{"dir":1}`, `{"args":"go"}`, `{"args":["go",1]}`, `{"dir":".","dir":"x"}`,
		`{"dir":"."} x`,
	} {
		if g, ok := ParseGoRun(EntryGoRun + " " + body); ok {
			t.Errorf("ParseGoRun accepted %s as %#v", body, g)
		}
	}
	if _, ok := ParseGoRun(`go-run{"dir":"."}`); ok {
		t.Error("ParseGoRun accepted an entry without the separating space")
	}
	g, ok := ParseGoRun(EntryGoRun + ` {"dir":null,"args":null,"goos":null}`)
	if !ok || !reflect.DeepEqual(g, GoRun{}) {
		t.Errorf("null fields: %#v, %v", g, ok)
	}
}
