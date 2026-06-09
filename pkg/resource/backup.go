package resource

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type instanceBackup struct {
	baseDir   string
	backupDir string
	backedUp  map[string]bool
}

func newInstanceBackup(baseDir string) (*instanceBackup, error) {
	backupDir, err := os.MkdirTemp("", "sb-instance-backup-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create backup dir: %w", err)
	}
	return &instanceBackup{
		baseDir:   baseDir,
		backupDir: backupDir,
		backedUp:  make(map[string]bool),
	}, nil
}

// Backup records a file or directory for later restoration.
// path is relative to the instance baseDir.
func (b *instanceBackup) Backup(relPath string) error {
	if b.backedUp[relPath] {
		return nil // Already backed up
	}

	srcPath := filepath.Join(b.baseDir, relPath)
	info, err := os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File didn't exist before, so "restoring" means deleting it.
			// We track it by backing up an empty marker or just remembering it wasn't there.
			// But for simplicity, we'll just track that we touched it.
			b.backedUp[relPath] = true
			return nil
		}
		return err
	}

	dstPath := filepath.Join(b.backupDir, relPath)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	if info.IsDir() {
		// We could backup entire directories, but let's stick to files to keep it minimal.
		// Usually updates touch files. If a dir is removed, its contents will be backed up individually.
		return nil
	}

	// Copy file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	b.backedUp[relPath] = true
	return nil
}

func (b *instanceBackup) Restore() error {
	var lastErr error
	for relPath := range b.backedUp {
		srcPath := filepath.Join(b.backupDir, relPath)
		dstPath := filepath.Join(b.baseDir, relPath)

		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			// It didn't exist before, so delete it if it was created
			_ = os.Remove(dstPath)
		} else {
			// Restore from backup
			if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				lastErr = err
				continue
			}
			srcFile, err := os.Open(srcPath)
			if err != nil {
				lastErr = err
				continue
			}
			dstFile, err := os.Create(dstPath)
			if err != nil {
				srcFile.Close()
				lastErr = err
				continue
			}
			_, err = io.Copy(dstFile, srcFile)
			srcFile.Close()
			dstFile.Close()
			if err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

func (b *instanceBackup) Cleanup() {
	_ = os.RemoveAll(b.backupDir)
}
