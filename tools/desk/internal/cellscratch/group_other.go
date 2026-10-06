//go:build !unix

package cellscratch

import "errors"

func groupEmpty(id int) (bool, error) {
	return false, errors.New("abandoned child-tree proof unavailable on this platform")
}
