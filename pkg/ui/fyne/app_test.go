package fyne

import (
	"testing"
	"fyne.io/fyne/v2/test"
)

func TestNewFyneUI(t *testing.T) {
	// Use test.NewApp() for headless testing
	a := test.NewApp()
	w := a.NewWindow("Test")
	
	ui := &FyneUI{
		app:    a,
		window: w,
	}
	
	if ui == nil {
		t.Fatal("Failed to create FyneUI")
	}
}
