package contexthandoff

import (
	"github.com/medici-finance/assay/statusgen/internal/testschema"
	"testing"
)

type testSchema struct {
	*testschema.Schema
	root map[string]any
}

func loadSchema(t *testing.T, path string) *testSchema {
	s := testschema.Load(t, path)
	return &testSchema{s, s.Root}
}
func (s *testSchema) validate(v, node any, at string) error { return s.Validate(v, node, at) }
