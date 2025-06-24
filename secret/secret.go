package secret

import (
	"embed"
	"encoding/json"
	"io/fs"
	"strings"

	"golang.org/x/exp/slog"
)

//go:generate go run github.com/ikafly144/sabalauncher/secret/gen

//go:embed local/*
var localRaw embed.FS

var localEntry = make(map[string]string)

func init() {
	entry, err := fs.ReadDir(localRaw, "local")
	if err != nil {
		slog.Error("failed to read local secrets", "error", err)
		return
	}
	for _, e := range entry {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		var m map[string]string
		if func() bool {
			f, err := localRaw.Open("local/" + e.Name())
			if err != nil {
				slog.Error("failed to open local secret file", "file", e.Name(), "error", err)
				// ファイルが開けなかった場合はスキップ
				return false
			}
			defer f.Close()
			if err := json.NewDecoder(f).Decode(&m); err != nil {
				slog.Error("failed to decode local secret file", "file", e.Name(), "error", err)
				// JSONのデコードに失敗した場合はスキップ
				return false
			}
			return true
		}() {
			continue // ファイルが開けなかった、またはJSONのデコードに失敗した場合はスキップ
		}
		if localEntry == nil {
			localEntry = make(map[string]string)
		}
		for k, v := range m {
			if _, ok := localEntry[k]; ok {
				continue // 重複を無視
			}
			localEntry[k] = v
		}
	}
}

func GetSecret(key string) (value string) {
	if localEntry == nil {
		return
	}
	if v, ok := localEntry[key]; ok {
		return v
	}
	return ""
}
