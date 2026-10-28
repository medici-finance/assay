// Package gitquiet retains the desk test harness import path. The implementation
// lives in the shared gitquiet module so statusgen uses the same template owner.
package gitquiet

import (
	shared "github.com/medici-finance/assay/gitquiet"
	"testing"
)

var Settings = shared.Settings

func Install() (func(), error)        { return shared.Install() }
func Run(m *testing.M) int            { return shared.Run(m) }
func TemplateDir(t testing.TB) string { return shared.TemplateDir(t) }
