package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskfilestub"
)

// Tests in this package drive filing paths (the gateway's quarantine filer). A test
// run from a desk shell has the real deskfile, a role token and a default repo
// to hand, so an unguarded filing path opens a real issue. Every test here must
// see the refusing stub instead.
//
// regression: #2726
func TestNoRealDeskfile(t *testing.T) { deskfilestub.AssertStubbed(t) }
