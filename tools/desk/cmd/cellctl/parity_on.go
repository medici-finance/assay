//go:build parity

package main

import (
	"os"
	"strings"
)

// parityDropsPlanEnv is the DIVERGENCE INJECTOR, and it exists for one reason: a harness that
// diffs nothing — a normalisation bug, an empty matrix, a fixture that never built — is a green
// lamp wired to nothing. Building with `-tags parity` and setting
//
//	CELLCTL_PARITY_MUTATE=KUBECONFIG
//
// drops that one `[plan] env` line, and the parity harness MUST then go red naming the cell.
// That is the brief's row 5: the negative control on the oracle diff.
//
// KUBECONFIG is the canary on purpose — it is the cluster-isolation control, the most damaging
// line to lose silently, which is exactly why row 13 re-checks it is intact in a release build.
//
// This file is the ONLY one in the package that names CELLCTL_PARITY_MUTATE, and it carries
// `//go:build parity`, so a tagless compile contains none of this code (rows 13-14).
func parityDropsPlanEnv(kv string) bool {
	key := os.Getenv("CELLCTL_PARITY_MUTATE")
	if key == "" {
		return false
	}
	return strings.HasPrefix(kv, key+"=")
}

// parityBuild lets a test state which build it is running under.
const parityBuild = true
