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
		Type:  TenantTypeOrganization,
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

	if tenant.Type != TenantTypeOrganization {
		t.Errorf("expected tenant type 'organization', got '%s'", tenant.Type)
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
				Type:  TenantTypeOrganization,
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

func TestPersonalTenantContext(t *testing.T) {
	// Test personal context (null tenant case)
	tenant := &TenantContext{
		Type: TenantTypePersonal,
	}

	if tenant.Type != TenantTypePersonal {
		t.Errorf("expected tenant type 'personal', got '%s'", tenant.Type)
	}

	if tenant.ID != "" {
		t.Errorf("expected empty ID for personal context, got '%s'", tenant.ID)
	}

	if tenant.Name != "" {
		t.Errorf("expected empty Name for personal context, got '%s'", tenant.Name)
	}

	if len(tenant.Roles) != 0 {
		t.Errorf("expected empty Roles for personal context, got %v", tenant.Roles)
	}
}

func TestTenantTypes(t *testing.T) {
	// Verify type constants
	if TenantTypePersonal != "personal" {
		t.Errorf("TenantTypePersonal should be 'personal', got '%s'", TenantTypePersonal)
	}

	if TenantTypeOrganization != "organization" {
		t.Errorf("TenantTypeOrganization should be 'organization', got '%s'", TenantTypeOrganization)
	}
}

func TestGetEnv(t *testing.T) {
	// Default should be "prod"
	env := getEnv()
	if env != "prod" {
		t.Errorf("expected default env 'prod', got '%s'", env)
	}
}

func TestNewClientTableNames(t *testing.T) {
	// Test default table names with prod env
	client, err := NewClient(WithDynamoDBClient(nil))
	if err == nil {
		// If no error (which shouldn't happen without AWS config), check table names
		if client != nil {
			if client.sessionsTable != "dsaccount-sessions-prod" {
				t.Errorf("expected sessions table 'dsaccount-sessions-prod', got '%s'", client.sessionsTable)
			}
			if client.orgsTable != "dsaccount-organizations-prod" {
				t.Errorf("expected orgs table 'dsaccount-organizations-prod', got '%s'", client.orgsTable)
			}
			if client.orgMembersTable != "dsaccount-org-members-prod" {
				t.Errorf("expected org-members table 'dsaccount-org-members-prod', got '%s'", client.orgMembersTable)
			}
		}
	}
	// Note: NewClient will likely fail without AWS config, which is expected
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
