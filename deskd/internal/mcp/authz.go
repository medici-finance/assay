package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// Scope says how [Server.Dispatch] authorizes a tool call BEFORE its handler
// runs. Authentication (the [CallerAuth] gate) answers "who is calling"; Scope
// answers "may that caller touch what this call names". Both are required:
// an authenticated principal is not, by being authenticated, entitled to any
// cell's items or to file into any repository.
type Scope int

const (
	// ScopeCell is the ZERO value, so a tool registered without thinking about
	// authorization is cell-scoped and fails closed. The call must name a
	// configured `cell`, the caller must be a configured loop of that cell
	// (the same cells.yaml join attribution uses), and any `repo` it names
	// must be one of that cell's repositories.
	ScopeCell Scope = iota
	// ScopeNone marks a tool that reads no cell data at all: the delegated
	// could-not-check reads, which return a fixed verdict naming their source.
	// It is never valid on a tool that reads the index or writes the platform;
	// TestEveryDataToolIsCellScoped asserts that.
	ScopeNone
)

// ErrForbidden is a call the caller is authenticated for but not authorized
// to make. It is reported as a tool error, and no handler ran.
type ErrForbidden struct{ Reason string }

func (e *ErrForbidden) Error() string { return "forbidden: " + e.Reason }

// scopeArgs is the slice of every cell-scoped tool's arguments the
// authorization decision reads. Tools keep decoding their own full shape.
type scopeArgs struct {
	Cell string `json:"cell"`
	Repo string `json:"repo"`
}

// authorize is the per-call access decision. It runs for EVERY tools/call,
// in Dispatch, so a handler cannot forget it.
func (s *Server) authorize(t Tool, call Call) error {
	if t.Scope == ScopeNone {
		return nil
	}
	var a scopeArgs
	if err := json.Unmarshal(nonNull(call.Args), &a); err != nil {
		return fmt.Errorf("%s arguments did not decode: %w", t.Name, err)
	}
	cellName := strings.TrimSpace(a.Cell)
	if cellName == "" {
		return &ErrForbidden{Reason: t.Name + " is cell-scoped: name the cell to act on (cell is required)"}
	}
	cell := s.findCell(cellName)
	if cell == nil {
		return &ErrForbidden{Reason: fmt.Sprintf("no such cell %q is configured", cellName)}
	}
	if !s.callerIsLoopOf(cell, call.CallerLogin) {
		return &ErrForbidden{Reason: fmt.Sprintf(
			"caller %q is not a configured loop of cell %q — only that cell's own loops may use its tools", call.CallerLogin, cellName)}
	}
	if repo := strings.TrimSpace(a.Repo); repo != "" && !cellOwnsRepo(cell, repo) {
		return &ErrForbidden{Reason: fmt.Sprintf("repository %q is not one of cell %q's repositories", repo, cellName)}
	}
	return nil
}

// callerIsLoopOf joins the caller login to the cell's configured loops. An
// empty caller (a loopback request that set no X-Desk-Caller) joins nothing.
func (s *Server) callerIsLoopOf(cell *schema.Cell, caller string) bool {
	caller = strings.TrimSpace(caller)
	if caller == "" || s.Dir == nil {
		return false
	}
	_, ok := s.Dir.Lookup(domain.CellName(cell.Name), normaliseCaller(caller))
	return ok
}

func cellOwnsRepo(cell *schema.Cell, repo string) bool {
	for _, r := range cell.Repos {
		if strings.EqualFold(r, repo) {
			return true
		}
	}
	return false
}

// cellRepos is the set of a cell's repositories, for filtering per-repo
// health to the cell the call is authorized for.
func cellRepos(cell *schema.Cell) map[string]bool {
	out := map[string]bool{}
	if cell == nil {
		return out
	}
	for _, r := range cell.Repos {
		out[strings.ToLower(r)] = true
	}
	return out
}
