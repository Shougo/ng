package detach

import "syscall"

// ioctlGetTermios is the ioctl that reads the terminal attributes.
const ioctlGetTermios = syscall.TCGETS

// dupTo duplicates oldfd onto newfd. Linux arm64 has no dup2.
func dupTo(oldfd, newfd int) error { return syscall.Dup3(oldfd, newfd, 0) }
