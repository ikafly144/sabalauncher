//go:build !windows
// +build !windows

package runcmd

import "syscall"

func getSysProcAttr() *syscall.SysProcAttr {
	// Unix-like systems do not require any special SysProcAttr settings for running commands
	// in a new process. The default settings are sufficient.
	return nil
}
