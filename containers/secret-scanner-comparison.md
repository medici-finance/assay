# Source and image credential scanning

## Decision

Use Gitleaks 8.30.1 for ordinary credential detection. Keep the full source-code
scan in `leaksweep-pattern.yml` and replace the image gate's credential regexes
with the same stock engine. Source is the primary scan surface. The packaging
gate covers image config/history and each layer's text; normal OSS binary
handling applies. No compiler-packing exceptions or binary text projections
remain. The separate disclosure sweep retains its separate purpose.

The image adapter only traverses Docker-save archives, isolates image content
from scanner policy, invokes a pinned offline scanner, maps results to image
surfaces and turns incomplete checks into exit 2. It never follows image links
or extracts attacker-controlled paths onto the host.

## Comparison (2026-10-18)

Versions measured: Gitleaks 8.30.1, TruffleHog 3.99.2, Trivy 0.76.0. All
credential verification was disabled. The detector comparison preserves raw
file bytes and normal binary handling; it does not disguise binaries as text.

| Corpus | Gitleaks | TruffleHog | Trivy |
|---|---|---|---|
| 16 original Docker fixtures, all layers/config flattened into separate raw files | 19 findings in 14 fixtures | 0 findings | 0 findings |
| All 71 files from the failed v1.0.49 Linux suite | 0 findings | 908 unverified reports (907 GitLab, 1 TLy) | 0 findings |
| Valid-format synthetic Anthropic API key | Detected | Not selected for the gate; noisy binary results above | Missed |
| Full local v1.0.46 desk-base image | 137 individual findings in 28 upstream text files before baseline | Not selected | Native image scan: 0 findings with default exclusions |

The 16 original fixtures contain deliberately invalid PEM bodies and abbreviated
model-key shapes. Their old expected outcomes describe the old regex policy,
not provider credential validity. Gitleaks still catches their private-key
markers; the other engines' zeroes must not be called successful recall on real
credentials. The old generic-prefix and exact-packed-label expectations are
retired rather than copied into custom OSS rules.

The replacement integration suite uses freshly generated throwaway RSA keys and
valid-format synthetic GitHub personal, installation and fine-grained tokens,
Anthropic keys and OpenAI project keys. It proves detection in source, packaged
text, config/history and earlier layers hidden by later changes. It separately
asserts normal binary skipping. Source scanning, not incidental compiled string
layout, carries the compiled-code credential check.

The failed suite is the CI artifact from release run `39093362287`, artifact
`12124117860`. Its Linux archive SHA256 is
`e0383d6d4791fda0952409b01a53ad6b2ec5e5de01cb26ebf99029b452397283`.
The retained artifact's ZIP CRC and CI checksum were verified before this
comparison; it is a diagnostic CI artifact, not published pin evidence.

## Reviewed upstream baseline

Every stock rule remains enabled. The content-bound baseline contains 137 individual
findings across 28 upstream text files: public Go tests/documentation/constants,
Node headers and package sources, and two Debian Python compiler-adapter sources.
Every file was compared byte-for-byte with its official distribution archive
after checking the archive SHA256. Go and Node archive checksums match the base
Dockerfile pins; Debian package checksums came from the official Bookworm
package index. No binary or project source enters this baseline.

`image-secret-baseline-inventory.json` records exact files, hashes, finding
locations and distribution URLs/checksums. `image-secret-baseline.json` uses
Gitleaks' standard baseline format, without matched values or source excerpts.
The input filename hashes the logical image path, a separator and the entire
file. Gitleaks' baseline comparison therefore cannot clear a changed file just
because its new finding has the same line, length or entropy. Another path also
gets another identity. No path prefix, directory or detector family is exempt.

The integration suite proves an exact baselined input passes while a one-byte
change and the same content at another path are detected. Mutation tests also
remove the content binding and require the identity assertions to fail.

Upstream upgrades may introduce new findings or invalidate old baseline entries.
Review those findings and verify upstream bytes before changing the baseline.
There is no automatic enrollment. Config/history is never baselined. The
contract remains that production credentials arrive only at runtime.

## Reproduce

Download the version-and-digest-pinned scanner explicitly, then test offline:

```sh
sh containers/scripts/layer-secret-scan.sh --prepare
sh containers/scripts/layer-secret-scan.test.sh
sh containers/scripts/layer-secret-scan.mutate.sh
sh containers/scripts/layer-secret-scan.sh <local-image>
```

The scanner container is fixed to Linux amd64, matching the release runner, to
keep baseline coordinates and entropy measurements reproducible across hosts.
A scan never downloads a scanner, passes credentials into it or enables its
network. A synthetic canary is mandatory; scanner errors, malformed reports,
missing archive members and corrupt blob digests cannot produce a clean verdict.

To audit baseline provenance, place the four archives named in the inventory in
one directory, then run the read-only verifier (it downloads and edits nothing):

```sh
python3 containers/scripts/verify_image_secret_baseline.py <upstream-archive-directory>
```

To reproduce the detector comparison, retain the original Docker-save fixtures
from pre-replacement revision `7fb4bf01106169783254b2b68fda6e9b99f83372`, install
the measured CLI versions, and provide the exact failed Linux suite archive:

```sh
python3 containers/scripts/compare_secret_scanners.py \
  --legacy-images <directory-of-original-docker-save-tars> \
  --release-tar <failed-desk-tools-linux-amd64.tar.gz> \
  --output comparison.json
```

The comparison reports counts and rule identifiers only. It never publishes
matched credential material. Repeat the source check with CI's relative paths
so the repository's existing anchored fixture policy applies:

```sh
gitleaks dir . --config .gitleaks.toml --no-banner --redact --exit-code 9
```

## Boundaries

A zero on the 71 binaries is expected binary handling, not proof of an absence of
compiled credentials. Encoded/encrypted payloads and unsupported credential types
are not covered by a universal secret-free guarantee. The image gate complements
the source scan and runtime-injection contract; it does not replace either.
The full local base-image scan passed after the verified baseline. No release,
registry publication or workflow dispatch was performed for this investigation.
