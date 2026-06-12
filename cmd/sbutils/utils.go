package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

func addDataToZip(w *zip.Writer, data []byte, zipPath string) error {
	header := &zip.FileHeader{
		Name:   zipPath,
		Method: zip.Deflate,
	}
	writer, err := w.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func mapZipFiles(r *zip.Reader) map[string]*zip.File {
	m := make(map[string]*zip.File)
	for _, f := range r.File {
		m[f.Name] = f
	}
	return m
}

func copyZipFile(w *zip.Writer, f *zip.File) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	header := f.FileHeader
	header.Method = zip.Deflate

	writer, err := w.CreateHeader(&header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, rc)
	return err
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
