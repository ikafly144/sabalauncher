package runcmd

import "syscall"

func GetSysProcAttr() *syscall.SysProcAttr {
	// Windows does not require any special SysProcAttr settings for running commands
	// in a new process. The default settings are sufficient.
	return getSysProcAttr()
}
