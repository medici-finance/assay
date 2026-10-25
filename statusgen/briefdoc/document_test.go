package briefdoc

import (
	"strings"
	"testing"
)

func TestQuotedRiskAndMalformed(t *testing.T) {
	for _, input := range []string{`risk: {regulatory: yes}`, `risk: {regulatory: "yes"}`} {
		f, e := ParseFields(input)
		if e != nil || !f.Risk.Regulatory {
			t.Fatal(input, f, e)
		}
	}
	for _, input := range []string{"risk: [yes]", "gate: unsupported", "exec-tier: unsupported", "risk: {typo: yes}", "risk: {regulatory: maybe}", "gate: model\ngate: human", "risk: {regulatory: no, regulatory: yes}"} {
		if _, e := ParseFields(input); e == nil {
			t.Fatalf("malformed frontmatter accepted: %s", input)
		}
	}
}
func TestSectionConsumers(t *testing.T) {
	for _, heading := range []string{"## Verify", " ## Verify (executable)", "##\tVerify (executable)", "##   Verify"} {
		body := heading + "\r\nCOMMAND\r\n## Evidence\r\nOBSERVED\r\n"
		if strings.TrimSpace(Section(body, "Verify", true)) != "COMMAND" || strings.TrimSpace(Section(body, "Evidence", false)) != "OBSERVED" {
			t.Fatal(heading)
		}
	}
}
