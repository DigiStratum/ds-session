# ds-session

Shared Go module for direct DynamoDB session reads across DigiStratum apps.

## Purpose

This module provides read-only access to DSAccount session data, enabling apps to validate sessions and retrieve tenant context without HTTP round-trips to DSAccount.

**Key principle:** DSAccount writes sessions, all other apps read via this module.

## Installation

```bash
go get github.com/DigiStratum/ds-session
```

## Usage

```go
import dssession "github.com/DigiStratum/ds-session"

// Create client (uses default AWS credentials)
client, err := dssession.NewClient()
if err != nil {
    log.Fatal(err)
}

// Get session context from session ID (from ds_session cookie)
ctx, err := client.GetContext(sessionID)
if err != nil {
    // Session not found, expired, or logged out
    return err
}

// Access session and tenant info
fmt.Printf("User: %s\n", ctx.Session.UserID)
fmt.Printf("Tenant: %s\n", ctx.Tenant.ID)
fmt.Printf("Roles: %v\n", ctx.Tenant.Roles)
```

## Permission Evaluation

Apps define their own role-to-permission mappings and use helpers to compute effective permissions.

### Role Checking

```go
// Check single role
if dssession.HasRole(ctx, "admin") {
    // User has admin role in current tenant
}

// Check any of multiple roles
if dssession.HasAnyRole(ctx, []string{"admin", "owner"}) {
    // User has at least one of admin or owner
}

// Check all roles required
if dssession.HasAllRoles(ctx, []string{"admin", "billing"}) {
    // User has both admin AND billing roles
}
```

### Permission Mapping

```go
// Define app-specific role-to-permission mapping
var myAppPermissions = dssession.RolePermissionMap{
    "owner": {
        dssession.Permission("tenant:manage"),
        dssession.Permission("users:*"),
        dssession.Permission("billing:*"),
    },
    "admin": {
        dssession.Permission("users:read"),
        dssession.Permission("users:write"),
        dssession.Permission("content:*"),
    },
    "member": {
        dssession.Permission("users:read"),
        dssession.Permission("content:read"),
    },
}

// Compute effective permissions (union of all role permissions)
perms := dssession.GetPermissions(ctx, myAppPermissions)

// Check specific permission
if dssession.HasPermission(ctx, myAppPermissions, dssession.Permission("billing:*")) {
    // User has billing:* permission
}

// Check any of multiple permissions
if dssession.HasAnyPermission(ctx, myAppPermissions, []dssession.Permission{
    "content:write",
    "content:*",
}) {
    // User can write content
}

// Compute permissions from arbitrary roles
perms := dssession.ComputePermissions([]string{"admin", "member"}, myAppPermissions)
```

## Types

### SessionContext

```go
type SessionContext struct {
    Session *Session       // Session details
    Tenant  *TenantContext // Current tenant context (may be nil)
}
```

### Session

```go
type Session struct {
    ID             string
    UserID         string
    CurrentTenant  string    // Currently selected org ID
    ExpiresAt      time.Time
    LastActivityAt time.Time
}
```

### TenantContext

```go
type TenantContext struct {
    ID    string   // Organization ID
    Name  string   // Organization name
    Roles []string // User's roles in this tenant
}
```

### Permission Types

```go
// Permission represents a single permission string
type Permission string

// RolePermissionMap maps role names to their granted permissions
type RolePermissionMap map[string][]Permission
```

## Configuration

The client uses standard AWS SDK credential chain. Configure via:

- Environment variables (`AWS_REGION`, `AWS_ACCESS_KEY_ID`, etc.)
- AWS credentials file
- IAM role (recommended for Lambda)

### Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `AWS_REGION` | `us-west-2` | AWS region |
| `DSACCOUNT_ENV` | `prod` | Environment suffix for table names |

## Table Access

This module requires read access to these DynamoDB tables:

- `dsaccount-sessions-{env}`
- `dsaccount-orgs-{env}`
- `dsaccount-org-members-{env}`

## License

MIT
