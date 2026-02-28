package core

import (
	"github.com/ikafly144/sabalauncher/pkg/resource"
)

type discordManager struct {
	auth      Authenticator
	instances InstanceManager
}

func NewDiscordManager(auth Authenticator, instances InstanceManager) DiscordManager {
	return &discordManager{
		auth:      auth,
		instances: instances,
	}
}

func (d *discordManager) SetActivity(instanceName string) error {
	inst, err := d.instances.GetInstance(instanceName)
	if err != nil {
		return err
	}

	mcProfile, err := d.auth.GetMinecraftProfile()
	if err != nil {
		return err
	}

	_, err = resource.SetActivity(inst, mcProfile)
	return err
}

func (d *discordManager) ClearActivity() error {
	resource.Logout()
	return resource.Login()
}
