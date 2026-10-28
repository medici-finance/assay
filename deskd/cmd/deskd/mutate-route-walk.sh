#!/usr/bin/env bash
# mutate-route-walk.sh — fail-first evidence for the served route-walk
# (the authorization contract, worker-prompt kit §9 and §14).
#
# WHY MUTATION AND NOT A PRE-FIX RED RUN. The tests this change adds call
# buildServedMux, which the change also adds; at the pre-fix commit they do not
# compile, and a build error is not evidence. So each case below plants ONE
# route that skips the identity wrap, the way a plausible future edit would,
# and asserts the matching test then FAILS. The first case is the issue's
# mutation, verbatim, at both places it can now be written; the rest are the
# reviews' findings on #248, by round. Round 1: F1 a helper in served.go; F2 a
# Handler reassignment; S1-S3 and the ServeHTTP sibling (a second server, an
# aliased import, a method value, path dispatch inside the recorder). Round 2:
# F3 net/http imported twice under different names; F4 the recorder's inner mux
# reached from buildServedMux or swapped in main(); A3 the recorder's body
# changed with its statement count intact; S1 siblings: a rebound or shadowed
# `served`, serving packages (fcgi, httptest) and a registering import (pprof).
#
# It never edits the working tree: the module is copied to a tempdir, one
# mutation is applied to the copy, and the copy is thrown away. It is fully
# offline: no network, no cluster (KUBECONFIG=/dev/null).
#
# Usage:  cmd/deskd/mutate-route-walk.sh
# Exit:   0 every planted route was caught
#         1 a planted route was not caught
#         2 usage / environment error
set -uo pipefail
export KUBECONFIG=/dev/null

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)

command -v go >/dev/null 2>&1 || { echo "go is not on PATH" >&2; exit 2; }
[ -f "$repo/go.mod" ] || { echo "no go.mod at $repo" >&2; exit 2; }

tmp=$(mktemp -d "${TMPDIR:-/tmp}/mutate-route-walk.XXXXXX") || exit 2
trap 'rm -rf "$tmp"' EXIT

fails=0
cases=0

fresh_copy() {
  local d="$tmp/$1"
  rm -rf "$d"; mkdir -p "$d"
  (cd "$repo" && tar --exclude='./.git' --exclude='./cockpit/node_modules' --exclude='./web/node_modules' -cf - .) | (cd "$d" && tar -xf -)
  echo "$d"
}

# mutate <dir> <file> <perl-expr> — apply a perl -0 substitution and insist it
# changed the file (a mutation that silently matches nothing proves nothing).
mutate() {
  local dir=$1 file=$2 expr=$3
  local before after
  before=$(cksum < "$dir/$file")
  perl -0pi -e "$expr" "$dir/$file"
  after=$(cksum < "$dir/$file")
  if [ "$before" = "$after" ]; then
    echo "MUTATION DID NOT APPLY: $file :: $expr" >&2
    exit 2
  fi
}

# expect_red <label> <dir> <test-regex>
expect_red() {
  local label=$1 dir=$2 re=$3
  cases=$((cases + 1))
  local out
  out=$(cd "$dir" && go test -count=1 -timeout 180s -run "$re" ./cmd/deskd/ 2>&1)
  if [ $? -eq 0 ]; then
    fails=$((fails + 1))
    printf 'NOT CAUGHT: %s\n  %s still PASSES with the route planted\n' "$label" "$re"
  else
    printf 'red as expected: %s\n' "$label"
    if printf '%s\n' "$out" | grep -q -E 'undefined|build failed|syntax error|declared and not used'; then
      fails=$((fails + 1))
      printf 'RED FOR THE WRONG REASON (build error): %s\n' "$label"
    fi
    printf '%s\n' "$out" | grep -E '^\s+[a-z0-9_]+\.go:[0-9]+' | head -2 | sed 's/^/    /'
  fi
}

S=cmd/deskd/served.go
M=cmd/deskd/main.go
after_session='s|(\ttop\.Handle\("/session/", d\.Session\)\n)|$1'

echo "== the issue's mutation, inside buildServedMux: a read route beside /session =="
d=$(fresh_copy issue-served)
mutate "$d" $S "${after_session}\tv1API := d.Routes.V1\n\ttop.Handle(\"/v1/inbox\", v1API)\n|"
expect_red "top.Handle(\"/v1/inbox\", v1API) beside the /session mounts" "$d" '^TestServedRouteWalk$'

echo "== the issue's mutation, in main(): a mux around the served one =="
d=$(fresh_copy issue-main)
mutate "$d" $M 's|(\tsrv := &http\.Server\{\n\t\tAddr:              \*addr,\n\t\tHandler:           )served,|\ttop := http.NewServeMux()\n\ttop.Handle("/", served)\n\ttop.Handle("/v1/inbox", v1API)\n$1top,|'
expect_red "main() serves a mux carrying top.Handle(\"/v1/inbox\", v1API)" "$d" '^TestServedMuxIsTheOnlyMux$'
d=$(fresh_copy issue-main-direct)
mutate "$d" $M 's|(\tsrv := &http\.Server\{)|\tserved.Handle("/v1/inbox", v1API)\n$1|'
expect_red "main() mounts on the mux buildServedMux returned" "$d" '^TestServedMuxIsTheOnlyMux$'

echo "== the class: a new prefix tree =="
d=$(fresh_copy prefix)
mutate "$d" $S "${after_session}\ttop.Handle(\"/v2/\", d.Routes.V1)\n|"
expect_red "top.Handle(\"/v2/\", ...) outside the wrap" "$d" '^TestServedRouteWalk$'

echo "== the class: a HandleFunc mount =="
d=$(fresh_copy handlefunc)
mutate "$d" $S "${after_session}\ttop.HandleFunc(\"/healthz\", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })\n|"
expect_red "top.HandleFunc(\"/healthz\", ...) outside the wrap" "$d" '^TestServedRouteWalk$'

echo "== the class: the guard loop mounts the bare handler =="
d=$(fresh_copy unwrapped)
mutate "$d" $S 's|top\.Handle\(r\.Pattern, d\.Guard\.Handler\(r\)\)|top.Handle(r.Pattern, r.Handler)|'
expect_red "every table route mounted without d.Guard" "$d" '^TestServedRouteWalk$'

echo "== review F1 (#248): a helper in served.go, outside buildServedMux, mounts on the returned mux =="
d=$(fresh_copy helper-in-served)
printf '\nfunc mountExtras(m *servedMux, h http.Handler) { m.Handle("/v1/inbox", h) }\n' >> "$d/$S"
mutate "$d" $M 's|(\tsrv := &http\.Server\{)|\tmountExtras(served, v1API)\n$1|'
expect_red "mountExtras(served, v1API) called from main()" "$d" '^TestServedMuxIsTheOnlyMux$'

echo "== review F2 (#248): srv.Handler reassigned after the literal =="
d=$(fresh_copy handler-reassigned)
mutate "$d" $M 's|(\t\tReadHeaderTimeout: 10 \* time\.Second,\n\t\}\n)|$1\tsrv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n\t\tif r.URL.Path == "/v1/inbox" {\n\t\t\tv1API.ServeHTTP(w, r)\n\t\t\treturn\n\t\t}\n\t\tserved.ServeHTTP(w, r)\n\t})\n|'
expect_red "srv.Handler = a HandlerFunc that serves v1API at /v1/inbox" "$d" '^TestServedMuxIsTheOnlyMux$'

echo "== security S1-S3 (#248): sibling shapes of F1/F2 =="
d=$(fresh_copy second-server)
mutate "$d" $M 's|(\tsrv := &http\.Server\{)|\ts2 := new(http.Server)\n\ts2.Handler = v1API\n\tgo s2.ListenAndServe()\n$1|'
expect_red "a second listener built with new(http.Server) serving v1API" "$d" '^TestServedMuxIsTheOnlyMux$'
d=$(fresh_copy import-alias)
printf 'package main\n\nimport nh "net/http"\n\nfunc init() { go nh.ListenAndServe(":1", nil) }\n' > "$d/cmd/deskd/extra.go"
expect_red "nh.ListenAndServe through an aliased net/http import" "$d" '^TestServedMuxIsTheOnlyMux$'
d=$(fresh_copy method-value)
mutate "$d" $M 's|(\tsrv := &http\.Server\{)|\tmountVia := served.Handle\n\tmountVia("/v1/inbox", v1API)\n$1|'
expect_red "h := served.Handle; h(\"/v1/inbox\", v1API) in main()" "$d" '^TestServedMuxIsTheOnlyMux$'
d=$(fresh_copy servehttp-dispatch)
mutate "$d" $S 's|(func \(m \*servedMux\) ServeHTTP\(w http\.ResponseWriter, r \*http\.Request\)) \{ m\.mux\.ServeHTTP\(w, r\) \}|$1 {\n\tif r.URL.Path == "/v1/inbox" {\n\t\tw.WriteHeader(http.StatusOK)\n\t\treturn\n\t}\n\tm.mux.ServeHTTP(w, r)\n}|'
expect_red "servedMux.ServeHTTP answers /v1/inbox itself" "$d" '^TestServedMuxIsTheOnlyMux$'

echo "== review round 2 (#248): F3, F4, A3 and the security-lane S1 siblings =="
d=$(fresh_copy f3-double-import)
printf 'package main\n\nimport (\n\t"net/http"\n\tnh "net/http"\n)\n\nfunc init() { _ = http.StatusOK; go nh.ListenAndServe("127.0.0.1:0", nil) }\n' > "$d/cmd/deskd/extra.go"
expect_red "go nh.ListenAndServe with net/http imported twice (F3)" "$d" '^TestServedMuxIsTheOnlyMux$'
d=$(fresh_copy f4a-inner-mux)
mutate "$d" $S "${after_session}\ttop.mux.Handle(\"/v9/\", d.Routes.V1)\n|"
expect_red "top.mux.Handle(\"/v9/\", ...) inside buildServedMux, bypassing the recorder (F4a)" "$d" '^TestServedMuxIsTheOnlyMux$'
d=$(fresh_copy f4b-default-mux)
mutate "$d" $M 's|(\tsrv := &http\.Server\{)|\tserved.mux = http.DefaultServeMux\n$1|'
printf 'package main\n\nimport _ "net/http/pprof"\n' > "$d/cmd/deskd/extra.go"
expect_red "served.mux = http.DefaultServeMux plus a blank net/http/pprof import (F4b)" "$d" '^TestServedMuxIsTheOnlyMux$'
d=$(fresh_copy a3-recorder-content)
mutate "$d" $S 's|\tm\.mounts = append\(m\.mounts, pattern\)|\tif pattern != "/v9/" {\n\t\tm.mounts = append(m.mounts, pattern)\n\t}|'
mutate "$d" $S "${after_session}\ttop.Handle(\"/v9/\", d.Routes.V1)\n|"
expect_red "Handle skips recording /v9/ and buildServedMux mounts it (A3)" "$d" '^TestServedMuxIsTheOnlyMux$'
d=$(fresh_copy s1b-shadowed-served)
mutate "$d" $M 's|(\tsrv := &http\.Server\{)|\t{\n\t\tserved := v1API\n\t\ts2 := &http.Server{Addr: ":1", Handler: served}\n\t\tgo s2.ListenAndServe()\n\t}\n$1|'
expect_red "served := v1API beside a second http.Server literal (S1b)" "$d" '^TestServedMuxIsTheOnlyMux$'
d=$(fresh_copy s1b-rebound-served)
mutate "$d" $M 's|(\tsrv := &http\.Server\{)|\tserved = nil\n$1|'
expect_red "served rebound after buildServedMux returned (S1b)" "$d" '^TestServedMuxIsTheOnlyMux$'
d=$(fresh_copy s1c-fcgi)
printf 'package main\n\nimport "net/http/fcgi"\n\nvar _ = fcgi.Serve\n' > "$d/cmd/deskd/extra.go"
expect_red "net/http/fcgi imported in a non-test file (S1c)" "$d" '^TestServedMuxIsTheOnlyMux$'
d=$(fresh_copy s1c-httptest)
printf 'package main\n\nimport "net/http/httptest"\n\nvar _ = httptest.NewServer\n' > "$d/cmd/deskd/extra.go"
expect_red "net/http/httptest imported in a non-test file (S1c)" "$d" '^TestServedMuxIsTheOnlyMux$'

echo "== review round 3 (#248): F6, the constructor body and the Mounts accessor =="
d=$(fresh_copy f6a-constructor-mount)
mutate "$d" $S 's|func newServedMux\(\) \*servedMux \{ return &servedMux\{mux: http\.NewServeMux\(\)\} \}|func newServedMux() *servedMux {\n\tm := &servedMux{mux: http.NewServeMux()}\n\tm.mux.Handle("/v9/", http.FileServer(http.Dir("/")))\n\treturn m\n}|'
expect_red "newServedMux mounts a file server on the inner mux before returning (F6a)" "$d" '^TestServedMuxIsTheOnlyMux$'
d=$(fresh_copy f6b-mounts-filter)
mutate "$d" $S 's|return append\(\[\]string\(nil\), m\.mounts\.\.\.\)|return slices.DeleteFunc(append([]string(nil), m.mounts...), func(p string) bool { return p == "/v9/" })|'
mutate "$d" $S 's|import \(|import (\n\t"slices"|'
mutate "$d" $S "${after_session}\ttop.Handle(\"/v9/\", d.Routes.V1)\n|"
expect_red "Mounts filters /v9/ out of the record and buildServedMux mounts it (F6b)" "$d" '^TestServedMuxIsTheOnlyMux$'

echo
echo "cases: $cases, not caught: $fails"
[ "$fails" -eq 0 ] || exit 1
exit 0
