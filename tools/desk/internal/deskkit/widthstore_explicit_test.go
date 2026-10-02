package deskkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolvedWidthExplicitStateSharesPolicy(t *testing.T) {
	dir := t.TempDir()
	previous := dirOverride
	dirOverride = dir
	t.Cleanup(func() { dirOverride = previous })
	t.Setenv(EnvTokenConcurrencyTrip, "8")
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	store := filepath.Join(dir, "roster", "width")
	if err := os.MkdirAll(store, 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		width int
		age   time.Duration
	}{
		{"fresh", 2, 0}, {"clamped", 999, 0}, {"expired", 2, 2 * time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, _ := json.Marshal(WidthEntry{Loop: "pr-review-desk", Width: tc.width, Updated: now.Add(-tc.age).Format(time.RFC3339)})
			if err := os.WriteFile(filepath.Join(store, "pr-review-desk.json"), encoded, 0600); err != nil {
				t.Fatal(err)
			}
			a, as, ae := ResolvedWidthAt("pr-review-desk", now)
			b, bs, be := ResolvedWidthInStateDir("pr-review-desk", dir, now)
			if ae != nil || be != nil || a != b || as != bs {
				t.Fatalf("old %d %q %v; explicit %d %q %v", a, as, ae, b, bs, be)
			}
			if b > 4 {
				t.Fatalf("token ceiling ignored: %d", b)
			}
		})
	}
	if _, _, err := ResolvedWidthInStateDir("worker-desk", "relative", now); err == nil {
		t.Fatal("relative state dir admitted")
	}
	if _, _, err := ResolvedWidthInStateDir("unknown", dir, now); err == nil {
		t.Fatal("unknown role admitted")
	}
	if err := os.WriteFile(filepath.Join(store, "pr-review-desk.json"), []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ResolvedWidthInStateDir("pr-review-desk", dir, now); err == nil {
		t.Fatal("corrupt state became default")
	}
}
