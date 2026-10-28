package deskkit

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func snapshotObserver(t *testing.T) func(string) int {
	t.Helper()
	activationIndex.Lock()
	activationIndex.slot = nil
	activationIndex.Unlock()
	old := activationWork
	counts := map[string]int{}
	var mu sync.Mutex
	activationWork = func(event string) { mu.Lock(); counts[event]++; mu.Unlock() }
	t.Cleanup(func() {
		activationWork = old
		activationIndex.Lock()
		activationIndex.slot = nil
		activationIndex.Unlock()
	})
	return func(event string) int { mu.Lock(); defer mu.Unlock(); return counts[event] }
}
func snapshotParity(t *testing.T, root string) {
	t.Helper()
	if err := snapshotCompare(root); err != nil {
		t.Fatal(err)
	}
}
func snapshotCompare(root string) error {
	got, skips, err := activationManifests(root)
	want, wantSkips, wantErr := DiscoverManifests(root)
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(skips, wantSkips) || fmt.Sprint(err) != fmt.Sprint(wantErr) {
		return fmt.Errorf("snapshot differs from reference: got=%+v %v %v want=%+v %v %v", got, skips, err, want, wantSkips, wantErr)
	}
	return nil
}
func snapshotFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

// regression: F-activation-snapshot
func TestActivationSnapshotMatchesFreshDiscovery(t *testing.T) {
	count := snapshotObserver(t)
	root := t.TempDir()
	writeManifest(t, root, "test/verbs", []string{"assay.desk.verbs"}, nil, nil)
	snapshotParity(t, root)
	before := count("enumeration")
	snapshotParity(t, root)
	// snapshotParity itself calls the reference exactly once.
	if count("enumeration") != before+1 || count("reuse") != 1 || count("validation-directory") == 0 {
		t.Fatal("warm fixture repeated full enumeration or hid validation")
	}
	changes := []struct {
		name  string
		apply func()
	}{
		{"nested untracked", func() {
			snapshotFile(t, filepath.Join(root, "untracked", "deep", "component.yaml"), "component: test/new\n")
		}},
		{"ignored", func() {
			snapshotFile(t, filepath.Join(root, ".gitignore"), "ignored/\n")
			snapshotFile(t, filepath.Join(root, "ignored", "component.yaml"), "component: test/ignored\n")
		}},
		{"dirty", func() {
			snapshotFile(t, filepath.Join(root, "test-verbs", "component.yaml"), "component: test/edited\n")
		}},
		{"same length dirty", func() {
			snapshotFile(t, filepath.Join(root, "test-verbs", "component.yaml"), "component: test/change\n")
		}},
		{"duplicate", func() { snapshotFile(t, filepath.Join(root, "a", "component.yaml"), "component: test/change\n") }},
		{"malformed", func() { snapshotFile(t, filepath.Join(root, "bad", "component.yaml"), "not: [valid: yaml") }},
		{"missing name", func() { snapshotFile(t, filepath.Join(root, "empty", "component.yaml"), "provides: []\n") }},
		{"removal", func() {
			if err := os.Remove(filepath.Join(root, "ignored", "component.yaml")); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, c := range changes {
		t.Run(c.name, func(t *testing.T) { c.apply(); snapshotParity(t, root); snapshotParity(t, root) })
	}
	// In-place data tampering by a caller cannot poison subsequent reuse.
	ms, _, _ := activationManifests(root)
	ms[0].Component = "poison"
	snapshotParity(t, root)
}

// regression: F-activation-snapshot
func TestActivationSnapshotIsNotAuthority(t *testing.T) {
	count := snapshotObserver(t)
	root := t.TempDir()
	writeManifest(t, root, "test/verbs", []string{"assay.desk.verbs"}, []string{"test.provider"}, nil)
	writeManifest(t, root, "test/provider", []string{"test.provider"}, []string{"assay.roster.trust", "assay.roster.ext.repo-aliases"}, nil)
	withRoster(t, goldenRoster())
	if msg := VerbActivationRefusal(root); msg != "" {
		t.Fatal(msg)
	}
	r := goldenRoster()
	r[EnvRepoAliases] = "not=a=valid=shape"
	withRoster(t, r)
	if msg := VerbActivationRefusal(root); !strings.Contains(msg, "INACTIVE") {
		t.Fatalf("cached discovery became authority: %q", msg)
	}
	withRoster(t, map[string]string{})
	if msg := VerbActivationRefusal(root); !strings.Contains(msg, "unset or malformed") {
		t.Fatalf("lower API did not refuse current trust: %q", msg)
	}
	if count("reuse") < 2 {
		t.Fatal("configuration checks did not exercise reused discovery")
	}
	withRoster(t, goldenRoster())
	writeManifest(t, root, "test/provider", []string{"test.provider"}, []string{"test.changed-requirement"}, nil)
	if msg := VerbActivationRefusal(root); !strings.Contains(msg, "test.changed-requirement") {
		t.Fatal("provider edit did not recompute activation:", msg)
	}
	snapshotParity(t, root)
	// A root spelling reused after replacement is not the same source.
	other := t.TempDir()
	writeManifest(t, other, "test/other", []string{"assay.desk.verbs"}, []string{"test.missing"}, nil)
	snapshotParity(t, other)
	snapshotParity(t, root)
	old := root + "-old"
	if err := os.Rename(root, old); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(old) })
	snapshotFile(t, filepath.Join(root, "component.yaml"), "component: test/replaced\nprovides: [assay.desk.verbs]\ninject:\n  required: [{key: test.missing}]\n")
	if msg := VerbActivationRefusal(root); !strings.Contains(msg, "test.missing") {
		t.Fatal("root replacement reused acceptance:", msg)
	}
}

// regression: F-activation-snapshot
func TestActivationSnapshotCorruptionAndBounds(t *testing.T) {
	snapshotObserver(t)
	root := t.TempDir()
	writeManifest(t, root, "test/verbs", []string{"assay.desk.verbs"}, nil, nil)
	snapshotParity(t, root)
	for _, mode := range []string{"corrupt", "oversized", "unsupported"} {
		t.Run(mode, func(t *testing.T) {
			activationIndex.Lock()
			slot := activationIndex.slot
			if slot == nil {
				t.Fatal("expected populated slot")
			}
			switch mode {
			case "corrupt":
				slot.payload = []byte(`{"Revision":"activation-discovery/1","Manifests":[]}`) // valid JSON, wrong checksum
			case "oversized":
				slot.payload = make([]byte, manifestLimit+1)
				slot.checksum = sha256.Sum256(slot.payload)
			case "unsupported":
				var p manifestPayload
				_ = json.Unmarshal(slot.payload, &p)
				p.Revision = "future"
				slot.payload, _ = json.Marshal(p)
				slot.checksum = sha256.Sum256(slot.payload)
			}
			activationIndex.Unlock()
			snapshotParity(t, root)
		})
	}
	var wg sync.WaitGroup
	errors := make(chan error, 24)
	for i := 0; i < 12; i++ {
		// Distinct roots force publication as well as concurrent reads. One slot
		// must not mix source identity, diagnostics or manifests between writers.
		writerRoot := t.TempDir()
		writeManifest(t, writerRoot, fmt.Sprintf("test/concurrent-%d", i), nil, nil, nil)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 2; j++ {
				if err := snapshotCompare(writerRoot); err != nil {
					errors <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	nested := filepath.Join(root, "nested")
	snapshotFile(t, filepath.Join(nested, "component.yaml"), "component: test/nested\n")
	snapshotParity(t, root)
	for _, marker := range []string{"file", "directory", "removed"} {
		switch marker {
		case "file":
			snapshotFile(t, filepath.Join(nested, ".git"), "gitdir: absent\n")
		case "directory":
			if err := os.Remove(filepath.Join(nested, ".git")); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(nested, ".git"), 0700); err != nil {
				t.Fatal(err)
			}
		case "removed":
			if err := os.Remove(filepath.Join(nested, ".git")); err != nil {
				t.Fatal(err)
			}
		}
		snapshotParity(t, root)
	}
	snapshotFile(t, filepath.Join(root, "large", "component.yaml"), "component: test/large\n#"+strings.Repeat("x", manifestLimit))
	snapshotParity(t, root)
	if activationIndex.slot != nil {
		t.Fatal("oversized source retained")
	}
}

// regression: F-snapshot-byte-parity
func TestSnapshotByteParity(t *testing.T) {
	snapshotObserver(t)
	t.Run("binary keys", func(t *testing.T) {
		root := t.TempDir()
		snapshotFile(t, filepath.Join(root, "verbs", "component.yaml"), "component: test/verbs\nprovides: [assay.desk.verbs]\ninject:\n  required: [{key: !!binary bmVlZP8=}]\n")
		snapshotFile(t, filepath.Join(root, "provider", "component.yaml"), "component: test/provider\nprovides: [!!binary bmVlZP4=]\n")
		first := VerbActivationRefusal(root)
		if !strings.Contains(first, "inactive") {
			t.Fatalf("reference precondition: %q", first)
		}
		if second := VerbActivationRefusal(root); second != first {
			t.Fatalf("reused binary keys changed refusal: first=%q second=%q", first, second)
		}
		snapshotParity(t, root)
	})
	t.Run("binary component", func(t *testing.T) {
		root := t.TempDir()
		snapshotFile(t, filepath.Join(root, "component.yaml"), "component: !!binary eP8=\nprovides: [!!binary bmVlZP8=]\ninject:\n  optional: [{key: !!binary bmVlZP4=}]\n")
		snapshotParity(t, root)
		snapshotParity(t, root)
	})
	// A second instance outside activation keys: raw pathname and skipped diagnostic
	// bytes must not be silently rewritten by any snapshot representation.
	t.Run("raw paths", func(t *testing.T) {
		rawRoot := t.TempDir()
		raw := filepath.Join(rawRoot, string([]byte{'x', 255}), "component.yaml")
		if err := os.MkdirAll(filepath.Dir(raw), 0700); err != nil {
			t.Skipf("filesystem does not admit raw path: %v", err)
		}
		if err := os.WriteFile(raw, []byte("component: test/raw\n"), 0600); err != nil {
			t.Fatal(err)
		}
		snapshotParity(t, rawRoot)
		snapshotParity(t, rawRoot)
		snapshotFile(t, raw, "not: [valid: yaml")
		snapshotParity(t, rawRoot)
		snapshotParity(t, rawRoot)
	})
}

// regression: F-snapshot-source-binding
func TestSnapshotSourceBinding(t *testing.T) {
	count := snapshotObserver(t)
	a, b := t.TempDir(), t.TempDir()
	for _, root := range []string{a, b} {
		snapshotFile(t, filepath.Join(root, "component.yaml"), "component: test/same\n")
	}
	for _, root := range []string{a, b, b + string(os.PathSeparator) + "."} {
		before := count("enumeration")
		if _, _, err := activationManifests(root); err != nil {
			t.Fatal(err)
		}
		if count("enumeration") != before+1 {
			t.Fatal("reused discovery across root/alias binding")
		}
	}
	// Keep content and spelling identical while changing the actual directory.
	if _, _, err := activationManifests(b); err != nil {
		t.Fatal(err)
	}
	old := b + "-old"
	if err := os.Rename(b, old); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(old) })
	snapshotFile(t, filepath.Join(b, "component.yaml"), "component: test/same\n")
	before := count("enumeration")
	if _, _, err := activationManifests(b); err != nil {
		t.Fatal(err)
	}
	if count("enumeration") != before+1 {
		t.Fatal("same-content root identity replacement reused discovery")
	}
}

// regression: F-snapshot-admission-recheck
func TestSnapshotAdmissionRecheck(t *testing.T) {
	for _, phase := range []string{"during capture", "before first reuse"} {
		t.Run(phase, func(t *testing.T) {
			count := snapshotObserver(t)
			root := t.TempDir()
			path := filepath.Join(root, "component.yaml")
			snapshotFile(t, path, "component: test/before\n")
			observer := activationWork
			changed := false
			if phase == "during capture" {
				activationWork = func(event string) {
					observer(event)
					if event == "parse" && !changed {
						changed = true
						if err := os.WriteFile(path, []byte("component: test/afterx\n"), 0600); err != nil {
							t.Error(err)
						}
					}
				}
			}
			first, _, err := activationManifests(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(first) != 1 || first[0].Component != "test/before" {
				t.Fatal("cold reference result was not the exact parsed observation")
			}
			if phase == "before first reuse" {
				snapshotFile(t, path, "component: test/afterx\n")
			}
			second, _, err := activationManifests(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(second) != 1 || second[0].Component != "test/afterx" || count("reuse") != 0 {
				t.Fatal("candidate returned stale bytes without successful independent validation")
			}
			snapshotParity(t, root)
		})
	}
}

// regression: F-snapshot-partial-candidate
func TestSnapshotUnreadableCapture(t *testing.T) {
	snapshotObserver(t)
	root := t.TempDir()
	path := filepath.Join(root, "component.yaml")
	// A dangling symlink deterministically fails the reference read even as root.
	if err := os.Symlink(filepath.Join(root, "missing"), path); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	_, skipped, err := activationManifests(root)
	if err != nil || len(skipped) != 1 || !strings.Contains(skipped[0], "unreadable") {
		t.Fatalf("reference read failure missing: %v %v", skipped, err)
	}
	if activationIndex.slot != nil {
		t.Error("partial reference read retained as a candidate")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	snapshotParity(t, root)
}

// regression: F-snapshot-validation-error
func TestSnapshotValidationError(t *testing.T) {
	count := snapshotObserver(t)
	root := t.TempDir()
	writeManifest(t, root, "a-first", nil, nil, nil)
	trailing := filepath.Join(root, "z-empty")
	if err := os.Mkdir(trailing, 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := activationManifests(root); err != nil {
		t.Fatal(err)
	}
	if activationIndex.slot == nil {
		t.Fatal("precondition: complete readable tree must retain a candidate")
	}
	captured := activationIndex.slot.source
	if err := os.Chmod(trailing, 0); err != nil {
		t.Skipf("could-not-check: directory permission change unsupported: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(trailing, 0700); err != nil {
			t.Error(err)
		}
	})
	// Root, ACLs or unsupported filesystems can bypass POSIX modes. Never count a
	// fixture that remained readable as proof of the validation error predicate.
	if _, err := os.ReadDir(trailing); err == nil {
		t.Skip("could-not-check: permissions did not make the trailing empty directory unreadable")
	} else if !os.IsPermission(err) {
		t.Skipf("could-not-check: expected permission denial, got %v", err)
	}
	observed, validationErr := validateManifests(root)
	if validationErr == nil || observed.digest() != captured {
		t.Fatalf("precondition: trailing manifest-free walk error must leave digest unchanged: err=%v equal=%v", validationErr, observed.digest() == captured)
	}
	want, wantSkipped, wantErr := DiscoverManifests(root)
	if wantErr == nil {
		t.Fatal("precondition: fresh enumeration must observe the real permission error")
	}
	got, gotSkipped, gotErr := activationManifests(root)
	if gotErr == nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotSkipped, wantSkipped) || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) || count("reuse") != 0 {
		t.Fatalf("validation error was accepted despite unchanged digest: got=%+v %v %v want=%+v %v %v reuse=%d", got, gotSkipped, gotErr, want, wantSkipped, wantErr, count("reuse"))
	}
}
