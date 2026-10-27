// Package container is the first [terminal.Target] driver (the service contract):
// docker exec into the desk container, the attachable rail a one-container
// solo cell gets before any cluster driver exists (spec §5.1, §1).
//
// # The ONE docker mechanism
//
// This package is the ONLY place `docker` is touched anywhere in the cockpit
// (a reviewer proves that by grepping exec.Command/NewClientWithOpts outside
// it and finding zero — Verify row 9). The mechanism is the `docker` CLI via
// os/exec, run under a locally-allocated pty (github.com/creack/pty — already
// a direct dependency of this module, used the same way by the terminal spike
// spike): `docker inspect` answers reachability/liveness synchronously before
// ever attaching, and `docker exec -i -t <container> bash -l` is the
// attachment itself, started with [pty.StartWithSize] so its stdio is a real
// terminal (echo, line discipline, resize) from the first byte.
//
// The Docker Engine API client library (github.com/docker/docker/client) was
// considered and REFUSED: pinning it pulls in ~30 transitive modules this
// driver does not otherwise need (OpenTelemetry's SDK and exporters, gRPC,
// several unrelated vendor SDKs) for what the brief's own spec text already
// names as a CLI operation ("docker exec -it into the desk container", spec
// §5.1) — a cost with no behavioural benefit here, since the CLI reaches the
// exact same daemon over the exact same socket. Either mechanism satisfies
// the brief; this package picks the one with zero NEW third-party runtime
// dependencies (docker-on-PATH is already a precondition of a "desk
// container" existing at all) and reuses this module's existing pty library
// rather than adding a second terminal abstraction.
//
// # What this driver adds vs. what the terminal core already shipped
//
// The [terminal.Target]/[terminal.Session] interface, the registry, the
// cursor-indexed ring, and the two-signal agent-state derivation belong to
// the terminal package — cited there, not re-derived here. This package adds only the bytes-in/bytes-out over one
// real transport. The ring's byte/line caps (~1 MiB / 10 000 lines) and the
// ~200ms idle-plus-trailing-prompt heuristic are the terminal spike's spike answers,
// consumed by the registry — this driver does not reimplement or
// second-guess them; see the [terminal] package doc.
//
// # Two independent layers guard the exec (the container driver SPOF)
//
// The registry's [terminal.TargetSpec] — never a request — names the
// container (the terminal core's layer). This driver adds one of its own: the exec'd
// command is a fixed default login shell, never anything a caller supplies,
// and the exec runs with the container's OWN user and environment — no
// `-u`/`--user`, `-e`/`--env`, or `--privileged` flag is ever passed to
// `docker exec` — so a bug in this driver cannot elevate beyond what the
// container already is. astguard_test.go audits that at the source level, the
// same class of guarantee as the terminal package's norest_test.go pattern.
package container
