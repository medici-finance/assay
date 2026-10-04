---
id: DR-desktools-v2-03
date: "2026-10-02"
title: "Desk reads authenticate only as a minted App token scoped to the repository being read: an unminted or empty token is refused with no ambient fallback, and the read verbs migrate one at a time, each reach-around deleted in the same change"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/1911#issuecomment-5946883577"
alternatives:
  - "Fold the native read client into forge-neutral (option 2) — ruled out: put to the driver on #1911 and not taken. Minting and custody stay with forge-neutral; this stream only consumes a minted token through the resolver it already has."
  - "Hold until the credential-custody model has more design (option 3) — ruled out: not taken. The ruling selected option 1."
  - "Migrate every read verb in one change, including the tools whose reads and writes share one ambient posting identity — ruled out: the ruling approves the read path only, verb by verb. A tool whose read and write identities are one documented ambient contract (deskdigest, deskadvisory), or whose read runs in a role-less hook (deskpushguard), would need a custody decision about its write path or its role, which this ruling does not make."
accepted:
  - "A migrated read verb reaches the forge only through `deskkit.ForgeFor`, under a token minted for this session's App role. If the role cannot be resolved, the mint fails, or the token comes back empty, the read is a could-not-check and no request leaves the process. GH_TOKEN, GITHUB_TOKEN, the gh keyring and HOME are never consulted as a fallback."
  - "The token is minted for the repository named at the call, never for GH_REPO or an inherited token's installation, so even a present ambient token cannot move a read to another installation (#628)."
  - "Each migrated verb's gh reach-around is deleted in the same change, never left dormant behind a flag. The forge-CLI permit register drops its row and the ratchet ceiling drops with it."
  - "Tools that are not migrated keep their sanctioned permits, and the pull request that lands each migration lists them, each with the reason it remains. A class guard stops any new function from reading a forge token out of the environment unless a reviewed permit names it."
  - "A migrated verb now needs a session App role (DESK_LOOP) to read. A tool run with no loop identity refuses rather than reading as the operator."
---

**Ruling recorded (2026-10-02): option 1 — approve as scoped.** The question was put as the
three options in the brief's `## Human decision`
(`docs/streams/desktools-v2/brief-03-native-read-client.md`): (1) approve as scoped (read path
only, refuse if unminted, installation derived from the repository); (2) approve, but fold the
client into forge-neutral; (3) hold. The driver answered on the brief's decision issue,
[issue #1911](https://github.com/medici-finance/assay/issues/1911), under the driver's own login,
never a role App, in the
[ratifying comment](https://github.com/medici-finance/assay/issues/1911#issuecomment-5946054626)
(2026-10-02T05:18:54Z), which reads "1 — approve as scoped". This record transcribes that
ruling into the register. It does not mint a new one.

The driver then approved this record by its id on the same issue, in a second comment under
the driver's own login
([approval](https://github.com/medici-finance/assay/issues/1911#issuecomment-5946883577),
2026-10-02T06:43:35Z), which reads "approve — DR-desktools-v2-03". The record's `ruling:`
link points at that comment.

**The decision.** A desk read obtains its credential in exactly one way: a token minted for the
session's App role and for the repository being read. That token is handed to the in-process
forge client through the resolver every migrated verb already uses. The brief's `gate-why`
asks the human to confirm two properties, and the ruling confirms both:

- the client refuses an unminted or empty token instead of resolving an ambient one;
- the installation comes from the repository argument, not from the environment.

**The constraint behind it.** The brief's `single-point-of-failure` line names one control: the
refuse-if-unminted contract on the read path. A second, independent layer stands behind it. The
installation is derived from the repository read, so a present ambient token cannot redirect a
read to another installation. The two layers catch different faults:

- an unminted call is caught at the transport;
- a wrong-installation call is caught at identity.

Each has its own negative-path test with the other layer bypassed. Two further controls sit
around them:

- **The forge-CLI permit register.** It is a ratchet on its exact length, so a migrated
  reach-around cannot stay listed.
- **A class guard over the desk tree.** It refuses any new function that reads a forge token
  from the environment unless a reviewed permit names it.

**What this record does not decide.**

- It does not migrate any write path.
- It does not give deskdigest, deskadvisory, deskdisposition or deskpushguard a minted identity.
  Each of those is a custody decision for its own brief.
- It does not change how tokens are minted. Minting stays with forge-neutral.
- It does not touch the ambient-identity preflight, which exists to observe the very credential
  this path refuses.
- It does not, by itself, prove that the approver is a different identity from the brief's
  author (`registers-v1.md` §7.4).
- It does not attest that the design is correct. That is the job of the brief's Verify rows and
  the mutation entries.
