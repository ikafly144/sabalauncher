//go:build windows
// +build windows

package msa

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	dllcrypt32 = syscall.NewLazyDLL("crypt32.dll")
	procEncryptData = dllcrypt32.NewProc("CryptProtectData")
	procDecryptData = dllcrypt32.NewProc("CryptUnprotectData")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func encrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var in, out dataBlob
	in.cbData = uint32(len(data))
	in.pbData = &data[0]

	ret, _, err := procEncryptData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("CryptProtectData failed: %w", err)
	}
	defer syscall.LocalFree(syscall.Handle(unsafe.Pointer(out.pbData)))

	res := make([]byte, out.cbData)
	copy(res, (*[1 << 30]byte)(unsafe.Pointer(out.pbData))[:out.cbData:out.cbData])
	return res, nil
}

func decrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var in, out dataBlob
	in.cbData = uint32(len(data))
	in.pbData = &data[0]

	ret, _, err := procDecryptData.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("CryptUnprotectData failed: %w", err)
	}
	defer syscall.LocalFree(syscall.Handle(unsafe.Pointer(out.pbData)))

	res := make([]byte, out.cbData)
	copy(res, (*[1 << 30]byte)(unsafe.Pointer(out.pbData))[:out.cbData:out.cbData])
	return res, nil
}
