#!/usr/bin/env python3
"""Independent implementation of spec/project-obligations-v1.md section 3.

Written from the spec text, not from statusgen/projectobligations.go, so the
golden digests in projectobligations_test.go and the subject digests in
obligations.json (and the decision records under tree/) are not computed by
the code under test. Not run by CI; re-run by hand when the fixture changes:

    python3 -I canonical_digest.py obligations.json

Prints the source content digests, every mapping digest and every decision's
subject digest.
"""
import hashlib
import json
import sys


def net(s):
    b = s.encode("utf-8")
    return str(len(b)).encode("ascii") + b":" + b + b","


def net_list(xs):
    return net(b"".join(net(x) for x in sorted(xs, key=lambda x: x.encode("utf-8"))).decode("utf-8"))


def digest(parts):
    return "sha256:" + hashlib.sha256(b"".join(parts)).hexdigest()


def mapping_digest(m):
    ctx = m.get("context") or {}
    return digest([
        net("project-obligations-v1/mapping"),
        net(m["id"]), net(str(m["revision"])),
        net(m["source"]["id"]), net(m["source"]["revision"]),
        net(m["clause"]), net(m["paraphrase"]),
        net_list(m.get("reqs") or []),
        net(m["profile"]["id"]), net(m["profile"]["revision"]),
        net(ctx.get("entity", "")), net(ctx.get("activity", "")), net(ctx.get("jurisdiction", "")),
        net_list(m.get("controls") or []),
        net(m["owner"]),
    ])


def subject_digest(src, m, d):
    return digest([
        net("project-obligations-v1/subject"),
        net(src["id"]), net(src["revision"]), net(src.get("contentDigest", "")),
        net(m["id"]), net(str(m["revision"])), net(mapping_digest(m)),
        net(m["profile"]["id"]), net(m["profile"]["revision"]),
        net(d["id"]), net(d["outcome"]), net(d.get("reason", "")), net(d["scope"]),
        net(d["effectiveFrom"]), net(d.get("effectiveTo", "")), net(d["decidedAt"]),
        net(d["proposedBy"]), net(d["reviewer"]), net(d.get("supersedes", "")),
    ])


def main(path):
    with open(path, encoding="utf-8") as f:
        doc = json.load(f)
    srcs = {(s["id"], s["revision"]): s for s in doc["sources"]}
    maps = {(m["id"], m["revision"]): m for m in doc["mappings"]}
    for s in doc["sources"]:
        if s.get("text"):
            print("source", s["id"], "content", "sha256:" + hashlib.sha256(s["text"].encode("utf-8")).hexdigest())
    for m in doc["mappings"]:
        print("mapping", m["id"], m["revision"], mapping_digest(m))
    for d in doc["decisions"]:
        m = maps[(d["mappingId"], d["mappingRevision"])]
        src = srcs[(m["source"]["id"], m["source"]["revision"])]
        print("subject", d["id"], subject_digest(src, m, d))


if __name__ == "__main__":
    main(sys.argv[1])
