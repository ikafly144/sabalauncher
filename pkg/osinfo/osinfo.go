package osinfo

func GetOsVersion() string {
	return osInfo().Version
}

func GetTotalPhysicalMemory() uint64 {
	return getTotalPhysicalMemory()
}

type OsInfo struct {
	Name    string
	Version string
}
