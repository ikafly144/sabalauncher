package account

import (
	"log/slog"

	"github.com/ikafly144/sabalauncher/pkg/msa"
	"github.com/ikafly144/sabalauncher/pkg/resource"
)

func (p *Page) startLogin() {
	session, err := msa.NewSession()
	if err != nil {
		return
	}
	p.session = session

	go func(p *Page) {
		success := false
		defer func() {
			p.success = &success
			p.session = nil
		}()
		if err := session.StartLogin(); err != nil {
			slog.Error("Login failed", "error", err)
			return
		}
		result, err := p.session.AuthResult()
		if err != nil {
			slog.Error("Failed to get auth result", "error", err)
			return
		}

		a, err := msa.NewMinecraftAccount(result.AccessToken, result.ExpiresOn)
		if err != nil {
			slog.Error("Failed to get Minecraft account", "error", err)
			return
		}
		if _, err := a.GetMinecraftAccount(); err != nil {
			slog.Error("Failed to get Minecraft account", "error", err)
			return
		}
		p.MinecraftAccount = a

		if err := resource.SaveCredential(a); err != nil {
			slog.Error("Failed to save credential", "error", err)
		}

		success = true
	}(p)
}
