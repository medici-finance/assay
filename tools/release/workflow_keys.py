"""A closed reader for the plain block-style YAML the release workflows use.

The release guards ask what a workflow sets: which keys a job holds and with
which values. YAML has many spellings of one setting: a quoted key, a space
before the colon, a block or quoted scalar, an escape sequence, a flow mapping,
an anchor or merge key, a trailing comment, a key written twice. A guard that
compares the text it reads with a value it refuses fails open on any spelling
whose text differs from the value YAML gives it. So this reader accepts only
the forms the guarded workflow files (both release.yml copies and ci.yml)
actually use, each read exactly as written, and reports every other line as
unread. A guard refuses every unread line, wherever it sits, so a workflow it
passes holds no setting it did not read.

The accepted forms, and nothing else:
- blank lines and comment lines;
- `key:` and `key: value`, also as a `- ` sequence item, with a plain key;
- as a value: a plain scalar on one line that holds no `: ` and does not end
  in `:`; a double-quoted scalar on one line with no backslash; a
  single-quoted scalar on one line with no quote inside it; a one-line flow
  sequence of such plain or quoted items; or a bare `|` block scalar header
  (no `>`, no chomping or indentation indicator);
- plain scalar sequence items.
A `|` block scalar is accepted only as a step's `run` value or as the `path`
under a step's `with`, the two places the workflows use one; every body line
must be indented at least as far as its first. A `uses` value must name an
action pinned to a full commit SHA, `owner/repo[/path]@<40 hex>`. A trailing
` #` comment is dropped from every value, and keys are read in lower case so
that no two spellings of one key differ only by case. A line holding a tab, a
carriage return or any other control, format or separator character but the
plain space is unread, wherever it sits. Stdlib only; offline.
"""
import re
import unicodedata

ENTRY = re.compile(r'( *)(- +)?([A-Za-z_][A-Za-z0-9_.-]*):(?: +(.*))?')
PLAIN_ITEM = r'[^\s:#\'"{}\[\]&*!|>%@`,][^\s:#,]*'
ITEM = re.compile(r'( *)- +(' + PLAIN_ITEM + ')')
# A quoted scalar is read only when its text between the quotes is its value:
# no escape sequence in a double-quoted one, no doubled quote in a single-quoted one.
QUOTED_ITEM = r'"[^"\\\n]*"|\'[^\'\n]*\''
QUOTED = re.compile(QUOTED_ITEM)
# A flow sequence item holds no comma, bracket or brace, so splitting on commas reads it.
FLOW_ITEM = r'(?:[^\s:#\'"{}\[\]&*!|>%@`,][^\s:#,{}\[\]]*|"[^"\\\n,]*"|\'[^\'\n,]*\')'
FLOW = re.compile(r'\[ *(?:' + FLOW_ITEM + r' *, *)*' + FLOW_ITEM + r' *\]')
SCALAR = re.compile(r'\|')
ACTION = re.compile(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_./-]+)?@[0-9a-f]{40}')
REFUSED_START = tuple('{&*!%@`,?:-|>[')
REFUSED_CATEGORIES = ('Cc', 'Cf', 'Cn', 'Co', 'Cs', 'Zl', 'Zp', 'Zs')


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
    """RAW without its trailing comment, or None unless it is one accepted form."""
    if raw.startswith(('"', "'")):
        match = QUOTED.match(raw)
        if match and re.fullmatch(r'(?:\s+#.*)?', raw[match.end():]):
            return match.group(0)
        return None
    value = '' if raw.startswith('#') else re.split(r'\s#', raw, maxsplit=1)[0].rstrip()
    if SCALAR.fullmatch(value) or FLOW.fullmatch(value):
        return value
    if value.startswith(REFUSED_START) or ': ' in value or value.endswith(':'):
        return None
    return value


NOT_PRINTABLE_ASCII = re.compile(r'[^ -~]')


def odd_character(line):
    """The first character of LINE outside the plain set, or None.

    Printable ASCII is never in a refused category, so only the other
    characters are looked up.
    """
    for match in NOT_PRINTABLE_ASCII.finditer(line):
        char = match.group(0)
        if unicodedata.category(char) in REFUSED_CATEGORIES:
            return char
    return None


def read(text):
    """(entries, unread) for TEXT; unread lists (path, line, text) it cannot read.

    A key repeated within one mapping (in any case), or a key or item nested
    under a key that already has a value, is unread too: YAML would reject or
    overwrite it. So is a block scalar header that scalar_allowed refuses (its
    body is still skipped, so no line of it is read as a key), a body line
    indented less than the body's first line, a `uses` value that is not a
    pinned action, and any line holding an odd character.
    """
    entries, unread = [], []
    for number, line in enumerate(text.split('\n'), 1):
        char = odd_character(line)
        if char is not None:
            unread.append(((), number, 'U+%04X in: %s' % (ord(char), ascii(line))))
    root = [-1, None, '', set()]
    stack = [root]
    scalar = None
    for number, line in enumerate(text.splitlines(), 1):
        body = line.strip()
        indent = len(line) - len(line.lstrip(' '))
        if scalar is not None and (not body or indent > scalar[0]):
            if body and scalar[1] is None:
                scalar[1] = indent
            elif body and indent < scalar[1]:
                unread.append((tuple(frame[1] for frame in stack[1:]), number, line))
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
        if value is None or parent[2] or key in parent[3] or (key == 'uses' and not ACTION.fullmatch(value)):
            unread.append((path, number, line))
            continue
        if SCALAR.fullmatch(value):
            scalar = [depth, None]
            if not scalar_allowed(path, key):
                unread.append((path, number, line))
                continue
        parent[3].add(key)
        entries.append(Entry(number, path, key, value, bool(match.group(2))))
        stack.append([depth, key, value, set()])
    return entries, unread


def scalar_allowed(path, key):
    """Whether a block scalar may stand at PATH under KEY: a step's `run`
    value, or the `path` under a step's `with`."""
    return (path[-1:] == ('steps',) and key == 'run') or (path[-2:] == ('steps', 'with') and key == 'path')


def unread_problems(unread):
    """One problem per unread line, wherever it sits.

    No line is skipped by place: an unread line in a job no guard names can be
    a write grant, an environment or an upload in another spelling, so a guard
    passes a workflow only when the reader read all of it.
    """
    return ['line %d is not in the plain form this check reads: %s' % (number, line.strip())
            for path, number, line in unread]


def flow_items(value):
    """The items of a one-line flow sequence, or None for any other value."""
    if not FLOW.fullmatch(value):
        return None
    return [item.strip() for item in value[1:-1].split(',')]
