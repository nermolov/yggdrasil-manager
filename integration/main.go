package integration

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/unix"
)

func setNetworkNamespace(nsName string) {
	nsPath := "/run/netns/" + nsName

	// Lock the OS thread to ensure consistent namespace operations
	runtime.LockOSThread()

	// Open the network namespace file
	fd, err := unix.Open(nsPath, unix.O_RDONLY, 0)
	if err != nil {
		panic(fmt.Errorf("failed to open netns: %v", err))
	}
	defer unix.Close(fd)

	// Set the network namespace
	err = unix.Setns(fd, unix.CLONE_NEWNET)
	if err != nil {
		panic(fmt.Errorf("failed to set netns: %v", err))
	}
}
