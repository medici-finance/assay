#!/usr/bin/env python3
"""Bounded production faults in a clean owned linked worktree; no live setup.

Each control must compile, then fail at its named runtime assertion. Fixture or
compiler failure is invalid evidence. Original source bytes are always restored.
Only the explicit disposable PostgreSQL test DSN is consumed by the Go harness.
"""

import os
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]
REPO = ROOT.parent


def run(arguments, cwd=REPO, timeout=120):
    env = os.environ.copy()
    env.update(GOWORK="off", KUBECONFIG="/dev/null")
    for key in list(env):
        if key.startswith("GH_") or key.startswith("GITHUB_"):
            del env[key]
    return subprocess.run(arguments, cwd=cwd, env=env, stdout=subprocess.PIPE,
                          stderr=subprocess.STDOUT, timeout=timeout, check=False)


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def main():
    require(os.environ.get("ASSAY_TEST_POSTGRES_DSN"), "explicit disposable test DSN required")
    require((REPO / ".git").is_file(), "owned linked worktree required")
    require(run(["git", "status", "--porcelain"]).stdout == b"", "clean feature worktree required")
    branch = run(["git", "branch", "--show-current"]).stdout.decode().strip()
    require(branch.startswith("codex/"), "owned feature branch required")
    core = "workflow/store/refresh_provision.go"
    role = "workflow/store/refresh_role_profile.go"
    migration = "workflow/store/migrations/00004_refresh_initialization.sql"
    ci = ".github/workflows/assay-workflow.yml"
    faults = [
        ("known-baseline", core, "return u.Known && initialInstant", "return initialInstant",
         "workflow", "./store", "^TestPGRefreshProvision$/unknown-source$",
         "unqualified initial accounting accepted"),
        ("immutable-replay", core, "!bytes.Equal(data, r.data)", "false",
         "workflow", "./store", "^TestPGRefreshProvisionReplay$",
         "conflicting immutable baseline accepted"),
        ("atomic-initialization", core,
         "if e = provisionRefreshRecord(ctx, tx, p, record, now); e != nil {\n\t\t\treturn RefreshProvisionReceipt{}, e\n\t\t}",
         "if e = provisionRefreshRecord(ctx, tx, p, record, now); e != nil {\n\t\t\treturn RefreshProvisionReceipt{}, e\n\t\t}\n\t\tif record.kind == \"source\" { if tx.Commit() != nil { return RefreshProvisionReceipt{}, refresh.ErrHeld } }",
         "workflow", "./store", "^TestPGRefreshProvision$/atomic$",
         "failed initialization retained a staged prefix"),
        ("runtime-profile-insert", role,
         'operatorInsert: "source_id profile known window_at window_ns http_used http_limit inflight_limit reservation_limit retry_at", operatorUpdate: "known", runtimeUpdate:',
         'operatorInsert: "source_id profile known window_at window_ns http_used http_limit inflight_limit reservation_limit retry_at", operatorUpdate: "known", runtimeInsert: "source_id profile known window_at window_ns http_used http_limit inflight_limit reservation_limit", runtimeUpdate:',
         "workflow", "./store", "^TestPGRefreshRuntimePrivileges$",
         "runtime escaped declared role boundary insert-budget"),
        ("runtime-profile-known", role,
         'operatorUpdate: "known", runtimeUpdate: "window_at http_used retry_at unknown_hold"',
         'operatorUpdate: "known", runtimeUpdate: "known window_at http_used retry_at unknown_hold"',
         "workflow", "./store", "^TestPGRefreshRuntimePrivileges$",
         "runtime escaped declared role boundary known-upgrade"),
        ("effective-membership", role,
         "OR EXISTS(SELECT 1 FROM pg_auth_members WHERE member=actor)", "OR false",
         "workflow", "./store", "^TestPGRefreshRuntimePrivileges$/effective-membership$",
         "unsafe effective role was accepted membership"),
        ("effective-definer", role,
         "OR EXISTS(SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE p.prosecdef AND n.nspname !~ '^pg_' AND n.nspname<>'information_schema' AND has_schema_privilege(n.oid,'USAGE') AND has_function_privilege(p.oid,'EXECUTE'))", "OR false",
         "workflow", "./store", "^TestPGRefreshRuntimePrivileges$/effective-security-definer$",
         "unsafe effective role was accepted security-definer"),
        ("lower-eviction", migration,
         "CREATE TRIGGER refresh_eviction_guard BEFORE DELETE ON public.refresh_fills FOR EACH ROW EXECUTE FUNCTION public.guard_refresh_eviction();", "",
         "workflow", "./store", "^TestPGRefreshProvision$/live-fill-delete$",
         "direct SQL deleted a live accounted fill"),
        ("provenance-immutability", migration,
         "CREATE TRIGGER refresh_initialization_guard BEFORE UPDATE OR DELETE ON public.refresh_initializations FOR EACH ROW EXECUTE FUNCTION public.guard_refresh_initialization();", "",
         "workflow", "./store", "^TestPGRefreshProvisionSchema$",
         "raw SQL rewrote immutable initialization provenance"),
        ("dedicated-native-ci", ci,
         "grep -F -- '--- PASS: TestMirrorProvisionedReadThrough ' \"$provisioned\"", ":",
         "tools/desk", "./internal/deskkit", "^TestWorkflowRefreshProvisionCI$",
         "dedicated provisioned-role join lacks required execution proof"),
    ]
    snapshots = {name: (REPO / name).read_bytes() for name in {f[1] for f in faults}}
    # A qualifying baseline must run the same assertions successfully first.
    baselines = [
        ("workflow", "./store", "^TestPGRefresh(Provision|ProvisionReplay|ProvisionSchema|RuntimePrivileges)$"),
        ("tools/desk", "./internal/deskkit", "^TestWorkflowRefreshProvisionCI$"),
    ]
    scratch = Path(tempfile.mkdtemp(prefix="provision-controls-"))
    os.chmod(scratch, 0o700)
    for index, (module, package, pattern) in enumerate(baselines):
        result = run(["go", "test", "-count=1", "-timeout=60s", "-v", "-run", pattern, package], REPO / module)
        (scratch / f"baseline-{index}.log").write_bytes(result.stdout)
        require(result.returncode == 0 and b"--- SKIP:" not in result.stdout,
                "baseline did not qualify; no production fault credit")
    try:
        for name, path, old, new, module, package, pattern, assertion in faults:
            original = snapshots[path]
            require(original.count(old.encode()) == 1, f"{name}: source anchor changed")
            (REPO / path).write_bytes(original.replace(old.encode(), new.encode(), 1))
            compiled = run(["go", "test", "-run", "^$", package], REPO / module)
            (scratch / f"{name}-compile.log").write_bytes(compiled.stdout)
            require(compiled.returncode == 0, f"{name}: INVALID compiler/setup fault")
            result = run(["go", "test", "-count=1", "-timeout=60s", "-v", "-run", pattern, package], REPO / module)
            (scratch / f"{name}-runtime.log").write_bytes(result.stdout)
            require(result.returncode != 0 and assertion.encode() in result.stdout
                    and b"--- SKIP:" not in result.stdout,
                    f"{name}: INVALID runtime fault or missing independent assertion")
            (REPO / path).write_bytes(original)
            print(f"KILLED: {name}; compiled; independent runtime assertion", flush=True)
    finally:
        for path, original in snapshots.items():
            (REPO / path).write_bytes(original)
        require(all((REPO / path).read_bytes() == original for path, original in snapshots.items()),
                "exact source restoration failed")
    print("PASS: provision production controls killed; exact bytes restored", flush=True)


if __name__ == "__main__":
    main()
