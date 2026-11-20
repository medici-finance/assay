package deskkit

import (
	"math/rand"
	"strings"
	"testing"
)

// Rule 9 (Go GODEBUG settings-list arm, #6821). Every literal here is split so no fragment
// reaches runThreshold on the page: deskpr scans the diff that adds this file with
// whichever scanner is installed, not the one in the diff.

const (
	// godebugHead is the build-info key `go version -m` prints for a binary's defaults.
	godebugHead = "Default" + "GODEBUG="
	// godebugSettings is the reported row's settings list, minus its head. The first
	// item, glued to the head, is the 33-char run the scan reported.
	godebugSettings = "crypto" + "customrand=1,httpservecontentmaxranges=1," +
		"tlssecpmlkem=0,tracebacklabels=0,urlstrictcolons=0,x509sslcertoverrideplatform=0"
	// godebugMixed40 is synthetic mixed-case base64, 40 characters.
	godebugMixed40 = "kP3vQ8xLm2Rt7WnY4bZc" + "Hs6JdF9aE1uG5oVyTqXi"
)

// regression: #6821
func TestBodycheckGoBuildInfoLine(t *testing.T) {
	line := "\tbuild\t" + godebugHead + godebugSettings
	if err := BodyCheck([]byte(line)); err != nil {
		t.Fatalf("the build-info DefaultGODEBUG row is refused: %v", err)
	}
	dump := "app: go1.27.4\n\tpath\texample.com/app\n\tbuild\t-trimpath=true\n" +
		line + "\n\tbuild\tCGO_ENABLED=0\n\tbuild\tGOOS=linux\n"
	if err := BodyCheck([]byte(dump)); err != nil {
		t.Fatalf("a build-info block carrying the row is refused: %v", err)
	}
}

// regression: #6821
func TestBodycheckGoDebugEnvForm(t *testing.T) {
	// The same list as an environment assignment: the head is GODEBUG, and a run
	// reaches the threshold once the first setting's name is 22 characters or more.
	for _, body := range []string{
		"run GODEBUG=httpservecontent" + "keepheaders=1 go test ./...",
		"export GODEBUG=x509sslcert" + "overrideplatform=0,tlssecpmlkem=0",
		"`GODEBUG=httpservecontent" + "maxranges=1`",
	} {
		if err := BodyCheck([]byte(body)); err != nil {
			t.Errorf("GODEBUG env assignment %q refused: %v", body, err)
		}
	}
}

// regression: #6821
func TestBodycheckGoDebugControls(t *testing.T) {
	refused := []struct{ name, body string }{
		{"token value behind the head", "\tbuild\t" + godebugHead + godebugMixed40},
		{"mixed-case name", godebugHead + godebugMixed40[:20] + "=1"},
		{"hex-only name", godebugHead + hexRun(32) + "=1"},
		{"hex-letter name, few digits", godebugHead + "fedcbaabcdef" + "fedcba12=1"},
		{"name past the length cap", godebugHead + strings.Repeat("ab", 14) + "z=1"},
		{"name carrying seven digits", godebugHead + "tls1234567server=1"},
		{"name opening with a digit", godebugHead + "9cryptocustomrand=1"},
		{"value that is not digits", godebugHead + "cryptocustomrand=yes"},
		{"value of five digits", godebugHead + "cryptocustomrand=12345"},
		{"empty value", godebugHead + "httpservecontentmaxranges="},
		{"third '=' in the run", godebugHead + "cryptocustomrand=1=1"},
		{"slash glued to the value", godebugHead + "cryptocustomrand=1/abc"},
		{"plus glued to the value", godebugHead + "cryptocustomrand=1+abc"},
		{"head glued behind an env prefix", "TOKEN_" + godebugHead + "cryptocustomrand=1"},
		{"unlisted head", "MY" + "GODEBUG=httpservecontent" + "keepheaders=1"},
		{"list item that is not a setting", godebugHead + "cryptocustomrand=1,Tk9vQ2xL"},
		{"list item glued to more run bytes", godebugHead + "cryptocustomrand=1,tlssecpmlkem=0_x"},
	}
	for _, c := range refused {
		if err := BodyCheck([]byte(c.body)); !IsRefused(err) {
			t.Errorf("%s: %q admitted (err=%v)", c.name, c.body, err)
		}
	}
}

// TestBodycheckGoDebugOnRandom pins what isGoDebugList costs on token material. A random
// base64 run cannot open with the literal head, so it admits none; behind the head, a
// random value with no `=<digits>` tail admits none either. The one shape the rule
// admits is a lowercase-alphanumeric name behind the head and in front of `=<digits>`,
// so every admitted sample is checked to be exactly that.
func TestBodycheckGoDebugOnRandom(t *testing.T) {
	const trials = 200000
	rng := rand.New(rand.NewSource(20261118))
	rnd := func(n int) string {
		b := make([]byte, n)
		for j := range b {
			b[j] = base64Alphabet[rng.Intn(len(base64Alphabet))]
		}
		return string(b)
	}
	for _, n := range []int{runThreshold, 40} {
		plain, valueOnly := 0, 0
		for i := 0; i < trials; i++ {
			if s := rnd(n); isGoDebugList(s, 0) {
				plain++
			}
			if s := godebugHead + rnd(n); isGoDebugList(s, 0) {
				valueOnly++
			}
		}
		if plain > 0 || valueOnly > 0 {
			t.Errorf("%d-char runs: random admitted %d, head+random admitted %d (ceiling 0)", n, plain, valueOnly)
		}
	}
	admitted := 0
	for i := 0; i < trials; i++ {
		s := godebugHead + rnd(20) + "=1"
		if !isGoDebugList(s, 0) {
			continue
		}
		admitted++
		if name := s[len(godebugHead) : len(s)-2]; strings.ToLower(name) != name || strings.ContainsAny(name, "+/=") {
			t.Errorf("admitted a name outside lowercase alphanumerics: %q", name)
		}
	}
	t.Logf("head + random 20-char base64 name + =1: admitted %d of %d", admitted, trials)
}
