//go:build unix

package cellscratch

import (
	"errors"
	"syscall"
)

func groupEmpty(id int) (bool, error) {
	err := syscall.Kill(-id, 0)
	if errors.Is(err, syscall.ESRCH) {
		return true, nil
	}
	return false, err
}
