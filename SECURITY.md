# Security Policy

## Reporting a vulnerability

Please **do not** open a public issue for security problems. This repository ships tooling
that other projects consume by pinned version — public disclosure before a fix is available
puts every pinned consumer at risk.

Use GitHub's **private vulnerability reporting** instead: **Security tab → "Report a
vulnerability"** on this repository. Reports go directly and privately to the maintainers —
GitHub's standard coordinated-disclosure channel.

Please include: the affected file and how to reproduce it, the version you are running
(tag or commit SHA), the impact scenario where it matters, and — if you have one — a
suggested fix. You will get an acknowledgement as soon as a maintainer reviews it.

## Expected response window

There is no formal SLA on this repository. The maintainers review reports as they arrive;
acknowledgement normally within a few business days, faster for higher-impact reports. If
you have received no response after a reasonable window and private reporting is your only
channel, that itself is worth noting in the report so we can agree a faster path with you.

## Supported versions

Only the **latest release line** receives security fixes. This repository is consumed by
tag or commit SHA, and a pinned consumer does not receive a fix until the pin is bumped —
nothing in your environment will tell you a fix exists. **Watch Releases and Security
advisories on this repository;** advisories name the patched version, and every
consumer-relevant change ships as a tag. If you are running an older pin, the fix is to
move to the latest tag, not to backport.

## Scope

**In scope:** anything in this repository — tooling defects with security impact, unsafe
defaults, workflow or CI weaknesses, and documentation that leads adopters into insecure
setups.

**Out of scope:** vulnerabilities in upstream platforms (your Git host, CI provider, or any
third party these tools call out to) — report those to the upstream vendor. The behaviour
of a consumer's *own* repository after they configure these tools is the consumer's
responsibility, not a defect here, unless the tooling pushed them into the insecure state.

## How fixes ship

Two paths, decided by whether disclosure has to wait.

**Most fixes take the ordinary path.** A defect with no live embargo — one we found
ourselves, or a reported one already public — is fixed through the front door: a branch, a
pull request, maintainer review, and the automated checks, before it lands on the default
branch. If it warranted an advisory, we publish one once the fix has shipped.

**A fix that must not be visible before it ships** is developed privately, in the temporary
private fork attached to its draft advisory, and reviewed there before it lands. We use
this path only when early visibility would put consumers at risk — not by default.

Either way: the fix lands, a release tag is cut so consumers can move to it, and the
advisory names that tag as the patched version. If you reported the issue and want credit,
say so and we will name you.

**On review enforcement.** A maintainer may merge an embargoed fix from the advisory page,
which is a different path from an ordinary pull request. Our policy is to grant no actor a
standing bypass of the default branch's review requirement — a ruleset bypass is scoped to a
whole ruleset and cannot be limited to one merge, so granting one would mean unreviewed
pushes to the default branch indefinitely. Where an approval is posted by automation, that
approval is posted by a separate identity the author cannot impersonate; it is never a
self-approval.
