//go:build !windows
// +build !windows

package msa

func encrypt(data []byte) ([]byte, error) {
	return data, nil
}

func decrypt(data []byte) ([]byte, error) {
	return data, nil
}
