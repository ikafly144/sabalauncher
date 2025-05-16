package account

import (
	"launcher/pkg/msa"
	"launcher/pkg/resource"
	"log/slog"
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
		p.MinecraftAccount = a
		if _, err := a.GetMinecraftAccount(); err != nil {
			slog.Error("Failed to get Minecraft account", "error", err)
			return
		}

		if err := resource.SaveCredential(a); err != nil {
			slog.Error("Failed to save credential", "error", err)
		}

		p.session = nil
		success = true
	}(p)
}
