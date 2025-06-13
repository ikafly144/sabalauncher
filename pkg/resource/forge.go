package resource

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ikafly144/sabalauncher/pkg/runcmd"
)

func DownloadForge(versionName, forgeDirName, dataPath string) (*DownloadWorker, string, error) {
	var worker DownloadWorker
	tmpPath := os.TempDir()
	tmpFile, err := os.CreateTemp(tmpPath, forgeDirName+"-*.jar")
	if err != nil {
		return nil, "", err
	}
	worker.addTask(func() error {
		defer tmpFile.Close()
		httpClient := &http.Client{}
		url := forgeDownloadURL
		url = strings.ReplaceAll(url, "${version}", versionName)
		resp, err := httpClient.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to download forge jar: %s", resp.Status)
		}

		_, err = io.Copy(tmpFile, resp.Body)
		if err != nil {
			return err
		}
		slog.Info("Forge installer jar downloaded", "path", tmpFile.Name())
		return nil
	})

	return &worker, tmpFile.Name(), nil
}

func InstallForge(installerPath, dataPath string) error {
	if installerPath == "" {
		return fmt.Errorf("installer jar path is not set")
	}
	profiles, err := os.Create(filepath.Join(dataPath, "launcher_profiles.json"))
	if err != nil {
		return err
	}
	defer os.Remove(profiles.Name())
	defer profiles.Close()
	_, err = profiles.WriteString("{\"profiles\":{}}")
	if err != nil {
		return err
	}
	cmd := exec.Command("java", "-jar", installerPath, "--installClient", dataPath)
	cmd.Dir = filepath.Join(filepath.Dir(installerPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = runcmd.GetSysProcAttr()
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
