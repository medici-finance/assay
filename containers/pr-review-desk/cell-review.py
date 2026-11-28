#!/usr/bin/env python3
"""Run selected, bounded review-desk passes for one configured GitHub cell."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import re
import signal
import sqlite3
import shutil
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timedelta, timezone


SLUG = re.compile(r"[a-z0-9_.-]+/[a-z0-9_.-]+")
SHA = re.compile(r"(?:[a-f0-9]{40}|[a-f0-9]{64})")
ROLE = "pr-review-desk"
SKILL = "/opt/assay/plugin/skills/pr-review-desk/SKILL.md"


def require(ok, reason):
    if not ok:
        raise ValueError(reason)


def load_profile(path):
    profile = json.loads(path.read_text())
    require(profile["schema"] == "cell-review-v1", "unsupported profile")
    require(re.fullmatch(r"[a-z0-9][a-z0-9-]*", profile["cell"]), "invalid cell name")
    repos = profile["repositories"]
    require(isinstance(repos, dict) and repos and all(SLUG.fullmatch(slug) and isinstance(root, str)
            and Path(root).is_absolute() for slug, root in repos.items()), "expected distinct GitHub repository slugs")
    require(isinstance(profile["selection_policy"], dict), "missing operator selection policy")
    return profile


def observe(profile, folder, env):
    # Use the cell's write boundary (the same set deskboard collects), and
    # reject mis-mounted house configuration before making a forge request.
    require((Path(env["ASSAY_CONFIG_HOME"]) / "READY").is_file(), "cell provisioning receipt missing")
    roster = subprocess.check_output(["deskroster", "repos", "--scope", "write"],
                                     env=env, text=True, timeout=30)
    repos = [line.strip() for line in roster.splitlines() if line.strip() and not line.startswith("#")]
    require(sorted(repos) == sorted(profile["repositories"]), "cell roster differs from enrolled repositories")
    scope = [dict(provider="github", host="github.com", repository=s) for s in repos]
    snapshot = folder / "observation.json"
    print(f"Observing {profile['cell']}: {', '.join(repos)}", flush=True)
    subprocess.run(["deskboard", "review-queue", "--out", str(snapshot)], env=env, cwd=profile["repositories"][repos[0]],
                   check=True, timeout=300)
    observation = json.loads(snapshot.read_text())
    now = datetime.now(timezone.utc)
    selection = dict(Policy=profile["selection_policy"], Capacity=len(observation["inputs"]["changes"]),
                     ObservationPolicy=observation["inputs"]["policy"], Scope=scope,
                     Oldest=(now - timedelta(minutes=5)).isoformat())
    encoded = subprocess.check_output(["reviewloop", "plan", "--snapshot", str(snapshot),
        "--selection", json.dumps(selection), "--now", now.isoformat()], env=env, timeout=60)
    plan = json.loads(encoded)
    require(plan["Requested"] == scope and plan["Admitted"] == scope, "incomplete or foreign scope")
    return snapshot, plan


def review_keys(plan):
    for candidate in plan["Candidates"]:
        if candidate["Disposition"] != "selected":
            continue
        proposal = candidate["Proposal"]
        key = proposal["Key"]
        if key["Verb"] != "review":
            continue
        require(not proposal["Suppressed"] and key["Provider"] == "github" and key["Host"] == "github.com"
                and SLUG.fullmatch(key["Repo"]) and type(key["PR"]) is int and key["PR"] > 0
                and SHA.fullmatch(key["Head"]), "invalid selected review identity")
        yield key["Repo"], key["PR"], key["Head"]


def review_pass(key, root, snapshot, env, budget, lock_fd):
    repo, number, head = key
    # Only identifiers from the validated plan enter the prompt. Review content
    # stays in the private observation and in the desk's independently read kit.
    prompt = (f"Read {SKILL}. Use the cell-selected tick procedure. Run one bounded review-dispatch pass for {repo}#{number} at {head}. "
              f"Every deskdispatch must use --claim-root {env['ASSAY_CLAIM_ROOT']}. "
              f"The private review-queue observation for deskdispatch --observation is {snapshot}. "
              "Use the ordinary desk admission, lane, model stamp, independent reviewer, claim, "
              "finding continuity and verdict readback procedures. Recheck the head and stop if it moved. "
              "This pass owns only the selected review. Do not boot a new watcher, process other PRs, "
              "flip ready or merge. Finish or hand off the reviewer claims before exiting.")
    child_env = dict(env, ASSAY_TICK="1", ASSAY_TICK_DEADLINE=str(budget))
    argv = ["codex", "exec", "--cd", root, "--model", env["ASSAY_REVIEW_MODEL"],
            "-c", "features.goals=false", "-c", "features.shell_snapshot=false",
            "-c", "allow_login_shell=false", "-c", "shell_environment_policy.experimental_use_profile=false"]
    # As in cellctl, bind only non-secret shell context on the harness argv.
    for name in ("HOME", "PATH", "ASSAY_CONFIG_HOME", "CODEX_HOME", "ASSAY_CLAIM_ROOT",
                 "ASSAY_CELL", "DESK_CELL", "DESK_ROLE", "DESK_LOOP", "DESK_SESSION",
                 "DESK_ROOTS", "KUBECONFIG", "ASSAY_TICK", "ASSAY_TICK_DEADLINE"):
        argv.extend(["-c", f"shell_environment_policy.set.{name}={json.dumps(child_env[name])}"])
    argv.append("-")
    # Keep the lock if the controller dies while the harness is still running.
    try:
        child = subprocess.Popen(argv, stdin=subprocess.PIPE, text=True, env=child_env,
                                 start_new_session=True, pass_fds=(lock_fd,),
                                 stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    except OSError as exc:
        raise LaunchUnavailable("harness could not start") from exc
    try:
        child.communicate(prompt, timeout=budget)
    finally:
        if child.poll() is None:
            os.killpg(child.pid, signal.SIGKILL)
            child.wait()
    require(child.returncode == 0, f"review harness exited {child.returncode}")


class LaunchUnavailable(RuntimeError):
    """Popen confirmed no harness was started."""


def cycle(profile, state, db, env, budget, lock_fd, retry=None):
    with tempfile.TemporaryDirectory(dir=state) as temp:
        snapshot, plan = observe(profile, Path(temp), env)
        keys = list(review_keys(plan))
        print(f"Queue actions: {json.dumps(plan['Depths']['actions'], sort_keys=True)}; "
              f"selected reviews: {len(keys)}", flush=True)
        # A new complete plan can discharge old intent. Retain unresolved
        # intents while a change waits on CI or temporarily loses eligibility.
        # A later request at the same head may follow a confirmed verdict.
        changes = json.loads(snapshot.read_text())["inputs"]["changes"]
        pending = set(keys)
        observations = {}
        for change in changes:
            identity = change["identity"]
            observations[(identity["repository"]["repository"], identity["number"])] = change
        with db:
            for repo, pr, head in db.execute("SELECT repo,pr,head FROM attempts").fetchall():
                change = observations.get((repo, pr))
                if (repo, pr, head) in pending:
                    continue
                if (change is None or head != change["identity"]["head"]
                        or change["facts"]["reviews"]["AtHead"]):
                    db.execute("DELETE FROM attempts WHERE repo=? AND pr=? AND head=?", (repo, pr, head))
        if retry:
            require(any(f"{r}#{n}@{h}" == retry for r, n, h in keys),
                    "retry target is no longer selected at this head")
        held = False
        for key in keys:
            target = f"{key[0]}#{key[1]}@{key[2]}"
            if retry and retry != target:
                continue
            require(key[0] in profile["repositories"], "selected change is outside the cell")
            root = profile["repositories"][key[0]]
            origin = subprocess.check_output(["git", "-C", root, "remote", "get-url", "origin"],
                                             text=True, timeout=15, env=env).strip()
            match = re.fullmatch(r"(?:https://github\.com(?::443)?/|ssh://git@github\.com/|git@github\.com:)([^?#]+)", origin)
            require(match and match[1].removesuffix(".git").lower() == key[0],
                    "checkout origin differs from selected review")
            require(shutil.which("codex", path=env["PATH"]), "configured Codex executable unavailable")
            # Commit intent before spawning. A lost spawn acknowledgement or a
            # failed pass cannot start the same review twice after a restart.
            with db:
                added = db.execute("INSERT OR IGNORE INTO attempts VALUES (?, ?, ?)", key).rowcount
            if retry:
                if added:
                    with db:
                        db.execute("DELETE FROM attempts WHERE repo=? AND pr=? AND head=?", key)
                    raise ValueError("retry target has no outstanding attempt")
            elif not added:
                held = True
                print(f"Outstanding review already attempted: {key[0]}#{key[1]}@{key[2]}", flush=True)
                continue
            print(f"Dispatching {key[0]}#{key[1]}@{key[2]}", flush=True)
            try:
                review_pass(key, root, snapshot, env, budget, lock_fd)
            except LaunchUnavailable as error:
                if added:
                    with db:
                        db.execute("DELETE FROM attempts WHERE repo=? AND pr=? AND head=?", key)
                raise ValueError(str(error)) from error
            # A process exit is not a verdict. Recollect before selecting more work.
            return 0
        return 2 if held else 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--profile", type=Path, required=True)
    parser.add_argument("--state-dir", type=Path, required=True)
    parser.add_argument("--allow-forge-reads", action="store_true", required=True)
    parser.add_argument("--once", action="store_true")
    parser.add_argument("--interval", type=int, default=120)
    parser.add_argument("--budget", type=int, default=1200)
    parser.add_argument("--retry-held", metavar="OWNER/REPO#PR@HEAD",
                        help="operator reconciled the outstanding attempt and its claims; retry once")
    args = parser.parse_args()
    require(not args.retry_held or args.once, "--retry-held requires --once")
    require(args.interval > 0 and args.budget >= 61, "invalid interval or pass budget")
    profile = load_profile(args.profile)
    cell = profile["cell"]
    env = os.environ.copy()
    require(env.get("ASSAY_DESK") == ROLE and env.get("ASSAY_CELL") == cell,
            "profile differs from the provisioned review role or cell")
    require(env.get("ASSAY_REVIEW_MODEL") and Path(env["ASSAY_CONFIG_HOME"]).is_absolute()
            and (Path(env["ASSAY_CONFIG_HOME"]) / "roster.env").is_file(), "cell roster and model required")
    require(Path(env["CODEX_HOME"]).is_absolute(), "cell-owned CODEX_HOME required")
    require((Path(env["ASSAY_CONFIG_HOME"]) / "READY").is_file(), "cell provisioning receipt missing")
    require((Path(env["CODEX_HOME"]) / "auth.json").is_file() or env.get("CODEX_API_KEY"),
            "cell Codex credential missing")
    require(Path(env["ASSAY_CLAIM_ROOT"]).is_absolute() and Path(env["ASSAY_CLAIM_ROOT"]).is_dir(),
            "cell claim helper root required")
    env["DESK_ROOTS"] = ",".join(f"{repo}={root}" for repo, root in profile["repositories"].items())
    for name in ("GH_TOKEN", "GITHUB_TOKEN"):
        env.pop(name, None)
    env.update(DESK_LOOP=ROLE, DESK_ROLE=ROLE, DESK_CELL=cell, ASSAY_CELL_ROLE=ROLE, KUBECONFIG="/dev/null")
    env["DESK_SESSION"] = f"{cell}-review-{os.getpid()}-{time.time_ns()}"
    os.umask(0o077)
    require(args.state_dir.is_absolute(), "persistent state requires an absolute path")
    state = args.state_dir / cell / ROLE
    state.mkdir(parents=True, exist_ok=True)
    with (state / "owner.lock").open("a") as owner:
        fcntl.flock(owner, fcntl.LOCK_EX | fcntl.LOCK_NB)
        db = sqlite3.connect(state / "launches.sqlite3")
        db.execute("CREATE TABLE IF NOT EXISTS attempts (repo TEXT, pr INTEGER, head TEXT, PRIMARY KEY(repo,pr,head))")
        while True:
            try:
                result = cycle(profile, state, db, env, args.budget, owner.fileno(), args.retry_held)
            except (ValueError, KeyError, OSError, subprocess.SubprocessError) as error:
                print(f"Review cycle held: {error}", file=sys.stderr, flush=True)
                if args.once:
                    return 1
            if args.once:
                return result
            time.sleep(args.interval)


if __name__ == "__main__":
    signal.signal(signal.SIGTERM, lambda *_: sys.exit(143))
    try:
        sys.exit(main())
    except (KeyError, ValueError, OSError, sqlite3.Error) as error:
        sys.exit(f"Review service held: {error}")
