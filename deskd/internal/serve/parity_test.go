package serve_test

import (
	"testing"

	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/mcp"
)

// readRouteToTool maps each v1 READ route to the MCP tool that returns the same
// shape (§4, "feature = tool"). It is the SINGLE source the parity guard walks;
// a read route added to the contract without an entry here — or an entry whose
// tool the MCP surface does not register — fails the build, so the two surfaces
// cannot drift. WRITE routes (the named actions) are deliberately absent: the
// agent surface authors nothing as a human (§11), so a named-action write has no
// MCP twin.
var readRouteToTool = map[string]string{
	contract.Prefix + "/inbox":            "desk__inbox_list",
	contract.Prefix + "/escalations/{id}": "desk__item_get",
	contract.Prefix + "/roadmap":          "desk__roadmap",
	contract.Prefix + "/portfolio":        "desk__portfolio",
	contract.Prefix + "/metrics":          "desk__metrics_query",
	contract.Prefix + "/cells":            "desk__status",
	// The cell-override read (the service contract) has its MCP twin desk__overrides
	// — the tool the loops call each tick to read what the HTTP write set.
	contract.Prefix + "/cells/{id}/overrides": "desk__overrides",
	contract.Prefix + "/desks/{id}/terminal":  "desk__desks",
	// The desk registry read (the service contract) shares its MCP twin with the
	// terminal-handle route above: the parity map is keyed by PATH, so one
	// tool legitimately answers for two routes (§4 note in the map's own doc).
	contract.Prefix + "/desks": "desk__desks",
}

// TestContractToolParity — every v1 READ route has an MCP tool
// twin, and every mapped tool is actually registered on the MCP surface. A read
// added to one side without the other fails here, so the HTTP contract and the
// agent surface stay in lockstep. Named-action WRITE routes are asserted to have
// NO twin (the agent surface never authors as a human).
func TestContractToolParity(t *testing.T) {
	// The set of tools the MCP surface actually registers.
	srv := mcp.NewServer(mcp.Options{})
	haveTool := map[string]bool{}
	for _, tool := range srv.Tools() {
		haveTool[tool.Name] = true
	}

	seenRead := 0
	for _, r := range contract.Routes {
		switch r.Kind {
		case contract.KindRead:
			seenRead++
			tool, mapped := readRouteToTool[r.Path]
			if !mapped {
				t.Errorf("v1 READ route %q has no MCP tool twin in readRouteToTool — a read added to the contract "+
					"without its MCP twin (or the mapping) fails parity (§4: feature = tool)", r.Path)
				continue
			}
			if !haveTool[tool] {
				t.Errorf("v1 READ route %q maps to MCP tool %q, but the MCP surface does not register it", r.Path, tool)
			}
		case contract.KindNamedAction:
			// A named-action write must NOT be mirrored as an MCP tool: the agent
			// surface authors nothing as a human (§11). The forbidden names are
			// desk__<action>.
			forbidden := "desk__" + string(r.Action)
			if haveTool[forbidden] {
				t.Errorf("named-action write route %q has an MCP tool %q — the agent surface must not author a human write (§11)", r.Path, forbidden)
			}
		}
	}

	if seenRead == 0 {
		t.Fatal("could-not-check: the contract declares no read routes, so parity proved nothing")
	}
	if seenRead != len(readRouteToTool) {
		t.Errorf("the contract declares %d read routes but the parity map has %d entries — a route was removed from the contract "+
			"without removing its mapping (or vice versa)", seenRead, len(readRouteToTool))
	}
}
