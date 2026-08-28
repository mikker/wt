//go:build darwin || linux

package main

import (
	"errors"

	"golang.org/x/sys/unix"
)

func reflinkError(err error) error {
	if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) ||
		errors.Is(err, unix.ENOTTY) || errors.Is(err, unix.EXDEV) ||
		errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EPERM) ||
		errors.Is(err, unix.ENOSYS) {
		return errors.Join(errReflinkUnsupported, err)
	}
	return err
}
