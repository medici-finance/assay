package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// desk__overrides is the READ twin of GET /v1/cells/{id}/overrides
// (the service contract): the tool the loops call each tick to learn a cell's
// current operator overrides. It is READ-ONLY and authors nothing — an MCP read
// tool holds no Filer and writes no platform state (Mutates:false), so the
// authority model's "no tool authors as a human" claim is untouched and
// authority_test.go stays green.
//
// It returns the cell's table AND ONLY the table (never the config file): the
// loops read one answer, which is the one-reader-path leg of the SPOF design —
// the local table is authoritative until a record PR merges and config reload
// adopts it.

// OverridesArgs names the cell whose override table to read.
type OverridesArgs struct {
	Cell string `json:"cell"`
}

// OverridesResult is desk__overrides' result. It embeds [domain.Overrides] so
// each set knob (dispatch_width, hold_merges, …) appears at the top level — the
// same shape GET /v1/cells/{id}/overrides serves — and carries the table's
// as_of (nil when the cell has no overrides).
type OverridesResult struct {
	Cell string `json:"cell"`
	domain.Overrides
	AsOf *time.Time `json:"as_of"`
}

func handleOverrides(_ context.Context, s *Server, call Call) (any, error) {
	var a OverridesArgs
	if err := json.Unmarshal(nonNull(call.Args), &a); err != nil {
		return nil, fmt.Errorf("desk__overrides arguments did not decode: %w", err)
	}
	if a.Cell == "" {
		return nil, fmt.Errorf("desk__overrides: cell is required")
	}
	if s.DB == nil {
		return nil, fmt.Errorf("desk__overrides: no index to read the override table from (BLIND, not empty)")
	}
	ov, asOf, err := s.DB.Overrides(a.Cell)
	if err != nil {
		return nil, fmt.Errorf("desk__overrides: reading cell %q override table failed: %w", a.Cell, err)
	}
	return OverridesResult{Cell: a.Cell, Overrides: ov, AsOf: asOf}, nil
}

// overridesSchema is the tool's JSON Schema (a single required cell argument).
func overridesSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"cell": map[string]any{"type": "string", "description": "the cell whose override table to read"},
		},
		"required": []string{"cell"},
	}
}
