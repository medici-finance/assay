package container

import (
	"testing"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// TestContainerDriverRegisteredForKindContainer is the fail-first regression
// test for the authorization contract: the container driver's driver (PR 137) never called
// terminal.RegisterDriver for kind: container (PR 137's own body: registering
// it "left as a follow-up"), so a `kind: container` desk configured in
// cells.yaml resolved NO driver at all — terminal.DriverFor(schema.TargetContainer)
// answered ok=false, exactly like an unconfigured/unimported kind, even
// though this package's Driver was fully implemented and tested. This proves
// the kind resolves, through the registry's driver table, to THIS package's
// Driver — the same self-registration shape pod
// (internal/terminal/pod/register.go) and local (internal/terminal/local's
// driver.go init) already carry.
func TestContainerDriverRegisteredForKindContainer(t *testing.T) {
	factory, ok := terminal.DriverFor(schema.TargetContainer)
	if !ok {
		t.Fatal("no driver registered for kind: container — terminal.RegisterDriver was not called (the container driver task 2 / the authorization contract)")
	}
	target, err := factory()
	if err != nil {
		t.Fatalf("factory(): %v", err)
	}
	if _, ok := target.(*Driver); !ok {
		t.Fatalf("registered kind: container factory produced %T, want *container.Driver", target)
	}
}

// TestUnknownKindHasNoRegisteredDriver proves the registry's driver table is
// keyed by kind, never a catch-all: a kind nothing ever registered still
// refuses (ok=false) — the fail-closed half of DriverFor's own doc comment
// ("a build that excludes the pod driver" fails closed rather than silently
// answering with an unrelated driver).
func TestUnknownKindHasNoRegisteredDriver(t *testing.T) {
	if _, ok := terminal.DriverFor(schema.TargetKind("bogus-kind-no-one-registered")); ok {
		t.Fatal("DriverFor resolved an unregistered kind — want ok=false")
	}
}
