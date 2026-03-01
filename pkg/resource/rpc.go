package resource

import (
	_ "embed"
	"time"

	"github.com/hugolgst/rich-go/client"
	"github.com/ikafly144/sabalauncher/pkg/i18n"
	"github.com/ikafly144/sabalauncher/pkg/msa"
	"github.com/ikafly144/sabalauncher/secret"
)

var DiscordClientID = secret.GetSecret("DISCORD_CLIENT_ID")

func Login() error {
	return client.Login(DiscordClientID)
}

func Logout() {
	client.Logout()
}

func SetActivity(inst *Instance, mcProfile *msa.MinecraftProfile) (*client.Activity, error) {
	activity := mapActivity(*inst, mcProfile)
	if err := client.SetActivity(activity); err != nil {
		return nil, err
	}
	return &activity, nil
}

func ClearActivity() error {
	Logout()
	return Login()
}

func mapActivity(inst Instance, mcProfile *msa.MinecraftProfile) client.Activity {
	t := time.Now()
	// Removed unused version variable since it's not being formatting locally
	return client.Activity{
		State:      i18n.T("playing_state", inst.Name),
		Details:    i18n.T("playing_details"),
		LargeImage: "launcher_icon",
		LargeText:  i18n.T("app_title"),
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
