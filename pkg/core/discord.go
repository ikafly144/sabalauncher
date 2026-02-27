package core

import (
	"fmt"
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
	profiles, err := d.profiles.GetProfiles()
	if err != nil {
		return err
	}

	var targetProfile *Profile
	for _, p := range profiles {
		if p.Name == profileName {
			targetProfile = &p
			break
		}
	}

	if targetProfile == nil {
		return fmt.Errorf("profile not found: %s", profileName)
	}

	mcProfile, err := d.auth.GetMinecraftProfile()
	if err != nil {
		return err
	}

	// We need a resource.Profile here. 
	// Our core.Profile is a simplified version.
	// For now, let's create a minimal resource.Profile for the RPC.
	resProfile := &resource.Profile{
		PublicProfile: resource.PublicProfile{
			Name:        targetProfile.Name,
			DisplayName: targetProfile.DisplayName + " (" + targetProfile.VersionName + ")",
		},
	}
	
	_, err = resource.SetActivity(resProfile, mcProfile)
	return err
}

func (d *discordManager) ClearActivity() error {
	resource.Logout()
	return resource.Login()
}
