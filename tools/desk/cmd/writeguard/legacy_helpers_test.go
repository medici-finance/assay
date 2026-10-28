package main

import "github.com/medici-finance/assay/deskcore/writepolicy"

// These names preserve the inherited regression cases. They delegate to the
// production observation/evaluation path; no legacy decision code remains here.
func (c Config) CheckFilePath(tool, target string) Verdict {
	c.SharedOK = false
	c.claim = nil
	v, _, _ := c.evaluate(tool, target)
	return v
}
func (c Config) CheckBash(target string) Verdict { return c.CheckFilePath("Bash", target) }
func (c Config) CheckClaimSentinel(tool, target string) Verdict {
	v, _, _ := c.evaluate(tool, target)
	return v
}
func (c Config) Exempt() bool {
	c.SentinelPath = ""
	c.Callout = ""
	_, out, _ := c.evaluate("Write", c.SharedRoot+"/exemption-probe")
	return out.Reason == writepolicy.Exempt
}
func sharedOKClaimed(root string) bool {
	claim := observeClaim()
	return (Config{SharedRoot: root, ProjectDir: root, Cwd: root, claim: &claim}).Exempt()
}
func pathTokens(command string) []string {
	return writepolicy.Classify(writepolicy.Request{Tool: "Bash", Target: command}, writepolicy.LexicalContext{Platform: writepolicy.POSIX}).PathTokens()
}
func maskLiterals(command string) string {
	return writepolicy.Classify(writepolicy.Request{Tool: "Bash", Target: command}, writepolicy.LexicalContext{Platform: writepolicy.POSIX}).Masked()
}
