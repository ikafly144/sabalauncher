//go:build windows

package osinfo

import (
	"os/exec"
	"regexp"
	"syscall"
	"unsafe"
)

func osInfo() OsInfo {
	return OsInfo{
		Name:    "Windows",
		Version: getWindowsVersion(),
	}
}

var verRegex = regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+`)

func getWindowsVersion() string {
	cmd := exec.Command("cmd", "ver")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	_text, err := cmd.Output()
	if err != nil {
		return ""
	}
	text := string(_text)
	matches := verRegex.FindAllString(text, -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[0]
}

type memoryStatusEx struct {
	cbSize                  uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

func getTotalPhysicalMemory() uint64 {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	globalMemoryStatusEx := kernel32.NewProc("GlobalMemoryStatusEx")

	var ms memoryStatusEx
	ms.cbSize = uint32(unsafe.Sizeof(ms))
	ret, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if ret == 0 {
		return 0
	}
	return ms.ullTotalPhys
}
