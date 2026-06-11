//go:build !windows

package osinfo

func osInfo() OsInfo {
	return OsInfo{
		Name:    "Other",
		Version: "unknown",
	}
}

func getTotalPhysicalMemory() uint64 {
	// Fallback: return 0 or a large value? 0 means we can't check.
	return 0
}
