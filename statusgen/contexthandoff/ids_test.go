package contexthandoff

import "testing"

// regression: #4219
func TestLegacyQIDParity(t *testing.T) {
	ctx := &QID{Cell: "example", Repo: "tools", Stream: "declaring", NN: "01"}
	for ref, want := range map[string]string{
		"build:00": "example:tools:build:00", "build:11a": "example:tools:build:11a",
		"other:build:1234z": "example:other:build:1234z", "cell:repo:build:11": "cell:repo:build:11",
		"m-stream:11": "example:tools:m-stream:11", "b-stream:12": "example:tools:b-stream:12",
	} {
		got, err := ResolveQualifiedID(ref, ctx)
		if err != nil || got.String() != want {
			t.Errorf("%s = %s, %v", ref, got, err)
		}
	}
	for _, ref := range []string{"build/11", "build:b-11", "build:m-03@1", "build:11@1", "example:tools:build/b-11", "build:1", "build:12345", "Build:11", "build:11A", "#11", "a:b:c:build:11", " build:11", "build:11\t", "build:11\u00a0", ":build:11"} {
		if got, err := ResolveQualifiedID(ref, ctx); err == nil {
			t.Errorf("legacy strict profile broadened: %q = %s", ref, got)
		}
	}
	if _, err := ResolveQualifiedID("build:11", nil); err == nil {
		t.Error("elision without declaration accepted")
	}
	if got, err := ResolveQualifiedID("example:unregistered:build:11", nil); err != nil || got.Repo != "unregistered" {
		t.Errorf("legacy API gained registry validation: %s, %v", got, err)
	}
	// Historical expansion copies supplied context, rather than validating unrelated fields.
	if got, err := ResolveQualifiedID("build:11", &QID{Cell: "Legacy", Repo: "legacy_repo"}); err != nil || got.Cell != "Legacy" || got.Repo != "legacy_repo" {
		t.Errorf("declaring context behavior changed: %s, %v", got, err)
	}
}
