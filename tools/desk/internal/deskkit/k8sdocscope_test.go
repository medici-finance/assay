package deskkit

import (
	"strconv"
	"strings"
	"testing"
)

// asAddedFile renders a manifest as one new file's hunk in a unified diff the way deskpr
// scans it: the `diff --git`/`---`/`+++` header lines stripped (stripDiffMetaLines), the
// `@@ -0,0 +1,N @@` range line kept, and every content line marked `+`.
func asAddedFile(manifest string) string {
	body := strings.TrimRight(manifest, "\n")
	lines := strings.Split(body, "\n")
	for i, ln := range lines {
		lines[i] = "+" + ln
	}
	return "@@ -0,0 +1," + strconv.Itoa(len(lines)) + " @@\n" + strings.Join(lines, "\n") + "\n"
}

const (
	// A ConfigMap's data is plaintext by design — it is not a Secret and never carries
	// ciphertext. The values are ordinary config, under every entropy threshold.
	configMapFixture = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: restore-test-config\ndata:\n  SNAPSHOT_PREFIX: openbao/\n  CANARY_PATH: resilience/canary\n"
	configMapJSON    = "{\n  \"apiVersion\": \"v1\",\n  \"kind\": \"ConfigMap\",\n  \"data\": {\n    \"MODE\": \"weekly\"\n  }\n}\n"
	listOfBoth       = "apiVersion: v1\nkind: List\nitems:\n- apiVersion: v1\n  kind: ConfigMap\n  data:\n    MODE: weekly\n- apiVersion: v1\n  kind: Secret\n  data:\n    password: aHVudGVyMg==\n"
	// kubectl get -o yaml sorts keys, so `data:` comes BEFORE `kind:` in its output.
	sortedDecryptedSecret = "apiVersion: v1\ndata:\n  password: aHVudGVyMg==\nkind: Secret\nmetadata:\n  name: app-creds\n"
)

// TestK8sSecretDocScope pins the document scoping of the decrypted-k8s-secret rule
// (the multi-document false-positive shape): the rule arms on a `kind: Secret` anywhere in the
// text, and it used to read EVERY data/stringData mapping in that text as the Secret's —
// so a branch diff with one correctly sops-encrypted Secret and any ConfigMap refused on
// the ConfigMap. Each "pass" row below refused before the fix; each "refuse" row is a
// positive control that the scoping never admits a decrypted Secret value.
func TestK8sSecretDocScope(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		refuse bool
	}{
		// --- pass: a ConfigMap's data is not a Secret's, whatever else is in the text ---
		{"diff: encrypted Secret file then ConfigMap file",
			asAddedFile(encryptedSecretFixture) + asAddedFile(configMapFixture), false},
		{"diff: ConfigMap file then encrypted Secret file",
			asAddedFile(configMapFixture) + asAddedFile(encryptedSecretFixture), false},
		{"multi-doc YAML: encrypted Secret --- ConfigMap",
			encryptedSecretFixture + "---\n" + configMapFixture, false},
		{"body: ConfigMap and encrypted Secret in separate fences",
			"```yaml\n" + configMapFixture + "```\n\nand\n\n```yaml\n" + encryptedSecretFixture + "```\n", false},
		{"diff: JSON ConfigMap file beside encrypted Secret file",
			asAddedFile(encryptedSecretFixture) + asAddedFile(configMapJSON), false},

		// --- refuse: the scoping must never pass a decrypted Secret value ---
		{"diff: DECRYPTED Secret file then ConfigMap file",
			asAddedFile(decryptedSecretFixture) + asAddedFile(configMapFixture), true},
		{"diff: ConfigMap file then DECRYPTED Secret file",
			asAddedFile(configMapFixture) + asAddedFile(decryptedSecretFixture), true},
		{"multi-doc YAML: ConfigMap --- DECRYPTED Secret",
			configMapFixture + "---\n" + decryptedSecretFixture, true},
		{"multi-doc YAML: ConfigMap --- sorted DECRYPTED Secret (data above kind)",
			configMapFixture + "---\n" + sortedDecryptedSecret, true},
		{"List holding a ConfigMap and a DECRYPTED Secret (one document)",
			listOfBoth, true},
		// A Secret hunk whose own `kind: Secret` fell outside the diff context, carrying a
		// DEEPER kind (an ownerReference). That kind is not the mapping's owner and must not
		// count as evidence; the Secret kind elsewhere in the diff keeps the rule armed.
		{"diff: Secret hunk without its kind line but with a deeper ownerReference kind",
			asAddedFile(encryptedSecretFixture) +
				"@@ -4,6 +4,6 @@\n   ownerReferences:\n   - apiVersion: apps/v1\n     kind: Deployment\n     name: app\n data:\n-  password: b2xkb2xk\n+  password: aHVudGVyMg==\n", true},
		// A document with NO kind line is not evidence either way: pre-fix behaviour holds.
		{"diff: kind-less data hunk beside an encrypted Secret file",
			asAddedFile(encryptedSecretFixture) + "@@ -9,2 +9,2 @@\n data:\n+  password: aHVudGVyMg==\n", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := decryptedK8sSecret(c.in); got != c.refuse {
				t.Fatalf("decryptedK8sSecret = %v, want %v (k8sLine=%d)\n%s", got, c.refuse, k8sLine(c.in), c.in)
			}
			// The same verdict through the scan engine deskpr calls on a branch diff: a
			// pass row must not refuse with THIS rule (another arm may still weigh the
			// sops metadata), and a refuse row must refuse with it.
			err := ScanSurfaceSecrets("branch diff", []byte(c.in))
			hit := err != nil && strings.Contains(err.Error(), "DECRYPTED Kubernetes Secret")
			if hit != c.refuse {
				t.Fatalf("ScanSurfaceSecrets decrypted-k8s-secret hit = %v, want %v (err=%v)", hit, c.refuse, err)
			}
		})
	}
}
