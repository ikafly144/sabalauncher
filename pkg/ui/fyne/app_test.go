package fyne

import (
	"fyne.io/fyne/v2/test"
	"github.com/ikafly144/sabalauncher/pkg/core"
	"testing"
)

func TestNewFyneUI(t *testing.T) {
	a := test.NewApp()
	m := new(mockAuthenticator)
	ui := NewFyneUI(a, m, nil, nil, nil, "1.0.0")

	if ui == nil {
		t.Fatal("Failed to create FyneUI")
	}
	if ui.auth != (core.Authenticator)(m) {
		t.Error("Authenticator not set")
	}
}
