package runcmd

import "syscall"

func getSysProcAttr() *syscall.SysProcAttr {
	// Windows does not require any special SysProcAttr settings for running commands
	// in a new process. The default settings are sufficient.
	return &syscall.SysProcAttr{
		HideWindow: true, // Hide the console window
	}
}
