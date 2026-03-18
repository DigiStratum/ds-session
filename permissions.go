// Permission evaluation helpers for app-specific RBAC.
//
// Apps define their own RolePermissionMap to map roles to permissions.
// These helpers compute effective permissions from the user's roles.
package dssession

// Permission represents a single permission string (e.g., "read:users", "write:billing").
type Permission string

// RolePermissionMap maps role names to their granted permissions.
// Apps define their own instances to configure role-based access.
//
// Example:
//
//	var myAppPermissions = dssession.RolePermissionMap{
//	    "admin":  {"read:*", "write:*", "delete:*"},
//	    "editor": {"read:*", "write:articles"},
//	    "viewer": {"read:articles"},
//	}
type RolePermissionMap map[string][]Permission

// HasRole checks if the session context has a specific role in the current tenant.
// Returns false if there's no tenant context or roles are empty.
func HasRole(ctx *SessionContext, role string) bool {
	if ctx == nil || ctx.Tenant == nil {
		return false
	}
	for _, r := range ctx.Tenant.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the session context has at least one of the specified roles.
// Returns false if there's no tenant context, roles are empty, or none match.
func HasAnyRole(ctx *SessionContext, roles []string) bool {
	if ctx == nil || ctx.Tenant == nil || len(roles) == 0 {
		return false
	}
	// Build lookup set for efficiency
	roleSet := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		roleSet[r] = struct{}{}
	}
	for _, r := range ctx.Tenant.Roles {
		if _, ok := roleSet[r]; ok {
			return true
		}
	}
	return false
}

// HasAllRoles checks if the session context has all of the specified roles.
// Returns false if there's no tenant context or any role is missing.
func HasAllRoles(ctx *SessionContext, roles []string) bool {
	if ctx == nil || ctx.Tenant == nil || len(roles) == 0 {
		return false
	}
	// Build lookup set of user's roles
	userRoles := make(map[string]struct{}, len(ctx.Tenant.Roles))
	for _, r := range ctx.Tenant.Roles {
		userRoles[r] = struct{}{}
	}
	for _, required := range roles {
		if _, ok := userRoles[required]; !ok {
			return false
		}
	}
	return true
}

// ComputePermissions returns the union of all permissions granted by the given roles.
// Duplicate permissions are removed. Unknown roles are silently ignored.
func ComputePermissions(roles []string, permMap RolePermissionMap) []Permission {
	if len(roles) == 0 || permMap == nil {
		return nil
	}

	// Use a map to deduplicate permissions
	seen := make(map[Permission]struct{})
	var result []Permission

	for _, role := range roles {
		perms, ok := permMap[role]
		if !ok {
			continue
		}
		for _, p := range perms {
			if _, exists := seen[p]; !exists {
				seen[p] = struct{}{}
				result = append(result, p)
			}
		}
	}

	return result
}

// GetPermissions is a convenience method that computes permissions for the
// session context's current tenant roles using the provided permission map.
// Returns nil if there's no tenant context.
func GetPermissions(ctx *SessionContext, permMap RolePermissionMap) []Permission {
	if ctx == nil || ctx.Tenant == nil {
		return nil
	}
	return ComputePermissions(ctx.Tenant.Roles, permMap)
}

// HasPermission checks if any of the user's roles grant a specific permission.
// This is a convenience method combining GetPermissions with a lookup.
func HasPermission(ctx *SessionContext, permMap RolePermissionMap, permission Permission) bool {
	perms := GetPermissions(ctx, permMap)
	for _, p := range perms {
		if p == permission {
			return true
		}
	}
	return false
}

// HasAnyPermission checks if any of the user's roles grant at least one
// of the specified permissions.
func HasAnyPermission(ctx *SessionContext, permMap RolePermissionMap, permissions []Permission) bool {
	perms := GetPermissions(ctx, permMap)
	if len(perms) == 0 || len(permissions) == 0 {
		return false
	}

	// Build lookup set for requested permissions
	requested := make(map[Permission]struct{}, len(permissions))
	for _, p := range permissions {
		requested[p] = struct{}{}
	}

	for _, p := range perms {
		if _, ok := requested[p]; ok {
			return true
		}
	}
	return false
}
