//go:build !unix && !windows

package cellscratch

import (
	"errors"
	"os"
)

func singleInputLink(file *os.File) error {
	return errors.New("input link identity could not be checked on this platform")
}
