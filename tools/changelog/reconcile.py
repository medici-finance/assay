#!/usr/bin/env python3
"""Recognize the narrow, non-notable board reconciliation class, offline.

Metadata comes from the runner's pull_request event, never commit authors, PR
prose or a checked-out file. The complete immutable Git diff must also qualify.
This classifies release-note relevance; it does not verify lifecycle witnesses.
An unreadable or unsupported shape retains the normal changelog requirement.
"""
import json
import os
import re
import subprocess
import sys

BEGIN = '<!-- statusgen:briefs:begin -->'
END = '<!-- statusgen:briefs:end -->'
HEADER = '| # | Brief | Wave | Effort | Status | Verified | Reviewed |'
SEPARATOR = '|---|-------|------|--------|--------|----------|----------|'
STATES = {'todo', 'in-progress', 'implemented', 'verified', 'done', 'blocked'}


def git(*args):
    return subprocess.check_output(['git', *args], stderr=subprocess.PIPE)


def normalized(raw):
    """Erase only well-formed Status cells; preserve every other byte."""
    text = raw.decode('utf-8')
    if '\x00' in text or text.count(BEGIN) != 1 or text.count(END) != 1:
        raise ValueError('missing or ambiguous generated table markers')
    prefix, rest = text.split(BEGIN)
    region, suffix = rest.split(END)
    # Deliberately accept the producer's canonical format only. Unknown table
    # shapes retain the normal requirement instead of guessing which cell moved.
    frontmatter = prefix.split('---\n', 2)
    if (len(frontmatter) != 3 or frontmatter[0] != ''
            or frontmatter[1].splitlines().count('board: generated') != 1):
        raise ValueError('not an existing generated board')
    lines = region.splitlines(keepends=True)
    if len(lines) < 4 or lines[:3] != ['\n', HEADER+'\n', SEPARATOR+'\n']:
        raise ValueError('unsupported generated table shape')
    seen = set()
    for index in range(3, len(lines)):
        line = lines[index]
        cells = re.split(r'(?<!\\)\|', line)
        if len(cells) != 9 or cells[0] != '' or cells[-1] != '\n':
            raise ValueError('not a seven-column generated row')
        number = cells[1].strip()
        if not re.fullmatch(r'[0-9]+', number) or number in seen:
            raise ValueError('invalid or duplicate brief row')
        seen.add(number)
        if cells[5] != ' '+cells[5].strip()+' ' or cells[5].strip() not in STATES:
            raise ValueError('unsupported Status value')
        cells[5] = ' <status> '
        lines[index] = '|'.join(cells)
    return prefix + BEGIN + ''.join(lines) + END + suffix


def classify():
    if os.environ.get('GITHUB_EVENT_NAME') != 'pull_request':
        return False
    with open(os.environ['GITHUB_EVENT_PATH'], encoding='utf-8') as source:
        event = json.load(source)
    pr = event['pull_request']
    repo = os.environ['GITHUB_REPOSITORY']
    author, base, head = pr['user'], pr['base'], pr['head']
    if (author['login'] != 'assay-board-writer[bot]' or author['type'] != 'Bot'
            or author['id'] != 319272446):
        return False
    if (event['repository']['full_name'] != repo or base['repo']['full_name'] != repo
            or head['repo']['full_name'] != repo or base['ref'] != 'main'
            or head['ref'] != 'board/reconcile'):
        return False
    base_sha, head_sha = os.environ['BASE_SHA'], os.environ['HEAD_SHA']
    if (not re.fullmatch('[0-9a-f]{40}', base_sha) or not re.fullmatch('[0-9a-f]{40}', head_sha)
            or base['sha'] != base_sha or head['sha'] != head_sha):
        return False
    # One merge base; ambiguous histories are not evidence for an exemption.
    bases = git('merge-base', '--all', base_sha, head_sha).splitlines()
    if len(bases) != 1:
        return False
    ancestor = bases[0].decode('ascii')
    changes = git('diff', '--no-ext-diff', '--no-renames', '--raw', '-z',
                  ancestor, head_sha, '--').split(b'\x00')
    if changes == [b''] or changes[-1] != b'' or len(changes) % 2 != 1:
        return False
    for index in range(0, len(changes)-1, 2):
        meta, path = changes[index].split(), changes[index+1]
        if (len(meta) != 5 or meta[0] != b':100644' or meta[1] != b'100644'
                or meta[4] != b'M' or not re.fullmatch(rb'docs/streams/[A-Za-z0-9_-]+/README\.md', path)):
            return False
        name = path.decode('ascii')
        before = git('show', ancestor+':'+name)
        after = git('show', head_sha+':'+name)
        if normalized(before) != normalized(after):
            return False
    return True


if __name__ == '__main__':
    try:
        accepted = classify()
    except (OSError, ValueError, KeyError, TypeError, subprocess.CalledProcessError) as error:
        print('board reconciliation: could-not-check — normal changelog requirement retained ('
              + type(error).__name__ + ')')
        sys.exit(1)
    if accepted:
        print('PASS: board reconciliation — trusted same-repository Status-cell-only diff; no release-note fragment required.')
    else:
        print('board reconciliation: not a qualifying Status-cell-only PR; normal changelog requirement retained.')
    sys.exit(0 if accepted else 1)
