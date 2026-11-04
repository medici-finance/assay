#!/usr/bin/env python3
"""Explicit source-bound owned fixture build, actual flow/controls and cleanup.

Every endpoint/tool/cache/target is required. This never selects a default engine,
pulls an image, mounts host paths, imports an operational key or invokes a shell.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import uuid

def main():
    parser = argparse.ArgumentParser()
    for field in ["source", "work", "go", "python", "docker", "endpoint", "platform", "module-cache", "go-cache"]:
        parser.add_argument("--" + field, required=True)
    args = parser.parse_args()
    source = Path(args.source).resolve(strict=True)
    work = Path(args.work).resolve()
    go = Path(args.go).resolve(strict=True)
    python = Path(args.python).resolve(strict=True)
    # Preserve a selected CLI's invocation name (multicall binaries may dispatch
    # on argv0). Hash/read its resolved target, but never execute a renamed target.
    docker = Path(os.path.abspath(args.docker))
    docker_real = docker.resolve(strict=True)
    if not (source / ".git").is_file() or work.exists() or not args.endpoint.startswith("unix:///") or args.platform not in ["linux/amd64", "linux/arm64"]:
        raise SystemExit("UNQUALIFIED: isolated source/work/local endpoint/platform required")
    socket = Path(args.endpoint.removeprefix("unix://"))
    if not socket.is_absolute() or not stat.S_ISSOCK(socket.stat().st_mode):
        raise SystemExit("UNQUALIFIED: explicit local Unix socket required")
    for path in [go, python, docker, Path(args.module_cache), Path(args.go_cache)]:
        if not path.is_absolute():
            raise SystemExit("UNQUALIFIED: explicit absolute tool/cache required")
    work.mkdir(mode=0o700)
    cli = work / "docker-config"
    cli.mkdir(mode=0o700)
    (cli / "config.json").write_text("{}")
    (cli / "config.json").chmod(0o600)
    env = {"PATH": str(go.parent) + os.pathsep + str(python.parent) + os.pathsep + str(docker.parent) + os.pathsep + "/usr/bin:/bin", "GOENV": "off", "GOWORK": "off", "GOPROXY": "off", "GOTOOLCHAIN": "local", "GOMODCACHE": args.module_cache, "GOCACHE": args.go_cache, "GOTMPDIR": str(work), "KUBECONFIG": "/dev/null", "DOCKER_HOST": args.endpoint, "DOCKER_CONFIG": str(cli)}
    calls = []
    def run(label, argv, cwd=source, timeout=60, accept=False):
        try:
            result = subprocess.run(argv, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout)
        except subprocess.TimeoutExpired:
            calls.append({"label":label,"argv":[str(x) for x in argv],"state":"TIMEOUT_UNQUALIFIED"})
            (work / "CALLS.json").write_text(json.dumps(calls,indent=2)+"\n")
            raise RuntimeError("UNQUALIFIED: bounded call timeout: "+label) from None
        (work / (label + ".stdout")).write_bytes(result.stdout)
        (work / (label + ".stderr")).write_bytes(result.stderr)
        calls.append({"label": label, "argv": [str(x) for x in argv], "exit": result.returncode})
        (work / "CALLS.json").write_text(json.dumps(calls, indent=2) + "\n")
        if result.returncode and not accept:
            raise RuntimeError("UNQUALIFIED: " + label)
        return result
    head = run("source", ["git", "rev-parse", "HEAD"]).stdout.decode().strip()
    if len(head) != 40 or run("clean", ["git", "status", "--porcelain"]).stdout.strip():
        raise SystemExit("UNQUALIFIED: immutable clean linked source required")
    digest = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
    owner = "authority-fixture-" + uuid.uuid4().hex[:16]
    recipe = source / "tools/desk/internal/forgeservice/auth/testdata/runtime"
    context = work / "context"
    context.mkdir()
    shutil.copyfile(recipe / "Dockerfile", context / "Dockerfile")
    shutil.copytree(recipe / "seeds", context / "seeds")
    declared = {"source": head, "owner": owner, "endpoint": args.endpoint, "platform": args.platform, "go_sha256": digest(go), "python_sha256":digest(python), "docker_invocation":str(docker), "docker_realpath":str(docker_real), "docker_sha256": digest(docker_real), "recipe_sha256": digest(context / "Dockerfile"), "state": "DECLARED_NO_ENGINE_EFFECTS"}
    (work / "DECLARATION.json").write_text(json.dumps(declared, indent=2) + "\n")
    print("Owned fixture target: " + args.endpoint, flush=True)
    env.update(CGO_ENABLED="0", GOOS="linux", GOARCH=args.platform.split("/")[1])
    run("compile", [str(go), "build", "-trimpath", "-buildvcs=false", "-o", str(context / "authority-helper"), "./internal/forgeservice/auth/testdata/runtime"], source / "tools/desk", 180)
    # Host tests below must execute on their host platform, not the helper target.
    del env["GOOS"], env["GOARCH"], env["CGO_ENABLED"]
    declared["binary_sha256"] = digest(context / "authority-helper")
    declared["context_sha256"] = {str(p.relative_to(context)): digest(p) for p in sorted(context.rglob("*")) if p.is_file()}
    declared["state"] = "SOURCE_BINARY_RECIPE_DECLARED"
    (work / "DECLARATION.json").write_text(json.dumps(declared, indent=2) + "\n")
    prefix = [str(docker), "--config", str(cli), "--host", args.endpoint]
    def dc(label, argv, **kwargs):
        return run(label, prefix + argv, **kwargs)
    engine = json.loads(dc("engine", ["info", "--format", "{{json .}}"]).stdout)
    architecture = {"aarch64": "arm64", "arm64": "arm64", "x86_64": "amd64", "amd64": "amd64"}.get(engine["Architecture"])
    if engine["OSType"] != "linux" or "linux/" + str(architecture) != args.platform:
        raise RuntimeError("UNQUALIFIED: selected engine platform differs")
    tag = "authority-fixture:" + owner
    image = ""
    packet = None
    cleanup = []
    qualification = None
    try:
        dc("build", ["buildx", "build", "--builder=default", "--load", "--provenance=false", "--network=none", "--pull=false", "--no-cache", "--platform=" + args.platform, "--label", "io.assay.authority.fixture=" + owner, "--iidfile", str(work / "image-id"), "-t", tag, str(context)], timeout=180)
        image = (work / "image-id").read_text().strip()
        row = json.loads(dc("image", ["image", "inspect", image]).stdout)[0]
        if row["Id"] != image or row["Config"]["Labels"].get("io.assay.authority.fixture") != owner:
            raise RuntimeError("UNQUALIFIED: source-bound image ownership mismatch")
        packet = {"DockerBinary": str(docker), "DockerConfig": str(cli), "Host": args.endpoint, "Engine": engine["ID"], "Image": image, "Platform": args.platform, "Owner": "auth57-" + owner.removeprefix("authority-fixture-"), "Cell": "auth57-" + owner.removeprefix("authority-fixture-")}
        packet_path = work / "CUSTODY-INPUT.json"
        packet_path.write_text(json.dumps(packet))
        env["AUTH57_CUSTODY_INPUT"] = str(packet_path)
        flow = run("actual-flow", [str(go), "test", "-race", "-count=1", "-timeout=5m", "-v", "-run", "^TestForgeLocalAuthorityCustody$", "./internal/forgeservice/auth"], source / "tools/desk", 330)
        text = flow.stdout.decode()
        if "--- PASS: TestForgeLocalAuthorityCustody " not in text or "--- SKIP:" in text:
            raise RuntimeError("UNQUALIFIED: actual named custody PASS required")
        controls = run("production-controls", [str(python), "tools/desk/internal/forgeservice/auth/testdata/local_authority_mutations.py"], timeout=900)
        if b"PASS: local authority production controls killed; exact bytes restored" not in controls.stdout:
            raise RuntimeError("UNQUALIFIED: compiling production controls required")
        if run("final-clean", ["git", "status", "--porcelain"]).stdout.strip() or run("final-head",["git","rev-parse","HEAD"]).stdout.decode().strip()!=head:
            raise RuntimeError("UNQUALIFIED: original source checkout changed")
        qualification={**declared, "state": "ACTUAL_SOURCE_BOUND_FLOW_AND_PRODUCTION_CONTROLS_PASS", "engine": engine["ID"], "image": image, "limits": "Disposable local fixture only; no operational issuer/key/TTL/deploy/production restore or56 served SQL/native acceptance."}
    finally:
        # Runtime tests own exact labelled container/volume cleanup. Never force
        # an image or remove an unrelated resource if a failing test left residue.
        current=dc("cleanup-engine",["info","--format","{{.ID}}"])
        if current.stdout.decode().strip()!=engine["ID"]:raise RuntimeError("HOLD: selected engine changed")
        if packet:
            resources=[("container","assay-authority-"+packet["Cell"]+"-"+suffix) for suffix in ["producer","caller-caller-a","caller-caller-b","query-query","sibling-sibling"]]
            resources += [("volume",packet["Owner"]+"-"+suffix) for suffix in ["private","current","bootstrap","grant-a","grant-b"]]
            for index,(kind,name) in enumerate(resources):
                absent=dc("cleanup-resource-"+str(index),[kind,"inspect",name],accept=True)
                if absent.returncode==0 or b"no such" not in absent.stderr.lower():raise RuntimeError("HOLD: exact owned fixture residue or uncertain absence")
                cleanup.append({"kind":kind,"name":name,"absent":True})
        if not image:
            candidate=dc("cleanup-unacknowledged-tag",["image","inspect",tag],accept=True)
            if candidate.returncode==0:
                row=json.loads(candidate.stdout)[0]
                if row["Config"]["Labels"].get("io.assay.authority.fixture")!=owner:raise RuntimeError("HOLD: unacknowledged image ownership mismatch")
                image=row["Id"]
            elif b"no such" not in candidate.stderr.lower():raise RuntimeError("HOLD: image residue could not be checked")
        if image:
            row = json.loads(dc("cleanup-image-owner", ["image", "inspect", image]).stdout)[0]
            if row["Id"] != image or row["Config"]["Labels"].get("io.assay.authority.fixture") != owner:
                raise RuntimeError("HOLD: image ownership changed")
            dc("cleanup-image", ["image", "rm", image])
            absent=dc("cleanup-image-absence",["image","inspect",image],accept=True)
            if absent.returncode==0 or b"no such" not in absent.stderr.lower():raise RuntimeError("HOLD: image absence uncertain")
            cleanup.append({"kind":"image","id":image,"absent":True})
        (work / "CLEANUP.json").write_text(json.dumps({"image": image, "image_removed": bool(image), "resources":cleanup}, indent=2) + "\n")
    if qualification is None or len(cleanup)!=11:raise RuntimeError("UNQUALIFIED: complete flow/control/cleanup required")
    qualification["cleanup"]=cleanup
    (work / "QUALIFICATION.json").write_text(json.dumps(qualification,indent=2)+"\n")
    print("PASS: actual source-bound local authority custody and production controls; cleanup complete")

if __name__ == "__main__":
    main()
