package secret

import (
	"embed"
	"encoding/json"
	"io/fs"
	"strings"
)

//go:generate go run github.com/ikafly144/sabalauncher/secret/gen

//go:embed local/*
var localRaw embed.FS

var localEntry map[string]string

func init() {
	entry, err := fs.ReadDir(localRaw, "local")
	if err != nil {
		panic(err)
	}
	for _, e := range entry {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		f, err := localRaw.Open("local/" + e.Name())
		if err != nil {
			panic(err)
		}
		defer f.Close()
		var m map[string]string
		if err := json.NewDecoder(f).Decode(&m); err != nil {
			panic(err)
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
