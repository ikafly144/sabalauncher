package resource

import (
	"encoding/json"
	"testing"
)

func TestPublicProfile_UnmarshalJSON(t *testing.T) {
	data := `{
		"name": "test-profile",
		"mod_loader": "fabric",
		"version": 1,
		"manifest": {
			"loaderType": "vanilla",
			"version": "1.21"
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
	if p.Manifest.VersionName() != "1.21" {
		t.Errorf("Expected manifest version '1.21', got '%s'", p.Manifest.VersionName())
	}
}

func TestPublicProfile_UnmarshalJSON_MissingModLoader(t *testing.T) {
	// For now, we only add the field. Mandatory check is in Phase 4.
	data := `{
		"name": "test-profile",
		"version": 1,
		"manifest": {
			"loaderType": "vanilla",
			"version": "1.21"
		}
	}`

	var p PublicProfile
	if err := json.Unmarshal([]byte(data), &p); err != nil {
		t.Fatalf("Failed to unmarshal PublicProfile: %v", err)
	}

	if p.ModLoader != "" {
		t.Errorf("Expected mod_loader to be empty, got '%s'", p.ModLoader)
	}
}
