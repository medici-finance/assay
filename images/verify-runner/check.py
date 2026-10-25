"""Release-owned capability probe; no repository inputs or network needed."""
import importlib.metadata
import json
import os
import platform
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

print(json.dumps({'python': platform.python_version(),
                  'PyYAML': importlib.metadata.version('PyYAML'),
                  'jsonschema': importlib.metadata.version('jsonschema'),
                  'entrypoints': list(ENTRYPOINTS)}, sort_keys=True))
