package fyne

import (
	"testing"
	"fyne.io/fyne/v2/test"
)

func TestNewFyneUI(t *testing.T) {
	a := test.NewApp()
	m := new(mockAuthenticator)
	ui := NewFyneUI(a, m, nil, nil)
	
	if ui == nil {
		t.Fatal("Failed to create FyneUI")
	}
	if ui.auth != m {
		t.Error("Authenticator not set")
	}
}
