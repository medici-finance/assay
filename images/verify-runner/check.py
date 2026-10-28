"""Release-owned capability probe; no repository inputs or network needed."""
import importlib.metadata
import json
import os
import platform
import shutil
import stat
import subprocess

import jsonschema
import yaml

schema = {
    '$schema': 'https://json-schema.org/draft/2020-12/schema',
    'type': 'object',
    'required': ['nodes'],
    'properties': {'nodes': {'type': 'array', 'items': {'type': 'string'}}},
    'additionalProperties': False,
}
jsonschema.Draft202012Validator.check_schema(schema)
validator = jsonschema.Draft202012Validator(schema)
validator.validate(yaml.safe_load('nodes: [example]'))
try:
    validator.validate(yaml.safe_load('nodes: [42]'))
except jsonschema.ValidationError:
    pass
else:
    raise RuntimeError('invalid schema instance was accepted')

# The cluster backend's fixed entrypoints: a regular file (never a symlink:
# lstat) at a fixed path, owned by root user and root group, executable,
# writable by nobody but root, with no set-id bit, in a directory that only root
# can change, and runnable as a program by the non-root user this probe runs as.
# Run with no argument and no input, each prints its usage and exits 2 before
# it reads or writes anything. A script that cannot be exec'd directly (no #!
# line, no exec bit) raises OSError here.
#
# Not os.access(W_OK): on this probe's read-only root it is false for every
# file whatever its owner or mode, so it would pass vacuously. Owner and mode
# bits are what decide who could write the file on a writable root.
ENTRYPOINT_DIR = '/usr/local/bin'
dst = os.lstat(ENTRYPOINT_DIR)
if not stat.S_ISDIR(dst.st_mode):
    raise RuntimeError(f'{ENTRYPOINT_DIR} is not a directory')
if dst.st_uid != 0 or dst.st_gid != 0:
    raise RuntimeError(f'{ENTRYPOINT_DIR} is not owned by root:root')
if dst.st_mode & 0o022:
    raise RuntimeError(f'{ENTRYPOINT_DIR} is writable by its group or by others')
ENTRYPOINTS = ('stage-source.sh', 'stage-cache.sh', 'witness.sh')
for name in ENTRYPOINTS:
    path = ENTRYPOINT_DIR + '/' + name
    st = os.lstat(path)
    if not stat.S_ISREG(st.st_mode):
        raise RuntimeError(f'{path} is not a regular file')
    if st.st_mode & 0o111 != 0o111:
        raise RuntimeError(f'{path} is not executable by everyone')
    if st.st_uid != 0 or st.st_gid != 0:
        raise RuntimeError(f'{path} is not owned by root:root')
    if st.st_mode & 0o022:
        raise RuntimeError(f'{path} is writable by its group or by others')
    if st.st_mode & (stat.S_ISUID | stat.S_ISGID):
        raise RuntimeError(f'{path} carries a set-user-id or set-group-id bit')
    run = subprocess.run([path], stdin=subprocess.DEVNULL, capture_output=True, timeout=60)
    if run.returncode != 2:
        raise RuntimeError(f'{path} with no argument exited {run.returncode}, not 2')
    if run.stdout or not run.stderr.startswith(b'usage: '):
        raise RuntimeError(f'{path} with no argument did not print only its usage on stderr')

# Row tools (#4684, #4318): rows run with no network, so a tool they shell out
# to has to be in the image. Each is resolved on PATH and run, so a missing or
# broken one fails this probe before anything is pushed. jq evaluates an
# expression and shellcheck lints a clean script from stdin; tmux only reports
# its version, since starting a server needs more than this fence allows.
# name: (probe args, probe stdin, expected stdout or None, version args).
ROW_TOOLS = {
    'jq': (['-n', '-c', '{"sum": (1 + 1)}'], b'', b'{"sum":2}\n', ['--version']),
    'shellcheck': (['--shell=sh', '-'], b'#!/bin/sh\nprintf \'%s\\n\' "$1"\n', b'',
                   ['--version']),
    'tmux': (['-V'], b'', None, ['-V']),
}
tools = {}
for name, (args, stdin, want, version_args) in ROW_TOOLS.items():
    path = shutil.which(name)
    if path is None:
        raise RuntimeError(f'{name} is not on PATH')
    run = subprocess.run([path, *args], input=stdin, capture_output=True, timeout=60)
    if run.returncode != 0:
        raise RuntimeError(f'{name} exited {run.returncode}: {run.stderr!r}')
    if want is not None and run.stdout != want:
        raise RuntimeError(f'{name} printed {run.stdout!r}, not {want!r}')
    version = subprocess.run([path, *version_args], stdin=subprocess.DEVNULL,
                             capture_output=True, timeout=60)
    if version.returncode != 0:
        raise RuntimeError(f'{name} {" ".join(version_args)} exited {version.returncode}')
    lines = version.stdout.decode(errors='replace').splitlines()
    tools[name] = next((line.strip() for line in lines if any(c.isdigit() for c in line)), '')

print(json.dumps({'python': platform.python_version(),
                  'PyYAML': importlib.metadata.version('PyYAML'),
                  'jsonschema': importlib.metadata.version('jsonschema'),
                  'entrypoints': list(ENTRYPOINTS),
                  'tools': tools}, sort_keys=True))
