package perfdata

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestSchemaFiles keeps schemas/performance-event-v1.json and
// schemas/performance-batch-v1.json in step with the Go contract: every object's
// property set and required set equal the struct's JSON keys (omitempty keys are
// optional), and the kind enum and its workflow-only kinds equal the Go table. The Go
// validator stays canonical; this catches a schema that silently drifts from it.
func TestSchemaFiles(t *testing.T) {
	event := loadSchema(t, "../schemas/performance-event-v1.json")
	batch := loadSchema(t, "../schemas/performance-batch-v1.json")
	defs := event["$defs"].(map[string]any)
	bdefs := batch["$defs"].(map[string]any)

	for _, c := range []struct {
		name   string
		schema any
		typ    reflect.Type
	}{
		{"event", event, reflect.TypeOf(Event{})},
		{"event.instrument", event["properties"].(map[string]any)["instrument"], reflect.TypeOf(Instrument{})},
		{"event.build", event["properties"].(map[string]any)["build"], reflect.TypeOf(Build{})},
		{"context", defs["context"], reflect.TypeOf(Context{})},
		{"measurement", defs["measurement"], reflect.TypeOf(Measurement{})},
		{"distribution", defs["distribution"], reflect.TypeOf(Distribution{})},
		{"toolCompletion", defs["toolCompletion"], reflect.TypeOf(ToolCompletion{})},
		{"toolCompletion.cpu", prop(defs["toolCompletion"], "cpu"), reflect.TypeOf(CPU{})},
		{"toolCompletion.children", prop(defs["toolCompletion"], "children"), reflect.TypeOf(Children{})},
		{"phase", defs["phase"], reflect.TypeOf(Phase{})},
		{"attempt", defs["attempt"], reflect.TypeOf(Attempt{})},
		{"usage", defs["usage"], reflect.TypeOf(Usage{})},
		{"usage.cost", prop(defs["usage"], "cost"), reflect.TypeOf(Cost{})},
		{"points", defs["points"], reflect.TypeOf(Points{})},
		{"httpAttempts", defs["httpAttempts"], reflect.TypeOf(HTTPAttempts{})},
		{"serviceCall", defs["serviceCall"], reflect.TypeOf(ServiceCall{})},
		{"serviceCall.outcomes", prop(defs["serviceCall"], "outcomes"), reflect.TypeOf(CallOutcomes{})},
		{"handler", defs["handler"], reflect.TypeOf(Handler{})},
		{"handler.cache", prop(defs["handler"], "cache"), reflect.TypeOf(CacheOutcomes{})},
		{"handler.coalescing", prop(defs["handler"], "coalescing"), reflect.TypeOf(Coalescing{})},
		{"health", defs["health"], reflect.TypeOf(Health{})},
		{"correction", defs["correction"], reflect.TypeOf(Correction{})},
		{"batch", batch, reflect.TypeOf(Batch{})},
		{"receipt", bdefs["receipt"], reflect.TypeOf(Receipt{})},
		{"result", bdefs["result"], reflect.TypeOf(Result{})},
	} {
		if diff := keyDrift(c.schema, c.typ); diff != "" {
			t.Errorf("%s: schema and Go struct differ: %s", c.name, diff)
		}
	}

	// The kind enum and the workflow-only kinds equal the Go table.
	var goKinds, goWorkflowOnly []string
	for k, ps := range kindProfiles {
		goKinds = append(goKinds, string(k))
		if len(ps) == 1 && ps[0] == ProfileWorkflow {
			goWorkflowOnly = append(goWorkflowOnly, string(k))
		}
	}
	var schemaKinds, schemaWorkflowOnly []string
	for _, v := range prop(event, "kind").(map[string]any)["enum"].([]any) {
		schemaKinds = append(schemaKinds, v.(string))
	}
	for _, c := range event["allOf"].([]any) {
		cond := c.(map[string]any)
		k, ok := prop(cond["if"], "kind").(map[string]any)
		if !ok {
			continue
		}
		if p, ok := prop(cond["then"], "profile").(map[string]any); ok && p["const"] == "workflow" {
			schemaWorkflowOnly = append(schemaWorkflowOnly, k["const"].(string))
		}
	}
	for _, pair := range [][2][]string{{goKinds, schemaKinds}, {goWorkflowOnly, schemaWorkflowOnly}} {
		sort.Strings(pair[0])
		sort.Strings(pair[1])
		if fmt.Sprint(pair[0]) != fmt.Sprint(pair[1]) {
			t.Errorf("kind table drift: Go %v, schema %v", pair[0], pair[1])
		}
	}

	// Planted control: a struct with an extra key and a missing key is reported.
	type drifted struct {
		Schema string `json:"schema"`
		Extra  string `json:"extra"`
	}
	if keyDrift(batch, reflect.TypeOf(drifted{})) == "" {
		t.Fatal("control: key drift was not detected")
	}
}

func loadSchema(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return m
}

func prop(schema any, name string) any {
	m, _ := schema.(map[string]any)
	ps, _ := m["properties"].(map[string]any)
	return ps[name]
}

// keyDrift compares a schema object's properties/required with a struct's JSON keys.
func keyDrift(schema any, typ reflect.Type) string {
	m, ok := schema.(map[string]any)
	if !ok {
		return "schema object missing"
	}
	var props, req []string
	for k := range m["properties"].(map[string]any) {
		props = append(props, k)
	}
	if r, ok := m["required"].([]any); ok {
		for _, k := range r {
			req = append(req, k.(string))
		}
	}
	var keys, mandatory []string
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		name, opts, _ := strings.Cut(tag, ",")
		keys = append(keys, name)
		if !strings.Contains(opts, "omitempty") {
			mandatory = append(mandatory, name)
		}
	}
	for _, l := range [][]string{props, req, keys, mandatory} {
		sort.Strings(l)
	}
	var out []string
	if fmt.Sprint(props) != fmt.Sprint(keys) {
		out = append(out, fmt.Sprintf("properties %v vs keys %v", props, keys))
	}
	if fmt.Sprint(req) != fmt.Sprint(mandatory) {
		out = append(out, fmt.Sprintf("required %v vs non-omitempty %v", req, mandatory))
	}
	return strings.Join(out, "; ")
}
