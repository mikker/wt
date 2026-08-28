//go:build !darwin && !linux

package main

import (
	"os"
)

func reflinkFile(_, _ string, _ os.FileMode) error {
	return errReflinkUnsupported
}
