package answer

import "github.com/medici-finance/assay/deskd/internal/custody"

// custody.Credential is the PRODUCTION Credential the write path authors with.
// This binding is deliberate and load-bearing in two ways:
//
//   - It documents that the human-gate token comes from internal/custody and
//     nowhere else — the answer path holds no token of its own.
//   - It makes internal/answer → internal/custody a real import edge, which is
//     the POSITIVE CONTROL for internal/mcp's TestHumanCredentialUnreachable:
//     that test asserts the MCP graph never reaches custody AND that the write
//     path does, so a misspelt forbidden-path string cannot pass vacuously.
//
// The direction of this edge is the whole security story: the WRITE PATH may
// reach the credential; the MCP surface may not, and cannot, because nothing it
// imports imports custody.
//
// Under custody model (b) (#1819, the service contract increment 2) the concrete
// type is a PER-REQUEST value rather than an at-rest store: it is built from
// the credential the caller supplied with a named action and it lives only in
// that request's context. The interface is unchanged, which is the point — the
// write path's meaning did not move when its credential's lifetime did.
var _ Credential = (*custody.Credential)(nil)
