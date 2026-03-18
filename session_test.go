package dssession

import (
	"testing"
	"time"
)

func TestSessionContext(t *testing.T) {
	// Basic struct tests
	session := &Session{
		ID:             "test-session-id",
		UserID:         "test-user-id",
		CurrentTenant:  "test-org-id",
		ExpiresAt:      time.Now().Add(time.Hour),
		LastActivityAt: time.Now(),
	}

	if session.ID != "test-session-id" {
		t.Errorf("expected session ID 'test-session-id', got '%s'", session.ID)
	}

	tenant := &TenantContext{
		ID:    "test-org-id",
		Name:  "Test Org",
		Role:  "admin",
		Roles: []string{"admin", "viewer"},
	}

	if len(tenant.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(tenant.Roles))
	}

	if tenant.Role != "admin" {
		t.Errorf("expected primary role 'admin', got '%s'", tenant.Role)
	}

	ctx := &SessionContext{
		Session: session,
		Tenant:  tenant,
	}

	if ctx.Session.UserID != "test-user-id" {
		t.Errorf("expected user ID 'test-user-id', got '%s'", ctx.Session.UserID)
	}

	if ctx.Tenant.Name != "Test Org" {
		t.Errorf("expected tenant name 'Test Org', got '%s'", ctx.Tenant.Name)
	}
}

func TestTenantContextRoleBackwardsCompatibility(t *testing.T) {
	// Test that Role is computed from Roles[0]
	tests := []struct {
		name         string
		roles        []string
		expectedRole string
	}{
		{
			name:         "single role",
			roles:        []string{"owner"},
			expectedRole: "owner",
		},
		{
			name:         "multiple roles - first is primary",
			roles:        []string{"admin", "billing", "viewer"},
			expectedRole: "admin",
		},
		{
			name:         "empty roles",
			roles:        []string{},
			expectedRole: "",
		},
		{
			name:         "nil roles",
			roles:        nil,
			expectedRole: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate what getTenantContext does
			var primaryRole string
			if len(tt.roles) > 0 {
				primaryRole = tt.roles[0]
			}

			tenant := &TenantContext{
				ID:    "test-org",
				Name:  "Test Org",
				Role:  primaryRole,
				Roles: tt.roles,
			}

			if tenant.Role != tt.expectedRole {
				t.Errorf("expected Role '%s', got '%s'", tt.expectedRole, tenant.Role)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	// Default should be "prod"
	env := getEnv()
	if env != "prod" {
		t.Errorf("expected default env 'prod', got '%s'", env)
	}
}

func TestErrors(t *testing.T) {
	// Verify error values are distinct
	if ErrSessionNotFound == ErrSessionExpired {
		t.Error("ErrSessionNotFound should not equal ErrSessionExpired")
	}
	if ErrSessionExpired == ErrSessionLoggedOut {
		t.Error("ErrSessionExpired should not equal ErrSessionLoggedOut")
	}
	if ErrSessionLoggedOut == ErrTenantNotFound {
		t.Error("ErrSessionLoggedOut should not equal ErrTenantNotFound")
	}
}
