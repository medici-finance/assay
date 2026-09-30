---
id: DR-forge-neutral-11
date: "2026-09-23"
title: "Install without `gh`: plain-HTTPS acquisition verified against the pin file, approved with one addition — refuse any non-HTTPS URL or redirect during the download"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Keep `gh release download` as the acquisition step — ruled out: the brief's own finding is that the CLI was never load-bearing (the release assets are public and the pin file already carries the digests), and requiring it stops a GitLab-only adopter before anything else in the stream can matter."
  - "Verify against the release's own `checksums.txt` fetched alongside the asset — ruled out: a digest fetched from the same place as the binary lets a substituted asset vouch for itself. The pin file (`.assay-versions`, written from the plugin's reviewed `paired-versions.yaml`) stays the single source of the expected value, as the decision issue asked the human to confirm."
  - "Treat an unreadable or missing expected digest as \"nothing to compare, proceed\" — ruled out: the decision issue names this as the failure the brief must not introduce; a digest that cannot be read is a refusal, exactly like a mismatch."
  - "Approve as briefed with no transport constraint — not taken: the ruling approved the brief AND added one requirement, that any non-HTTPS URL or redirect during the download is refused."
accepted:
  - "The pin file is the single source of the expected sha256; a mismatch, or a digest that cannot be read, refuses the install and installs nothing; no path installs a binary whose digest was not positively verified."
  - "The download is HTTPS-only on the initial URL AND on every redirect hop; any other scheme is refused with nothing written to the destination. Cross-host HTTPS redirects are followed — GitHub's release-asset links redirect cross-host — so the constraint is on the scheme, not the host."
  - "The HTTPS requirement narrows the transport and is never a substitute for the digest comparison: the pinned sha256 remains the integrity check on every download, and a correctly-served HTTPS asset whose digest does not match the pin is still refused."
  - "Because `wget`'s `--https-only` binds recursive link-following rather than redirects, `wget` is not offered as a fallback fetcher; `curl` (with its protocol restriction on the transfer and on redirects) is the one fetcher, and a host without it is could-not-check, never an unverified install."
  - "The second, independent layer is unchanged: after acquisition `statusgen --version` must name the pinned tag and `--lint` must exit 0, or the install is reported not proven."
---

**Ruling recorded (2026-09-23): approve as briefed, with one addition.** The driver
(`human:<name>`) ruled on the brief's decision-gate issue,
[issue #1554](https://github.com/medici-finance/assay/issues/1554) — the
[ruling comment](https://github.com/medici-finance/assay/issues/1554#issuecomment-5801176341)
(2026-09-23T19:08:46Z, posted under the driver's own login — the trusted human identity, not a
role App) reads: *"Approve as briefed, with one addition: refuse any
non-HTTPS URL or redirect during the download; the pinned sha256 remains the integrity
check."* The issue offered three options: approve as briefed, approve with changes, or
hold/reject. The recorded answer is **approve with changes**, with the single change quoted
above. This record transcribes that ruling into the register. It does not mint a new ruling:
the human act is the driver's comment on #1554 and the driver's merge of the pull request that
lands this file, not this file itself.

The brief's `gate-why` asked the human to confirm three properties of the acquisition step,
which replaces the CLI that fetched the pinned releases with a plain HTTPS fetch:

- the pin file remains the single source of the expected digest;
- a digest mismatch, or an unavailable digest, refuses the install rather than continuing;
- no path installs a binary whose digest was not positively verified.

The ruling confirms all three and adds a fourth, on the transport: HTTPS only, on the initial
URL and on every redirect hop. The implementation carries it as a scheme check before the
fetch, `curl`'s protocol restriction on the transfer and on redirects, and a refusal (exit 5)
when a hop is to any other scheme. An offline test serves the fixture release from a local
HTTPS server whose redirect lands on a real plain-HTTP server holding the same good asset. A
script that followed that hop would install the asset, so the refusal is the only thing that
keeps the destination empty.

**What this record does not decide.** It does not choose the release home or the pinned tag;
those stay with `paired-versions.yaml` and its re-pin process. It does not, by itself, prove
the approver is a different identity from the brief's author. That is the same
attribution-not-identity limit `lifecycle-v1.md` §7.1.2 declares for verification, and
`registers-v1.md` §7.4 for this register. And it does not attest that the chosen design is
correct: this record shows the alternatives were weighed. Whether the acquisition step is right
is the review gate's judgement, and then the change's own validation after it lands.
