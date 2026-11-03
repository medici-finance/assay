#!/usr/bin/env python3
"""Offline startup-policy mutation qualification; each source edit is restored."""
from pathlib import Path
import os
import subprocess
ROOT = Path(__file__).resolve().parents[2]
ENV = dict(os.environ, GOWORK="off", GOPROXY="off", GOSUMDB="off", GOTOOLCHAIN="local", KUBECONFIG="/dev/null")
DISABLED = '\tif !humanid.Enabled() {\n\t\treturn humanid.ErrDisabled\n\t}\n'
CASES = [
 ("stale-proving-fixture", "config/schema/cells.go", 'map[string]bool{"assay": true, "house": true}', 'map[string]bool{"assay": true}', "./config/schema", "TestPerUserIdentity_CommittedFixturesKeepTheirVerdicts"),
 ("stale-runtime-policy", "config/schema/cells.go", 'map[string]bool{"assay": true, "house": true}', 'map[string]bool{"assay": true}', "./config/schema", "TestPerUserIdentity_RuntimePredicateReDerivesEveryCondition"),
 ("nested-init", "cmd/deskd/main.go", "humanid.Configure(*humanLogin)", "func() error { return humanid.Configure(*humanLogin) }()", "./internal/humanid", "TestOnlyCommandStartupInitializesPolicy"),
 ("stale-proving-policy", "config/schema/cells.go", 'map[string]bool{"assay": true, "house": true}', 'map[string]bool{"assay": true}', "./config/schema", "TestPerUserIdentity_OptInOnHouseCellWithDevEnvironmentPasses"),
 ("proving-near-miss", "config/schema/cells.go", 'map[string]bool{"assay": true, "house": true}', 'map[string]bool{"assay": true, "house": true, "household": true}', "./config/schema", "TestPerUserIdentity_HouseNearMissNamesStayRefused"),
 ("proving-extra-cell", "config/schema/cells.go", 'map[string]bool{"assay": true, "house": true}', 'map[string]bool{"assay": true, "house": true, "other": true}', "./config/schema", "TestPerUserIdentity_ProvingCellAllowlistIsSmallAndClosed"),
 ("fixed-principal", "internal/humanid/policy.go", "return startup.get()", 'return "operator"', "./cmd/deskd", "TestHumanPolicySelectedPrincipalFlows"),
 ("reconfigure", "internal/humanid/policy.go", "if p.initialized {", "if false {", "./internal/humanid", "TestStartupPolicy"),
 ("invalid-login", "internal/humanid/policy.go", 'if login != "" && (len(login) > 39 || !loginRE.MatchString(login)) {', "if false {", "./internal/humanid", "TestStartupPolicy"),
 ("empty-enables", "internal/humanid/policy.go", "p.login = login", 'p.login = login; if login == "" { p.login = "unexpected" }', "./internal/humanid", "TestStartupPolicy"),
 ("blank-matches", "internal/humanid/policy.go", 'expected != "" && strings.TrimSpace(login) != "" && ', "", "./internal/humanid", "TestUnconfiguredMatchesNobody"),
 ("effectful-import", "internal/humanid/policy.go", "import (", 'import (\n _ "os"', "./internal/humanid", "TestPolicyHasNoEffectfulImports"),
 ("second-init-site", "cmd/deskd/main.go", "func main() {", 'func extraInitializer() { _ = humanid.Configure("operator") }\nfunc main() {', "./internal/humanid", "TestOnlyCommandStartupInitializesPolicy"),
 ("custody-bypass", "internal/custody/custody.go", "if !humanid.Matches(login) {", "if false {", "./cmd/deskd", "TestHumanPolicyExplicitPrincipal"),
 ("writer-bypass", "internal/answer/github.go", '\tif !humanid.Enabled() {\n\t\treturn "", humanid.ErrDisabled\n\t}\n', "", "./cmd/deskd", "TestHumanPolicyDisabledByDefault"),
 ("session-bypass", "internal/answer/session.go", '\tif !humanid.Enabled() {\n\t\treturn nil, humanid.ErrDisabled\n\t}\n', "", "./cmd/deskd", "TestHumanPolicyDisabledByDefault"),
 ("capability-bypass", "internal/identity/capability.go", 'humanid.Enabled() && ', "", "./cmd/deskd", "TestHumanPolicyDisabledByDefault"),
 ("terminal-bypass", "internal/identity/terminal_gate.go", '!humanid.Enabled() || ', "", "./internal/identity", "TestDisabledPolicyRefusesTerminalAuthority"),
 ("live-service-bypass", "internal/answer/answer.go", DISABLED, "", "./internal/answer", "TestDisabledPolicyRefusesDirectSignedAndLivePaths"),
 ("signed-service-bypass", "internal/answer/intent.go", DISABLED, "", "./internal/answer", "TestDisabledPolicyRefusesDirectSignedAndLivePaths"),
 ("local-state-bypass", "internal/index/setters.go", DISABLED, "", "./cmd/deskd", "TestHumanPolicyDisabledByDefault"),
]
def run(package, test):
 return subprocess.run(["go", "test", "-count=1", "-timeout=90s", package, "-run", "^"+test+"$"], cwd=ROOT, env=ENV, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=110)
for package,test in sorted({(x[4],x[5]) for x in CASES}):
 result=run(package,test)
 if result.returncode: raise SystemExit("baseline failed: "+test+"\n"+result.stdout)
for name,rel,old,new,package,test in CASES:
 path=ROOT/rel; original=path.read_text()
 if original.count(old)!=1: raise SystemExit(f"{name}: anchor count {original.count(old)}, expected one")
 try:
  changed=original.replace(old,new)
  if "humanid." not in changed and '"github.com/medici-finance/assay/deskd/internal/humanid"' in changed: changed+='\nvar _ = humanid.Login\n'
  path.write_text(changed); result=run(package,test)
  if result.returncode==0 or "--- FAIL: "+test not in result.stdout: raise SystemExit(f"{name}: expected named assertion failure, got {result.returncode}\n{result.stdout}")
  print(f"RED {name}: {test}",flush=True)
 finally: path.write_text(original)
print(f"PASS: {len(CASES)} mutations rejected; sources restored")
