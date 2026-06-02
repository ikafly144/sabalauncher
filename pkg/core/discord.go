package core

import "github.com/ikafly144/sabalauncher/v2/pkg/rpc"

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

	_, err = rpc.SetActivity(inst, mcProfile)
	return err
}

func (d *discordManager) ClearActivity() error {
	rpc.Logout()
	return rpc.Login()
}
