package core

import (
	"github.com/ikafly144/sabalauncher/pkg/resource"
)

type discordManager struct {
	auth     Authenticator
	profiles ProfileManager
}

func NewDiscordManager(auth Authenticator, profiles ProfileManager) DiscordManager {
	return &discordManager{
		auth:     auth,
		profiles: profiles,
	}
}

func (d *discordManager) SetActivity(profileName string) error {
	fullProfile, err := d.profiles.GetFullProfile(profileName)
	if err != nil {
		return err
	}

	mcProfile, err := d.auth.GetMinecraftProfile()
	if err != nil {
		return err
	}

	_, err = resource.SetActivity(fullProfile, mcProfile)
	return err
}

func (d *discordManager) ClearActivity() error {
	resource.Logout()
	return resource.Login()
}
