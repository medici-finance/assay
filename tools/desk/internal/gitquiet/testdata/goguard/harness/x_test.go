package harness

import "testing"

func TestBuild(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_ = Build(t, ".")
}
