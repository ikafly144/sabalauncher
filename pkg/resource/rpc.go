package resource

import (
	_ "embed"
	"fmt"
	"time"

	"github.com/hugolgst/rich-go/client"
	"github.com/ikafly144/sabalauncher/pkg/msa"
)

var DiscordClientID string

func Login() error {
	if err := client.Login(DiscordClientID); err != nil {
		return err
	}
	return nil
}

func Logout() {
	client.Logout()
}

func EndActivity(activity *client.Activity) error {
	if activity == nil {
		return nil
	}
	t := time.Now()
	if activity.Timestamps == nil {
		activity.Timestamps = &client.Timestamps{}
	}
	activity.Timestamps.End = &t
	if err := client.SetActivity(*activity); err != nil {
		return err
	}
	return nil
}

func SetActivity(inst *Instance, mcProfile *msa.MinecraftProfile) (*client.Activity, error) {
	activity := mapActivity(*inst, mcProfile)
	if err := client.SetActivity(activity); err != nil {
		return nil, err
	}
	return &activity, nil
}

func mapActivity(inst Instance, mcProfile *msa.MinecraftProfile) client.Activity {
	t := time.Now()
	version := "Unknown Version"
	for _, v := range inst.Versions {
		if v.ID == "minecraft" {
			version = v.Version
			break
		}
	}
	return client.Activity{
		State:      fmt.Sprintf("%sをプレイ中", inst.Name),
		Details:    fmt.Sprintf("%s %s", mcProfile.Username, version),
		LargeImage: "launcher_icon",
		LargeText:  "SabaLauncherでプレイ中",
		Timestamps: &client.Timestamps{
			Start: &t,
		},
		Buttons: []*client.Button{
			{
				Label: "SabaLauncher",
				Url:   "https://github.com/ikafly144/sabalauncher",
			},
		},
	}
}
