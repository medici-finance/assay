# Git fixture lifecycle

This stdlib-only test-support module owns the template previously implemented in
`tools/desk/internal/gitquiet`. Statusgen imports it only from its test harness;
the desk internal package delegates to it so existing callers retain their API.
No production binary imports it. Local `replace` directives resolve this module
from the complete source tree, including the source-build Docker stage.

`Run` installs a temporary template for the test binary and restores its caller's
environment afterward. Repository-local settings cover init, clone, and local
bare remotes even when Git removes command-level configuration on receive-pack.
An environment that deliberately drops `GIT_TEMPLATE_DIR` still needs an explicit
`TemplateDir` from its fixture owner; this module does not weaken environment
isolation to retain inherited values.

The desk package retains its behavior tests and package-coverage guard. This
module also carries focused behavior tests and the implementation mutations,
whose paths resolve inside this module. Desk adapter and coverage mutations stay
inside the desk module. Run `go test . -count=1 -timeout 120s` here; run the desk
manifest's test command inside `tools/desk`. The existing CI desk suite exercises
the adapter and shared implementation; general module discovery also builds and
vets this module.

## Go telemetry in a scratch home (`goquiet`)

The `goquiet` subpackage closes the same class of defect for the go command. Unless
telemetry is off, a go command starts a telemetry child it does not wait for, in the
telemetry directory under the config directory that HOME (or `XDG_CONFIG_HOME`)
selects; in a test's scratch home that child races the directory's removal.
`goquiet.Cmd` wraps a go command a test is about to run and runs `go telemetry off`
first in the scratch telemetry directory its environment selects; `goquiet.Off` does
the same for one environment (for a `TestMain` fixture that moves HOME), and
`goquiet.Home` returns a treated `t.TempDir()` home with a cleanup check that no go
command wrote telemetry there. None of them ever changes the telemetry setting of the
process's own config directory.

Its behavior tests and the repository-wide guard (`TestGoPackagesQuietHome`, with its
positive control over `testdata/goguard`) live in the desk adapter package
`tools/desk/internal/gitquiet`, which the CI desk suite runs; their mutations are in
that package's `mutations.json`.
