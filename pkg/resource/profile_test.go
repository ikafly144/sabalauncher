package resource

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublicProfile_UnmarshalJSON(t *testing.T) {
	data := `{
		"name": "test-profile",
		"mod_loader": "fabric",
		"version": 2,
		"manifest": {
			"version": "1.21",
			"loaderVersion": "0.15.11"
		}
	}`

	var p PublicProfile
	if err := json.Unmarshal([]byte(data), &p); err != nil {
		t.Fatalf("Failed to unmarshal PublicProfile: %v", err)
	}

	if p.Name != "test-profile" {
		t.Errorf("Expected name 'test-profile', got '%s'", p.Name)
	}
	if p.ModLoader != "fabric" {
		t.Errorf("Expected mod_loader 'fabric', got '%s'", p.ModLoader)
	}
	if p.Manifest == nil {
		t.Fatal("Expected manifest to be non-nil")
	}
	if p.Manifest.Type() != "fabric" {
		t.Errorf("Expected manifest type 'fabric', got '%s'", p.Manifest.Type())
	}
	if p.Manifest.VersionName() != "1.21-fabric-0.15.11" {
		t.Errorf("Expected manifest version '1.21-fabric-0.15.11', got '%s'", p.Manifest.VersionName())
	}
}

func TestPublicProfile_UnmarshalJSON_MissingModLoader(t *testing.T) {
	data := `{
		"name": "test-profile",
		"version": 2,
		"manifest": {
			"version": "1.21"
		}
	}`

	var p PublicProfile
	err := json.Unmarshal([]byte(data), &p)
	if err == nil {
		t.Fatal("Expected error due to missing mod_loader, but got nil")
	}
	expected := "missing mandatory field: 'mod_loader'"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestPublicProfile_UnmarshalJSON_InvalidModLoader(t *testing.T) {
	data := `{
		"name": "test-profile",
		"mod_loader": "invalid-loader",
		"version": 2,
		"manifest": {
			"version": "1.21"
		}
	}`

	var p PublicProfile
	err := json.Unmarshal([]byte(data), &p)
	if err == nil {
		t.Fatal("Expected error due to invalid mod_loader, but got nil")
	}
	if !strings.Contains(err.Error(), "invalid mod_loader 'invalid-loader'") {
		t.Errorf("Unexpected error message: %v", err)
	}
}
