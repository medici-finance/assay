# shellcheck shell=bash
# Expected-output adapter for the declared Go-only fixture extensions. The shell
# oracle stays frozen; both parity suites read these independent expectation files.
# Called only for the exact frozen oracle, on the named fixture scenarios. Never
# removes or rewrites Go output, diagnostics, exit codes, plans or scaffold files.
# See docs/cellctl-cli-compat.md, "The house cell's forge label set".
expect_extensions(){
  local impl="$1" kind="$2" harness="$3" verb="$4" cell="${5:-}"
  local actual oracle
  actual="$(cd "$(dirname "$impl")" && pwd -P)/$(basename "$impl")"
  oracle="$(cd "$HERE/../testdata" && pwd -P)/cellctl-shell-oracle.sh"
  if [[ "$actual" != "$oracle" ]]; then cat; return; fi
  local row
  if [[ "$harness/$verb" == codex/set ]]; then
    # The intake-pin delivery's (48f213f2e) post-write namespace notice: the fixture writes a Claude pin while
    # running Codex. Its value/write/backup and exit code stay compared unchanged.
    row="$(cat "$HERE/../testdata/parity/set-codex.txt")" || return 2
    awk -v row="$row" '{ print } $0 == "[set] DESK_MODEL_DEFAULT: sonnet -> haiku" { print row }'
    return
  fi
  if [[ "$kind" != house ]]; then cat; return; fi
  case "$verb" in
    check)
      row="$(cat "$HERE/../testdata/parity/labels-check.txt")" || return 2
      awk -v row="$row" '
        added && $0 == "[check] all preconditions met" {
          print "[check] every checked precondition met; 1 could-not-check row(s) (?) — not clean until each is checked"; next
        }
        { print }
        $0 == "  ok    root example-org/example-repo carries docs/streams/" { print row; added=1 }
      '
      ;;
    new)
      row="$(sed 's#<repo>#example-org/example-repo#g' "$HERE/../testdata/parity/labels-new.txt")" || return 2
      awk -v row="$row" -v cell="$cell" '
        { print }
        $0 == "[new] scaffolded <root>/cells/" cell " (house cell) — cellctl check " cell ", then cellctl desk " cell " <role>" { print row }
      '
      ;;
    *) cat ;;
  esac
}
