package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// gitNewFile renders a manifest as git prints a new file in a branch diff.
func gitNewFile(path, manifest string) []string {
	body := strings.Split(strings.TrimRight(manifest, "\n"), "\n")
	out := []string{
		"diff --git a/" + path + " b/" + path,
		"new file mode 100644",
		"index 0000000..1234567",
		"--- /dev/null",
		"+++ b/" + path,
		"@@ -0,0 +1," + strconv.Itoa(len(body)) + " @@",
	}
	for _, ln := range body {
		out = append(out, "+"+ln)
	}
	return out
}

// TestBranchDiffK8sSecretOwnership runs the decrypted-k8s-secret rule over the surface
// deskpr really scans — a git-format branch diff through stripDiffMetaLines — rather
// than a hand-built approximation of it: one sops-encrypted Secret file beside a new
// ConfigMap file passes, and a decrypted Secret value keeps refusing, whether it is a new
// file or an edit to an existing List whose next item's kind sits in the hunk context.
func TestBranchDiffK8sSecretOwnership(t *testing.T) {
	enc := func(d string) string {
		return "ENC[" + "AES256_GCM,data:" + d + ",iv:aXZpdml2aXZpdml2,tag:dGFndGFndGFn,type:str]"
	}
	encrypted := "apiVersion: v1\nkind: Secret\nmetadata:\n  name: app-creds\ndata:\n  password: " +
		enc("Zm9vYmFyZm9vYmFyZm9vYmFy") + "\nsops:\n  mac: " + enc("bWFjbWFjbWFjbWFjbWFj") + "\n  version: 3.7.3\n"
	decrypted := "apiVersion: v1\nkind: Secret\nmetadata:\n  name: app-creds\ndata:\n  password: aHVudGVyMg==\n"
	configMap := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cfg\ndata:\n  MODE: weekly\n  PREFIX: snapshots/\n"
	template := "apiVersion: v1\nkind: Secret\nmetadata:\n  name: tmpl\ndata:\n  password: ${PASSWORD}\n"
	listEdit := []string{
		"diff --git a/deploy/list.yaml b/deploy/list.yaml",
		"index 1111111..2222222 100644",
		"--- a/deploy/list.yaml",
		"+++ b/deploy/list.yaml",
		"@@ -7,7 +7,7 @@ items:",
		"     app: x",
		"     tier: y",
		"   data:",
		"-    password: b2xkb2xk",
		"+    password: aHVudGVyMg==",
		" - apiVersion: v1",
		"   kind: ConfigMap",
		"   data:",
	}
	// A ConfigMap item anchored and merged into a Secret item: one data node, two owners.
	merged := "apiVersion: v1\nkind: List\nitems:\n- &cm\n  apiVersion: v1\n  kind: ConfigMap\n  data:\n" +
		"    password: aHVudGVyMg==\n- <<: *cm\n  apiVersion: v1\n  kind: Secret\n"
	// A LINE SEPARATOR run in a leading comment shifts the parser's line numbers six lines
	// past a split on LF, lining the decrypted Secret's data up with the ConfigMap's.
	shifted := "#" + strings.Repeat("\u2028", 6) + "\napiVersion: v1\nkind: ConfigMap\ndata: {MODE: weekly}\n---\n" +
		"kind: !!str Secret\nmetadata: {name: h}\n---\napiVersion: v1\ndata:\n  password: aHVudGVyMg==\n" +
		"kind: Secret\nmetadata:\n  name: app-creds\n"
	cases := []struct {
		name   string
		diff   []string
		refuse bool
	}{
		{"encrypted Secret file and ConfigMap file",
			append(gitNewFile("deploy/secret.enc.yaml", encrypted), gitNewFile("deploy/cm.yaml", configMap)...), false},
		{"DECRYPTED Secret file and ConfigMap file",
			append(gitNewFile("deploy/secret.yaml", decrypted), gitNewFile("deploy/cm.yaml", configMap)...), true},
		{"template Secret file and List edit carrying a DECRYPTED value",
			append(gitNewFile("deploy/tmpl.yaml", template), listEdit...), true},
		{"List file merging a ConfigMap item into a Secret item",
			gitNewFile("deploy/list.yaml", merged), true},
		{"encrypted Secret file and a line-shifted DECRYPTED Secret file",
			append(gitNewFile("deploy/secret.enc.yaml", encrypted), gitNewFile("deploy/shifted.yaml", shifted)...), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			surface := stripDiffMetaLines(strings.Join(c.diff, "\n") + "\n")
			err := deskkit.ScanSurfaceSecrets("branch diff vs origin/main", []byte(surface))
			hit := err != nil && strings.Contains(err.Error(), "DECRYPTED Kubernetes Secret")
			if hit != c.refuse {
				t.Fatalf("decrypted-k8s-secret hit = %v, want %v (err=%v)\nsurface:\n%s", hit, c.refuse, err, surface)
			}
		})
	}
}
