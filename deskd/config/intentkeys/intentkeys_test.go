package intentkeys

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// genPub returns a freshly generated public key, base64. Every key in these
// tests is generated at test time: no key material is committed.
func genPub(t *testing.T) (ed25519.PublicKey, string) {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return pub, base64.StdEncoding.EncodeToString(pub)
}

func writeFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "intent-keys.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestLoadFailsClosed is ruling 1's fail-closed default: a missing, empty,
// unparsable or rule-breaking file yields a ring that trusts NO key, with a
// reason, and one bad entry refuses the whole file (the valid entry beside it
// is NOT left trusted).
func TestLoadFailsClosed(t *testing.T) {
	pub, good := genPub(t)
	_, other := genPub(t)
	valid := fmt.Sprintf("version: 1\nlogins:\n  alice:\n    - id: laptop\n      ed25519: %s\n", good)
	cases := map[string]string{
		"empty":             "",
		"whitespace":        "   \n\n",
		"garbage":           "{{{ not yaml",
		"wrong version":     strings.Replace(valid, "version: 1", "version: 2", 1),
		"no version":        strings.Replace(valid, "version: 1\n", "", 1),
		"unknown field":     valid + "trust_all: true\n",
		"unknown key field": strings.Replace(valid, "      ed25519:", "      expires: never\n      ed25519:", 1),
		"short key":         strings.Replace(valid, good, base64.StdEncoding.EncodeToString(pub[:31]), 1),
		"not base64":        strings.Replace(valid, good, "!!!!", 1),
		"uppercase login":   strings.Replace(valid, "  alice:", "  Alice:", 1),
		"bad login":         strings.Replace(valid, "  alice:", "  -alice:", 1),
		"empty key list":    valid + "  bob: []\n",
		"bad key id":        strings.Replace(valid, "id: laptop", "id: Laptop!", 1),
		"dup key id":        valid + fmt.Sprintf("    - id: laptop\n      ed25519: %s\n", other),
		"key under 2 users": valid + fmt.Sprintf("  bob:\n    - id: phone\n      ed25519: %s\n", good),
		"two documents":     valid + "---\nversion: 1\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			ring, reason := LoadOrEmpty(writeFile(t, body))
			if ring == nil || ring.Len() != 0 || ring.KeysFor("alice") != nil {
				t.Fatalf("a %s file left %d key(s) trusted — must trust none", name, ring.Len())
			}
			if reason == "" {
				t.Fatalf("a %s file was refused with no reason to log", name)
			}
		})
	}
	t.Run("missing", func(t *testing.T) {
		ring, reason := LoadOrEmpty(filepath.Join(t.TempDir(), "absent.yaml"))
		if ring.Len() != 0 || reason == "" {
			t.Fatalf("a missing file: len=%d reason=%q — want 0 keys and a reason", ring.Len(), reason)
		}
	})
	t.Run("nil ring trusts nothing", func(t *testing.T) {
		var r *Ring
		if r.KeysFor("alice") != nil || r.Len() != 0 {
			t.Fatal("a nil ring must trust nothing")
		}
	})
}

// TestLoadValidFile is the positive half: a valid file enrols exactly its keys,
// case-insensitively by login, and KeysFor hands out a copy.
func TestLoadValidFile(t *testing.T) {
	pub, good := genPub(t)
	pub2, good2 := genPub(t)
	body := fmt.Sprintf("version: 1\nlogins:\n  alice:\n    - id: laptop\n      ed25519: %s\n    - id: phone\n      ed25519: %s\n", good, good2)
	ring, reason := LoadOrEmpty(writeFile(t, body))
	if reason != "" {
		t.Fatalf("valid file refused: %s", reason)
	}
	if ring.Len() != 2 {
		t.Fatalf("Len = %d, want 2", ring.Len())
	}
	keys := ring.KeysFor("ALICE")
	if len(keys) != 2 || !keys[0].Pub.Equal(pub) || !keys[1].Pub.Equal(pub2) || keys[0].ID != "laptop" {
		t.Fatalf("KeysFor(alice) = %+v", keys)
	}
	keys[0].Pub[0] ^= 0xff
	if !ring.KeysFor("alice")[0].Pub.Equal(pub) {
		t.Fatal("mutating the returned key changed the ring — KeysFor must return a copy")
	}
	if ring.KeysFor("bob") != nil {
		t.Fatal("an unenrolled login returned keys")
	}
	if fp := keys[1].Fingerprint(); !strings.HasPrefix(fp, "SHA256:") {
		t.Fatalf("fingerprint %q", fp)
	}
}

// TestShippedFileParses pins the committed config/intent-keys.yaml as a file
// the loader accepts: a broken shipped file would fail closed at runtime (no key
// trusted) silently, so the breakage is caught here instead.
func TestShippedFileParses(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "intent-keys.yaml"))
	if err != nil {
		t.Fatalf("read shipped file: %v", err)
	}
	if _, err := Parse(data); err != nil {
		t.Fatalf("the shipped intent-keys.yaml does not parse: %v", err)
	}
}
