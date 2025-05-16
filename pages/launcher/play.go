package launcher

// import (
// 	"launcher/pkg/resource"
// 	"log/slog"
// 	"os"
// 	"strconv"
// 	"time"
// )

// var (
// 	procCount int = func() int {
// 		if n := os.Getenv("PROCESS_COUNT"); n != "" {
// 			if i, err := strconv.Atoi(n); err == nil {
// 				return i
// 			}
// 		}
// 		return 5
// 	}()
// )

// func (p *Page) startPlay(profile *Profile) {
// 	p.playResult = nil
// 	p.playStatus = "Starting game"
// 	playResult := false
// 	defer func() {
// 		p.playResult = &playResult
// 		p.playDownloadTotal = 0
// 		p.worker = nil
// 	}()
// 	version, err := resource.GetVersion(profile.Manifest.MinecraftVersion)
// 	if err != nil {
// 		slog.Error("Failed to get version", "error", err)
// 		p.playStatus = "Failed to get version"
// 		return
// 	}
// 	manifest, err := resource.GetClientManifest(version)
// 	if err != nil {
// 		slog.Error("Failed to get client manifest", "error", err)
// 		p.playStatus = "Failed to get client manifest"
// 		return
// 	}

// 	worker, err := resource.DownloadClientJar(manifest, dataDir)
// 	if err != nil {
// 		slog.Error("Failed to download client jar", "error", err)
// 		p.playStatus = "Failed to download client jar"
// 		return
// 	}
// 	p.worker = worker
// 	p.playStatus = "Downloading client jar"
// 	p.playDownloadTotal = worker.Remain()
// 	go func() {
// 		for {
// 			if err := worker.Run(); err != nil {
// 				slog.Error("Failed to download client jar", "error", err)
// 				time.Sleep(5 * time.Second)
// 				continue
// 			}
// 			break
// 		}
// 	}()
// 	worker.Wait()
// 	p.playStatus = "Client jar downloaded"
// 	p.playDownloadTotal = 0
// 	p.worker = nil

// 	worker, err = resource.DownloadJVM(manifest, dataDir)
// 	if err != nil {
// 		slog.Error("Failed to download jvm", "error", err)
// 		p.playStatus = "Failed to download jvm"
// 		return
// 	}
// 	p.worker = worker
// 	p.playStatus = "Downloading jvm"
// 	p.playDownloadTotal = worker.Remain()
// 	slog.Info("Downloading jvm", "total", p.playDownloadTotal)
// 	for range procCount {
// 		go func() {
// 			for {
// 				if err := worker.Run(); err != nil {
// 					slog.Error("Failed to download jvm", "error", err)
// 					time.Sleep(5 * time.Second)
// 					continue
// 				}
// 				break
// 			}
// 		}()
// 	}
// 	worker.Wait()
// 	p.playStatus = "JVM downloaded"
// 	slog.Info("JVM downloaded")

// 	worker, err = resource.DownloadAssets(manifest, dataDir)
// 	if err != nil {
// 		slog.Error("Failed to download assets", "error", err)
// 		p.playStatus = "Failed to download assets"
// 		return
// 	}
// 	p.worker = worker
// 	p.playDownloadTotal = worker.Remain()
// 	p.playStatus = "Downloading assets"
// 	for range procCount {
// 		go func() {
// 			for {
// 				if err := worker.Run(); err != nil {
// 					slog.Error("Failed to download assets", "error", err)
// 					time.Sleep(5 * time.Second)
// 					continue
// 				}
// 				break
// 			}
// 		}()
// 	}
// 	worker.Wait()
// 	p.playStatus = "Assets downloaded"

// 	p.playDownloadTotal = 0
// 	p.worker = nil

// 	worker, err = resource.DownloadLibraries(manifest, dataDir)
// 	if err != nil {
// 		slog.Error("Failed to download libraries", "error", err)
// 		p.playStatus = "Failed to download libraries"
// 		return
// 	}
// 	p.worker = worker
// 	p.playStatus = "Downloading library"
// 	p.playDownloadTotal = worker.Remain()
// 	for range procCount {
// 		go func() {
// 			for {
// 				if err := worker.Run(); err != nil {
// 					slog.Error("Failed to download libraries", "error", err)
// 					time.Sleep(5 * time.Second)
// 					continue
// 				}
// 				break
// 			}
// 		}()
// 	}
// 	worker.Wait()
// 	p.playStatus = "Libraries downloaded"
// 	p.playDownloadTotal = 0
// 	p.worker = nil

// 	p.playDownloadTotal = 0
// 	p.worker = nil
// 	p.playStatus = "Starting game"

// 	if p.Router.MinecraftAccount == nil {
// 		slog.Error("Minecraft account is nil")
// 		p.playStatus = "Minecraft account isn't logged in"
// 		return
// 	}

// 	result, err := p.Router.MinecraftAccount.GetMinecraftAccount()
// 	if err != nil {
// 		slog.Error("Failed to get Minecraft account", "error", err)
// 		p.playStatus = "Failed to get Minecraft account"
// 		return
// 	}

// 	if err := resource.BootGame(manifest, &profile.Profile, result, dataDir); err != nil {
// 		slog.Error("Failed to boot game", "error", err)
// 		p.playStatus = "Failed to boot game"
// 		return
// 	}
// 	playResult = true

// 	p.playStatus = "Game Ended"

// }

// func (p *Page) playProgress() float64 {
// 	if p.playStatus == "" {
// 		return 0
// 	}
// 	if p.playStatus == "Downloading assets" {
// 		return 1 - float64(p.worker.Remain())/float64(p.playDownloadTotal)
// 	}
// 	if p.playStatus == "Downloading library" {
// 		return 1 - float64(p.worker.Remain())/float64(p.playDownloadTotal)
// 	}
// 	if p.playStatus == "Downloading client jar" {
// 		return 1 - float64(p.worker.Remain())/float64(p.playDownloadTotal)
// 	}
// 	if p.playStatus == "Downloading jvm" {
// 		return 1 - float64(p.worker.Remain())/float64(p.playDownloadTotal)
// 	}
// 	return 1
// }
