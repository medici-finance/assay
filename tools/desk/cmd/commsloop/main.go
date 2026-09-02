package main

// main.go — cmd/commsloop's entry point.
//
// The full drain loop (SelectQueue / TierPolicy / Dispatch / Land, the
// deterministic D-in pre-checks, and the contained prose router) lands in
// follow-up changes. This first change adds only the (action, class, risk)
// -> Tier assign table those will consult, plus the pinned decider entry
// (internal/runnertable). This stub keeps the package buildable as
// `package main` — required for `go build ./...` / `go vet ./...` (the
// module-wide CI gate) — in the interim; it is replaced by the real loop
// wiring later, not extended here.
func main() {
	println("commsloop: not yet wired for the drain loop")
}
