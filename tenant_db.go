// Package dssession provides a TenantScopedDB wrapper for DynamoDB that enforces tenant isolation.
//
// Usage:
//
//	sessionCtx, err := client.GetContext(sessionID)
//	if err != nil {
//	    return err
//	}
//
//	// Wrap the DynamoDB client with tenant scoping
//	tenantDB, err := dssession.TenantDB(sessionCtx, dynamoClient)
//	if err != nil {
//	    return err // e.g., ErrNoTenantContext
//	}
//
//	// All operations now automatically scoped to tenant
//	tenantDB.PutItem(ctx, input)  // auto-stamps tenant_id
//	tenantDB.Query(ctx, input)    // auto-adds tenant_id condition
package dssession

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// TenantDBAttributeName is the attribute name used for tenant isolation.
const TenantDBAttributeName = "tenant_id"

// Errors for TenantScopedDB operations.
var (
	// ErrNoTenantContext is returned when attempting to create a TenantScopedDB
	// without a valid tenant context (e.g., personal context or nil).
	ErrNoTenantContext = errors.New("tenant context required: user is in personal context or tenant not set")

	// ErrTenantMismatch is returned when a write operation attempts to set a
	// different tenant_id than the scoped tenant.
	ErrTenantMismatch = errors.New("tenant_id mismatch: cannot write to different tenant")
)

// DynamoDBAPI defines the subset of DynamoDB client methods that TenantScopedDB wraps.
// This allows for easier testing and dependency injection.
type DynamoDBAPI interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	BatchGetItem(ctx context.Context, params *dynamodb.BatchGetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchGetItemOutput, error)
	BatchWriteItem(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
	TransactWriteItems(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
	TransactGetItems(ctx context.Context, params *dynamodb.TransactGetItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactGetItemsOutput, error)
}

// TenantScopedDB wraps a DynamoDB client and enforces tenant isolation.
// All queries automatically add tenant_id conditions.
// All writes automatically stamp tenant_id.
type TenantScopedDB struct {
	db       DynamoDBAPI
	tenantID string
}

// TenantDB creates a tenant-scoped DynamoDB wrapper from a SessionContext.
// Returns ErrNoTenantContext if the session has no tenant (personal context).
//
// Example:
//
//	tenantDB, err := dssession.TenantDB(sessionCtx, dynamoClient)
//	if err != nil {
//	    if errors.Is(err, dssession.ErrNoTenantContext) {
//	        // Handle personal context - maybe use raw client with explicit scoping
//	    }
//	    return err
//	}
func TenantDB(sessionCtx *SessionContext, db DynamoDBAPI) (*TenantScopedDB, error) {
	if sessionCtx == nil || sessionCtx.Tenant == nil {
		return nil, ErrNoTenantContext
	}

	if sessionCtx.Tenant.Type != TenantTypeOrganization || sessionCtx.Tenant.ID == "" {
		return nil, ErrNoTenantContext
	}

	return &TenantScopedDB{
		db:       db,
		tenantID: sessionCtx.Tenant.ID,
	}, nil
}

// TenantDBFromID creates a tenant-scoped DynamoDB wrapper from an explicit tenant ID.
// Use this for service-to-service calls where you already have the tenant ID.
// Returns ErrNoTenantContext if tenantID is empty.
func TenantDBFromID(tenantID string, db DynamoDBAPI) (*TenantScopedDB, error) {
	if tenantID == "" {
		return nil, ErrNoTenantContext
	}

	return &TenantScopedDB{
		db:       db,
		tenantID: tenantID,
	}, nil
}

// TenantID returns the tenant ID this wrapper is scoped to.
func (t *TenantScopedDB) TenantID() string {
	return t.tenantID
}

// Unwrap returns the underlying DynamoDB client.
// Use with caution - operations on the unwrapped client bypass tenant scoping.
func (t *TenantScopedDB) Unwrap() DynamoDBAPI {
	return t.db
}

// GetItem retrieves an item, adding tenant_id to the key.
func (t *TenantScopedDB) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if params.Key == nil {
		params.Key = make(map[string]types.AttributeValue)
	}
	params.Key[TenantDBAttributeName] = &types.AttributeValueMemberS{Value: t.tenantID}
	return t.db.GetItem(ctx, params, optFns...)
}

// PutItem writes an item, stamping tenant_id.
// Returns ErrTenantMismatch if the item already has a different tenant_id.
func (t *TenantScopedDB) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	if params.Item == nil {
		params.Item = make(map[string]types.AttributeValue)
	}

	// Check for tenant_id mismatch
	if existing, ok := params.Item[TenantDBAttributeName]; ok {
		if s, ok := existing.(*types.AttributeValueMemberS); ok && s.Value != t.tenantID {
			return nil, ErrTenantMismatch
		}
	}

	// Stamp tenant_id
	params.Item[TenantDBAttributeName] = &types.AttributeValueMemberS{Value: t.tenantID}

	// Add condition to prevent cross-tenant overwrites
	tenantCondition := fmt.Sprintf("attribute_not_exists(%s) OR %s = :tenant_id", TenantDBAttributeName, TenantDBAttributeName)
	if params.ConditionExpression != nil && *params.ConditionExpression != "" {
		combined := fmt.Sprintf("(%s) AND (%s)", *params.ConditionExpression, tenantCondition)
		params.ConditionExpression = aws.String(combined)
	} else {
		params.ConditionExpression = aws.String(tenantCondition)
	}

	if params.ExpressionAttributeValues == nil {
		params.ExpressionAttributeValues = make(map[string]types.AttributeValue)
	}
	params.ExpressionAttributeValues[":tenant_id"] = &types.AttributeValueMemberS{Value: t.tenantID}

	return t.db.PutItem(ctx, params, optFns...)
}

// UpdateItem updates an item, adding tenant_id condition.
func (t *TenantScopedDB) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	if params.Key == nil {
		params.Key = make(map[string]types.AttributeValue)
	}
	params.Key[TenantDBAttributeName] = &types.AttributeValueMemberS{Value: t.tenantID}

	// Add tenant_id condition to ensure we only update our tenant's items
	tenantCondition := fmt.Sprintf("%s = :tenant_id", TenantDBAttributeName)
	if params.ConditionExpression != nil && *params.ConditionExpression != "" {
		combined := fmt.Sprintf("(%s) AND (%s)", *params.ConditionExpression, tenantCondition)
		params.ConditionExpression = aws.String(combined)
	} else {
		params.ConditionExpression = aws.String(tenantCondition)
	}

	if params.ExpressionAttributeValues == nil {
		params.ExpressionAttributeValues = make(map[string]types.AttributeValue)
	}
	params.ExpressionAttributeValues[":tenant_id"] = &types.AttributeValueMemberS{Value: t.tenantID}

	return t.db.UpdateItem(ctx, params, optFns...)
}

// DeleteItem deletes an item, adding tenant_id to key and condition.
func (t *TenantScopedDB) DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	if params.Key == nil {
		params.Key = make(map[string]types.AttributeValue)
	}
	params.Key[TenantDBAttributeName] = &types.AttributeValueMemberS{Value: t.tenantID}

	// Add tenant_id condition to prevent deleting other tenant's items
	tenantCondition := fmt.Sprintf("%s = :tenant_id", TenantDBAttributeName)
	if params.ConditionExpression != nil && *params.ConditionExpression != "" {
		combined := fmt.Sprintf("(%s) AND (%s)", *params.ConditionExpression, tenantCondition)
		params.ConditionExpression = aws.String(combined)
	} else {
		params.ConditionExpression = aws.String(tenantCondition)
	}

	if params.ExpressionAttributeValues == nil {
		params.ExpressionAttributeValues = make(map[string]types.AttributeValue)
	}
	params.ExpressionAttributeValues[":tenant_id"] = &types.AttributeValueMemberS{Value: t.tenantID}

	return t.db.DeleteItem(ctx, params, optFns...)
}

// Query executes a query, adding tenant_id to the key condition.
func (t *TenantScopedDB) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	// Add tenant_id to key condition expression
	tenantCondition := fmt.Sprintf("%s = :tenant_id", TenantDBAttributeName)
	if params.KeyConditionExpression != nil && *params.KeyConditionExpression != "" {
		combined := fmt.Sprintf("(%s) AND (%s)", tenantCondition, *params.KeyConditionExpression)
		params.KeyConditionExpression = aws.String(combined)
	} else {
		params.KeyConditionExpression = aws.String(tenantCondition)
	}

	if params.ExpressionAttributeValues == nil {
		params.ExpressionAttributeValues = make(map[string]types.AttributeValue)
	}
	params.ExpressionAttributeValues[":tenant_id"] = &types.AttributeValueMemberS{Value: t.tenantID}

	return t.db.Query(ctx, params, optFns...)
}

// Scan executes a scan, adding tenant_id filter.
// Note: Scans are expensive and should be avoided when possible.
// Consider using Query with a GSI on tenant_id instead.
func (t *TenantScopedDB) Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	// Add tenant_id to filter expression
	tenantFilter := fmt.Sprintf("%s = :tenant_id", TenantDBAttributeName)
	if params.FilterExpression != nil && *params.FilterExpression != "" {
		combined := fmt.Sprintf("(%s) AND (%s)", tenantFilter, *params.FilterExpression)
		params.FilterExpression = aws.String(combined)
	} else {
		params.FilterExpression = aws.String(tenantFilter)
	}

	if params.ExpressionAttributeValues == nil {
		params.ExpressionAttributeValues = make(map[string]types.AttributeValue)
	}
	params.ExpressionAttributeValues[":tenant_id"] = &types.AttributeValueMemberS{Value: t.tenantID}

	return t.db.Scan(ctx, params, optFns...)
}

// BatchGetItem retrieves multiple items, adding tenant_id to all keys.
func (t *TenantScopedDB) BatchGetItem(ctx context.Context, params *dynamodb.BatchGetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchGetItemOutput, error) {
	for tableName, keysAndAttrs := range params.RequestItems {
		for i := range keysAndAttrs.Keys {
			if keysAndAttrs.Keys[i] == nil {
				keysAndAttrs.Keys[i] = make(map[string]types.AttributeValue)
			}
			keysAndAttrs.Keys[i][TenantDBAttributeName] = &types.AttributeValueMemberS{Value: t.tenantID}
		}
		params.RequestItems[tableName] = keysAndAttrs
	}
	return t.db.BatchGetItem(ctx, params, optFns...)
}

// BatchWriteItem writes multiple items, stamping tenant_id on all puts.
// Returns ErrTenantMismatch if any item has a different tenant_id.
func (t *TenantScopedDB) BatchWriteItem(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
	for tableName, writeRequests := range params.RequestItems {
		for i := range writeRequests {
			req := &writeRequests[i]
			if req.PutRequest != nil {
				if req.PutRequest.Item == nil {
					req.PutRequest.Item = make(map[string]types.AttributeValue)
				}
				// Check for tenant_id mismatch
				if existing, ok := req.PutRequest.Item[TenantDBAttributeName]; ok {
					if s, ok := existing.(*types.AttributeValueMemberS); ok && s.Value != t.tenantID {
						return nil, ErrTenantMismatch
					}
				}
				req.PutRequest.Item[TenantDBAttributeName] = &types.AttributeValueMemberS{Value: t.tenantID}
			}
			if req.DeleteRequest != nil {
				if req.DeleteRequest.Key == nil {
					req.DeleteRequest.Key = make(map[string]types.AttributeValue)
				}
				req.DeleteRequest.Key[TenantDBAttributeName] = &types.AttributeValueMemberS{Value: t.tenantID}
			}
		}
		params.RequestItems[tableName] = writeRequests
	}
	return t.db.BatchWriteItem(ctx, params, optFns...)
}

// TransactWriteItems executes a transaction, adding tenant scoping to all operations.
// Returns ErrTenantMismatch if any put item has a different tenant_id.
func (t *TenantScopedDB) TransactWriteItems(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	for i := range params.TransactItems {
		item := &params.TransactItems[i]

		if item.Put != nil {
			if item.Put.Item == nil {
				item.Put.Item = make(map[string]types.AttributeValue)
			}
			// Check for tenant_id mismatch
			if existing, ok := item.Put.Item[TenantDBAttributeName]; ok {
				if s, ok := existing.(*types.AttributeValueMemberS); ok && s.Value != t.tenantID {
					return nil, ErrTenantMismatch
				}
			}
			item.Put.Item[TenantDBAttributeName] = &types.AttributeValueMemberS{Value: t.tenantID}

			// Add tenant condition
			t.addTenantConditionToPut(item.Put)
		}

		if item.Update != nil {
			if item.Update.Key == nil {
				item.Update.Key = make(map[string]types.AttributeValue)
			}
			item.Update.Key[TenantDBAttributeName] = &types.AttributeValueMemberS{Value: t.tenantID}
			t.addTenantConditionToUpdate(item.Update)
		}

		if item.Delete != nil {
			if item.Delete.Key == nil {
				item.Delete.Key = make(map[string]types.AttributeValue)
			}
			item.Delete.Key[TenantDBAttributeName] = &types.AttributeValueMemberS{Value: t.tenantID}
			t.addTenantConditionToDelete(item.Delete)
		}

		if item.ConditionCheck != nil {
			if item.ConditionCheck.Key == nil {
				item.ConditionCheck.Key = make(map[string]types.AttributeValue)
			}
			item.ConditionCheck.Key[TenantDBAttributeName] = &types.AttributeValueMemberS{Value: t.tenantID}
			t.addTenantConditionToConditionCheck(item.ConditionCheck)
		}
	}
	return t.db.TransactWriteItems(ctx, params, optFns...)
}

// TransactGetItems retrieves multiple items in a transaction, adding tenant_id to all keys.
func (t *TenantScopedDB) TransactGetItems(ctx context.Context, params *dynamodb.TransactGetItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactGetItemsOutput, error) {
	for i := range params.TransactItems {
		item := &params.TransactItems[i]
		if item.Get != nil {
			if item.Get.Key == nil {
				item.Get.Key = make(map[string]types.AttributeValue)
			}
			item.Get.Key[TenantDBAttributeName] = &types.AttributeValueMemberS{Value: t.tenantID}
		}
	}
	return t.db.TransactGetItems(ctx, params, optFns...)
}

// Helper methods for adding tenant conditions to transaction operations

func (t *TenantScopedDB) addTenantConditionToPut(put *types.Put) {
	tenantCondition := fmt.Sprintf("attribute_not_exists(%s) OR %s = :tenant_id", TenantDBAttributeName, TenantDBAttributeName)
	if put.ConditionExpression != nil && *put.ConditionExpression != "" {
		combined := fmt.Sprintf("(%s) AND (%s)", *put.ConditionExpression, tenantCondition)
		put.ConditionExpression = aws.String(combined)
	} else {
		put.ConditionExpression = aws.String(tenantCondition)
	}

	if put.ExpressionAttributeValues == nil {
		put.ExpressionAttributeValues = make(map[string]types.AttributeValue)
	}
	put.ExpressionAttributeValues[":tenant_id"] = &types.AttributeValueMemberS{Value: t.tenantID}
}

func (t *TenantScopedDB) addTenantConditionToUpdate(update *types.Update) {
	tenantCondition := fmt.Sprintf("%s = :tenant_id", TenantDBAttributeName)
	if update.ConditionExpression != nil && *update.ConditionExpression != "" {
		combined := fmt.Sprintf("(%s) AND (%s)", *update.ConditionExpression, tenantCondition)
		update.ConditionExpression = aws.String(combined)
	} else {
		update.ConditionExpression = aws.String(tenantCondition)
	}

	if update.ExpressionAttributeValues == nil {
		update.ExpressionAttributeValues = make(map[string]types.AttributeValue)
	}
	update.ExpressionAttributeValues[":tenant_id"] = &types.AttributeValueMemberS{Value: t.tenantID}
}

func (t *TenantScopedDB) addTenantConditionToDelete(del *types.Delete) {
	tenantCondition := fmt.Sprintf("%s = :tenant_id", TenantDBAttributeName)
	if del.ConditionExpression != nil && *del.ConditionExpression != "" {
		combined := fmt.Sprintf("(%s) AND (%s)", *del.ConditionExpression, tenantCondition)
		del.ConditionExpression = aws.String(combined)
	} else {
		del.ConditionExpression = aws.String(tenantCondition)
	}

	if del.ExpressionAttributeValues == nil {
		del.ExpressionAttributeValues = make(map[string]types.AttributeValue)
	}
	del.ExpressionAttributeValues[":tenant_id"] = &types.AttributeValueMemberS{Value: t.tenantID}
}

func (t *TenantScopedDB) addTenantConditionToConditionCheck(check *types.ConditionCheck) {
	tenantCondition := fmt.Sprintf("%s = :tenant_id", TenantDBAttributeName)
	if check.ConditionExpression != nil && *check.ConditionExpression != "" {
		combined := fmt.Sprintf("(%s) AND (%s)", *check.ConditionExpression, tenantCondition)
		check.ConditionExpression = aws.String(combined)
	} else {
		check.ConditionExpression = aws.String(tenantCondition)
	}

	if check.ExpressionAttributeValues == nil {
		check.ExpressionAttributeValues = make(map[string]types.AttributeValue)
	}
	check.ExpressionAttributeValues[":tenant_id"] = &types.AttributeValueMemberS{Value: t.tenantID}
}
