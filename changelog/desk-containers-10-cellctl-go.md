### Changed
- `cellctl` is now a Go program (`tools/desk/cmd/cellctl`), built and shipped like every other
  desk verb instead of being a `sed`-stamped shell script copied into the release tarball. The
  shell launcher stays in the tree as the parity ORACLE: `tools/cellctl/tests/parity.test.sh`
  diffs both implementations' `DRY_RUN=1` plans across every kind × harness × cockpit × verb —
  200 cells, byte for byte, including the whole tree `new` scaffolds — and the existing
  behavioural suites now run against either implementation through a `CELLCTL` override.
- The per-platform tarballs now carry a real `cellctl` built for their own platform, the windows
  legs included, where before every platform got the same shell script. The Windows build
  compiles today but is UNPROVEN: standing a Windows cell up belongs to the windows-port stream.
- `cellctl` now declares its tool class and writes the P3 effective-config echo to stderr once per
  run, like every other desk verb that reads the roster — it consults the cell home's roster to
  answer `check`'s write-authorisation rows, so a narrowing of that surface is now visible at run
  time rather than only in a diff. The class is the write class: the cell's config-home file is the
  only admissible source, never the environment. Scripts that parse `cellctl` output should read
  stdout, which is unchanged; the echo is stderr-only and the parity oracle normalises it away.
- `cellctl new` no longer writes internal stream identifiers into the README and roster it
  scaffolds. The grammar, custody and check sentences keep their meaning; only the citations are
  gone, so a scaffolded cell no longer carries pointers to material an adopter cannot read.
