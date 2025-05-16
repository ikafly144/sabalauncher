package osinfo

func GetOsVersion() string {
	return osInfo().Version
}

type OsInfo struct {
	Name    string
	Version string
}
