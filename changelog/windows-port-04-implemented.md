### Changed
- **windows-port/04 board row flipped to `implemented`.** The Windows CI leg is delivered — the
  staged `windows-ci-leg.yml` landed (#569) and was promoted into `.github/workflows/` (#583),
  where the `windows-smoke` job runs green at `d684440` on the LF checkout the repo
  `.gitattributes` provides (#584/#585). The status flip was omitted from those PRs and is
  recorded here; `gate: human` verification of the Verify rows remains a separate step. (#592)
