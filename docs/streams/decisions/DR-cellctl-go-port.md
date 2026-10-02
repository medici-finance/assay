---
id: DR-cellctl-go-port
date: "2026-10-02"
title: "cellctl is ported to Go, with the bash kept as the oracle until parity"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/1193#issuecomment-5962048119"
alternatives:
  - "Keep bash and only fold the verbs in — the other answer the decision issue put to the operator (#1193 `## What is needed`, item 2: 'proceed, or keep bash and only fold the verbs in'). Ruled out by the ruling, which chose the port. The case against it is the issue author's, set out in #1193 `## Why a Go port rather than more bash` (the ruling comments give no reasons of their own): `tools/cellctl/cellctl` is ~2,100 lines of bash that one laptop already runs as three divergent copies; `deskkit` already carries the forge resolver, role-token custody, roster parsing and the writeguard that bash cellctl re-implements slices of, and the bridge exists because extending the bash was harder than wrapping it; and the windows-port stream needs a native-Windows cellctl that bash cannot reach."
accepted:
  - "The bash script stays in the tree as the oracle until parity (now at `tools/cellctl/testdata/cellctl-shell-oracle.sh`): for the life of the port two implementations of the launcher coexist, and the Go side is held to byte-parity with the bash on a dry-run matrix (brief desk-containers/10 `## Context`, the oracle and parity-harness facts). Its removal is a later brief."
  - "The replacement is irreversible once shipped (brief desk-containers/10 `gate-why`): once a release ships the Go binary under the tarball's `cellctl` entry, every pinned install that upgrades runs the port, and a boot that goes wrong goes wrong on every cell at once. That cutover stays a separate human sign-off on the brief; this record does not decide it."
  - "The port is bound to `deskkit`: forge resolution, role-token custody and roster parsing are called from `deskkit`, and the write guard from the `tools/desk/cmd/writeguard` package, never re-derived (brief desk-containers/10 `## Context`, 'deskkit reuse is a requirement, not a preference')."
  - "Building the Go cellctl needs a Go toolchain; once a cutover ships the binary, `docs/cellctl.md` §Install's 'It is a shell script, not a Go build … It does not need a Go toolchain' retires (brief desk-containers/10 `sources:`)."
  - "Native Windows is a named consequence, not delivered here: `deskkit` does not compile for `GOOS=windows` until windows-port/00 lands, so a Go cellctl inherits that dependency and the windows-port stream owns building and proving a Windows cellctl (brief desk-containers/10 `## Context`)."
---

**Ruling recorded (2026-10-02): go port, bash kept as the oracle until parity.** The operator
ruled on the decision issue [#1193](https://github.com/medici-finance/assay/issues/1193) under
the operator's own login. The first ruling comment
([2026-09-16T15:00:55Z](https://github.com/medici-finance/assay/issues/1193#issuecomment-5699633449))
reads `ratified - go port`; it does not name this record, so the record links the
operator's corroborating comment
([2026-10-02T21:51:03Z](https://github.com/medici-finance/assay/issues/1193#issuecomment-5962048119)),
which reads: `Ratified: cellctl is ported to Go with the bash kept as the oracle until parity —
DR-cellctl-go-port.` The record's `date:` is that linked comment's posting date (UTC). This
record transcribes that ruling and the text the issue put in front of the operator into the
register for the lifecycle's design-approval gate (`spec/lifecycle-v1.md` §4.4). It does not
mint a new decision: the human act is the operator's comments on #1193, not this file.

**The decision.** `cellctl` is ported from the bash script to Go, with the bash kept as the
oracle until the Go side reaches parity. That is the whole of what the linked comment rules; it
matches #1193 `## What is needed`, item 1(b) ("the port to Go with the bash kept as the oracle
until parity"), and answers item 2, which put "proceed, or keep bash and only fold the verbs
in" to the operator.

**The plan, which this record does not rule.** Where the binary lives and how it ships are the
brief's plan, not part of the ruling: `docs/streams/desk-containers/brief-10-cellctl-go-port.md`
places it at `tools/desk/cmd/cellctl`, following the shape #1193 `## Why a Go port rather than
more bash` (first bullet) proposed — a Go binary shipped in the desk-tools tarball and pinned by
sha256 like every other desk verb. Whether the binary ships in the tarball at all, and when, is
the brief's open human decision (its `## Human decision`, whose options include keeping the
binary out of the tarball until a later ruling).

**The constraint behind it.** One laptop already runs three divergent copies of the shell
launcher, and the first cell it could not express grew a fourth implementation in a fourth
language. Every other desk verb is a Go binary in one tarball, built on the same `deskkit` the
launcher re-implements slices of by hand (brief desk-containers/10 `why:`).

**What this record does not decide.** It scopes to the port only. It does not decide the
CUTOVER — whether and when the tarball's `cellctl` entry switches from the script to the
binary — which stays the human gate on brief desk-containers/10 (its `## Human decision`). It
does not decide the fold-in of the host-local cell verbs or the bridge's retirement, which #1193
also asked for and which are their own briefs. And, per the register's own limits, it records that the
alternatives were weighed and names a human approver; whether the port is correct is the review
gate's judgement and the parity matrix's.
