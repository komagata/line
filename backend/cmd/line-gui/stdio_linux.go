package main

import (
	"golang.org/x/sys/unix"
	"os"
)

// An inherited blocking pipe is not registered with Go's netpoller. Re-open its
// descriptor as nonblocking *before* NewFile, so Close interrupts blocked reads
// and writes. Only these private descriptors are used by the protocol.
func pollable(file *os.File) (*os.File, error) {
	fd, err := unix.FcntlInt(file.Fd(), unix.F_DUPFD_CLOEXEC, 3)
	if err != nil {
		return nil, err
	}
	if err = unix.SetNonblock(fd, true); err != nil {
		unix.Close(fd)
		return nil, err
	}
	return os.NewFile(uintptr(fd), file.Name()), nil
}
