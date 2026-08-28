//go:build linux

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

func reflinkFile(source, target string, mode os.FileMode) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if err := unix.IoctlFileClone(int(targetFile.Fd()), int(sourceFile.Fd())); err != nil {
		targetFile.Close()
		os.Remove(target)
		return reflinkError(err)
	}
	if err := targetFile.Chmod(mode); err != nil {
		targetFile.Close()
		os.Remove(target)
		return err
	}
	return targetFile.Close()
}
