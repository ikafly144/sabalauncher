package resource

import (
	_ "embed"
	"fmt"
	"time"

	"github.com/hugolgst/rich-go/client"
	"github.com/ikafly144/sabalauncher/pkg/msa"
)

//go:embed __client_id.txt
var clientID string

func Login() error {
	if err := client.Login(clientID); err != nil {
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

func SetActivity(profile *Profile, mcProfile *msa.MinecraftProfile) (*client.Activity, error) {
	activity := mapActivity(*profile, mcProfile)
	if err := client.SetActivity(activity); err != nil {
		return nil, err
	}
	return &activity, nil
}

func mapActivity(profile Profile, mcProfile *msa.MinecraftProfile) client.Activity {
	t := time.Now()
	return client.Activity{
		State:      fmt.Sprintf("%sをプレイ中", profile.Display()),
		Details:    fmt.Sprintf("%s %s", mcProfile.Username, profile.Manifest.VersionName()),
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
