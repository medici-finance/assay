// Package deskkit is the fixture hub: gitcore is listed, extra is not.
package deskkit

import (
	_ "example.com/fixture/tools/desk/internal/extra"
	_ "example.com/fixture/tools/desk/internal/gitcore"
)
