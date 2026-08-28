//go:build darwin

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

func reflinkFile(source, target string, _ os.FileMode) error {
	return reflinkError(unix.Clonefile(source, target, unix.CLONE_NOFOLLOW))
}
