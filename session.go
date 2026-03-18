// Package dssession provides read-only access to DSAccount session data.
//
// This module enables DigiStratum apps to validate sessions and retrieve
// tenant context via direct DynamoDB reads, avoiding HTTP round-trips to DSAccount.
//
// DSAccount writes sessions; all other apps read via this module.
package dssession

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Common errors returned by the client.
var (
	ErrSessionNotFound  = errors.New("session not found")
	ErrSessionExpired   = errors.New("session expired")
	ErrSessionLoggedOut = errors.New("session logged out")
	ErrTenantNotFound   = errors.New("tenant not found")
)

// TenantType indicates whether the context is personal or organizational.
type TenantType string

const (
	// TenantTypePersonal indicates the user is operating in their personal context (no org selected).
	TenantTypePersonal TenantType = "personal"
	// TenantTypeOrganization indicates the user is operating within an organization.
	TenantTypeOrganization TenantType = "organization"
)

// Session represents a user session from DSAccount.
type Session struct {
	ID             string     `dynamodbav:"id"`
	UserID         string     `dynamodbav:"user_id"`
	CurrentTenant  string     `dynamodbav:"current_tenant,omitempty"`
	ExpiresAt      time.Time  `dynamodbav:"expires_at,unixtime"`
	LastActivityAt time.Time  `dynamodbav:"last_activity_at,unixtime"`
	LogoutAt       *time.Time `dynamodbav:"logout_at,omitempty,unixtime"`
}

// TenantContext represents the user's context within a tenant/organization.
type TenantContext struct {
	Type  TenantType // "personal" or "organization"
	ID    string     // Organization ID (empty for personal)
	Name  string     // Organization name (empty for personal)
	Role  string     // Primary role (first role, for backwards compatibility)
	Roles []string   // User's roles in this tenant (e.g., ["owner", "admin"])
}

// SessionContext combines session and tenant information.
type SessionContext struct {
	Session *Session       // Session details
	Tenant  *TenantContext // Current tenant context (never nil - uses personal context if no org)
}

// organization represents the DynamoDB organization record.
// Note: DSAccount stores org ID in "PK" attribute, not "id".
type organization struct {
	ID   string `dynamodbav:"PK"`   // Primary key attribute in DynamoDB
	Name string `dynamodbav:"name"`
}

// orgMember represents the DynamoDB org membership record.
type orgMember struct {
	UserID string   `dynamodbav:"user_id"`
	OrgID  string   `dynamodbav:"org_id"`
	Role   string   `dynamodbav:"role"`   // Current schema: singular role
	Status string   `dynamodbav:"status"`
}

// Client provides read-only access to DSAccount session data.
type Client struct {
	db              *dynamodb.Client
	sessionsTable   string
	orgsTable       string
	orgMembersTable string
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// WithDynamoDBClient sets a custom DynamoDB client.
func WithDynamoDBClient(db *dynamodb.Client) ClientOption {
	return func(c *Client) {
		c.db = db
	}
}

// WithTablePrefix sets a custom table prefix (default: "dsaccount").
func WithTablePrefix(prefix string) ClientOption {
	return func(c *Client) {
		env := getEnv()
		c.sessionsTable = fmt.Sprintf("%s-sessions-%s", prefix, env)
		c.orgsTable = fmt.Sprintf("%s-organizations-%s", prefix, env)
		c.orgMembersTable = fmt.Sprintf("%s-org-members-%s", prefix, env)
	}
}

// NewClient creates a new session client.
func NewClient(opts ...ClientOption) (*Client, error) {
	env := getEnv()
	c := &Client{
		sessionsTable:   fmt.Sprintf("dsaccount-sessions-%s", env),
		orgsTable:       fmt.Sprintf("dsaccount-orgs-%s", env),
		orgMembersTable: fmt.Sprintf("dsaccount-org-members-%s", env),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.db == nil {
		cfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
		c.db = dynamodb.NewFromConfig(cfg)
	}

	return c, nil
}

// GetContext retrieves the session and tenant context for a session ID.
//
// Returns ErrSessionNotFound if the session doesn't exist,
// ErrSessionExpired if the session has expired,
// ErrSessionLoggedOut if the session was invalidated.
//
// If the session has no current tenant (CurrentTenant is empty), returns
// a TenantContext with Type=TenantTypePersonal and empty ID/Name/Roles.
func (c *Client) GetContext(sessionID string) (*SessionContext, error) {
	ctx := context.Background()

	// Get session
	session, err := c.getSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	result := &SessionContext{Session: session}

	// Get tenant context - either organization or personal
	if session.CurrentTenant != "" {
		tenant, err := c.getTenantContext(ctx, session.UserID, session.CurrentTenant)
		if err != nil && !errors.Is(err, ErrTenantNotFound) {
			return nil, fmt.Errorf("failed to get tenant context: %w", err)
		}
		if tenant != nil {
			result.Tenant = tenant
		} else {
			// Tenant not found - fall back to personal context
			result.Tenant = &TenantContext{Type: TenantTypePersonal}
		}
	} else {
		// No tenant selected - personal context
		result.Tenant = &TenantContext{Type: TenantTypePersonal}
	}

	return result, nil
}

// GetSession retrieves just the session without tenant context.
func (c *Client) GetSession(sessionID string) (*Session, error) {
	return c.getSession(context.Background(), sessionID)
}

func (c *Client) getSession(ctx context.Context, sessionID string) (*Session, error) {
	result, err := c.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.sessionsTable),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: sessionID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if result.Item == nil {
		return nil, ErrSessionNotFound
	}

	var session Session
	if err := attributevalue.UnmarshalMap(result.Item, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	// Check if logged out
	if session.LogoutAt != nil {
		return nil, ErrSessionLoggedOut
	}

	// Check if expired
	if time.Now().UTC().After(session.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	return &session, nil
}

func (c *Client) getTenantContext(ctx context.Context, userID, orgID string) (*TenantContext, error) {
	// Get organization details
	// Note: DSAccount organizations table uses "PK" as the primary key attribute
	orgResult, err := c.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.orgsTable),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: orgID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	if orgResult.Item == nil {
		return nil, ErrTenantNotFound
	}

	var org organization
	if err := attributevalue.UnmarshalMap(orgResult.Item, &org); err != nil {
		return nil, fmt.Errorf("failed to unmarshal organization: %w", err)
	}

	// Get user's membership/roles in this org via GSI query
	// The org-members table uses PK (UUID) + SK (user_id) as primary key,
	// but has a GSI "user-orgs-index" on user_id for efficient lookups.
	memberResult, err := c.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(c.orgMembersTable),
		IndexName:              aws.String("user-orgs-index"),
		KeyConditionExpression: aws.String("user_id = :user_id"),
		FilterExpression:       aws.String("org_id = :org_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":user_id": &types.AttributeValueMemberS{Value: userID},
			":org_id":  &types.AttributeValueMemberS{Value: orgID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query org membership: %w", err)
	}

	var roles []string
	if len(memberResult.Items) > 0 {
		var member orgMember
		if err := attributevalue.UnmarshalMap(memberResult.Items[0], &member); err != nil {
			return nil, fmt.Errorf("failed to unmarshal membership: %w", err)
		}
		// Only include roles if membership is active
		if member.Status == "active" || member.Status == "" {
			if member.Role != "" { roles = []string{member.Role} }
		}
	}

	// Compute primary role (first role) for backwards compatibility
	var primaryRole string
	if len(roles) > 0 {
		primaryRole = roles[0]
	}

	return &TenantContext{
		Type:  TenantTypeOrganization,
		ID:    org.ID,
		Name:  org.Name,
		Role:  primaryRole,
		Roles: roles,
	}, nil
}

func getEnv() string {
	env := os.Getenv("DSACCOUNT_ENV")
	if env == "" {
		env = "prod"
	}
	return env
}
