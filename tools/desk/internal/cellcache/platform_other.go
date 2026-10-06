//go:build !darwin && !linux

package cellcache

import (
	"fmt"
	"os"
)

const noFollow = 0

func unsupported() error                           { return fmt.Errorf("managed Go cache requires macOS or Linux") }
func validatePath(string) error                    { return unsupported() }
func identity(os.FileInfo) (uint64, uint64, error) { return 0, 0, unsupported() }
func checkEntry(os.FileInfo, uint64) error         { return unsupported() }
func diskFree(string) (uint64, error)              { return 0, unsupported() }
func filesystemName(uint64) string                 { return "unavailable" }
