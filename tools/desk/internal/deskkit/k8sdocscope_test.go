package deskkit

import (
	"strconv"
	"strings"
	"testing"
)

// newFileHunk renders a manifest as one new file's hunk on the surface deskpr really
// scans: stripDiffMetaLines drops the `diff --git`/`index`/`---`/`+++` header lines,
// keeps the `@@ -0,0 +1,N @@` range line, and strips each content line's `+` marker.
func newFileHunk(manifest string) string {
	body := strings.TrimRight(manifest, "\n")
	n := strings.Count(body, "\n") + 1
	return "@@ -0,0 +1," + strconv.Itoa(n) + " @@\n" + body + "\n"
}

// markedNewFile renders the same hunk with every content line still marked `+`: a
// diff quoted in a PR body or comment, which never passes through stripDiffMetaLines.
func markedNewFile(manifest string) string {
	lines := strings.Split(strings.TrimRight(manifest, "\n"), "\n")
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
	// A template Secret: its only value is a placeholder, so it is clean on its own. It
	// arms the rule for every other mapping in the same diff.
	templateSecret = "apiVersion: v1\nkind: Secret\nmetadata:\n  name: tmpl\ndata:\n  password: ${PASSWORD}\n"
	// The next item's `kind:` sits at the column of the previous item's `data:`.
	listSiblingHunk = "@@ -7,7 +7,7 @@\n    app: x\n    tier: y\n  data:\n    password: b2xkb2xk\n    password: aHVudGVyMg==\n- apiVersion: v1\n  kind: ConfigMap\n  data:\n"
	// kubectl-sorted List items: a ConfigMap item's trailing `kind:` in the same hunk as
	// the next Secret item's `data:`, whose own `kind: Secret` is below the hunk.
	sortedListHunk = "@@ -5,9 +5,9 @@\n  kind: ConfigMap\n  metadata:\n    name: cfg\n- apiVersion: v1\n  data:\n    password: b2xkb2xk\n    password: aHVudGVyMg==\n    user: dXNlcg==\n    host: aG9zdA==\n    port: NTQzMg==\n"
	// A List whose Secret item carries a block-scalar annotation holding a fence line.
	listFenceInScalar = "apiVersion: v1\nkind: List\nitems:\n- apiVersion: v1\n  kind: Secret\n  metadata:\n    annotations:\n      k: |\n        ```\n  data:\n    password: aHVudGVyMg==\n- apiVersion: v1\n  kind: ConfigMap\n  data:\n    MODE: weekly\n"
	// A Secret whose double-quoted scalar continues at column 0 across a fence line and a
	// `kind:` line that are both scalar content.
	quotedScalarSecret = "apiVersion: v1\nkind: Secret\nnote: \"x\n~~~\nkind: ConfigMap\"\ndata:\n  password: aHVudGVyMg==\n"
	// The same through a flow mapping spanning several lines.
	flowMapSecret = "apiVersion: v1\nkind: Secret\nmetadata: {name: app,\n~~~: x,\nkind: ConfigMap,\nlabels: {}}\ndata:\n  password: aHVudGVyMg==\n"
	// The same through a 1-space block scalar holding `---` and a `kind:` line.
	blockScalarSecret = "apiVersion: v1\nkind: Secret\nnote: |\n ---\n kind: ConfigMap\ndata:\n  password: aHVudGVyMg==\n"
	// Fake hunk headers in a body, the first chunk ending inside an open quoted scalar
	// that the second chunk closes: read contiguously it is one Secret.
	fakeHeaderBridge = "@@ -0,0 +1,3 @@\napiVersion: v1\nkind: Secret\nnote: \"x\n@@ -0,0 +1,4 @@\nkind: ConfigMap\nx: y\"\ndata:\n  password: aHVudGVyMg==\n"
	// Two kind keys in one mapping: ownership is not established, so the mapping is read.
	duplicateKind = "apiVersion: v1\nkind: ConfigMap\nkind: Secret\ndata:\n  password: aHVudGVyMg==\n"
	// An unrelated modification hunk elsewhere in the same diff.
	readmeHunk = "@@ -10,3 +10,4 @@\n line one\n line two\n line three\n line four\n"
)

// TestK8sSecretDocScope pins the ownership scoping of the decrypted-k8s-secret rule (the
// multi-document false-positive shape): the rule arms on a `kind: Secret` anywhere in the
// text, and it used to read EVERY data/stringData mapping in that text as the Secret's —
// so a branch diff with one correctly sops-encrypted Secret and any ConfigMap refused on
// the ConfigMap. Each "pass" row refused before the fix. Each "refuse" row is a value the
// base refuses and must keep refusing: a mapping is skipped only when a YAML parse, in
// every way the text can be read, proves its owning object is another kind.
func TestK8sSecretDocScope(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		refuse bool
	}{
		// --- pass: a ConfigMap's data is not a Secret's, whatever else is in the text ---
		{"diff: encrypted Secret file then ConfigMap file",
			newFileHunk(encryptedSecretFixture) + newFileHunk(configMapFixture), false},
		{"diff: ConfigMap file then encrypted Secret file",
			newFileHunk(configMapFixture) + newFileHunk(encryptedSecretFixture), false},
		{"marked diff: encrypted Secret file then ConfigMap file",
			markedNewFile(encryptedSecretFixture) + markedNewFile(configMapFixture), false},
		{"diff: Secret and ConfigMap files, later modification hunk",
			newFileHunk(encryptedSecretFixture) + newFileHunk(configMapFixture) + readmeHunk, false},
		{"multi-doc YAML: encrypted Secret --- ConfigMap",
			encryptedSecretFixture + "---\n" + configMapFixture, false},
		{"multi-doc YAML: encrypted Secret ... --- ConfigMap",
			encryptedSecretFixture + "...\n---\n" + configMapFixture, false},
		// A bare `...` with no `---` after it is not a valid stream to the parser, so
		// ownership is never established there: could-not-check keeps the refusal.
		{"bare ... end marker before a ConfigMap",
			encryptedSecretFixture + "...\n" + configMapFixture, true},
		{"body: ConfigMap and encrypted Secret in separate fences",
			"```yaml\n" + configMapFixture + "```\n\nand\n\n```yaml\n" + encryptedSecretFixture + "```\n", false},
		{"diff: JSON ConfigMap file beside encrypted Secret file",
			newFileHunk(encryptedSecretFixture) + newFileHunk(configMapJSON), false},
		{"List holding a ConfigMap and an encrypted Secret",
			strings.Replace(listOfBoth, "aHVudGVyMg==", encVal("Zm9vYmFyZm9vYmFy"), 1), false},

		// --- refuse: the scoping must never pass a decrypted Secret value ---
		{"diff: DECRYPTED Secret file then ConfigMap file",
			newFileHunk(decryptedSecretFixture) + newFileHunk(configMapFixture), true},
		{"diff: ConfigMap file then DECRYPTED Secret file",
			newFileHunk(configMapFixture) + newFileHunk(decryptedSecretFixture), true},
		{"marked diff: ConfigMap file then DECRYPTED Secret file",
			markedNewFile(configMapFixture) + markedNewFile(decryptedSecretFixture), true},
		{"multi-doc YAML: ConfigMap --- DECRYPTED Secret",
			configMapFixture + "---\n" + decryptedSecretFixture, true},
		{"multi-doc YAML: ConfigMap --- sorted DECRYPTED Secret (data above kind)",
			configMapFixture + "---\n" + sortedDecryptedSecret, true},
		{"List holding a ConfigMap and a DECRYPTED Secret (one document)",
			listOfBoth, true},
		// A Secret hunk whose own `kind: Secret` fell outside the diff context, carrying a
		// DEEPER kind (an ownerReference). A modification hunk never proves an owner.
		{"diff: Secret hunk without its kind line, deeper ownerReference kind",
			newFileHunk(encryptedSecretFixture) +
				"@@ -4,6 +4,6 @@\n  ownerReferences:\n  - apiVersion: apps/v1\n    kind: Deployment\n    name: app\ndata:\n  password: b2xkb2xk\n  password: aHVudGVyMg==\n", true},
		{"marked diff: ownerReference kind on an added line",
			markedNewFile(encryptedSecretFixture) +
				"@@ -4,6 +4,7 @@\n   ownerReferences:\n   - apiVersion: apps/v1\n+    kind: Deployment\n     name: app\n data:\n-  password: b2xkb2xk\n+  password: aHVudGVyMg==\n", true},
		// A document with NO kind line is not evidence either way: pre-fix behaviour holds.
		{"diff: kind-less data hunk beside an encrypted Secret file",
			newFileHunk(encryptedSecretFixture) + "@@ -9,2 +9,2 @@\ndata:\n  password: aHVudGVyMg==\n", true},
		{"diff: List edit, next item's kind in context",
			newFileHunk(templateSecret) + listSiblingHunk, true},
		{"diff: sorted List edit, previous item's kind in context",
			newFileHunk(templateSecret) + sortedListHunk, true},
		{"List: fence line inside a Secret item's block scalar",
			listFenceInScalar, true},
		{"diff: fence line inside a Secret item's block scalar",
			newFileHunk(encryptedSecretFixture) + newFileHunk(listFenceInScalar), true},
		{"body: marked diff of the same inside a tilde fence",
			"~~~\n" + markedNewFile(encryptedSecretFixture) + markedNewFile(listFenceInScalar) + "~~~\n", true},
		{"Secret: quoted scalar continuing at column 0", quotedScalarSecret, true},
		{"Secret: multi-line flow mapping", flowMapSecret, true},
		{"Secret: 1-space block scalar, hunk header elsewhere",
			blockScalarSecret + newFileHunk(configMapFixture), true},
		{"body: fake hunk headers bridging a quoted scalar", fakeHeaderBridge, true},
		{"duplicate kind keys in one mapping", duplicateKind, true},
		{"duplicate kind keys, Secret first",
			"apiVersion: v1\nkind: Secret\nkind: ConfigMap\ndata:\n  password: aHVudGVyMg==\n", true},
		// A mapping that names another kind INSIDE a Secret is still the Secret's text.
		{"Secret holding a nested mapping that names another kind",
			"apiVersion: v1\nkind: Secret\nmetadata:\n  name: x\n  annotations:\n    kind: ConfigMap\n    data:\n      password: aHVudGVyMg==\n", true},
		// Lines added into an existing file parse cleanly on their own, but a fragment
		// cannot see what encloses it: here, an existing Secret's annotation.
		{"diff: lines added inside an existing file, shaped like another kind",
			newFileHunk(templateSecret) + "@@ -7,0 +8,3 @@\n      kind: ConfigMap\n      data:\n        password: aHVudGVyMg==\n", true},
		// A mapping whose keys may come from a merge key is never proven, whatever the
		// merged anchor is: here, a Secret item.
		{"merge key in the owning mapping",
			"apiVersion: v1\nkind: List\nitems:\n- &s\n  apiVersion: v1\n  kind: Secret\n  type: Opaque\n- <<: *s\n  kind: ConfigMap\n  data:\n    password: aHVudGVyMg==\n", true},
		// Fence lines that are also YAML content (a key, a scalar continuation): the
		// fence reading proves a ConfigMap, the plain-YAML reading does not. Every reading
		// that applies has to agree.
		{"fence lines that are also YAML content",
			"~~~: x\nkind: Secret\na: b\n ~~~\n~~~~: y\nkind: ConfigMap\ndata:\n  password: aHVudGVyMg==\n   ~~~~\n", true},
		// A hunk whose count stops short of its content: the entries after `data:` lie
		// outside every hunk, so the text is not a well-formed diff and proves nothing.
		{"diff: hunk count short of its content",
			newFileHunk(templateSecret) + "@@ -0,0 +1,3 @@\napiVersion: v1\nkind: ConfigMap\ndata:\n  password: aHVudGVyMg==\n", true},
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
