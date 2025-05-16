//go:build windows
// +build windows

package browser

import "os/exec"

func Open(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
