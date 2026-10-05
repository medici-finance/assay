package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type strictInner struct {
	Name string `json:"name"`
	Flag bool   `json:"flag"`
}

func (strictInner) RequiredKeys() []string { return []string{"name", "flag"} }

type strictOuter struct {
	Kind  string                 `json:"kind"`
	At    time.Time              `json:"at"`
	Items []strictInner          `json:"items"`
	ByKey map[string]strictInner `json:"by_key"`
	Raw   json.RawMessage        `json:"raw,omitempty"`
	Ptr   *strictInner           `json:"ptr,omitempty"`
}

const strictOK = `{"kind":"k","at":"2026-10-01T12:00:00Z","items":[{"name":"a","flag":true}],"by_key":{"x":{"name":"b","flag":false}},"raw":{"any":[1,2]},"ptr":{"name":"c","flag":true}}`

func TestDecodeStrictAcceptsTheContract(t *testing.T) {
	var v strictOuter
	if err := DecodeStrict([]byte(strictOK), &v); err != nil {
		t.Fatalf("a contract-exact document was refused: %v", err)
	}
	if v.Kind != "k" || len(v.Items) != 1 || !v.Items[0].Flag || v.ByKey["x"].Name != "b" || v.Ptr.Name != "c" {
		t.Fatalf("decoded %+v", v)
	}
}

func TestDecodeStrictRefusesNonContractKeys(t *testing.T) {
	cases := map[string]string{
		"top-level case variant":   strings.Replace(strictOK, `"kind":"k"`, `"Kind":"k"`, 1),
		"top-level duplicate":      strings.Replace(strictOK, `"kind":"k"`, `"kind":"k","kind":"j"`, 1),
		"slice element case":       strings.Replace(strictOK, `{"name":"a","flag":true}`, `{"name":"a","flag":false,"FLAG":true}`, 1),
		"slice element duplicate":  strings.Replace(strictOK, `{"name":"a","flag":true}`, `{"name":"a","flag":false,"flag":true}`, 1),
		"map value case":           strings.Replace(strictOK, `{"name":"b","flag":false}`, `{"NAME":"b","flag":false}`, 1),
		"map key duplicate":        strings.Replace(strictOK, `"x":{"name":"b","flag":false}`, `"x":{"name":"b","flag":false},"x":{"name":"d","flag":true}`, 1),
		"pointer target case":      strings.Replace(strictOK, `"ptr":{"name":"c"`, `"ptr":{"Name":"c"`, 1),
		"opaque payload duplicate": strings.Replace(strictOK, `{"any":[1,2]}`, `{"any":[1],"any":[2]}`, 1),
		"unknown key":              strings.Replace(strictOK, `"kind":"k"`, `"kind":"k","extra":1`, 1),
		"required key missing":     strings.Replace(strictOK, `{"name":"a","flag":true}`, `{"name":"a"}`, 1),
		"required key null":        strings.Replace(strictOK, `{"name":"a","flag":true}`, `{"name":"a","flag":null}`, 1),
		"trailing value":           strictOK + `{}`,
		"trailing garbage":         strictOK + ` x`,
	}
	for name, doc := range cases {
		if doc == strictOK {
			t.Fatalf("%s: the fixture edit did not apply", name)
		}
		var v strictOuter
		if err := DecodeStrict([]byte(doc), &v); err == nil {
			t.Errorf("%s: DecodeStrict accepted %s", name, doc)
		}
	}
	var v strictOuter
	if err := DecodeStrict([]byte(strictOK), v); err == nil {
		t.Error("a non-pointer target was accepted")
	}
}
