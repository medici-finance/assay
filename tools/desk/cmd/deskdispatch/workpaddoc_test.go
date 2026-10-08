package main

import (
	"regexp"
	"strings"
	"testing"
)

// The common kit and its objective mirror carry the workpad SHAPE (marker plus the four
// sections in order) and point at the one copyable template, `deskreply --help` WORKPAD
// BODY. They do not carry a second copy of that template.
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
		for _, required := range []string{"`<!-- assay:workpad -->`", "`## Plan`", "`## Acceptance criteria`", "`## Validation`", "`## Notes`", "`deskreply --help`", "WORKPAD BODY", "--dry-run"} {
			if !strings.Contains(text, required) {
				t.Errorf("%s omits %q", name, required)
			}
		}
		if strings.Contains(text, "<<'WORKPAD_BODY'") {
			t.Errorf("%s carries a second copy of the help template", name)
		}
	}
}

// loopValue matches a concrete loop-identity assignment.
var loopValue = regexp.MustCompile(`DESK_LOOP=[A-Za-z]`)

// The common kit reaches every dispatch class, so it must never name a concrete loop
// identity: each class's own kit or assignment sets that (#1029 class, #2122 SEC-1).
func TestCommonKitNamesNoLoop(t *testing.T) {
	common, err := commonKitText()
	if err != nil {
		t.Fatal(err)
	}
	if m := loopValue.FindString(common); m != "" {
		t.Errorf("the all-class common kit names a loop identity (%q)", m)
	}
	if loopValue.FindString("x DESK_LOOP=worker-desk y") == "" || loopValue.FindString("$DESK_LOOP unset") != "" {
		t.Fatal("loopValue control: matcher no longer separates a value from a mention")
	}
}
