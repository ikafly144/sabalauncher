//go:build windows
// +build windows

package osinfo

import (
	"os/exec"
	"regexp"
	"syscall"
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
