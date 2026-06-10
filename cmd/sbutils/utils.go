package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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
	relPath := f.Name
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	header, err := zip.FileInfoHeader(f.FileInfo())
	if err != nil {
		return err
	}
	header.Name = relPath
	header.Method = zip.Deflate

	writer, err := w.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, rc)
	return err
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	numWorkers := runtime.NumCPU()
	type task struct {
		f *zip.File
	}
	tasks := make(chan task, len(r.File))
	var wg sync.WaitGroup
	var lastErr error
	var errMu sync.Mutex

	for range numWorkers {
		wg.Go(func() {
			for t := range tasks {
				f := t.f
				fpath := filepath.Join(dest, f.Name)
				if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
					errMu.Lock()
					lastErr = fmt.Errorf("illegal file path: %s", fpath)
					errMu.Unlock()
					continue
				}

				if f.FileInfo().IsDir() {
					_ = os.MkdirAll(fpath, os.ModePerm)
					continue
				}

				if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
					errMu.Lock()
					lastErr = err
					errMu.Unlock()
					continue
				}

				outFile, err := os.Create(fpath)
				if err != nil {
					errMu.Lock()
					lastErr = err
					errMu.Unlock()
					continue
				}

				rc, err := f.Open()
				if err != nil {
					outFile.Close()
					errMu.Lock()
					lastErr = err
					errMu.Unlock()
					continue
				}

				_, err = io.Copy(outFile, rc)
				outFile.Close()
				rc.Close()
				if err != nil {
					errMu.Lock()
					lastErr = err
					errMu.Unlock()
				}
			}
		})
	}

	for _, f := range r.File {
		tasks <- task{f}
	}
	close(tasks)
	wg.Wait()

	return lastErr
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
