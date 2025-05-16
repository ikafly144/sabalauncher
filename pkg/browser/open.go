//go:build !windows
// +build !windows

package browser

func Open(url string) error {
	panic("Open not implemented for this platform")
}
