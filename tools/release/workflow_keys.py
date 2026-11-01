"""A strict reader for the plain block-style YAML the release workflows use.

The release guards ask what a workflow sets: which keys a job holds and with
which values. A regular expression over the text answers that for one spelling
only, and YAML has others: a quoted key, a space before the colon, a block or
quoted scalar, a flow mapping, an anchor or merge key, a trailing comment, a key
written twice. This reader accepts one spelling of each setting and reports
every other line it meets, so a guard that refuses those lines never reads a
setting it does not understand.

Accepted lines: blank lines and comment lines; `key:` and `key: value` (also as
a `- ` sequence item) with a plain key, where the value is a plain scalar, a
single-line quoted scalar, a one-line flow sequence or a `|`/`>` block scalar
header; and plain scalar sequence items. A block scalar is accepted only as a
step's `run` value or as a value under a step's `with`, the two places the
workflows use one. Anywhere else it is refused: this reader skips a block
scalar's body, and that body could hold text shaped like steps or settings.
A trailing ` #` comment is dropped from every value, and keys are read in
lower case so that no two spellings of one key differ only by case. Stdlib
only; offline.
"""
import re

ENTRY = re.compile(r'( *)(- +)?([A-Za-z_][A-Za-z0-9_.-]*):(?: +(.*))?')
ITEM = re.compile(r'( *)- +([^\s:#\'"{}\[\]&*!|>%@`,][^\s:#,]*)')
QUOTED = re.compile(r'"(?:[^"\\\n]|\\.)*"|\'(?:[^\'\n]|\'\')*\'')
FLOW = re.compile(r'\[[^\[\]{}#\n]*\]')
SCALAR = re.compile(r'[|>](?:[1-9][-+]?|[-+][1-9]?)?')
REFUSED_START = tuple('{&*!%@`,?:-|>')


class Entry:
    """One key line: where it sits, what it says, and which line it came from."""

    def __init__(self, line, path, key, value, item):
        self.line = line
        self.path = path
        self.key = key
        self.value = value
        self.item = item

    def under(self, *prefix):
        return self.path[:len(prefix)] == prefix


def plain_value(raw):
    """RAW without its trailing comment, or None when it is not one line long."""
    if raw.startswith(('"', "'")):
        match = QUOTED.match(raw)
        if match and re.fullmatch(r'(?:\s+#.*)?', raw[match.end():]):
            return match.group(0)
        return None
    value = '' if raw.startswith('#') else re.split(r'\s#', raw, maxsplit=1)[0].rstrip()
    if SCALAR.fullmatch(value):
        return value
    if value.startswith(REFUSED_START):
        return None
    if value.startswith('[') and not FLOW.fullmatch(value):
        return None
    return value


def read(text):
    """(entries, unread) for TEXT; unread lists (path, line) it cannot read.

    A key repeated within one mapping (in any case), or a key or item nested
    under a key that already has a value, is unread too: YAML would reject or
    overwrite it. So is a block scalar header that scalar_allowed refuses; its
    body is still skipped, so no line of it is read as a key.
    """
    entries, unread = [], []
    root = [-1, None, '', set()]
    stack = [root]
    scalar = None
    for number, line in enumerate(text.splitlines(), 1):
        body = line.strip()
        indent = len(line) - len(line.lstrip(' '))
        if scalar is not None and (not body or indent > scalar):
            continue
        scalar = None
        if not body or body.startswith('#'):
            continue
        match = ENTRY.fullmatch(line)
        value = plain_value(match.group(4) or '') if match else None
        depth = indent + len(match.group(2) or '') if match else indent
        while len(stack) > 1 and stack[-1][0] >= depth:
            stack.pop()
        path = tuple(frame[1] for frame in stack[1:])
        parent = stack[-1]
        if not match:
            if parent[2] or not ITEM.fullmatch(re.split(r'\s#', line, maxsplit=1)[0].rstrip()):
                unread.append((path, number, line))
            continue
        key = match.group(3).lower()
        if match.group(2):
            parent[3] = set()
        if value is None or parent[2] or key in parent[3]:
            unread.append((path, number, line))
            continue
        if SCALAR.fullmatch(value):
            scalar = depth
            if not scalar_allowed(path, key):
                unread.append((path, number, line))
                continue
        parent[3].add(key)
        entries.append(Entry(number, path, key, value, bool(match.group(2))))
        stack.append([depth, key, value, set()])
    return entries, unread


def scalar_allowed(path, key):
    """Whether a block scalar may stand at PATH under KEY: a step's `run` value,
    or a value under a step's `with`."""
    return (path[-1:] == ('steps',) and key == 'run') or path[-2:] == ('steps', 'with')


GRANTS = ('permissions', 'environment')


def unread_problems(unread, *jobs):
    """One problem per unread line among the top-level keys, the keys directly
    under any of them (each job's name, for one), anywhere in one of JOBS, and,
    in every job, among the job's own keys or under its GRANTS.

    The last rule holds in jobs no guard names: an unread line among a job's
    keys can be a write grant or an environment in another spelling (a quoted
    key, say), and so can an unread line under its `permissions` or
    `environment`, so neither is ever skipped.
    """
    problems = []
    for path, number, line in unread:
        if len(path) < 2 or (path[0] == 'jobs' and (
                path[1] in jobs or len(path) == 2 or path[2] in GRANTS)):
            problems.append('line %d is not in the plain form this check reads: %s'
                            % (number, line.strip()))
    return problems


def flow_items(value):
    """The items of a one-line flow sequence, or None for any other value."""
    if not FLOW.fullmatch(value):
        return None
    return [item.strip() for item in value[1:-1].split(',')]
