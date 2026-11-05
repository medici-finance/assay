"""Stage and check the Sigstore provenance bundle beside every release asset.

The release job attests every published asset in one step, which yields ONE
Sigstore bundle whose in-toto statement names every asset by sha256. deskinstall
fetches `<asset>.sigstore.json` beside the asset it installs and refuses
without it, so the release must publish that file beside EVERY asset, and must
refuse to publish when any asset lacks one.

This module is the release-lane half of that contract. It makes no network
call and verifies no signature: the cryptographic check (certificate chain,
transparency-log SET, signer identity) belongs to deskinstall and to
`gh attestation verify`. What it does check is everything that would make
those verifiers refuse for a reason the release can still catch before the
tag is pushed:

* the bundle is a Sigstore bundle under the 1 MiB bound deskinstall reads,
  with a signing certificate (never a bare key), exactly one DSSE signature,
  and a Rekor dsse/0.0.1 entry carrying a signed entry timestamp (a bundle
  with only a Rekor v2 entry is refused by deskinstall, so it is refused here);
* the statement is in-toto v1 SLSA provenance v1;
* the statement names the sha256 of the bytes on disk for every listed asset,
  and every file in checksums.txt is listed and still has the digest
  checksums.txt records, so the attested bytes are the checksummed bytes.

Imported by fixed release workflow calls; it has no command-line entrypoint.
"""
import base64
import binascii
import hashlib
import json
from pathlib import Path
import re

# deskinstall's bundleAssetExt and maxBundleBytes (tools/desk/cmd/deskinstall/attest.go).
BUNDLE_EXT = '.sigstore.json'
MAX_BUNDLE_BYTES = 1 << 20

MEDIA_TYPES = {
    'application/vnd.dev.sigstore.bundle+json;version=0.1',
    'application/vnd.dev.sigstore.bundle+json;version=0.2',
    'application/vnd.dev.sigstore.bundle+json;version=0.3',
    'application/vnd.dev.sigstore.bundle.v0.3+json',
}
IN_TOTO_PAYLOAD = 'application/vnd.in-toto+json'
STATEMENT_V1 = 'https://in-toto.io/Statement/v1'
SLSA_PROVENANCE_V1 = 'https://slsa.dev/provenance/v1'
NAME = re.compile(r'[A-Za-z0-9][A-Za-z0-9._-]*')
HEX64 = re.compile(r'[0-9a-f]{64}')


class BundleError(Exception):
    """The release must not publish: an asset or its bundle is not as required."""


def sha256_file(path):
    digest = hashlib.sha256()
    with open(path, 'rb') as handle:
        for block in iter(lambda: handle.read(1 << 20), b''):
            digest.update(block)
    return digest.hexdigest()


def read_assets(path):
    """The published asset names, one per line: plain file names, no bundles, no repeats."""
    names = Path(path).read_text(encoding='utf-8').split('\n')
    if names and names[-1] == '':
        names.pop()
    if not names:
        raise BundleError(f'{path} lists no release assets')
    seen = set()
    for name in names:
        if not NAME.fullmatch(name) or name.endswith(BUNDLE_EXT):
            raise BundleError(f'{path}: {name!r} is not a release asset name')
        if name in seen:
            raise BundleError(f'{path}: {name} is listed twice')
        seen.add(name)
    return names


def read_checksums(path):
    """checksums.txt as {name: sha256}, in sha256sum's text or binary form."""
    entries = {}
    for line in Path(path).read_text(encoding='utf-8').splitlines():
        match = re.fullmatch(r'([0-9a-f]{64}) [ *](.+)', line)
        if not match or match.group(2) in entries:
            raise BundleError(f'{path}: unreadable or repeated line {line!r}')
        entries[match.group(2)] = match.group(1)
    if not entries:
        raise BundleError(f'{path} lists no files')
    return entries


def _b64(value, what):
    if not isinstance(value, str):
        raise BundleError(f'{what} is missing')
    try:
        return base64.b64decode(value, validate=True)
    except (binascii.Error, ValueError):
        raise BundleError(f'{what} is not base64') from None


def statement_digests(raw):
    """Check the bundle's shape and return the set of sha256 digests it attests."""
    if len(raw) > MAX_BUNDLE_BYTES:
        raise BundleError(f'bundle is {len(raw)} bytes, over the {MAX_BUNDLE_BYTES}-byte bound deskinstall reads')
    try:
        bundle = json.loads(raw)
    except (ValueError, UnicodeDecodeError):
        raise BundleError('bundle does not parse as JSON') from None
    if not isinstance(bundle, dict) or bundle.get('mediaType') not in MEDIA_TYPES:
        media = bundle.get('mediaType') if isinstance(bundle, dict) else None
        raise BundleError(f'bundle media type {media!r} is not a Sigstore bundle')
    material = bundle.get('verificationMaterial')
    material = material if isinstance(material, dict) else {}
    chain = material.get('x509CertificateChain')
    certs = chain.get('certificates') if isinstance(chain, dict) else None
    cert = material.get('certificate')
    if isinstance(cert, dict):
        _b64(cert.get('rawBytes'), 'signing certificate')
    elif isinstance(certs, list) and certs and isinstance(certs[0], dict):
        _b64(certs[0].get('rawBytes'), 'signing certificate')
    else:
        raise BundleError('bundle carries no signing certificate (a bare public key is not accepted)')
    entries = material.get('tlogEntries')
    entries = entries if isinstance(entries, list) else []
    usable = False
    for entry in entries:
        if not isinstance(entry, dict):
            continue
        kind = entry.get('kindVersion')
        promise = entry.get('inclusionPromise')
        if (isinstance(kind, dict) and (kind.get('kind'), kind.get('version')) == ('dsse', '0.0.1')
                and isinstance(promise, dict) and promise.get('signedEntryTimestamp')):
            usable = True
    if not usable:
        raise BundleError('bundle has no Rekor dsse/0.0.1 log entry with a signed entry timestamp; '
                          'deskinstall refuses such a bundle')
    envelope = bundle.get('dsseEnvelope')
    if not isinstance(envelope, dict):
        raise BundleError('bundle carries no DSSE envelope')
    if envelope.get('payloadType') != IN_TOTO_PAYLOAD:
        raise BundleError(f'payload type {envelope.get("payloadType")!r} is not {IN_TOTO_PAYLOAD}')
    signatures = envelope.get('signatures')
    if not isinstance(signatures, list) or len(signatures) != 1:
        count = len(signatures) if isinstance(signatures, list) else 0
        raise BundleError(f'envelope carries {count} signatures, want exactly 1')
    try:
        statement = json.loads(_b64(envelope.get('payload'), 'envelope payload'))
    except (ValueError, UnicodeDecodeError):
        raise BundleError('attestation statement does not parse') from None
    if not isinstance(statement, dict) or statement.get('_type') != STATEMENT_V1:
        raise BundleError(f'statement _type is not {STATEMENT_V1}')
    if statement.get('predicateType') != SLSA_PROVENANCE_V1:
        raise BundleError(f'statement predicateType {statement.get("predicateType")!r} is not {SLSA_PROVENANCE_V1}')
    digests = set()
    for subject in statement.get('subject') or []:
        value = subject.get('digest', {}).get('sha256') if isinstance(subject, dict) else None
        if isinstance(value, str) and HEX64.fullmatch(value.lower()):
            digests.add(value.lower())
    return digests


def _attested(raw, name, digest):
    if digest not in statement_digests(raw):
        raise BundleError(f'bundle has no subject with the sha256 of {name} ({digest}); '
                          'it does not attest the bytes being published')


def stage(root, bundle_path, assets_path, checksums_path):
    """Copy the attestation bundle to <asset>.sigstore.json beside every listed asset.

    Every check runs before the first file is written, so a refusal leaves no
    bundle on disk.
    """
    root = Path(root)
    names = read_assets(assets_path)
    for name in names:
        path = root / name
        if path.is_symlink() or not path.is_file():
            raise BundleError(f'release asset {name} is not a regular file')
        if (root / (name + BUNDLE_EXT)).exists() or (root / (name + BUNDLE_EXT)).is_symlink():
            raise BundleError(f'{name}{BUNDLE_EXT} already exists before staging; '
                              'a bundle not produced by this run is never published')
    digests = {name: sha256_file(root / name) for name in names}
    for name, listed in read_checksums(checksums_path).items():
        if name not in digests:
            raise BundleError(f'{name} is in checksums.txt but not in the published asset list, '
                              'so it would ship unattested')
        if digests[name] != listed:
            raise BundleError(f'{name} no longer has the sha256 checksums.txt records; '
                              'the attested bytes would not be the checksummed bytes')
    raw = Path(bundle_path).read_bytes()
    for name in names:
        _attested(raw, name, digests[name])
    for name in names:
        (root / (name + BUNDLE_EXT)).write_bytes(raw)


def check(root, assets_path):
    """Problems that must stop the release: an asset with no bundle, or a bundle not for its bytes."""
    root = Path(root)
    problems = []
    for name in read_assets(assets_path):
        bundle = root / (name + BUNDLE_EXT)
        if bundle.is_symlink() or not bundle.is_file():
            problems.append(f'{name} has no {name}{BUNDLE_EXT}')
            continue
        if not (root / name).is_file():
            problems.append(f'{name} is missing')
            continue
        try:
            _attested(bundle.read_bytes(), name, sha256_file(root / name))
        except BundleError as err:
            problems.append(f'{name}{BUNDLE_EXT}: {err}')
    return problems


def published_problems(root, assets_path, listing):
    """Problems with the release's asset listing (the REST API's JSON list).

    Every listed asset and its bundle must be on the release exactly once, at
    the size of the local file uploaded for it.
    """
    root = Path(root)
    if not isinstance(listing, list):
        return ['the release asset listing is not a list']
    counts, sizes = {}, {}
    for asset in listing:
        if isinstance(asset, dict) and isinstance(asset.get('name'), str):
            counts[asset['name']] = counts.get(asset['name'], 0) + 1
            sizes[asset['name']] = asset.get('size')
    problems = []
    for name in read_assets(assets_path):
        for published in (name, name + BUNDLE_EXT):
            if counts.get(published, 0) != 1:
                problems.append(f'{published} is on the release {counts.get(published, 0)} times, want exactly once')
            elif sizes[published] != (root / published).stat().st_size:
                problems.append(f'{published} on the release is {sizes[published]} bytes, '
                                f'not the {(root / published).stat().st_size} bytes uploaded')
    return problems
