package main

// defaultAllow holds acronyms and CamelCase-shaped words common enough in
// ordinary technical prose that flagging them would be pure noise. It is
// intentionally short: callers scanning a specific corpus are expected to
// extend it via --allow-file with their own house terms (product names,
// internal codenames) rather than relying on this list to be exhaustive.
var defaultAllowWords = []string{
	// Common tech acronyms (3-8 caps).
	"API", "CLI", "SDK", "URL", "URI", "HTTP", "HTTPS", "JSON", "YAML",
	"HTML", "XML", "CSS", "SQL", "CSV", "PDF", "FAQ", "TODO", "README",
	"MIT", "BSD", "GPL", "RFC", "UTC", "ISO", "LLM", "IDE", "CPU", "GPU",
	"RAM", "UI", "UX", "QA", "MVP", "KPI", "ROI", "SLA", "SLO", "SSO",
	"OAuth", "JWT", "TLS", "SSH", "DNS", "CDN", "AWS", "GCP", "CI", "CD",
	"PR", "OS",
	// Common CamelCase-shaped product/technology names that are not the
	// third-party-competitor claim this tool exists to catch.
	"GitHub", "GitLab", "JavaScript", "TypeScript", "OAuth2", "WebSocket",
	"OpenAI", "DevOps", "MacOS", "iOS",
}

func newDefaultAllow() map[string]bool {
	m := make(map[string]bool, len(defaultAllowWords))
	for _, w := range defaultAllowWords {
		m[w] = true
	}
	return m
}
