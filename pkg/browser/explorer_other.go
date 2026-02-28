//go:build !windows

package browser

import "errors"

func SelectFile(parentHWND uintptr, filter string) (string, error) {
	return "", errors.New("SelectFile is only implemented for Windows")
}
