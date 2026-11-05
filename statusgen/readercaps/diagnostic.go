package readercaps

import (
	"fmt"
	"strings"
)

// DiagnosticLimit bounds one reader refusal before a host prints or logs it.
const DiagnosticLimit = 2048

// Diagnostic emits single-line ASCII text. Record values cannot inject terminal
// controls or grow an audit row past its reader's line limit. Reapplying it to
// truncated text preserves that bound but may extend the truncation marker.
func Diagnostic(raw string) string {
	const marker = "... [truncated]"
	var out strings.Builder
	for _, r := range raw {
		part := string(r)
		if r < 0x20 || r > 0x7e {
			quoted := fmt.Sprintf("%+q", r)
			part = quoted[1 : len(quoted)-1]
		}
		if out.Len()+len(part) > DiagnosticLimit-len(marker) {
			out.WriteString(marker)
			return out.String()
		}
		out.WriteString(part)
	}
	return out.String()
}
