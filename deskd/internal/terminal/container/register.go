package container

import (
	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// init registers this package's Driver as the kind: container constructor
// (terminal.RegisterDriver — see registry.go's "kind: container driver
// registration"). PR 137 landed the driver and its tests but left this call
// out (its own body: registering kind: container "left as a follow-up") —
// until this file existed, a `kind: container` desk in cells.yaml resolved
// no driver at all (terminal.DriverFor(schema.TargetContainer) answered
// ok=false), so the container rail was unreachable however deskd's
// per-desk attach loop eventually wires drivers in. A process that imports
// this package for its side effect (a blank import,
// `_ "github.com/medici-finance/assay/deskd/internal/terminal/container"`,
// from wherever a future brief wires deskd's driver set — the same
// coordination point registry.go's own doc comment names for 13/14/15) gets
// kind: container resolvable via terminal.DriverFor; a process that never
// imports it simply has no container driver registered, which is a
// build-time exclusion, not a runtime failure. See internal/terminal/pod's
// register.go for the identical shape (kind: pod) and
// internal/terminal/local's driver.go for kind: local's init-based twin.
func init() {
	terminal.RegisterDriver(schema.TargetContainer, newRegisteredDriver)
}

// newRegisteredDriver is the terminal.RegisterDriver factory: it builds a
// Driver with NewDriver(), which never fails (doc.go: "There is no client
// handle to build: the mechanism is the docker binary on PATH, invoked fresh
// on every Attach") — so, like local's factory and unlike pod's kubeconfig
// discovery, this factory always returns a nil error.
func newRegisteredDriver() (terminal.Target, error) {
	return NewDriver(), nil
}
