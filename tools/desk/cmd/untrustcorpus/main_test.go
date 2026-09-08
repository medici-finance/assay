package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const shippedRoot = "../../internal/deskkit/untrustcorpus/testdata"

// runCLI drives run() with a captured stdout/stderr.
func runCLI(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestCheckOK(t *testing.T) {
	code, out, errb := runCLI("check", "--root", shippedRoot)
	if code != deskkit.ExitOK {
		t.Fatalf("check exit = %d, want %d (stderr: %s)", code, deskkit.ExitOK, errb)
	}
	if !strings.Contains(out, "corpus-ok") {
		t.Errorf("check stdout = %q, want it to contain corpus-ok", out)
	}
}

func TestListEnumerates(t *testing.T) {
	code, out, _ := runCLI("list", "--root", shippedRoot)
	if code != deskkit.ExitOK {
		t.Fatalf("list exit = %d, want %d", code, deskkit.ExitOK)
	}
	for _, want := range []string{"exfil-callout-init", "injection-unicode-tagblock", "benign-prose"} {
		if !strings.Contains(out, want) {
			t.Errorf("list stdout missing %q; got:\n%s", want, out)
		}
	}
}

func TestCheckUnreadableRootIsUnverifiable(t *testing.T) {
	code, _, errb := runCLI("check", "--root", "/no/such/corpus/dir")
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("check on missing root exit = %d, want %d", code, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(errb, "could-not-determine") {
		t.Errorf("stderr = %q, want could-not-determine", errb)
	}
}

func TestUnknownSubcommandRefused(t *testing.T) {
	code, _, _ := runCLI("frobnicate", "--root", shippedRoot)
	if code != deskkit.ExitRefused {
		t.Fatalf("unknown subcommand exit = %d, want %d", code, deskkit.ExitRefused)
	}
}
