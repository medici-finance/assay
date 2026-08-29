# Leak check + Audience check — for any OUTWARD-FACING artifact (the outward-artifact review convention)

Put both sections in the reviewer's prompt **verbatim**. `pr-review-desk/SKILL.md` § The reviewer's
bar points here.

**When they are mandatory.** The PR touches a site page, a deck, a PDF, a public README, an OG
image, a grant or partner submission, release notes, or any file whose disposition is `copy` in
the repo's publication manifest. On a purely internal diff both sections are silent — say so
rather than omitting them.

**Read this first.** Everything else in the reviewer's bar asks whether a claim is TRUE. These two
axes are **orthogonal to the accuracy check**, and on both of them **a statement being TRUE is not
a defence**. A leak is true: a source comment naming an internal brief id, a private repo, the
approver and an internal approval rule is entirely accurate, and the truthfulness gate does not
merely miss it — it green-lights it. Candid internal judgements of a partner or of a named
individual are accurate too. No amount of fact-checking reaches either.

**The source file IS the artifact.** GitHub Pages serves it byte-for-byte; no build or minify step
stands between a maintainer comment and the public reader. `<!-- … -->` ships.

## Leak check — would an outsider gain something they should not have?

Ranked by what the outsider gains. Report per line, not per file.

1. **People and approvers** — who decided, who reviewed, who is on the list.
2. **Private repos, docs, endpoints** — anything an outsider cannot open but can now ask about.
3. **Internal machinery** — brief/stream ids (`<slug>/NN`), register ids (`F-…`/`I-…`),
   internal document paths, board-tool and desk vocabulary. **This leaks most freely because it reads
   as jargon** to the person writing it.
4. **Undecided or unreleased plans** — the roadmap we have not committed to.
5. **Shipping placeholders** — `TODO` / `TBD` / `FIXME` tell a reader we are mid-thought in public.
6. **Credentials** — already covered by the secret scan. That is the floor, not the ceiling.

## Audience check — is this true thing addressed to the wrong audience?

**KEEP / REMOVE — reproduce this distinction in the prompt without paraphrasing it. It is
load-bearing and inverting it is the expensive error.**

- **KEEP** an honest caveat that **SCOPES our own claim** — "not yet built", "as of &lt;date&gt;",
  "we assert nothing about X", "unverified beyond the sample stated here". **Do not strip these.**
  They are why our claims are defensible; a review that deletes them makes a true document
  misleading, and it trains authors to stop writing them.
- **REMOVE** internal candour that **JUDGES a third party or a named individual** — an assessment
  of a partner, of a funder's ecosystem, of a candidate. Test: *would I say this to the person it
  is about, in the room, with my name on it?*

A named living person who never consented is the highest-severity case, and **removing it from the
working tree does not retract it** — the commit, the fork and the web UI still serve it. Say so in
the finding, and route the retraction question to a human; a reviewer cannot un-publish.

## Both report three-state, and the mechanical scan is not the verdict

Each section reports **checked-clean / checked-failed / could-not-check** — never two states. A
file you could not open, a PDF whose text you could not extract, an image whose metadata you could
not read, and a repo history you did not look at are all **could-not-check**, stated as such.

Run the repo's disclosure scanner over the staged tree, where it ships one, and paste its
coverage and boundary lines into the review. Then read what it says it could not see, and cover that yourself. **A clean
scan is one instrument's narrow verdict, never the disclosure verdict** — a 29/29-green
pre-publication sweep was followed by an independent review that found real leaks in the same tree,
and a scanner is blind to a paraphrased judgement, an unregistered person, and anything outside the
tree it walked. The reviewer owns the verdict; the tool owns the coverage report.

**The reviewer does not strip candour-class text either.** Report it with the KEEP/REMOVE reasoning
and let the author or a human decide. The tool exits 4 rather than failing the build for exactly
this reason.
