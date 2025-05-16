//go:build !windows
// +build !windows

package osinfo

func osInfo() OsInfo {
	panic("osInfo not implemented for this platform")
}
