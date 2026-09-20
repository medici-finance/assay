//go:build !parity

package main

// parityDropsPlanEnv is the RELEASE build's answer: never. No environment variable is consulted,
// because this file — the one a tagless `go build` compiles — names none.
//
// The test-only injector lives in parity_on.go behind `//go:build parity`. Brief
// The port's brief demands the fail-open guard be proven closed in what ships, not asserted:
// row 14 greps every file naming CELLCTL_PARITY_MUTATE and requires `//go:build parity` on each,
// and row 13 builds with NO tag and shows the mutated and unmutated dry-run plans are
// byte-identical AND still carry KUBECONFIG=/dev/null.
func parityDropsPlanEnv(string) bool { return false }

// parityBuild lets a test state which build it is running under without naming the injector's
// environment variable (row 14 requires every file that names it to carry //go:build parity).
const parityBuild = false
