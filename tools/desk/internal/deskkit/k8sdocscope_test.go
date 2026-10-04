package deskkit

import (
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
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
	// A ConfigMap item anchored and merged into a Secret item: the parser's parent of the
	// `data:` key is the ConfigMap, yet the same node is also the Secret's data.
	mergedIntoSecret = "apiVersion: v1\nkind: List\nitems:\n- &cm\n  apiVersion: v1\n  kind: ConfigMap\n  data:\n    password: aHVudGVyMg==\n- <<: *cm\n  apiVersion: v1\n  kind: Secret\n"
	// The same through a bare root sequence rather than a List.
	rootSeqMerge = "- &cm\n  kind: ConfigMap\n  data:\n    password: aHVudGVyMg==\n- <<: *cm\n  kind: Secret\n"
	// A ConfigMap's data VALUE anchored on its own line and aliased as a Secret's data.
	aliasedData = "apiVersion: v1\nkind: List\nitems:\n- apiVersion: v1\n  kind: ConfigMap\n  data:\n    &d\n    password: aHVudGVyMg==\n- apiVersion: v1\n  kind: Secret\n  data: *d\n"
	// A Secret whose kind carries a custom tag holds a mapping that names another kind.
	customTagKind = "apiVersion: v1\nkind: !custom Secret\nmetadata:\n  annotations:\n    kind: ConfigMap\n    data:\n      password: aHVudGVyMg==\n"
	// `data:` text inside a ConfigMap's block scalar: the line is no parsed key at all.
	scalarDataText = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n  annotations:\n    note: |\n      data:\n        password: aHVudGVyMg==\n"
	// A ConfigMap carrying a second, tagged key that the Kubernetes decoder resolves to
	// `kind` (base64 of the word) with the value Secret: Kubernetes decodes a Secret.
	binaryKindKey = "apiVersion: v1\nkind: ConfigMap\n!!binary a2luZA==: Secret\ndata:\n  password: aHVudGVyMg==\n"
	// The typed JSON decoder matches field names case-insensitively, the last key winning.
	foldedKindJSON = "{\n  \"apiVersion\": \"v1\",\n  \"kind\": \"ConfigMap\",\n  \"Kind\": \"Secret\",\n  \"data\": {\n    \"password\": \"aHVudGVyMg==\"\n  }\n}\n"
	// A typed list decodes every item as its element type, whatever the item declares.
	secretListItem = "apiVersion: v1\nkind: SecretList\nitems:\n- apiVersion: v1\n  kind: ConfigMap\n  data:\n    password: aHVudGVyMg==\n"
	// A merge key with no anchor or alias: the earlier mapping in the merge list wins, so
	// the merged object is a Secret carrying the ConfigMap item's data.
	aliasFreeMerge = "apiVersion: v1\nkind: List\nitems:\n- <<:\n  - kind: Secret\n  - kind: ConfigMap\n    data:\n      password: aHVudGVyMg==\n"
	// Two kind keys, neither Secret: what the mapping encloses is not proven.
	duplicateKindEnclosing = "apiVersion: v1\nkind: Wrapper\nkind: Template\nspec:\n  object:\n    kind: ConfigMap\n    data:\n      password: aHVudGVyMg==\n"
)

// utf16le encodes an ASCII string as UTF-16LE.
func utf16le(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		b.WriteByte(s[i])
		b.WriteByte(0)
	}
	return b.String()
}

// bomPolyglot is one text read two ways. Split on LF, its line 1 is a decrypted Secret's
// `data:` mapping. To yaml.v3, which a leading UTF-16LE byte-order mark switches to
// UTF-16, lines 0-2 are one comment (every byte pair is a printable code unit, and the
// first LF that pairs with a NUL ends it), and the UTF-16 text after it files a
// ConfigMap's `data` key under line 1.
func bomPolyglot() string {
	return "\xff\xfe" + "#\x00" + "x\n" + "data:\n  password: aHVudGVyMg==\n" + "\x00" +
		utf16le("data:\n  MODE: weekly\nkind: ConfigMap\n")
}

// lineShifted builds a text that the YAML parser numbers differently from a split on
// LF: br, repeated, sits in a leading comment, so every parsed key is filed six lines
// below the line it is on. The decrypted Secret's `data:` (line 9) then lands on the
// ConfigMap's data key (line 3), and its `kind: Secret` (line 11) on a data-less helper
// Secret's kind (line 5), spelled with a tag so it is no declaration line of its own.
func lineShifted(br string, n int) string {
	return "#" + strings.Repeat(br, n) + "\napiVersion: v1\nkind: ConfigMap\ndata: {MODE: weekly}\n---\n" +
		"kind: !!str Secret\nmetadata: {name: h}\n---\napiVersion: v1\ndata:\n  password: aHVudGVyMg==\n" +
		"kind: Secret\nmetadata:\n  name: app-creds\n"
}

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
		// Anchors, aliases and merge keys let one parsed node belong to more than one
		// mapping, so its textual parent is no proof of its owner: could-not-check.
		{"List: ConfigMap item merged into a Secret item", mergedIntoSecret, true},
		{"diff: ConfigMap item merged into a Secret item", newFileHunk(mergedIntoSecret), true},
		{"root sequence: ConfigMap merged into a Secret", rootSeqMerge, true},
		{"List: ConfigMap data aliased as a Secret's data", aliasedData, true},
		{"fence: ConfigMap data aliased as a Secret's data", "```yaml\n" + aliasedData + "```\n", true},
		// Fence lines that are also YAML content: each fence proves its own object, but
		// the whole text is one YAML sequence whose second item merges the first. A
		// stream that parses always counts as a reading, so its alias still refuses.
		{"fences that are also YAML, ConfigMap merged into a Secret",
			"- &cm\n  ~~~: x\n  kind: ConfigMap\n  data:\n    password: aHVudGVyMg==\n  note: |\n   ~~~\n" +
				"- <<: *cm\n  ~~~: y\n  kind: Secret\n  note: |\n   ~~~\n", true},
		// A kind the parse cannot read as a plain string may still be Secret, so what it
		// encloses is not proven either.
		{"custom-tagged Secret kind enclosing another kind",
			encryptedSecretFixture + "---\n" + customTagKind, true},
		// Characters the parser counts as line breaks and a split on LF does not shift
		// every parsed key off the line it is filed under.
		{"LINE SEPARATOR run shifts parsed lines", lineShifted("\u2028", 6), true},
		{"NEXT LINE run shifts parsed lines", lineShifted("\u0085", 6), true},
		{"PARAGRAPH SEPARATOR run shifts parsed lines", lineShifted("\u2029", 6), true},
		{"lone CR run shifts parsed lines", lineShifted("\r", 7), true},
		{"diff: LINE SEPARATOR run in a new file", newFileHunk(lineShifted("\u2028", 6)), true},
		{"fence: PARAGRAPH SEPARATOR run", "```yaml\n" + lineShifted("\u2029", 6) + "```\n", true},
		// A fence proves only what it holds: a Secret pasted outside every fence is read.
		{"fenced ConfigMap, DECRYPTED Secret outside any fence",
			"```yaml\n" + configMapFixture + "```\n\n" + decryptedSecretFixture, true},
		// A `data:` line the parser holds no key for is could-not-check, not a pass.
		{"data: text in a ConfigMap block scalar beside a Secret",
			encryptedSecretFixture + "---\n" + scalarDataText, true},
		// A key the Kubernetes decoder can read as `kind` without the parse seeing a kind
		// key — a tagged key, or one that differs only in case — leaves the owner unproven.
		{"tagged key that decodes to kind beside a Secret",
			encryptedSecretFixture + "---\n" + binaryKindKey, true},
		{"diff: tagged key that decodes to kind, new file",
			newFileHunk(encryptedSecretFixture) + newFileHunk(binaryKindKey), true},
		{"fence: tagged key that decodes to kind",
			"```yaml\n" + encryptedSecretFixture + "```\n\n```yaml\n" + binaryKindKey + "```\n", true},
		{"diff: JSON kind and Kind keys, new file",
			newFileHunk(encryptedSecretFixture) + newFileHunk(foldedKindJSON), true},
		{"SecretList item declaring another kind",
			encryptedSecretFixture + "---\n" + secretListItem, true},
		{"merge key with no anchor or alias",
			encryptedSecretFixture + "---\n" + aliasFreeMerge, true},
		{"duplicate non-Secret kinds enclosing another kind",
			encryptedSecretFixture + "---\n" + duplicateKindEnclosing, true},
		// A byte-order mark that switches the parser to UTF-16 makes it parse a text no
		// line of the LF split holds: no reading is taken from it.
		{"diff: UTF-16 byte-order mark in a new file",
			newFileHunk(encryptedSecretFixture) + newFileHunk(bomPolyglot()), true},
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

// TestK8sForeignBreakCoversYAML is the class guard for line misalignment between the
// parser and k8sLine. It does not trust a hand-kept list: it asks the parser itself which
// runes end a line, by placing each one in a comment and reading back the line of the key
// that follows, and fails naming any rune the parser breaks on that k8sForeignBreak does
// not flag. A parser upgrade that adds a break character turns this red.
func TestK8sForeignBreakCoversYAML(t *testing.T) {
	breaks := 0
	for r := rune(1); r <= 0x10FFFF; r++ {
		if r == '\n' || !utf8.ValidRune(r) || (r > 0xFFFF && r&0xFFF != 0) {
			continue // LF is the split itself; astral planes are sampled, not swept
		}
		text := "a: 1 # x" + string(r) + "# y\nb: 2\n"
		var doc yaml.Node
		if yaml.Unmarshal([]byte(text), &doc) != nil || len(doc.Content) == 0 {
			continue // not valid YAML with this rune: no reading is taken from it
		}
		m := doc.Content[0]
		if m.Kind != yaml.MappingNode || len(m.Content) < 4 || m.Content[2].Line == 2 {
			continue
		}
		breaks++
		if !k8sForeignBreak(strings.Split(text, "\n")) {
			t.Errorf("the parser breaks a line on %U but k8sForeignBreak does not flag it", r)
		}
	}
	// Positive control: the probe must see the parser's known breaks, or it proves nothing.
	if breaks < 4 {
		t.Fatalf("probe found %d parser line breaks besides LF, want at least 4 (CR, NEL, LS, PS)", breaks)
	}
	if k8sForeignBreak(strings.Split("a: 1\r\nb: 2\r\n", "\n")) {
		t.Errorf("CRLF line endings flagged: they are one break to both counters")
	}
}

// TestK8sParseAlignment is the class guard for the parser reading a different text from
// the one k8sLine reads, whatever the mechanism (an encoding switch, a skipped mark, a
// line break the LF split does not count). It does not trust a list of known causes: for
// every two-byte prefix, ahead of a mapping spelled in UTF-8, UTF-16LE and UTF-16BE, it
// builds the owner index and fails naming any key a reading files under a line whose
// text does not hold that key's name.
func TestK8sParseAlignment(t *testing.T) {
	const doc = "alpha: 1\nkind: ConfigMap\n"
	utf16be := func(s string) string {
		var b strings.Builder
		for i := 0; i < len(s); i++ {
			b.WriteByte(0)
			b.WriteByte(s[i])
		}
		return b.String()
	}
	bodies := map[string]string{"UTF-8": doc, "UTF-16LE": utf16le(doc), "UTF-16BE": utf16be(doc)}
	filed, misfiled := 0, 0
	for p := 0; p < 1<<16; p++ {
		prefix := string([]byte{byte(p >> 8), byte(p)})
		for enc, body := range bodies {
			text := prefix + body
			lines := strings.Split(text, "\n")
			for _, r := range newK8sOwnerIndex(lines).readings {
				for _, c := range r.chunks {
					if !c.ok {
						continue
					}
					for ln, recs := range c.keys {
						for _, k := range recs {
							filed++
							if ln < 0 || ln >= len(lines) || !strings.Contains(lines[ln], k.name) {
								misfiled++
								if misfiled <= 5 {
									t.Errorf("prefix %q + %s mapping: key %q filed under line %d, whose text does not hold it", prefix, enc, k.name, ln)
								}
							}
						}
					}
				}
			}
		}
	}
	if misfiled > 5 {
		t.Errorf("... %d misfiled keys in all", misfiled)
	}
	// Positive control: the sweep must see readings at all, or it proves nothing.
	if filed == 0 {
		t.Fatal("no reading filed any key: the sweep checked nothing")
	}
}
