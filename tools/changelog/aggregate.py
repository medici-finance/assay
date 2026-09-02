#!/usr/bin/env python3
"""Changelog-fragment aggregation — the release-time engine.

A notable change is recorded as one fragment file per PR under ``changelog/``
(``changelog/<slug>.md``), not as a hand-edit of ``CHANGELOG.md``'s
``## Unreleased`` section. That kills the standing merge-conflict class where
every concurrent PR edited the same shared section. This module is the
release-time half of that convention: it AGGREGATES the fragments (sorted,
deduped) into the release body's highlights and into a dated ``CHANGELOG.md``
section, and it is also the shared parser the PR-gate check reuses.

Subcommands
-----------
unreleased-bullets <changelog>
    Print, one per line, the normalized bullet lines under ``## Unreleased`` in
    <changelog>. Used by the PR-gate deprecation guard (check.sh) to detect a
    PR that ADDS an ``## Unreleased`` bullet — a retired path.

highlights <fragment-dir> <changelog>
    Print the aggregated highlights as Keep-a-Changelog ``### <bucket>`` blocks.
    Sources: every ``<fragment-dir>/*.md`` except ``README.md``, PLUS any
    residual ``## Unreleased`` bullets still in <changelog> (the cutover fold —
    see below). Within each bucket the bullets are sorted and de-duplicated.
    Exit 2 with a clear message when there is nothing to aggregate — the
    empty-fragments refusal, consistent with the retired empty-``## Unreleased``
    refusal.

roll <fragment-dir> <changelog> <tag> <date>
    Rewrite <changelog> in place: replace the ``## Unreleased`` section body
    with the standing pointer note, and insert a fresh ``## <tag> — <date>``
    section carrying the aggregated highlights directly beneath it. Exit 2 (and
    leave the file untouched) when there is nothing to aggregate. The CALLER is
    responsible for ``git rm``-ing the aggregated fragment files in the same
    commit; this module only rewrites the changelog.

The cutover fold
----------------
``## Unreleased`` is retired: the PR-gate check refuses a PR that adds a bullet
there. But a residual bullet may already sit in the section at cutover (the last
grandfathered direct edit). ``highlights``/``roll`` therefore ALSO fold any
bullets still under ``## Unreleased`` into the aggregate, so the pending entries
that existed at cutover land in the first aggregated release rather than being
stranded. Going forward the section stays empty (the guard keeps it so), the
fold contributes nothing, and fragments are the only source.

Buckets are the Keep-a-Changelog trio (Added / Fixed / Changed); a bullet with
no explicit ``### <bucket>`` heading above it defaults to Changed. Emission order
is Added, Fixed, Changed — matching the existing CHANGELOG.md sections.
"""

import os
import re
import sys

# Canonical bucket order for emission (matches the existing CHANGELOG.md).
BUCKETS = ("Added", "Fixed", "Changed")
_BUCKET_LOWER = {b.lower(): b for b in BUCKETS}
DEFAULT_BUCKET = "Changed"

_BULLET_RE = re.compile(r"^\s*-\s+\S")
_HEADING_RE = re.compile(r"^\s*###\s+(.+?)\s*$")
_SECTION_RE = re.compile(r"^##\s+")

# The standing note left in the (now always-empty) ## Unreleased section after a
# roll. Prose, never a bullet — so neither the aggregator nor the deprecation
# guard ever mistakes it for a highlight.
POINTER_LINES = [
    "Pending notable changes are recorded as one-file-per-PR fragments under",
    "`changelog/` (see `changelog/README.md`), aggregated into a dated section",
    "here at release time. This section is written only by the release workflow;",
    "do not add highlight bullets to it directly.",
]


def _canonical_bucket(name):
    return _BUCKET_LOWER.get(name.strip().lower())


def _read_lines(path):
    with open(path, encoding="utf-8") as fh:
        return fh.read().splitlines()


def _parse_bullets(lines, start_bucket=DEFAULT_BUCKET):
    """Parse a run of markdown lines into (bucket, entry) pairs.

    A ``### Added/Fixed/Changed`` heading switches the current bucket; a
    ``- `` line opens a new entry; an indented, non-bullet, non-heading line is
    a continuation of the current entry (a multi-line highlight). Returns a list
    of (bucket, entry-text) in source order.
    """
    out = []
    bucket = start_bucket
    current = None  # index into out of the open entry, or None
    for ln in lines:
        heading = _HEADING_RE.match(ln)
        if heading:
            canon = _canonical_bucket(heading.group(1))
            if canon:
                bucket = canon
                current = None
                continue
            # A non-bucket ### heading closes any open entry; ignore the line.
            current = None
            continue
        if _SECTION_RE.match(ln):
            # A new ## section ends this run's relevance for a caller that passed
            # a whole file; parsers that slice first won't hit this.
            current = None
            continue
        if _BULLET_RE.match(ln):
            out.append([bucket, ln.strip()])
            current = len(out) - 1
            continue
        if current is not None and ln.strip() and (ln.startswith(" ") or ln.startswith("\t")):
            # Continuation line of the open multi-line entry.
            out[current][1] = out[current][1] + "\n  " + ln.strip()
            continue
        # Blank or unrelated line: close any open entry.
        current = None
    return [(b, e) for b, e in out]


def _unreleased_slice(lines):
    """Return the lines strictly inside the ## Unreleased section (excluding the
    heading), or [] when there is no such section."""
    out = []
    capturing = False
    for ln in lines:
        if not capturing and ln.strip().lower() == "## unreleased":
            capturing = True
            continue
        if capturing and _SECTION_RE.match(ln):
            break
        if capturing:
            out.append(ln)
    return out


def unreleased_bullets(changelog_path):
    """The normalized bullet ENTRIES under ## Unreleased (bucket-agnostic)."""
    if not os.path.exists(changelog_path):
        return []
    sl = _unreleased_slice(_read_lines(changelog_path))
    return [entry for _bucket, entry in _parse_bullets(sl)]


def _collect(fragment_dir, changelog_path):
    """Gather (bucket -> sorted unique entries) from every fragment plus the
    residual ## Unreleased fold. Returns an ordered dict-like {bucket: [entries]}
    restricted to non-empty buckets, and a flat count."""
    buckets = {b: [] for b in BUCKETS}

    # 1) Fragment files, in filename-sorted order for determinism.
    if os.path.isdir(fragment_dir):
        names = sorted(
            n for n in os.listdir(fragment_dir)
            if n.endswith(".md") and n != "README.md"
        )
        for name in names:
            path = os.path.join(fragment_dir, name)
            for bucket, entry in _parse_bullets(_read_lines(path)):
                buckets[bucket].append(entry)

    # 2) The cutover fold — residual ## Unreleased bullets, keeping their bucket.
    if os.path.exists(changelog_path):
        sl = _unreleased_slice(_read_lines(changelog_path))
        for bucket, entry in _parse_bullets(sl):
            buckets[bucket].append(entry)

    # Sort + dedupe within each bucket (case-sensitive exact-entry dedupe).
    result = {}
    total = 0
    for b in BUCKETS:
        uniq = sorted(dict.fromkeys(buckets[b]))
        if uniq:
            result[b] = uniq
            total += len(uniq)
    return result, total


def _render(collected):
    """Render {bucket: [entries]} as Keep-a-Changelog markdown blocks."""
    blocks = []
    for b in BUCKETS:
        entries = collected.get(b)
        if not entries:
            continue
        block = ["### " + b]
        block.extend(entries)
        blocks.append("\n".join(block))
    return "\n\n".join(blocks)


def _refuse_empty():
    sys.stderr.write(
        "changelog: nothing to aggregate — no fragment files under the "
        "changelog/ directory and no residual '## Unreleased' bullets. A "
        "release must carry descriptive highlights; add a fragment "
        "(changelog/<slug>.md) before cutting, or the release is refused "
        "(consistent with the retired empty-'## Unreleased' refusal).\n"
    )
    return 2


def cmd_unreleased_bullets(changelog_path):
    for entry in unreleased_bullets(changelog_path):
        # entries may be multi-line; print the first line as the identity key
        # callers compare on, but emit the whole entry so a diff is legible.
        sys.stdout.write(entry.replace("\n", " ") + "\n")
    return 0


def cmd_highlights(fragment_dir, changelog_path):
    collected, total = _collect(fragment_dir, changelog_path)
    if total == 0:
        return _refuse_empty()
    sys.stdout.write(_render(collected) + "\n")
    return 0


def cmd_roll(fragment_dir, changelog_path, tag, date):
    collected, total = _collect(fragment_dir, changelog_path)
    if total == 0:
        return _refuse_empty()
    highlights = _render(collected)

    lines = _read_lines(changelog_path)
    # Locate the ## Unreleased heading.
    idx = None
    for i, ln in enumerate(lines):
        if ln.strip().lower() == "## unreleased":
            idx = i
            break
    if idx is None:
        sys.stderr.write(
            "changelog: no '## Unreleased' heading in %s — refusing to guess "
            "where the dated section goes.\n" % changelog_path
        )
        return 2
    # Find the next ## section after Unreleased (the previous top version).
    j = idx + 1
    while j < len(lines) and not _SECTION_RE.match(lines[j]):
        j += 1
    tail = lines[j:]  # from the previous top version section onward

    new_section = ["## %s — %s" % (tag, date), "", highlights]
    rebuilt = (
        lines[: idx + 1]
        + [""]
        + POINTER_LINES
        + [""]
        + new_section
        + [""]
        + tail
    )
    with open(changelog_path, "w", encoding="utf-8") as fh:
        fh.write("\n".join(rebuilt).rstrip("\n") + "\n")
    return 0


def main(argv):
    if len(argv) < 2:
        sys.stderr.write("usage: aggregate.py <unreleased-bullets|highlights|roll> ...\n")
        return 2
    cmd = argv[1]
    if cmd == "unreleased-bullets":
        if len(argv) != 3:
            sys.stderr.write("usage: aggregate.py unreleased-bullets <changelog>\n")
            return 2
        return cmd_unreleased_bullets(argv[2])
    if cmd == "highlights":
        if len(argv) != 4:
            sys.stderr.write("usage: aggregate.py highlights <fragment-dir> <changelog>\n")
            return 2
        return cmd_highlights(argv[2], argv[3])
    if cmd == "roll":
        if len(argv) != 6:
            sys.stderr.write("usage: aggregate.py roll <fragment-dir> <changelog> <tag> <date>\n")
            return 2
        return cmd_roll(argv[2], argv[3], argv[4], argv[5])
    sys.stderr.write("aggregate.py: unknown subcommand %r\n" % cmd)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
