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
    // Handle: ErrSessionNotFound, ErrSessionExpired, ErrSessionLoggedOut
    return err
}

// Access session and tenant info
fmt.Printf("User: %s\n", ctx.Session.UserID)
fmt.Printf("Tenant Type: %s\n", ctx.Tenant.Type)  // "personal" or "organization"
fmt.Printf("Tenant ID: %s\n", ctx.Tenant.ID)      // empty for personal
fmt.Printf("Tenant Name: %s\n", ctx.Tenant.Name)
fmt.Printf("Roles: %v\n", ctx.Tenant.Roles)
```

### Handling Personal Context (null tenant)

When a user hasn't selected an organization, `GetContext` returns a TenantContext with `Type=TenantTypePersonal`:

```go
ctx, err := client.GetContext(sessionID)
if err != nil {
    return err
}

switch ctx.Tenant.Type {
case dssession.TenantTypePersonal:
    // User is operating in personal context (no org selected)
    // ctx.Tenant.ID, Name, Roles will be empty
    fmt.Println("Operating in personal context")
    
case dssession.TenantTypeOrganization:
    // User is operating within an organization
    fmt.Printf("Operating in org: %s (%s)\n", ctx.Tenant.Name, ctx.Tenant.ID)
    fmt.Printf("User roles: %v\n", ctx.Tenant.Roles)
}
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
    Tenant  *TenantContext // Current tenant context (never nil)
}
```

### Session

```go
type Session struct {
    ID             string
    UserID         string
    CurrentTenant  string    // Currently selected org ID (empty for personal)
    ExpiresAt      time.Time
    LastActivityAt time.Time
}
```

### TenantContext

```go
type TenantContext struct {
    Type  TenantType // "personal" or "organization"
    ID    string     // Organization ID (empty for personal)
    Name  string     // Organization name (empty for personal)
    Role  string     // Primary role (first role, for backwards compatibility)
    Roles []string   // User's roles in this tenant
}
```

### TenantType

```go
type TenantType string

const (
    TenantTypePersonal     TenantType = "personal"      // No org selected
    TenantTypeOrganization TenantType = "organization"  // Operating within an org
)
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

### Required IAM Permissions

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "dynamodb:GetItem"
            ],
            "Resource": [
                "arn:aws:dynamodb:*:*:table/dsaccount-sessions-*",
                "arn:aws:dynamodb:*:*:table/dsaccount-orgs-*"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "dynamodb:Query"
            ],
            "Resource": [
                "arn:aws:dynamodb:*:*:table/dsaccount-org-members-*/index/user-orgs-index"
            ]
        }
    ]
}
```

## License

MIT

## Tenant-Scoped DynamoDB Wrapper

For multi-tenant apps that store data in DynamoDB, use `TenantScopedDB` to enforce tenant isolation:

```go
// Get session context
sessionCtx, err := client.GetContext(sessionID)
if err != nil {
    return err
}

// Wrap DynamoDB client with tenant scoping
tenantDB, err := dssession.TenantDB(sessionCtx, dynamoClient)
if err != nil {
    if errors.Is(err, dssession.ErrNoTenantContext) {
        // User is in personal context - handle appropriately
    }
    return err
}

// All operations now automatically scoped to tenant
// Queries add tenant_id to key condition
// Writes stamp tenant_id on items
// Conditions prevent cross-tenant access
tenantDB.PutItem(ctx, input)   // auto-stamps tenant_id
tenantDB.Query(ctx, input)     // auto-adds tenant_id condition
tenantDB.GetItem(ctx, input)   // auto-adds tenant_id to key
```

### TenantScopedDB Features

- **Automatic tenant_id stamping**: All write operations (PutItem, BatchWriteItem, TransactWriteItems) automatically add `tenant_id` to items
- **Automatic query scoping**: Query and Scan operations automatically add tenant_id conditions
- **Cross-tenant protection**: Condition expressions prevent accidentally overwriting another tenant's data
- **Mismatch detection**: Returns `ErrTenantMismatch` if you try to write an item with a different tenant_id
- **Explicit tenant ID**: Use `TenantDBFromID(tenantID, db)` for service-to-service calls

### Supported Operations

| Operation | Tenant Scoping |
|-----------|----------------|
| GetItem | Adds tenant_id to key |
| PutItem | Stamps tenant_id, adds condition |
| UpdateItem | Adds tenant_id to key and condition |
| DeleteItem | Adds tenant_id to key and condition |
| Query | Adds tenant_id to key condition |
| Scan | Adds tenant_id to filter |
| BatchGetItem | Adds tenant_id to all keys |
| BatchWriteItem | Stamps tenant_id on all puts |
| TransactWriteItems | Full scoping on all operations |
| TransactGetItems | Adds tenant_id to all keys |

### Table Schema Requirement

Tables using TenantScopedDB must include `tenant_id` in their key schema (typically as partition key or part of composite key).
