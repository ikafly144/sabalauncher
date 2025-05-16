package resource

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ulikunitz/xz/lzma"
)

const DEFAULT_RUNTIME_ALL_URL = "https://launchermeta.mojang.com/v1/products/java-runtime/2ec0cc96c44e5a76b9c8b7c39df7210883d12871/all.json"

type JavaRuntimes struct {
	Linux      JavaRuntimeTargets `json:"linux"`
	LinuxI386  JavaRuntimeTargets `json:"linux-i386"`
	Mac        JavaRuntimeTargets `json:"mac-os"`
	MacARM     JavaRuntimeTargets `json:"mac-os-arm64"`
	Windows64  JavaRuntimeTargets `json:"windows-x64"`
	Windows32  JavaRuntimeTargets `json:"windows-x86"`
	WindowsARM JavaRuntimeTargets `json:"windows-arm"`
}

type JavaRuntimeTargets map[string][]JavaRuntimeTarget

type JavaRuntimeTarget struct {
	Availability JavaRuntimeAvailability `json:"availability"`
	Manifest     JDownloadInfo           `json:"manifest"`
	Version      JavaRuntimeVersion      `json:"version"`
}

type JavaRuntimeAvailability struct {
	Group    int `json:"group"`
	Progress int `json:"progress"`
}

type JDownloadInfo struct {
	Sha1 string `json:"sha1"`
	Size int    `json:"size"`
	URL  string `json:"url"`
}

type JavaRuntimeVersion struct {
	Name     string    `json:"name"`
	Released time.Time `json:"released"`
}

type JavaRuntimeManifest struct {
	Files map[string]JEntry `json:"files"`
}

func (j *JavaRuntimeManifest) UnmarshalJSON(data []byte) error {
	var raw struct {
		Files map[string]json.RawMessage `json:"files"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	j.Files = make(map[string]JEntry)
	for k, v := range raw.Files {
		var entry JEntryUnmarshal
		if err := json.Unmarshal(v, &entry); err != nil {
			return err
		}
		j.Files[k] = entry.JEntry
	}
	return nil
}

type JEntry interface {
	Type() JEntryType
	anyEntry()
}

type JEntryUnmarshal struct {
	JEntry `json:"-"`
}

func (j *JEntryUnmarshal) UnmarshalJSON(data []byte) error {
	var jType struct {
		Type JEntryType `json:"type"`
	}
	if err := json.Unmarshal(data, &jType); err != nil {
		return err
	}
	switch jType.Type {
	case JEntryTypeFile:
		var fileEntry JFileEntry
		if err := json.Unmarshal(data, &fileEntry); err != nil {
			return err
		}
		j.JEntry = fileEntry
	case JEntryTypeDirectory:
		var dirEntry JDirectoryEntry
		if err := json.Unmarshal(data, &dirEntry); err != nil {
			return err
		}
		j.JEntry = dirEntry
	case JEntryTypeLink:
		var linkEntry JLinkEntry
		if err := json.Unmarshal(data, &linkEntry); err != nil {
			return err
		}
		j.JEntry = linkEntry
	default:
		return fmt.Errorf("unknown entry type: %s", string(data))
	}
	return nil
}

type JEntryType string

const (
	JEntryTypeFile      JEntryType = "file"
	JEntryTypeDirectory JEntryType = "directory"
	JEntryTypeLink      JEntryType = "link"
)

type JDirectoryEntry struct {
}

func (JDirectoryEntry) Type() JEntryType {
	return JEntryTypeDirectory
}
func (JDirectoryEntry) anyEntry() {}

type JFileEntry struct {
	Executable bool `json:"executable"`
	Downloads  struct {
		Lzma *JDownloadInfo `json:"lzma"`
		Raw  JDownloadInfo  `json:"raw,omitempty"`
	} `json:"downloads"`
}

func (JFileEntry) Type() JEntryType {
	return JEntryTypeFile
}
func (JFileEntry) anyEntry() {}

type JLinkEntry struct {
	Target string `json:"target"`
}

func (JLinkEntry) Type() JEntryType {
	return JEntryTypeLink
}
func (JLinkEntry) anyEntry() {}

func installJavaRuntime(target string, dataDir string, worker *DownloadWorker) error {
	slog.Info("installJavaRuntime", "target", target, "dataDir", dataDir)
	slog.Info("all.json", "url", DEFAULT_RUNTIME_ALL_URL)
	resp, err := http.Get(DEFAULT_RUNTIME_ALL_URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get java runtime: %s", resp.Status)
	}
	var runtimes JavaRuntimes
	if err := json.NewDecoder(resp.Body).Decode(&runtimes); err != nil {
		return err
	}
	runtime, ok := runtimes.Windows64[target]
	if !ok {
		return fmt.Errorf("failed to find java runtime for target: %s", target)
	}
	if len(runtime) == 0 {
		return fmt.Errorf("no java runtime found for target: %s", target)
	}
	manifest := runtime[0].Manifest

	slog.Info("java runtime manifest", "manifest", manifest)

	resp, err = http.Get(manifest.URL)
	if err != nil {
		return fmt.Errorf("failed to get java runtime manifest: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get java runtime manifest: %s", resp.Status)
	}
	var javaRuntimeManifest JavaRuntimeManifest
	if err := json.NewDecoder(resp.Body).Decode(&javaRuntimeManifest); err != nil {
		return err
	}
	for name, entry := range javaRuntimeManifest.Files {
		switch e := entry.(type) {
		case JFileEntry:
			path := filepath.Join(dataDir, "runtime", target, osName(), name)
			_ = os.MkdirAll(filepath.Dir(path), 0755)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				worker.addTask(func() error {
					// File does not exist, download it
					isLzma := e.Downloads.Lzma != nil
					var reader io.Reader
					var size int
					if isLzma {
						resp, err := http.Get(e.Downloads.Lzma.URL)
						if err != nil {
							return err
						}
						defer resp.Body.Close()
						if resp.StatusCode != http.StatusOK {
							return fmt.Errorf("failed to get java runtime file: %s", resp.Status)
						}
						lzmaReader, err := decompressLzma(resp.Body)
						if err != nil {
							return err
						}
						reader = lzmaReader
						size = e.Downloads.Raw.Size
					} else {
						resp, err := http.Get(e.Downloads.Raw.URL)
						if err != nil {
							return err
						}
						defer resp.Body.Close()
						if resp.StatusCode != http.StatusOK {
							return fmt.Errorf("failed to get java runtime file: %s", resp.Status)
						}
						reader = resp.Body
						size = e.Downloads.Raw.Size
					}
					out, err := os.Create(path)
					if err != nil {
						return err
					}
					defer out.Close()

					if wrote, err := io.Copy(out, reader); err != nil {
						return err
					} else if wrote != int64(size) {
						return fmt.Errorf("downloaded size mismatch: expected %d, got %d", size, wrote)
					}
					if e.Executable {
						if err := os.Chmod(path, 0755); err != nil {
							return fmt.Errorf("failed to set executable permission: %s", err)
						}
					}
					return nil
				})
			} else if e.Downloads.Raw.Sha1 != "" {
				// Verify the SHA1 checksum
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()
				hasher := sha1.New()
				if _, err := io.Copy(hasher, file); err != nil {
					return err
				}
				sum := hasher.Sum(nil)
				if fmt.Sprintf("%x", sum) != e.Downloads.Raw.Sha1 {
					return fmt.Errorf("SHA1 checksum mismatch for %s: expected %s, got %x", name, e.Downloads.Raw.Sha1, sum)
				}
			}
		case JLinkEntry:
			path := filepath.Join(dataDir, "runtime", target, osName(), name)
			_ = os.MkdirAll(filepath.Dir(path), 0755)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				// File does not exist, create it
				if err := os.Symlink(e.Target, path); err != nil {
					return fmt.Errorf("failed to create symlink: %s", err)
				}
			}
		case JDirectoryEntry:
			path := filepath.Join(dataDir, "runtime", target, osName(), name)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				// Directory does not exist, create it
				if err := os.MkdirAll(path, 0755); err != nil {
					return fmt.Errorf("failed to create directory: %s", err)
				}
			}
		}
	}
	return nil
}

func decompressLzma(reader io.Reader) (io.Reader, error) {
	return lzma.NewReader(reader)
}

func GetJavaExecutablePath(target string, dataDir string) (string, error) {
	path := filepath.Join(dataDir, "runtime", target, osName(), "bin", "java")
	if osName() == "windows" {
		path += ".exe"
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("java executable not found: %s", path)
	}
	return path, nil
}
