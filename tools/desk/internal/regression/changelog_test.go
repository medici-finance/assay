package regression

import "testing"

// regression: #2686
func TestChangelogReconcile(t *testing.T) {
	shellFloor(t, "tools/changelog/reconcile_test.sh")
}
