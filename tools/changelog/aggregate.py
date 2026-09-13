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

credits <fragment-dir>
    Print, one line per fragment, ``<fragment><TAB><pull-request-number>`` — the
    pull request each fragment arrived on, resolved from GIT ALONE (the adding
    commit, the merge commit that brought it to this history, and the number in
    that commit's subject). A fragment that cannot be resolved CONFIDENTLY
    prints ``<fragment><TAB>unresolved`` rather than being omitted, so a reader
    can tell "no pull request found" from "not looked at". Always exits 0: a
    credit must never be able to fail a release.

    This is the GIT half of external-contributor credit. The FORGE half — the
    pull request's author login, whether that identity is external to the
    operator's roster, and whether the body carries the opt-out marker — needs
    the forge and lives in the release workflow step, which turns this output
    into a credits MAP file. This module never makes a network call.

highlights/roll ... --credits <map-file>
    Optional. <map-file> carries ``<fragment><TAB>@<login>`` lines (``#``
    comments and blank lines ignored). Every bullet of a credited fragment gains
    the credit suffix (default ``— thanks @<login>``) on its bullet line;
    continuation lines of a multi-line highlight are untouched. A fragment with
    no line, a line whose value is not an ``@login`` (``unresolved``,
    ``opt-out``), or a missing map file credits nobody — and then the output is
    BYTE-IDENTICAL to the same run without ``--credits``.

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

# The credit suffix appended to each bullet of a credited fragment. A CONFIGURED
# form, deliberately a module constant an adopter edits in one place: the wording
# of a thank-you is a house's own voice, not this engine's business.
CREDIT_SUFFIX = " \u2014 thanks %s"

# An explicit per-fragment marker, never an omission — "no pull request found"
# must be distinguishable from "not looked at".
UNRESOLVED = "unresolved"

# The merge-commit subject GitHub writes for a merge-commit landing, and the
# trailing `(#N)` of a squash landing. Both are anchored and require the number
# to be the WHOLE token: a loose parse is how a credit lands on the wrong person.
_MERGE_PR_RE = re.compile(r"^Merge pull request #(\d+) ")
_SQUASH_PR_RE = re.compile(r"\(#(\d+)\)\s*$")

# A credit value is honoured only in the `@login` form. GitHub logins are
# alphanumeric with single internal hyphens; anything else is not a login and is
# treated as "credit nobody".
_LOGIN_RE = re.compile(r"^@[A-Za-z0-9](?:[A-Za-z0-9]|-(?=[A-Za-z0-9])){0,38}$")

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


def _shq(value):
    """POSIX single-quote one argument for the shell git is run through.

    The module reaches git via os.popen (no new dependency — os is already the
    filesystem surface this engine stands on, and the offline unit tests stay
    offline because git is local). Every interpolated value goes through here:
    a fragment filename is attacker-influenced on a fork path, and an unquoted
    one would be a command-injection seam in a release job.
    """
    return "'" + str(value).replace("'", "'\\''") + "'"


def _git(repo, args):
    """Run `git -C <repo> <args>` and return stdout, or None when git failed.

    Failure is never fatal here: an unresolvable credit is a missing name, never
    a refused release.
    """
    cmd = "git -C %s %s 2>/dev/null" % (_shq(repo), args)
    try:
        with os.popen(cmd) as fh:
            return fh.read()
    except OSError:
        return None


def _pr_from_subject(subject):
    """The pull-request number in a landing commit's subject, or None.

    Two landing shapes, both anchored: a merge commit's `Merge pull request #N
    from ...`, and a squash commit's trailing `(#N)`. Anything else resolves to
    nothing — a number found loosely somewhere in a subject is exactly how a
    credit lands on the wrong person.
    """
    if not subject:
        return None
    m = _MERGE_PR_RE.match(subject)
    if m:
        return m.group(1)
    m = _SQUASH_PR_RE.search(subject)
    if m:
        return m.group(1)
    return None


def _git_ok(repo, args):
    """True when `git -C <repo> <args>` exited 0. Used for predicate commands
    (`merge-base --is-ancestor`) where the ANSWER is the exit status, not the
    output — `_git` deliberately hides the status, so it cannot answer this."""
    cmd = "git -C %s %s >/dev/null 2>&1" % (_shq(repo), args)
    try:
        with os.popen(cmd) as fh:
            fh.read()
            return fh.close() is None
    except OSError:
        return False


def _resolve_fragment_pr(repo, relpath):
    """fragment path -> the pull-request number it arrived on, or None.

    The chain, in the order a landing actually happens:

    1. The commit that ADDED the path (the OLDEST such commit, so an
       added/deleted/re-added path still names the landing it arrived on).
    2. If that commit's OWN subject carries the number — a squash landing's
       trailing ``(#N)``, or a merge commit's ``Merge pull request #N`` — the
       adding commit IS the landing commit and that is the answer.
    3. Otherwise the fragment arrived on a side branch, so walk the FIRST-PARENT
       line from there to HEAD and take the FIRST merge whose SECOND parent
       actually contains the adding commit — the merge that brought it in.
    4. Otherwise None.

    Step 2 before step 3, and the second-parent containment test in step 3, are
    both load-bearing. Reading the merges first credited a squash-landed
    fragment to whatever unrelated pull request merged next, and a merge that
    merely SITS above the adding commit on the ancestry path did not bring it in
    — that is how unrelated fragments all collapsed onto the same few recent
    pull-request numbers.

    NOTE the depth requirement. On a shallow clone `git log` cannot reach the
    adding commit, so every fragment resolves to None and nobody is credited.
    The release workflow's checkout sets `fetch-depth: 0` for exactly this
    reason; see tools/changelog/README.md.
    """
    adds = _git(repo, "log --diff-filter=A --format=%%H -- %s" % _shq(relpath))
    if adds is None:
        return None
    shas = [ln.strip() for ln in adds.splitlines() if ln.strip()]
    if not shas:
        return None
    # `git log` prints newest-first, so the LAST line is the oldest adding
    # commit — the one the fragment first arrived on.
    adding = shas[-1]

    own = _git(repo, "log -1 --format=%%s %s" % _shq(adding))
    if own:
        pr = _pr_from_subject(own.strip())
        if pr:
            return pr

    merges = _git(
        repo,
        "log --first-parent --ancestry-path --merges --reverse "
        "--format=%%H%%x09%%s %s..HEAD" % _shq(adding),
    )
    for ln in (merges or "").splitlines():
        if not ln.strip():
            continue
        sha, _tab, subject = ln.partition("\t")
        sha = sha.strip()
        if not sha:
            continue
        if not _git_ok(
            repo,
            "merge-base --is-ancestor %s %s" % (_shq(adding), _shq(sha + "^2")),
        ):
            continue
        pr = _pr_from_subject(subject.strip())
        if pr:
            return pr
    return None


def _fragment_names(fragment_dir):
    """The fragment filenames in <fragment-dir>, in the same sorted, README-
    excluding order _collect reads them."""
    if not os.path.isdir(fragment_dir):
        return []
    return sorted(
        n for n in os.listdir(fragment_dir)
        if n.endswith(".md") and n != "README.md"
    )


def load_credits(path):
    """Read a credits MAP file into {fragment-filename: credit-suffix}.

    Lines are `<fragment><TAB>@<login>`; `#` comments and blank lines are
    ignored, and so is any value that is not an `@login` (`unresolved`,
    `opt-out`, an empty field). A missing or unreadable file is an EMPTY map,
    not an error — a credit must never be able to fail a release.
    """
    out = {}
    if not path:
        return out
    try:
        lines = _read_lines(path)
    except (OSError, UnicodeDecodeError):
        return out
    for ln in lines:
        if not ln.strip() or ln.lstrip().startswith("#"):
            continue
        parts = ln.rstrip("\n").split("\t")
        if len(parts) < 2:
            continue
        name = parts[0].strip()
        value = parts[1].strip()
        if not name or not _LOGIN_RE.match(value):
            continue
        out[name] = CREDIT_SUFFIX % value
    return out


def _credit_entry(entry, suffix):
    """Append the credit suffix to the BULLET LINE of a (possibly multi-line)
    entry. Continuation lines of a multi-line highlight are untouched, and the
    suffix is never written into the middle of the bullet's own text."""
    head, sep, tail = entry.partition("\n")
    return head + suffix + sep + tail


def cmd_credits(fragment_dir):
    """Print `<fragment><TAB><pr|unresolved>` for every fragment. Always 0."""
    names = _fragment_names(fragment_dir)
    if not names:
        # A fragment directory that is missing, empty, or unreadable is itself an
        # explicit unresolved line rather than silence: "nothing to credit" and
        # "never looked" must not print the same thing.
        sys.stdout.write("%s\t%s\n" % (fragment_dir, UNRESOLVED))
        return 0
    root = _git(fragment_dir, "rev-parse --show-toplevel")
    root = root.strip() if root else ""
    for name in names:
        pr = None
        if root:
            abspath = os.path.abspath(os.path.join(fragment_dir, name))
            relpath = os.path.relpath(abspath, root)
            pr = _resolve_fragment_pr(root, relpath)
        sys.stdout.write("%s\t%s\n" % (name, pr if pr else UNRESOLVED))
    return 0


def _collect(fragment_dir, changelog_path, credits=None):
    """Gather (bucket -> sorted unique entries) from every fragment plus the
    residual ## Unreleased fold. Returns an ordered dict-like {bucket: [entries]}
    restricted to non-empty buckets, and a flat count."""
    buckets = {b: [] for b in BUCKETS}

    # 1) Fragment files, in filename-sorted order for determinism.
    credits = credits or {}
    for name in _fragment_names(fragment_dir):
        path = os.path.join(fragment_dir, name)
        suffix = credits.get(name)
        for bucket, entry in _parse_bullets(_read_lines(path)):
            if suffix:
                entry = _credit_entry(entry, suffix)
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


def cmd_highlights(fragment_dir, changelog_path, credits_path=None):
    collected, total = _collect(
        fragment_dir, changelog_path, load_credits(credits_path))
    if total == 0:
        return _refuse_empty()
    sys.stdout.write(_render(collected) + "\n")
    return 0


def cmd_roll(fragment_dir, changelog_path, tag, date, credits_path=None):
    collected, total = _collect(
        fragment_dir, changelog_path, load_credits(credits_path))
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


def _take_credits(args):
    """Strip an optional `--credits <file>` from a positional argument list.

    Returns (remaining-positionals, credits-path-or-None), or (None, None) when
    the flag is malformed — the caller then prints its usage.
    """
    rest = []
    credits_path = None
    i = 0
    while i < len(args):
        a = args[i]
        if a == "--credits":
            if i + 1 >= len(args):
                return None, None
            credits_path = args[i + 1]
            i += 2
            continue
        if a.startswith("--credits="):
            credits_path = a.split("=", 1)[1]
            i += 1
            continue
        rest.append(a)
        i += 1
    return rest, credits_path


def main(argv):
    if len(argv) < 2:
        sys.stderr.write(
            "usage: aggregate.py <unreleased-bullets|highlights|roll|credits> ...\n")
        return 2
    cmd = argv[1]
    args, credits_path = _take_credits(argv[2:])
    if cmd == "unreleased-bullets":
        if args is None or len(args) != 1:
            sys.stderr.write("usage: aggregate.py unreleased-bullets <changelog>\n")
            return 2
        return cmd_unreleased_bullets(args[0])
    if cmd == "credits":
        if args is None or len(args) != 1:
            sys.stderr.write("usage: aggregate.py credits <fragment-dir>\n")
            return 2
        return cmd_credits(args[0])
    if cmd == "highlights":
        if args is None or len(args) != 2:
            sys.stderr.write(
                "usage: aggregate.py highlights <fragment-dir> <changelog> "
                "[--credits <map-file>]\n")
            return 2
        return cmd_highlights(args[0], args[1], credits_path)
    if cmd == "roll":
        if args is None or len(args) != 4:
            sys.stderr.write(
                "usage: aggregate.py roll <fragment-dir> <changelog> <tag> <date> "
                "[--credits <map-file>]\n")
            return 2
        return cmd_roll(args[0], args[1], args[2], args[3], credits_path)
    sys.stderr.write("aggregate.py: unknown subcommand %r\n" % cmd)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
