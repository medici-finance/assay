package deskkit

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// goldenDirDigest is the membership digest the producer wrote, BEFORE directory keys gained
// their trailing slash, for the literal directory tools/pkg of scopeKeyFixture. The key moved;
// the value must not, or every held legacy receipt would wake.
const goldenDirDigest = "3baaf58d5c0c6e5a30713296a7166646571549299744395fc14bea87836b064c"

// The same base pins for the literal file and the absent literal, whose keys keep their form.
const goldenFileDigest = "7531a10303ccd078fe1fa76af6ea938c6422e5e1fe001e6c7797f255fecb12c0"
const goldenAbsentDigest = "4524e523bad6db7d5e703ffdb953b1ba5c3dc5a31b7f27103f3f183aa7068e0f"

func scopeKeyFixture() fakeTree {
	return fakeTree{files: map[string]string{
		"tools/a.go":           "package a\n",
		"tools/pkg/one.go":     "package pkg\n",
		"tools/pkg/sub/two.go": "package sub\n",
	}}
}

// scopeKeyRoot writes a fixture tree to a fresh directory, for the offline reader.
func scopeKeyRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for p, b := range files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(b), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// scopeKeyReceipt builds a real record through BuildOutcomeRecord for a brief declaring decl,
// then re-keys its single scope: input to want (so either key form can be exercised on the same
// observed digest). It returns the receipt, the recorded digest and a reader over the same tree.
func scopeKeyReceipt(t *testing.T, decl, want string) (WakeReceipt, string, *RootRevisionReader, string) {
	t.Helper()
	tree := scopeKeyFixture()
	brief := "## Context\nfiles:\n- `" + decl + "`\n"
	tree.files[wakeTestBriefPath] = brief
	in := wakeTestInput()
	in.LandedBrief = []byte(brief)
	raw, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), in, tree)
	if err != nil {
		t.Fatal(err)
	}
	var r WakeReceipt
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	r.Brief = "demo/01"
	digest := ""
	for k, v := range r.Inputs {
		if strings.HasPrefix(k, InputKeyScopePrefix) {
			if digest != "" {
				t.Fatalf("one declaration produced two scope keys: %v", r.Inputs)
			}
			digest = v
			delete(r.Inputs, k)
		}
	}
	if digest == "" {
		t.Fatalf("no scope key produced: %v", r.Inputs)
	}
	r.Inputs[want] = digest
	root := scopeKeyRoot(t, tree.files)
	return r, digest, NewRootRevisionReader(root, in.ToolVersion), root
}

// randomBase62 is generated at run time: no token-shaped literal is committed.
func randomBase62(t *testing.T, n int) string {
	t.Helper()
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// Verify row 1: a literal directory is keyed scope:<decl>/; a literal file, an absent literal
// and a glob keep scope:<decl>; every digest equals the value the base wrote.
// regression: #3807
func TestScopeKeyDirectoryForm(t *testing.T) {
	scope, err := ResolveDeclaredScope([]string{"tools/pkg", "tools/a.go", "tools/planned.go", "tools/p*"}, scopeKeyFixture())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"scope:tools/pkg/":       goldenDirDigest,
		"scope:tools/a.go":       goldenFileDigest,
		"scope:tools/planned.go": goldenAbsentDigest,
		"scope:tools/p*":         goldenDirDigest, // the glob selects exactly tools/pkg
	}
	if len(scope.Shapes) != len(want) {
		t.Fatalf("keys = %v, want exactly %v", scope.Shapes, want)
	}
	for k, v := range want {
		if scope.Shapes[k] != v {
			t.Errorf("%s = %q, want %q (all keys: %v)", k, scope.Shapes[k], v, scope.Shapes)
		}
	}
	for _, never := range []string{"scope:tools/pkg", "scope:tools/a.go/", "scope:tools/planned.go/", "scope:tools/p*/"} {
		if _, ok := scope.Shapes[never]; ok {
			t.Errorf("emitted %s", never)
		}
	}
}

// Verify row 2: a receipt keyed in the legacy form (written before this change) still passes the
// completeness check, reads unchanged on an unchanged tree and wakes when a member is added; the
// directory form reads the same way.
// regression: #3807
func TestScopeKeyLegacyReceiptReads(t *testing.T) {
	for _, key := range []string{"scope:tools/pkg", "scope:tools/pkg/"} {
		t.Run(key, func(t *testing.T) {
			r, digest, rr, root := scopeKeyReceipt(t, "tools/pkg/", key)
			if digest != goldenDirDigest {
				t.Fatalf("recorded digest %s, want %s", digest, goldenDirDigest)
			}
			if err := rr.CheckReceiptScope(r); err != nil {
				t.Fatalf("completeness check refused %s: %v", key, err)
			}
			if got, ok := rr.Revision(key); !ok || got != digest {
				t.Fatalf("Revision(%s) = %q,%v, want %q,true", key, got, ok, digest)
			}
			if state, why := Unchanged(r, rr); state != InputsUnchanged {
				t.Fatalf("unchanged tree: %v %s", state, why)
			}
			if d := r.EvaluateWake(rr, time.Now()); d.State != WakeHold {
				t.Fatalf("unchanged tree did not hold: %v", d)
			}
			if err := os.WriteFile(filepath.Join(root, "tools/pkg/new.go"), []byte("package added\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if state, why := Unchanged(r, rr); state != InputsChanged {
				t.Errorf("added member held: %v %s", state, why)
			}
			if d := r.EvaluateWake(rr, time.Now()); d.State != WakeFire {
				t.Errorf("added member did not wake: %v", d)
			}
		})
	}
}

// Verify row 3: both forms present is refused (could-not-check); malformed keys read
// could-not-check; a slash key whose directory became a file wakes.
// regression: #3807
func TestScopeKeyAmbiguousForms(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		r, _, rr, _ := scopeKeyReceipt(t, "tools/pkg/", "scope:tools/pkg/")
		if state, why := Unchanged(r, rr); state != InputsUnchanged {
			t.Fatalf("directory-form receipt on an unchanged tree: %v %s", state, why)
		}
	})
	t.Run("both-forms", func(t *testing.T) {
		r, digest, rr, _ := scopeKeyReceipt(t, "tools/pkg/", "scope:tools/pkg/")
		r.Inputs["scope:tools/pkg"] = digest
		err := rr.CheckReceiptScope(r)
		if err == nil || !strings.Contains(err.Error(), "ambiguous scope key") {
			t.Fatalf("both forms accepted: %v", err)
		}
		if state, why := Unchanged(r, rr); state != InputsCouldNotCheck {
			t.Errorf("both forms: %v %s", state, why)
		}
		if d := r.EvaluateWake(rr, time.Now()); d.State != WakeCouldNotCheck {
			t.Errorf("both forms: %v", d)
		}
	})
	t.Run("malformed-keys", func(t *testing.T) {
		_, _, rr, _ := scopeKeyReceipt(t, "tools/pkg/", "scope:tools/pkg/")
		for _, key := range []string{"scope:tools/pkg//", "scope:/", "scope:", "scope:tools/p*/", "scope:tools/pkg/one.go//"} {
			if got, ok := rr.Revision(key); ok {
				t.Errorf("Revision(%q) = %q, read; want could-not-check", key, got)
			}
		}
	})
	t.Run("glob-slash-key-not-coverage", func(t *testing.T) {
		r, digest, rr, _ := scopeKeyReceipt(t, "tools/p*", "scope:tools/p*/")
		if digest != goldenDirDigest {
			t.Fatalf("glob digest %s", digest)
		}
		if err := rr.CheckReceiptScope(r); err == nil {
			t.Error("a slash-form glob key counted as coverage")
		}
		if state, why := Unchanged(r, rr); state != InputsCouldNotCheck {
			t.Errorf("slash-form glob key: %v %s", state, why)
		}
	})
	t.Run("directory-became-file", func(t *testing.T) {
		r, digest, rr, root := scopeKeyReceipt(t, "tools/pkg/", "scope:tools/pkg/")
		if err := os.RemoveAll(filepath.Join(root, "tools/pkg")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "tools/pkg"), []byte("now a file\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if got, ok := rr.Revision("scope:tools/pkg/"); !ok || got == digest || got == "" {
			t.Errorf("kind change read %q,%v; want a different digest", got, ok)
		}
		if state, why := Unchanged(r, rr); state != InputsChanged {
			t.Errorf("kind change: %v %s", state, why)
		}
		if d := r.EvaluateWake(rr, time.Now()); d.State != WakeFire {
			t.Errorf("kind change did not wake: %v", d)
		}
	})
}

// revisionOnly hides CheckReceiptScope: the wake reader alone.
type revisionOnly struct{ rr *RootRevisionReader }

func (w revisionOnly) Revision(in string) (string, bool)       { return w.rr.Revision(in) }
func (w revisionOnly) ActionCompleted(ref string) (bool, bool) { return w.rr.ActionCompleted(ref) }

// recordedRevision keeps the real CheckReceiptScope but stubs the wake reader to report every
// input exactly as recorded: the completeness check alone.
type recordedRevision struct {
	*RootRevisionReader
	recorded map[string]string
}

func (s recordedRevision) Revision(in string) (string, bool) {
	v, ok := s.recorded[in]
	return v, ok
}

// Verify row 4: each read layer holds with the other bypassed.
// regression: #3807
func TestScopeKeyLayersIndependent(t *testing.T) {
	t.Run("wake-reader-alone", func(t *testing.T) {
		for _, key := range []string{"scope:tools/pkg", "scope:tools/pkg/"} {
			r, digest, rr, _ := scopeKeyReceipt(t, "tools/pkg/", key)
			w := revisionOnly{rr}
			if got, ok := w.Revision(key); !ok || got != digest {
				t.Errorf("Revision(%s) = %q,%v, want the original digest", key, got, ok)
			}
			if state, why := Unchanged(r, w); state != InputsUnchanged {
				t.Errorf("%s unchanged tree: %v %s", key, state, why)
			}
			r.Inputs[key] = ""
			if state, why := Unchanged(r, w); state == InputsUnchanged {
				t.Errorf("%s: an empty recorded digest read as unchanged: %s", key, why)
			}
			if got, ok := w.Revision(key); ok && got == "" {
				t.Errorf("%s: reader returned an empty digest as a read", key)
			}
		}
	})
	t.Run("completeness-check-alone", func(t *testing.T) {
		r, _, rr, _ := scopeKeyReceipt(t, "tools/pkg/", "scope:tools/pkg/")
		stub := recordedRevision{rr, r.Inputs}
		if state, why := Unchanged(r, stub); state != InputsUnchanged {
			t.Fatalf("covered directory form: %v %s", state, why)
		}
		delete(r.Inputs, "scope:tools/pkg/")
		state, why := Unchanged(r, stub)
		if state != InputsCouldNotCheck || !strings.Contains(why, "missing declared scope tools/pkg") {
			t.Errorf("declaration missing in both forms: %v %s", state, why)
		}
	})
}

// Verify row 10: a scope: value that is not the producer's digest format is refused, in either
// key form; a well-formed digest is accepted.
// regression: #3807
func TestScopeKeyValueMustBeDigest(t *testing.T) {
	for _, key := range []string{"scope:tools/pkg/", "scope:tools/pkg"} {
		t.Run(key, func(t *testing.T) {
			r, digest, rr, _ := scopeKeyReceipt(t, "tools/pkg/", key)
			if err := rr.CheckReceiptScope(r); err != nil {
				t.Fatalf("well-formed digest refused: %v", err)
			}
			if state, why := Unchanged(r, rr); state != InputsUnchanged {
				t.Fatalf("well-formed digest: %v %s", state, why)
			}
			for _, bad := range []string{randomBase62(t, 40), strings.ToUpper(digest), digest + "0"} {
				r.Inputs[key] = bad
				err := rr.CheckReceiptScope(r)
				if err == nil || !strings.Contains(err.Error(), "not a scope digest") {
					t.Errorf("value of length %d accepted: %v", len(bad), err)
				}
				if state, _ := Unchanged(r, rr); state == InputsUnchanged {
					t.Errorf("value of length %d read as unchanged", len(bad))
				}
			}
		})
	}
}

// Verify row 5's producer half: a real record for a declared directory whose name ends in a
// credential keyword carries the slash key. With SCOPE_KEY_FIXTURE_DIR set it also writes the
// record, its legacy twin and a planted twin for the pinned scanner to read. Nothing token-shaped
// is committed: the planted value is generated here.
// regression: #3807
func TestScopeKeyLeakSweepFixture(t *testing.T) {
	const dir = "tools/desk/cmd/desktoken"
	brief := "## Context\nfiles:\n- `" + dir + "/`\n"
	tree := fakeTree{files: map[string]string{wakeTestBriefPath: brief, dir + "/main.go": "package main\n"}}
	in := wakeTestInput()
	in.LandedBrief = []byte(brief)
	raw, err := BuildOutcomeRecord(wakeTestRecord("blocked", BlockerUnknown, "#1"), in, tree)
	if err != nil {
		t.Fatal(err)
	}
	var r WakeReceipt
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	slashKey, plainKey := InputKeyScopePrefix+dir+"/", InputKeyScopePrefix+dir
	digest := r.Inputs[slashKey]
	if digest == "" {
		t.Fatalf("producer did not emit %s: %v", slashKey, r.Inputs)
	}
	if _, ok := r.Inputs[plainKey]; ok {
		t.Fatalf("producer emitted the legacy form %s", plainKey)
	}
	keyJSON, _ := json.Marshal(slashKey)
	plainJSON, _ := json.Marshal(plainKey)
	digestJSON, _ := json.Marshal(digest)
	if bytes.Count(raw, keyJSON) != 1 || bytes.Count(raw, digestJSON) != 1 {
		t.Fatalf("record does not carry the key and its digest exactly once:\n%s", raw)
	}
	out := os.Getenv("SCOPE_KEY_FIXTURE_DIR")
	if out == "" {
		return
	}
	planted, _ := json.Marshal("ghp_" + randomBase62(t, 36))
	twins := map[string][]byte{
		"new":     raw,
		"legacy":  bytes.Replace(raw, keyJSON, plainJSON, 1),
		"planted": bytes.Replace(raw, digestJSON, planted, 1),
	}
	for name, b := range twins {
		p := filepath.Join(out, name, "docs/streams/verify-outcomes/scope-fixture/01-20261021T000000Z-000000000000.json")
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, b, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
