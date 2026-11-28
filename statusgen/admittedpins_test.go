package main

import (
	"os"
	"strings"
	"testing"
)

// clearAdmittedRowPins removes, for this test only, the git config pins that
// admittedRowEnv appends to an admitted Verify row's environment, together
// with GIT_CONFIG_COUNT, so the count never names a key that is no longer
// set. A test that fakes the forge CLI behind ghGuard calls it: when the test
// itself runs as an admitted Verify row, those pins make ghGuard refuse every
// launch, the fake included. The pin names are read from admittedRowEnv, the
// same source admittedRowEnvMarked reads, so they cannot drift from what
// admission writes. t.Setenv restores every variable when the test ends.
// Nothing outside the test changes; ghGuard and admission are untouched.
func clearAdmittedRowPins(t *testing.T) {
	t.Helper()
	names := []string{"GIT_CONFIG_COUNT"}
	for _, kv := range admittedRowEnv(nil) {
		key, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
			names = append(names, key)
		}
	}
	for _, key := range names {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	if admittedRowEnvMarked() {
		t.Fatal("the admitted Verify row pins are still set after clearing them")
	}
}

// regression: #5014
// TestFakeGHUnderRowPins: the autonomy fixture's fake gh starts even when the
// test runs with an admitted Verify row's git config pins set and no forge
// token variable, the environment a verifier's Verify row gives it.
func TestFakeGHUnderRowPins(t *testing.T) {
	for _, kv := range admittedRowEnv(nil) {
		if key, val, _ := strings.Cut(kv, "="); strings.HasPrefix(key, "GIT_CONFIG_") {
			t.Setenv(key, val)
		}
	}
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN"} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
	if !admittedRowEnvMarked() {
		t.Fatal("the planted pins do not mark an admitted Verify row; the test checks nothing")
	}
	root, log := autonomyGHFixture(t, `[{"number":11,"mergedAt":"2026-07-03T12:00:00Z"}]`, goTestLintRollup)
	since, until := autonomyWindow(t)
	prs, ok, cause := autonomyGates(root, since, until)
	if !ok || cause != "" || len(prs) != 1 || prs[0].Number != 11 {
		t.Fatalf("under admitted row pins = %+v, ok=%v, cause=%q; want PR 11 read through the fake gh", prs, ok, cause)
	}
	if lines := gateCalls(t, log); len(lines) != 2 {
		t.Fatalf("the fake gh was not launched twice: %q", lines)
	}
}
