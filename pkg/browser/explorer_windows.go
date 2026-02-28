//go:build windows

package browser

import (
	"syscall"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	getActiveWindow  = user32.NewProc("GetActiveWindow")
	comdlg32         = syscall.NewLazyDLL("comdlg32.dll")
	getOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
)

type openFileNameW struct {
	lStructSize       uint32
	hwndOwner         syscall.Handle
	hInstance         syscall.Handle
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	flagsEx           uint32
}

func SelectFile(parentHWND uintptr, filter string) (string, error) {
	var ofn openFileNameW
	ofn.lStructSize = uint32(unsafe.Sizeof(ofn))

	if parentHWND == 0 {
		ret, _, _ := getActiveWindow.Call()
		ofn.hwndOwner = syscall.Handle(ret)
	} else {
		ofn.hwndOwner = syscall.Handle(parentHWND)
	}

	// Filter string needs to be double-null terminated
	var f []uint16
	if filter != "" {
		parts, err := syscall.UTF16FromString(filter)
		if err != nil {
			return "", err
		}
		for i, v := range parts {
			if v == '|' {
				parts[i] = 0
			}
		}
		f = parts
	} else {
		def, err := syscall.UTF16FromString("All Files\x00*.*\x00\x00")
		if err != nil {
			return "", err
		}
		f = def
	}
	ofn.lpstrFilter = &f[0]

	fileBuf := make([]uint16, 1024)
	ofn.lpstrFile = &fileBuf[0]
	ofn.nMaxFile = uint32(len(fileBuf))

	// OFN_EXPLORER | OFN_HIDEREADONLY | OFN_FILEMUSTEXIST
	ofn.flags = 0x00000800 | 0x00000004 | 0x00001000

	ret, _, _ := getOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		return "", nil // Cancelled or error
	}

	return syscall.UTF16ToString(fileBuf), nil
}
