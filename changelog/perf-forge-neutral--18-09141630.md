### Added
- `forge-neutral/18` — a brief for taking **statusgen off the forge CLI**. statusgen is the last
  tool in the suite that reaches a forge on its own: a separate Go module that does not import
  `deskkit`, carrying 26 of its own forge-CLI shell-outs, so the enumerated operation set, the
  refusal-instead-of-fallback rule and the per-forge backend all stop at its module boundary. The
  brief adds to desk-tools exactly the read statusgen needs — `deskread`, a read-only verb over
  operations the seam ALREADY enumerates, so the frozen `Forge` surface is consumed rather than
  widened — and makes `--lint` offline by default, with every forge-backed check reporting
  could-not-check as itself rather than reading green because it stopped looking.
- The brief's performance half, measured rather than asserted: on this repository (24 streams,
  165 briefs) a `--lint` spends **16.7 s of its 23.7 s** inside 11 forge-CLI subprocesses — 13.9 s
  of that fetching every issue ever opened across ten configured repos to print one advisory line
  whose inputs are open issues only — and makes **254 git subprocesses**, of which one whole-tree
  authorship walk (0.03 s) replaces 144 `git log` plus 62 `git blame` invocations. A memo closes
  the 3,351 brief parses that re-read 172 files up to 23 times each. Target: zero forge-CLI calls,
  100 git subprocesses or fewer, and a 60 % or better wall-clock cut on a 400-brief tree.
