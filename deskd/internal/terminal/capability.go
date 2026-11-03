// capability.go — what a deskd host can do for each desk kind, decided BEFORE
// any attach attempt (the service contract, report F-2).
//
// Linking a driver, or compiling it for a platform, is not the same as being
// able to attach with it. Both the container and the local driver open a HOST
// pty through github.com/creack/pty, whose Windows build returns
// ErrUnsupported. A Windows-hosted deskd therefore cannot attach either kind
// today, and the supervisor must know that up front instead of discovering it
// through a retry loop. Native ConPTY support is future driver work. Nothing
// here marks native Windows hosting supported until that work is tested.
//
// The cluster kind (pod) is could-not-check on every host under this brief:
// deskd holds no cluster access grant (DR-deskd-attach-start), so the
// supervisor never builds the pod driver and never contacts an API server.
package terminal

import (
	"runtime"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/contract"
)

// Host is the platform a deskd process runs on. Zero value = the running
// process's own runtime.GOOS / runtime.GOARCH.
type Host struct {
	OS   string
	Arch string
}

func (h Host) orRuntime() Host {
	if h.OS == "" {
		h.OS = runtime.GOOS
	}
	if h.Arch == "" {
		h.Arch = runtime.GOARCH
	}
	return h
}

// Capability support values (closed enum, see [contract.AttachCapability]).
const (
	SupportSupported     = "supported"
	SupportUnsupported   = "unsupported"
	SupportCouldNotCheck = "could-not-check"
)

// Capability reason values (closed enum).
const (
	CapReasonNone           = ""
	CapReasonNoTarget       = "no-target"
	CapReasonNoPTYBackend   = "no-pty-backend"
	CapReasonNoClusterGrant = "no-cluster-grant"
	CapReasonUnknownKind    = "unknown-kind"
)

// Execution security profiles (closed enum).
const (
	// ProfileTrustedHost is deskd's own OS user on a host the operator already
	// trusts. The local driver's env deny-list and the container engine's own
	// user are hygiene on that host, NOT isolation from a hostile worker.
	ProfileTrustedHost = "trusted-host"
	// ProfileClusterRBAC is an exec bounded by the API server's RBAC for the
	// identity deskd presents.
	ProfileClusterRBAC = "cluster-rbac"
)

// CapabilityFor answers "can a deskd on host attach a desk of this kind?"
// without touching the desk. spec may be nil (no configured target).
func CapabilityFor(spec *TargetSpec, host Host) contract.AttachCapability {
	host = host.orRuntime()
	c := contract.AttachCapability{HostOS: host.OS, HostArch: host.Arch}
	if spec == nil {
		c.Support, c.Reason = SupportCouldNotCheck, CapReasonNoTarget
		return c
	}
	switch spec.Kind {
	case schema.TargetContainer:
		c.Backend, c.Profile = "docker-exec", ProfileTrustedHost
	case schema.TargetLocal:
		c.Backend, c.Profile = "host-pty", ProfileTrustedHost
	case schema.TargetPod:
		// No cluster access grant exists for deskd (DR-deskd-attach-start).
		// could-not-check, not unsupported: the host could, the grant is what
		// is missing.
		c.Backend, c.Profile = "kube-exec", ProfileClusterRBAC
		c.Support, c.Reason = SupportCouldNotCheck, CapReasonNoClusterGrant
		return c
	default:
		c.Support, c.Reason = SupportCouldNotCheck, CapReasonUnknownKind
		return c
	}
	// container and local both open a host pty (creack/pty), which has no
	// Windows implementation.
	if host.OS == "windows" {
		c.Support, c.Reason = SupportUnsupported, CapReasonNoPTYBackend
		return c
	}
	c.Support = SupportSupported
	return c
}
