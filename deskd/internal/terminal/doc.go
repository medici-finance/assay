// Package terminal is the cockpit's TERMINAL SEAM (the service contract): the one
// Go interface every "where does a desk run" driver implements, the registry
// that resolves a desk id to a target, an in-memory cursor-indexed scrollback
// ring, and the agent-state derivation a terminal read reports.
//
// # The seam, in one sentence
//
// A [Target] is the ONLY place a process is ever spawned to reach a desk
// (docker exec, kubectl exec, or a local PTY — each its own later brief, 13/14/
// 15); the [Registry] owns the ring buffer and the derived agent state; a
// driver owns only bytes. The registry — never the client — chooses which
// target a desk id resolves to, from the parsed cells.yaml `target:` (spec
// §5.1). That single fact is the seam's load-bearing security property: if a
// client could name a target, it could attach to any container on the host.
//
// # Transport bounds and limits
//
// The byte transport performs no transcoding. The registry bounds scrollback
// by both bytes and lines; cumulative byte cursors resume exactly or report
// eviction explicitly. Idle detection uses output silence and a prompt regex
// when the producer supplies no structured prompt mark. Browser IME behavior
// needs separate client qualification and is not established by these tests.
//
// # No scrollback at rest
//
// The scrollback lives in memory ONLY (spec §5.1): a source audit
// (norest_test.go, the internal/custody/norest_test.go pattern) fails the build
// on any os.WriteFile, os.Create, os.OpenFile, os.MkdirAll, or os.ReadFile in
// this package.
// Nothing here spools a buffer to disk.
package terminal
