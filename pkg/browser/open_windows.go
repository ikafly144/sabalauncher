//go:build windows
// +build windows

package browser

import "os/exec"

func Open(url string) error {
	if url == "" {
		return nil
	}
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
