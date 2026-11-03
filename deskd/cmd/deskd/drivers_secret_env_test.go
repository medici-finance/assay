package main

import (
	"testing"

	"github.com/medici-finance/assay/deskd/config/schema"
)

// regression: F-security-surface — startup never told the local driver which
// variable holds the chat subscription key, so the driver could not strip it.
func TestConfiguredSecretEnvCoversEveryChatCell(t *testing.T) {
	cfg, err := schema.Parse([]byte(`cells:
  - name: assay
    repos: [example-org/console]
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [glm-5.2]
        app: assay-reviewer-app
    silence_threshold: 48h
    notification_tiers:
      review: push
      question: badge
      notify: none
    chat:
      models: [glm-5.2]
      key_env: MODEL_SUBSCRIPTION
  - name: quiet
    repos: [example-org/other]
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [glm-5.2]
        app: other-reviewer-app
    silence_threshold: 48h
    notification_tiers:
      review: push
      question: badge
      notify: none
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := configuredSecretEnv(cfg)
	if len(got) != 1 || got[0] != "MODEL_SUBSCRIPTION" {
		t.Fatalf("configuredSecretEnv = %v, want [MODEL_SUBSCRIPTION]", got)
	}
	if configuredSecretEnv(nil) != nil {
		t.Fatalf("a nil config must register nothing")
	}
}
