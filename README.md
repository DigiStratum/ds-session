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
import "github.com/DigiStratum/ds-session"

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
