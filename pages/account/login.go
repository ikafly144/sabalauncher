package account

import (
	"log/slog"

	"github.com/ikafly144/sabalauncher/pkg/msa"
)

func (p *Page) startLogin() {
	session, err := msa.NewSession(p.Cache)
	if err != nil {
		return
	}
	p.session = session

	go func(p *Page) {
		success := false
		p.loginErr = nil
		defer func() {
			p.success = &success
			p.session = nil
		}()
		if err := session.StartLogin(); err != nil {
			slog.Error("Login failed", "error", err)
			p.loginErr = err
			return
		}
		if _, err := p.session.AuthResult(); err != nil {
			slog.Error("Failed to get auth result", "error", err)
			p.loginErr = err
			return
		}

		a, err := msa.NewMinecraftAccount(session)
		if err != nil {
			slog.Error("Failed to get Minecraft account", "error", err)
			p.loginErr = err
			return
		}
		ma, err := a.GetMinecraftAccount()
		if err != nil {
			slog.Error("Failed to get Minecraft account", "error", err)
			p.loginErr = err
			return
		}

		profile, err := ma.GetMinecraftProfile()
		if err != nil {
			slog.Error("Failed to get Minecraft profile", "error", err)
			p.loginErr = err
			return
		}

		p.accountName = &profile.Username
		success = true
	}(p)
}
