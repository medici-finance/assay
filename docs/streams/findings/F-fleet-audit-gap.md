---
id: F-fleet-audit-gap
date: "2026-09-23"
title: "The portability audit never triaged the GitLab fleet-provisioning script as an adopter-facing needs-port surface"
affects: ["windows-port/02"]
class: one-off
resolved: false
---

**What was found.** The windows-port portability audit
(`docs/streams/windows-port/portability-audit.md`) mentions `tools/create-fleet-gitlab.sh`
exactly once — in its `/tmp` row, where the script inherits that row's disposition as one of
several shell scripts defaulting to `${TMPDIR:-/tmp}`. No row triages the script itself: a
1167-line `bash` + `curl` + `jq` program that a GitLab adopter must run to provision the desk
fleet at all. Its own adoption documentation states the consequence outright — on native
Windows it must be run from Git-Bash or WSL, not from PowerShell — so a Windows adopter on
GitLab kept a bash prerequisite after every audit-driven brief in the stream had shipped.

**Evidence.** The audit's surface table has no row whose location is the script; the only
occurrence of its name is inside the `/tmp` row's location list. The script's Windows
prerequisite is stated in the GitLab adoption guide's fleet-provisioning section and echoed in
the Windows prerequisites of the main adoption guide.

**Fix direction.** The surface is ported by `windows-port/08` (a Go `deskfleet` verb that
provisions the fleet without bash, curl or jq; the script stays as the reference
implementation). Resolution of THIS finding is the audit's own correction: add a row that
triages the script as an adopter-facing `needs-port` surface routed to `windows-port/08`, then
set `resolved: true`. It is recorded here rather than patched silently so the omission — an
audit that sampled surfaces by the POSIX construct they use (`/tmp`, `#!/bin/sh`) rather than
by the adopter journey they sit on — is on the record for the next audit.
