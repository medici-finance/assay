package riskpath

import (
	"reflect"
	"testing"
)

func TestPatternAndAdditiveUnion(t *testing.T) {
	for _, tc := range []struct {
		pattern, path string
		want          bool
	}{{"k8s/*/rbac.yaml", "k8s/a/rbac.yaml", true}, {"k8s/*/rbac.yaml", "k8s/a/b/rbac.yaml", false}, {"secrets/", "secrets/key", true}, {"secrets/", "secrets2/key", false}, {"exact", "exact/child", false}} {
		if got := Match(tc.pattern, tc.path); got != tc.want {
			t.Fatal(tc, got)
		}
	}
	if got := Union([]string{" a/ ", "b/"}, []string{"a/", "c/"}); !reflect.DeepEqual(got, []string{"a/", "b/", "c/"}) {
		t.Fatal(got)
	}
	b := Base()
	b[0] = "changed"
	if Base()[0] == "changed" {
		t.Fatal("caller mutated shared base policy")
	}
}
