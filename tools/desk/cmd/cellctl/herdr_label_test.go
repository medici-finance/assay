//go:build darwin || linux

package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cellop"
)

// regression: #6698
func TestSurfaceLabelPerCockpit(t *testing.T) {
	for _, c := range []struct{ cockpit, want string }{
		{"herdr", "worker-desk"},
		{"tmux", "example-worker-desk"},
		{"orca", "example-worker-desk"},
	} {
		if got := surfaceLabel(c.cockpit, "example", "worker-desk"); got != c.want {
			t.Errorf("%s label %q, want %q", c.cockpit, got, c.want)
		}
		cell := &Cell{Name: "example"}
		cell.noteSurface("worker-desk", surfaceReceipt{Cockpit: c.cockpit, Surface: "s1"})
		if got := cell.pendingSurfaces["worker-desk"].Name; got != c.want {
			t.Errorf("%s receipt name %q, want %q", c.cockpit, got, c.want)
		}
	}
}

// regression: #6698
func TestReceiptNameOldHerdrLabel(t *testing.T) {
	for _, c := range []struct {
		cockpit, name string
		ok            bool
	}{
		{"herdr", "worker-desk", true},
		{"herdr", "example-worker-desk", true}, // written before the bare label
		{"herdr", "other-worker-desk", false},
		{"herdr", "pr-review-desk", false},
		{"tmux", "example-worker-desk", true},
		{"tmux", "worker-desk", false},
		{"orca", "example-worker-desk", true},
		{"orca", "worker-desk", false},
	} {
		rec := surfaceReceipt{Cell: "example", Role: "worker-desk", Cockpit: c.cockpit, Name: c.name}
		if got := receiptNameOK(rec, "example"); got != c.ok {
			t.Errorf("%s %q: ok=%v, want %v", c.cockpit, c.name, got, c.ok)
		}
	}
}

// regression: #6698
func TestHerdrDownNewLabel(t *testing.T) {
	w := newLCWorld(t)
	w.tabs = `{"result":{"tabs":[{"tab_id":"tab7","label":"worker-desk"},{"tab_id":"tab9","label":"pr-review-desk"}]}}`
	w.cadenceRole("worker-desk", "herdr")
	labelReceipt(w, "worker-desk", "worker-desk")
	if _, err := w.down("worker-desk"); err != nil {
		t.Fatal(err)
	}
	if got := w.closes(); len(got) != 1 || got[0] != "herdr tab close tab7" {
		t.Fatalf("closes %v", got)
	}
}

// regression: #6698
func TestHerdrDownOldLabel(t *testing.T) {
	// A tab and receipt from before the bare label: <cell>-<role> on both.
	w := newLCWorld(t)
	w.cadenceRole("worker-desk", "herdr")
	labelReceipt(w, "worker-desk", "sample-worker-desk")
	if _, err := w.down("worker-desk"); err != nil {
		t.Fatal(err)
	}
	if got := w.closes(); len(got) != 1 || got[0] != "herdr tab close tab7" {
		t.Fatalf("closes %v", got)
	}
	if w.hasReceipt("worker-desk") {
		t.Fatal("the old-label receipt must go with its tab")
	}
}

// regression: #6698
func TestHerdrDownForeignLabel(t *testing.T) {
	for _, name := range []string{"other-worker-desk", "pr-review-desk", "sample-pr-review-desk"} {
		w := newLCWorld(t)
		w.cadenceRole("worker-desk", "herdr")
		labelReceipt(w, "worker-desk", name)
		_, err := w.down("worker-desk")
		wantReason(t, err, cellop.ReasonSurfaceIdentityMismatch)
		if got := w.closes(); len(got) != 0 {
			t.Fatalf("%s: closed %v", name, got)
		}
	}
}

// labelReceipt rewrites role's receipt with the given recorded label.
func labelReceipt(w *lcWorld, role, name string) {
	w.t.Helper()
	rec, err := readSurfaceReceipt(w.c.cadenceDir(role))
	if err != nil || rec == nil {
		w.t.Fatal(rec, err)
	}
	rec.Name = name
	if err := writeSurfaceReceipt(w.c.cadenceDir(role), *rec); err != nil {
		w.t.Fatal(err)
	}
}
