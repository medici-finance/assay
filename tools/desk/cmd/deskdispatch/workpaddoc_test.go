package main

import (
	"strings"
	"testing"
)

func TestKitWorkpadShape(t *testing.T) {
	common, err := commonKitText()
	if err != nil {
		t.Fatal(err)
	}
	objective, err := kitText("worker-objective")
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"emitted common": common, "objective mirror": objective} {
		for _, required := range []string{"\n<!-- assay:workpad -->\n", "cat > \"$WORKPAD\" <<'WORKPAD_BODY'", "## Acceptance criteria", "requires a source checkout", "./examples/workpad-render", "--dry-run"} {
			if !strings.Contains(text, required) {
				t.Errorf("%s omits %q", name, required)
			}
		}
	}
}
