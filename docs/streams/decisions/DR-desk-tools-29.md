---
id: DR-desk-tools-29
date: "2026-10-08"
title: "Split the verdict runner: `verifyloop verdict --unsigned-out` composes with no key, and `deskverdict sign` binds the host's signature to the composer's digest and the host's own dispatch record"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/2344#issuecomment-6050885129"
alternatives:
  - "Option 2, keep the composing step outside the container (the composer runs on the host with no key loaded and sends each check command into the sealed container one at a time) — not chosen. The decision issue states it is the only option that adds a second layer against forged results, and that it is a larger change for which this brief would be withdrawn and a new one authored."
  - "Option 3, the split mode only with no signer changes (the host's checks written down as a procedure but not enforced; the signer keeps signing any file it is handed) — not chosen. The decision issue calls it the least protection of the approvals."
  - "Option 4, mark split-path signatures (record in the signed results that they were composed elsewhere) — not chosen. The decision issue states it changes the signed format, breaks byte identity with the combined path, and needs a format version bump, a separate and larger change."
  - "Option 5, reject (keep the combined process only) — not chosen. The decision issue states that adopters who want the checks fenced from the key would then have no supported path, and that on the combined path a bad check command can steal the key and forge anything until the key is replaced."
accepted:
  - "Payload shaping inside the dispatched scope (the decision issue's trade): the check commands still run in the same container, as the same user, as the step that writes the results file, so a hostile check command can still invent results for the run it is part of. The signer's binding checks run only at signing time: they make the host refuse a results file whose fingerprint, repository, commit or timestamp does not match its own record, but they do not limit what the results claim, and nothing after signing checks the commit or the timestamp again (the transcriber checks only the repository). They do not prevent such a forgery, and no layer in this repository stops it (brief desk-tools/29, `single-point-of-failure`, fault (b): behind the host's signing step, NONE)."
  - "The new mode never resolves, opens or reads the verifier key at any location the resolver consults (`VERIFIER_PEM`, `$ASSAY_CONFIG_HOME/verifier-app.pem`, `~/.config/assay/verifier-app.pem`); the brief's Verify rows 1 and 2 are the discriminating proof."
  - "The unsigned payload is written only to an explicit `--unsigned-out` file and never printed as a fenced verdict body, so nothing downstream can mistake it for a verdict."
  - "The binding checks (`--expect-sha256`, `--expect-repo`, `--expect-head`, `--not-before`, `--not-after`) are optional flags on the signer; the host contract, not the signer, requires the host to pass all five, taking repo, head and both time bounds from its own dispatch record and only the digest from the composer's output."
  - "On every `deskverdict sign --payload` call, with or without the new flags, the signer reads only a regular file (never a link or a pipe), on unix from a parent directory no other user can write, and never writes its `.out` sibling through a link. A leftover or unwritable `.out` sibling, which used to be overwritten or skipped with a note (exit 0), now makes signing fail (exit 5)."
---

**Ruling recorded (2026-10-08): option 1, approve with the host-side binding.** The decision
issue [#2344](https://github.com/medici-finance/assay/issues/2344) put five options to the
driver, listed in its body (brief desk-tools/29 as merged in #2343). The driver's
[ruling comment](https://github.com/medici-finance/assay/issues/2344#issuecomment-6050885129),
posted under the driver's own login at 2026-10-08T02:24:01Z, reads `1`, and the issue was
closed a second later. This record transcribes that ruling and the text the issue put in
front of the driver into the register for the lifecycle's design-approval gate
(`spec/lifecycle-v1.md` §4.4). It does not mint a new decision: the human act is the driver's
comment on #2344, not this file. The ruling comment does not name this record's id, so the
register's `ruling:` corroboration (`registers-v1.md` §7.5) may report `record-not-named`
for it; that is a fact about the comment's text, and this record does not claim otherwise.

**Option 1, as written on the decision issue.**

> **Approve with the host-side binding (recommended).** The split mode, plus a signer that,
> when the host passes the expected fingerprint, repository, commit and time bounds, refuses a
> results file that does not match them. The checks are optional flags on the signer, so the
> host's written procedure requires it to pass all of them: the fingerprint from the composing
> step's output, everything else from its own record. On every signing call, with or without
> those flags, the signer reads only an ordinary file (never a link or a pipe), on unix from a
> directory no other user can write, and never writes its output through a link — so a leftover
> or unwritable output file next to the results file, which today is overwritten or skipped
> with a note, now makes signing fail. A hostile check command can still invent results for the
> run it is part of, but cannot get a stale or swapped file signed and never reaches the key.
> Everything is in this brief.

**The decision.** `docs/streams/desk-tools/brief-29-verifyloop-unsigned-compose.md` is
implemented as briefed: `verifyloop verdict --unsigned-out <file>` composes the verdict-v1
payload with no key and prints its digest, and `deskverdict sign` gains the five binding flags
and the file-handling rules above, under the host contract the brief's Task 9 states.

**The constraint behind it.** Today the process that runs arbitrary Verify-row shell commands
has already resolved the verifier key, so a malicious or buggy row can copy it and forge
verdicts until the key is rotated. The split trades that key theft for payload shaping bounded
to the run the host dispatched (brief desk-tools/29 `gate-why`).

**The consequence level.** `consequence: major` is not in the ruling comment, which states no
level. It is a desk default declared in the pull request that added this record (its
`## Desk-decided` section), amendable by the maintainer; it is not part of what the driver
ruled.

**What this record does not decide.** It does not attest the design is correct: it records
that the alternatives were weighed and names a human approver; whether the implementation
holds is the review gate's judgement (the brief's `## Review` questions) and then its Verify
table. It does not decide option 2's design, which would need its own brief. It does not
cover the operator's fence itself (that it mounts no key and runs the rows as a different uid
from the signing user), which lives outside this repository.
