package dssession

import (
	"reflect"
	"sort"
	"testing"
)

// Test helpers
func makeSessionCtx(roles []string) *SessionContext {
	return &SessionContext{
		Session: &Session{
			ID:     "test-session",
			UserID: "test-user",
		},
		Tenant: &TenantContext{
			ID:    "test-org",
			Name:  "Test Org",
			Roles: roles,
		},
	}
}

func TestHasRole(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *SessionContext
		role     string
		expected bool
	}{
		{
			name:     "nil context",
			ctx:      nil,
			role:     "admin",
			expected: false,
		},
		{
			name:     "nil tenant",
			ctx:      &SessionContext{Session: &Session{}},
			role:     "admin",
			expected: false,
		},
		{
			name:     "empty roles",
			ctx:      makeSessionCtx([]string{}),
			role:     "admin",
			expected: false,
		},
		{
			name:     "role found - single role",
			ctx:      makeSessionCtx([]string{"admin"}),
			role:     "admin",
			expected: true,
		},
		{
			name:     "role found - multiple roles",
			ctx:      makeSessionCtx([]string{"viewer", "editor", "admin"}),
			role:     "editor",
			expected: true,
		},
		{
			name:     "role not found",
			ctx:      makeSessionCtx([]string{"viewer", "editor"}),
			role:     "admin",
			expected: false,
		},
		{
			name:     "case sensitive",
			ctx:      makeSessionCtx([]string{"Admin"}),
			role:     "admin",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasRole(tt.ctx, tt.role)
			if result != tt.expected {
				t.Errorf("HasRole() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHasAnyRole(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *SessionContext
		roles    []string
		expected bool
	}{
		{
			name:     "nil context",
			ctx:      nil,
			roles:    []string{"admin"},
			expected: false,
		},
		{
			name:     "nil tenant",
			ctx:      &SessionContext{Session: &Session{}},
			roles:    []string{"admin"},
			expected: false,
		},
		{
			name:     "empty requested roles",
			ctx:      makeSessionCtx([]string{"admin"}),
			roles:    []string{},
			expected: false,
		},
		{
			name:     "has first role",
			ctx:      makeSessionCtx([]string{"admin", "viewer"}),
			roles:    []string{"admin", "superuser"},
			expected: true,
		},
		{
			name:     "has second role",
			ctx:      makeSessionCtx([]string{"admin", "viewer"}),
			roles:    []string{"superuser", "viewer"},
			expected: true,
		},
		{
			name:     "has none",
			ctx:      makeSessionCtx([]string{"viewer", "editor"}),
			roles:    []string{"admin", "superuser"},
			expected: false,
		},
		{
			name:     "single match",
			ctx:      makeSessionCtx([]string{"editor"}),
			roles:    []string{"editor"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasAnyRole(tt.ctx, tt.roles)
			if result != tt.expected {
				t.Errorf("HasAnyRole() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHasAllRoles(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *SessionContext
		roles    []string
		expected bool
	}{
		{
			name:     "nil context",
			ctx:      nil,
			roles:    []string{"admin"},
			expected: false,
		},
		{
			name:     "nil tenant",
			ctx:      &SessionContext{Session: &Session{}},
			roles:    []string{"admin"},
			expected: false,
		},
		{
			name:     "empty required roles",
			ctx:      makeSessionCtx([]string{"admin"}),
			roles:    []string{},
			expected: false,
		},
		{
			name:     "has all roles",
			ctx:      makeSessionCtx([]string{"admin", "viewer", "editor"}),
			roles:    []string{"admin", "viewer"},
			expected: true,
		},
		{
			name:     "missing one role",
			ctx:      makeSessionCtx([]string{"admin", "viewer"}),
			roles:    []string{"admin", "editor"},
			expected: false,
		},
		{
			name:     "has exact roles",
			ctx:      makeSessionCtx([]string{"admin", "viewer"}),
			roles:    []string{"admin", "viewer"},
			expected: true,
		},
		{
			name:     "single role match",
			ctx:      makeSessionCtx([]string{"admin"}),
			roles:    []string{"admin"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasAllRoles(tt.ctx, tt.roles)
			if result != tt.expected {
				t.Errorf("HasAllRoles() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestComputePermissions(t *testing.T) {
	permMap := RolePermissionMap{
		"admin":  {Permission("read:*"), Permission("write:*"), Permission("delete:*")},
		"editor": {Permission("read:*"), Permission("write:articles")},
		"viewer": {Permission("read:articles")},
	}

	tests := []struct {
		name     string
		roles    []string
		permMap  RolePermissionMap
		expected []Permission
	}{
		{
			name:     "nil roles",
			roles:    nil,
			permMap:  permMap,
			expected: nil,
		},
		{
			name:     "empty roles",
			roles:    []string{},
			permMap:  permMap,
			expected: nil,
		},
		{
			name:     "nil permMap",
			roles:    []string{"admin"},
			permMap:  nil,
			expected: nil,
		},
		{
			name:     "single role",
			roles:    []string{"viewer"},
			permMap:  permMap,
			expected: []Permission{"read:articles"},
		},
		{
			name:     "multiple roles - no overlap",
			roles:    []string{"viewer"},
			permMap:  permMap,
			expected: []Permission{"read:articles"},
		},
		{
			name:    "multiple roles - with overlap",
			roles:   []string{"admin", "editor"},
			permMap: permMap,
			// admin: read:*, write:*, delete:*
			// editor: read:*, write:articles
			// Union should dedupe read:*
			expected: []Permission{"read:*", "write:*", "delete:*", "write:articles"},
		},
		{
			name:     "unknown role ignored",
			roles:    []string{"unknown", "viewer"},
			permMap:  permMap,
			expected: []Permission{"read:articles"},
		},
		{
			name:     "all unknown roles",
			roles:    []string{"unknown1", "unknown2"},
			permMap:  permMap,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputePermissions(tt.roles, tt.permMap)

			// Handle nil vs empty slice comparison
			if tt.expected == nil {
				if result != nil {
					t.Errorf("ComputePermissions() = %v, want nil", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("ComputePermissions() len = %d, want %d", len(result), len(tt.expected))
				return
			}

			// Sort both for comparison since order may vary
			sortPerms := func(p []Permission) {
				sort.Slice(p, func(i, j int) bool { return p[i] < p[j] })
			}
			sortPerms(result)
			sortPerms(tt.expected)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("ComputePermissions() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetPermissions(t *testing.T) {
	permMap := RolePermissionMap{
		"admin":  {Permission("read:*"), Permission("write:*")},
		"viewer": {Permission("read:articles")},
	}

	tests := []struct {
		name     string
		ctx      *SessionContext
		expected int // expected number of permissions
	}{
		{
			name:     "nil context",
			ctx:      nil,
			expected: 0,
		},
		{
			name:     "nil tenant",
			ctx:      &SessionContext{Session: &Session{}},
			expected: 0,
		},
		{
			name:     "admin role",
			ctx:      makeSessionCtx([]string{"admin"}),
			expected: 2,
		},
		{
			name:     "viewer role",
			ctx:      makeSessionCtx([]string{"viewer"}),
			expected: 1,
		},
		{
			name:     "multiple roles",
			ctx:      makeSessionCtx([]string{"admin", "viewer"}),
			expected: 3, // read:*, write:*, read:articles
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPermissions(tt.ctx, permMap)
			if len(result) != tt.expected {
				t.Errorf("GetPermissions() len = %d, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestHasPermission(t *testing.T) {
	permMap := RolePermissionMap{
		"admin":  {Permission("read:*"), Permission("write:*")},
		"viewer": {Permission("read:articles")},
	}

	tests := []struct {
		name       string
		ctx        *SessionContext
		permission Permission
		expected   bool
	}{
		{
			name:       "nil context",
			ctx:        nil,
			permission: Permission("read:*"),
			expected:   false,
		},
		{
			name:       "has permission - admin",
			ctx:        makeSessionCtx([]string{"admin"}),
			permission: Permission("write:*"),
			expected:   true,
		},
		{
			name:       "does not have permission - viewer",
			ctx:        makeSessionCtx([]string{"viewer"}),
			permission: Permission("write:*"),
			expected:   false,
		},
		{
			name:       "has permission - viewer",
			ctx:        makeSessionCtx([]string{"viewer"}),
			permission: Permission("read:articles"),
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasPermission(tt.ctx, permMap, tt.permission)
			if result != tt.expected {
				t.Errorf("HasPermission() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHasAnyPermission(t *testing.T) {
	permMap := RolePermissionMap{
		"admin":  {Permission("read:*"), Permission("write:*")},
		"viewer": {Permission("read:articles")},
	}

	tests := []struct {
		name        string
		ctx         *SessionContext
		permissions []Permission
		expected    bool
	}{
		{
			name:        "nil context",
			ctx:         nil,
			permissions: []Permission{"read:*"},
			expected:    false,
		},
		{
			name:        "empty permissions",
			ctx:         makeSessionCtx([]string{"admin"}),
			permissions: []Permission{},
			expected:    false,
		},
		{
			name:        "has one of requested",
			ctx:         makeSessionCtx([]string{"viewer"}),
			permissions: []Permission{"read:articles", "write:articles"},
			expected:    true,
		},
		{
			name:        "has none of requested",
			ctx:         makeSessionCtx([]string{"viewer"}),
			permissions: []Permission{"write:*", "delete:*"},
			expected:    false,
		},
		{
			name:        "has all of requested",
			ctx:         makeSessionCtx([]string{"admin"}),
			permissions: []Permission{"read:*", "write:*"},
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasAnyPermission(tt.ctx, permMap, tt.permissions)
			if result != tt.expected {
				t.Errorf("HasAnyPermission() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRolePermissionMapUsage(t *testing.T) {
	// Example of how an app would define their permission map
	appPermissions := RolePermissionMap{
		"owner": {
			Permission("tenant:manage"),
			Permission("users:*"),
			Permission("billing:*"),
		},
		"admin": {
			Permission("users:read"),
			Permission("users:write"),
			Permission("content:*"),
		},
		"member": {
			Permission("users:read"),
			Permission("content:read"),
		},
	}

	// User with owner+member roles
	ctx := makeSessionCtx([]string{"owner", "member"})

	// Should have owner permissions
	if !HasPermission(ctx, appPermissions, Permission("tenant:manage")) {
		t.Error("owner should have tenant:manage")
	}

	// Should have member permissions too
	if !HasPermission(ctx, appPermissions, Permission("content:read")) {
		t.Error("member should have content:read")
	}

	// Should not have admin-only permissions
	if HasPermission(ctx, appPermissions, Permission("content:*")) {
		t.Error("owner+member should not have content:* (admin only)")
	}

	// Verify computed permissions
	perms := GetPermissions(ctx, appPermissions)
	expected := 5 // owner: 3, member: 2, but users:read overlaps = 4... wait let me recalculate
	// owner: tenant:manage, users:*, billing:* = 3
	// member: users:read, content:read = 2
	// Total unique = 5 (no overlap since users:* != users:read)
	if len(perms) != expected {
		t.Errorf("expected %d permissions, got %d: %v", expected, len(perms), perms)
	}
}
