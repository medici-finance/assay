package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kc "github.com/medici-finance/assay/tools/desk/internal/kubecompat"
	"github.com/medici-finance/assay/tools/desk/internal/kubecompat/native"
	"github.com/medici-finance/assay/tools/desk/internal/kubecompat/testfixture"
)

// The qualification fixture owns this receipt. A role's running/clean-stopped
// lifecycle receipt does not authorize a qualification target.
type qualificationReceipt struct {
	Run          string `json:"run"`
	ConfigDigest string `json:"config_digest"`
}

// Admit only this disposable run's exact target before invoking the ordinary
// cellctl command. No environment discovery or Kubernetes request is needed.
func offlineQualificationTarget(optIn bool, runID, profilePath, receiptPath string) error {
	if !optIn {
		return errors.New("explicit qualification opt-in required")
	}
	profile, digest, err := kc.Load(profilePath, "example")
	if err != nil {
		return err
	}
	if _, err := native.Config(profile); err != nil {
		return err
	}
	raw, err := os.ReadFile(receiptPath)
	if err != nil {
		return errors.New("run ownership receipt unavailable")
	}
	var admitted qualificationReceipt
	if kc.StrictJSON(raw, &admitted) != nil || runID == "" || admitted.Run != runID || admitted.ConfigDigest != digest {
		return errors.New("run ownership receipt differs")
	}
	return nil
}

// regression: #7348
func TestKubeCompatCustodyTarget(t *testing.T) {
	cases := []struct {
		name   string
		change func(*kc.Profile)
		reason string
	}{
		{"admitted", nil, ""},
		{"opt-in", nil, "explicit qualification opt-in required"},
		{"endpoint", nil, "selected API/TLS identity differs"},
		{"tls-name", func(p *kc.Profile) { p.TLSServerName = "other.invalid" }, "selected API/TLS identity differs"},
		{"ca-pin", func(p *kc.Profile) { p.CASHA256 = strings.Repeat("0", 64) }, "selected CA pin differs"},
		{"context", func(p *kc.Profile) { p.Context = "other-fixture" }, "selected context or namespace differs"},
		{"namespace", func(p *kc.Profile) { p.Namespace = "other-fixture" }, "selected context or namespace differs"},
		{"profile", func(p *kc.Profile) { p.Schema = "wrong-fixture-profile" }, "invalid explicit compatibility target"},
		{"valid-other-profile", func(p *kc.Profile) { p.Custody.Operator = "other-fixture-custodian" }, "run ownership receipt differs"},
		{"other-run", nil, "run ownership receipt differs"},
		{"missing-receipt", nil, "run ownership receipt unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, api := kubeWorld(t)
			runID := rand.Text()
			_, digest, err := kc.Load(api.ProfilePath, "example")
			if err != nil {
				t.Fatal(err)
			}
			receiptPath := filepath.Join(w.root, "qualification-target.json")
			raw, err := json.Marshal(qualificationReceipt{Run: runID, ConfigDigest: digest})
			if err != nil {
				t.Fatal(err)
			}
			if tc.name != "missing-receipt" {
				if err := os.WriteFile(receiptPath, raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			var otherAPI *testfixture.Server
			if tc.name == "endpoint" {
				otherAPI = testfixture.New(t)
				api.Profile.Endpoint = otherAPI.HTTP.URL
				api.Save(t)
			} else if tc.change != nil {
				tc.change(api.Profile)
				api.Save(t)
			}
			if tc.name == "other-run" {
				runID = rand.Text()
			}
			err = offlineQualificationTarget(tc.name != "opt-in", runID, api.ProfilePath, receiptPath)
			if err == nil {
				result := w.run(t, nil, "desk", "example", "worker-desk")
				if result.Code != 0 {
					t.Fatal(result)
				}
			}
			if tc.reason == "" {
				if err != nil || api.ContactCount() == 0 || api.ConnectionCount() == 0 {
					t.Fatalf("admitted fixture: err=%v connections=%d requests=%d", err, api.ConnectionCount(), api.ContactCount())
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("want %q, got %v", tc.reason, err)
			}
			// Exercise the compiled CLI directly for each native refusal too.
			// The ownership receipt and qualification opt-in belong to the suite.
			if tc.name == "endpoint" || (tc.change != nil && tc.name != "valid-other-profile") {
				result := w.run(t, nil, "desk", "example", "worker-desk")
				if result.Code == 0 || !strings.Contains(result.Stderr, tc.reason) {
					t.Fatalf("CLI did not report %q: %+v", tc.reason, result)
				}
			}
			if otherAPI != nil && (otherAPI.ConnectionCount() != 0 || otherAPI.ContactCount() != 0) {
				t.Fatalf("refusal contacted alternate fixture: connections=%d requests=%d", otherAPI.ConnectionCount(), otherAPI.ContactCount())
			}
			if api.ConnectionCount() != 0 || api.ContactCount() != 0 {
				t.Fatalf("refusal contacted fixture: connections=%d requests=%d", api.ConnectionCount(), api.ContactCount())
			}
		})
	}
}
