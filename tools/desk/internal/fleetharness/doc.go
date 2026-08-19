// Package fleetharness is the FLEET-WORKFLOW ACCEPTANCE HARNESS for the gated desk verbs.
//
// It contains no production code. Its whole content is a test that boots the workflow the
// desk skills MANDATE — a detached worktree created from `origin/<branch>`, in a repo that
// may not be the caller's own, with a cold App-token mint — and drives each gated verb
// through it end to end, recording how far each one gets.
//
// WHY A SEPARATE PACKAGE, AND WHY SUBPROCESSES. Every verb here already has a thorough
// unit suite, and every one of those suites was green on the day the fleet audit measured
// **zero successes across fourteen shepherd passes**. They were green because they exercise the
// verbs in the shape the verbs were WRITTEN for — a branch checked out, in the tool's own
// clone, with the seams (`getwd`, `publicRepoGateFn`, `execCommand`) rebound in-process.
// The defect was never in a function; it was in the relationship between a precondition
// and a workflow, and that relationship is invisible from inside either one.
//
// So this harness builds the real binaries and runs them as processes against real git
// repositories. Nothing is stubbed that the mandated workflow would not stub: `gh` and
// `desktoken` are PATH shims, exactly as a cold worker shell resolves them.
//
// The 2026-08-13 dispatch cycle is what makes this urgent rather than tidy: `deskreply`
// returned rc=6 on the mandated detached worktree for **5 of 5** workers, every one fell
// back to raw `gh pr comment`, and routing around `deskreply` routes around
// `deskkit.BodyCheck` — the secret scan the tool exists to run. A guard that 100% of a
// mandated workflow's invocations must bypass is a guard that is not running, and nothing
// in the artifact says so. This harness is the thing that says so, before a merge rather
// than after fourteen passes.
package fleetharness
