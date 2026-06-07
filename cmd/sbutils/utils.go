package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
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

func parallelHashOverrides(baseDir string) map[string]string {
	results := make(map[string]string)
	var mu sync.Mutex

	if _, err := os.Stat(baseDir); err != nil {
		return results
	}

	numWorkers := runtime.NumCPU()
	paths := make(chan string, 100)
	var wg sync.WaitGroup

	for range numWorkers {
		wg.Go(func() {
			for path := range paths {
				hash, err := hashFile(path)
				if err != nil {
					continue
				}
				rel, err := filepath.Rel(baseDir, path)
				if err != nil {
					continue
				}
				rel = filepath.ToSlash(rel)

				mu.Lock()
				results[rel] = hash
				mu.Unlock()
			}
		})
	}

	_ = filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths <- path
		}
		return nil
	})
	close(paths)
	wg.Wait()

	return results
}
