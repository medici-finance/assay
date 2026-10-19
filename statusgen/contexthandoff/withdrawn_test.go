package contexthandoff

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWithdrawalHandoffIsVersioned(t *testing.T) {
	raw := readFile(t, pairedHandoffPath)
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	v["impacts"].([]any)[0].(map[string]any)["lifecycle"] = "withdrawn"
	v1, _ := json.Marshal(v)
	if _, err := DecodeHandoff(v1); err == nil {
		t.Fatal("v1 must keep its closed lifecycle vocabulary")
	}
	v2 := bytes.Replace(v1, []byte(`"schema":"context-handoff-v1"`), []byte(`"schema":"context-handoff-v2"`), 1)
	schema := loadSchema(t, "../../schemas/context-handoff-v2.json")
	if err := schema.Accepts(v2); err != nil {
		t.Fatalf("v2 schema cannot represent withdrawal: %v", err)
	}
	h, err := DecodeHandoff(v2)
	if err != nil {
		t.Fatalf("v2 must represent withdrawn impacted history: %v", err)
	}
	if h.Impacts[0].Lifecycle != "withdrawn" {
		t.Fatal("withdrawn lifecycle lost")
	}
	again, err := DecodeHandoff(h.Canonical())
	if err != nil || again.Digest() != h.Digest() {
		t.Fatal("v2 canonical roundtrip changed provenance")
	}
	if _, err := DecodeHandoff(bytes.Replace(v2, []byte("context-handoff-v2"), []byte("context-handoff-v3"), 1)); err == nil {
		t.Fatal("future schema silently accepted")
	}
}

func TestWithdrawalV2SchemaKeepsV1Shape(t *testing.T) {
	raw := readFile(t, pairedHandoffPath)
	v2 := bytes.Replace(raw, []byte("context-handoff-v1"), []byte("context-handoff-v2"), 1)
	schema := loadSchema(t, "../../schemas/context-handoff-v2.json")
	for _, v := range mutations(parseTree(t, v2)) {
		data, _ := json.Marshal(v.tree)
		_, err := DecodeHandoff(data)
		schemaErr := schema.Accepts(data)
		if err == nil && schemaErr != nil {
			t.Errorf("%s: decoder accepts, schema refuses: %v", v.desc, schemaErr)
		}
		if err != nil && schemaErr == nil && !packageOnlyRules[RuleOf(err)] {
			t.Errorf("%s: decoder/schema drift: %v", v.desc, err)
		}
	}
}
