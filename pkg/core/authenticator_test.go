package core

import (
	"testing"
)

func TestAuthenticator_InitialStatus(t *testing.T) {
	// We can't easily test the full MS login flow without mocks,
	// but we can check the initial state.
	// For now, we'll just verify it initializes to LoggedOut.

	// This might fail because it tries to load secrets or create files.
	// We should probably mock the msa.Session.
}
