package dssession

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// mockDynamoDBClient implements DynamoDBAPI for testing
type mockDynamoDBClient struct {
	getItemFunc           func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	putItemFunc           func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	updateItemFunc        func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	deleteItemFunc        func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	queryFunc             func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	scanFunc              func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	batchGetItemFunc      func(ctx context.Context, params *dynamodb.BatchGetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchGetItemOutput, error)
	batchWriteItemFunc    func(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
	transactWriteItemsFunc func(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
	transactGetItemsFunc  func(ctx context.Context, params *dynamodb.TransactGetItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactGetItemsOutput, error)
}

func (m *mockDynamoDBClient) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if m.getItemFunc != nil {
		return m.getItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.GetItemOutput{}, nil
}

func (m *mockDynamoDBClient) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	if m.putItemFunc != nil {
		return m.putItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.PutItemOutput{}, nil
}

func (m *mockDynamoDBClient) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	if m.updateItemFunc != nil {
		return m.updateItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

func (m *mockDynamoDBClient) DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	if m.deleteItemFunc != nil {
		return m.deleteItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.DeleteItemOutput{}, nil
}

func (m *mockDynamoDBClient) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	if m.queryFunc != nil {
		return m.queryFunc(ctx, params, optFns...)
	}
	return &dynamodb.QueryOutput{}, nil
}

func (m *mockDynamoDBClient) Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	if m.scanFunc != nil {
		return m.scanFunc(ctx, params, optFns...)
	}
	return &dynamodb.ScanOutput{}, nil
}

func (m *mockDynamoDBClient) BatchGetItem(ctx context.Context, params *dynamodb.BatchGetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchGetItemOutput, error) {
	if m.batchGetItemFunc != nil {
		return m.batchGetItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.BatchGetItemOutput{}, nil
}

func (m *mockDynamoDBClient) BatchWriteItem(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
	if m.batchWriteItemFunc != nil {
		return m.batchWriteItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.BatchWriteItemOutput{}, nil
}

func (m *mockDynamoDBClient) TransactWriteItems(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	if m.transactWriteItemsFunc != nil {
		return m.transactWriteItemsFunc(ctx, params, optFns...)
	}
	return &dynamodb.TransactWriteItemsOutput{}, nil
}

func (m *mockDynamoDBClient) TransactGetItems(ctx context.Context, params *dynamodb.TransactGetItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactGetItemsOutput, error) {
	if m.transactGetItemsFunc != nil {
		return m.transactGetItemsFunc(ctx, params, optFns...)
	}
	return &dynamodb.TransactGetItemsOutput{}, nil
}

func TestTenantDB_CreationWithSessionContext(t *testing.T) {
	mock := &mockDynamoDBClient{}

	tests := []struct {
		name        string
		sessionCtx  *SessionContext
		expectError error
	}{
		{
			name:        "nil session context",
			sessionCtx:  nil,
			expectError: ErrNoTenantContext,
		},
		{
			name: "nil tenant",
			sessionCtx: &SessionContext{
				Session: &Session{ID: "sess-123", UserID: "user-123"},
				Tenant:  nil,
			},
			expectError: ErrNoTenantContext,
		},
		{
			name: "personal context",
			sessionCtx: &SessionContext{
				Session: &Session{ID: "sess-123", UserID: "user-123"},
				Tenant:  &TenantContext{Type: TenantTypePersonal},
			},
			expectError: ErrNoTenantContext,
		},
		{
			name: "organization context with empty ID",
			sessionCtx: &SessionContext{
				Session: &Session{ID: "sess-123", UserID: "user-123"},
				Tenant:  &TenantContext{Type: TenantTypeOrganization, ID: ""},
			},
			expectError: ErrNoTenantContext,
		},
		{
			name: "valid organization context",
			sessionCtx: &SessionContext{
				Session: &Session{ID: "sess-123", UserID: "user-123"},
				Tenant:  &TenantContext{Type: TenantTypeOrganization, ID: "org-456", Name: "Test Org"},
			},
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantDB, err := TenantDB(tt.sessionCtx, mock)
			if tt.expectError != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.expectError)
				} else if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tenantDB == nil {
					t.Error("expected tenantDB to be non-nil")
				} else if tenantDB.TenantID() != tt.sessionCtx.Tenant.ID {
					t.Errorf("expected tenant ID %s, got %s", tt.sessionCtx.Tenant.ID, tenantDB.TenantID())
				}
			}
		})
	}
}

func TestTenantDBFromID(t *testing.T) {
	mock := &mockDynamoDBClient{}

	tests := []struct {
		name        string
		tenantID    string
		expectError error
	}{
		{
			name:        "empty tenant ID",
			tenantID:    "",
			expectError: ErrNoTenantContext,
		},
		{
			name:        "valid tenant ID",
			tenantID:    "org-789",
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantDB, err := TenantDBFromID(tt.tenantID, mock)
			if tt.expectError != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.expectError)
				} else if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tenantDB == nil {
					t.Error("expected tenantDB to be non-nil")
				} else if tenantDB.TenantID() != tt.tenantID {
					t.Errorf("expected tenant ID %s, got %s", tt.tenantID, tenantDB.TenantID())
				}
			}
		})
	}
}

func TestTenantScopedDB_GetItem(t *testing.T) {
	tenantID := "org-123"
	var capturedParams *dynamodb.GetItemInput

	mock := &mockDynamoDBClient{
		getItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			capturedParams = params
			return &dynamodb.GetItemOutput{}, nil
		},
	}

	tenantDB, _ := TenantDBFromID(tenantID, mock)

	input := &dynamodb.GetItemInput{
		TableName: aws.String("test-table"),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: "item-1"},
		},
	}

	_, err := tenantDB.GetItem(context.Background(), input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check tenant_id was added to key
	if capturedParams == nil {
		t.Fatal("params were not captured")
	}
	if val, ok := capturedParams.Key[TenantDBAttributeName]; !ok {
		t.Error("tenant_id was not added to key")
	} else if s, ok := val.(*types.AttributeValueMemberS); !ok || s.Value != tenantID {
		t.Errorf("expected tenant_id %s, got %v", tenantID, val)
	}
}

func TestTenantScopedDB_PutItem(t *testing.T) {
	tenantID := "org-123"
	var capturedParams *dynamodb.PutItemInput

	mock := &mockDynamoDBClient{
		putItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
			capturedParams = params
			return &dynamodb.PutItemOutput{}, nil
		},
	}

	tenantDB, _ := TenantDBFromID(tenantID, mock)

	t.Run("stamps tenant_id on item", func(t *testing.T) {
		input := &dynamodb.PutItemInput{
			TableName: aws.String("test-table"),
			Item: map[string]types.AttributeValue{
				"id":   &types.AttributeValueMemberS{Value: "item-1"},
				"name": &types.AttributeValueMemberS{Value: "Test"},
			},
		}

		_, err := tenantDB.PutItem(context.Background(), input)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Check tenant_id was stamped
		if val, ok := capturedParams.Item[TenantDBAttributeName]; !ok {
			t.Error("tenant_id was not stamped on item")
		} else if s, ok := val.(*types.AttributeValueMemberS); !ok || s.Value != tenantID {
			t.Errorf("expected tenant_id %s, got %v", tenantID, val)
		}

		// Check condition was added
		if capturedParams.ConditionExpression == nil {
			t.Error("condition expression was not added")
		}
	})

	t.Run("rejects mismatched tenant_id", func(t *testing.T) {
		input := &dynamodb.PutItemInput{
			TableName: aws.String("test-table"),
			Item: map[string]types.AttributeValue{
				"id":        &types.AttributeValueMemberS{Value: "item-1"},
				"tenant_id": &types.AttributeValueMemberS{Value: "other-org"},
			},
		}

		_, err := tenantDB.PutItem(context.Background(), input)
		if !errors.Is(err, ErrTenantMismatch) {
			t.Errorf("expected ErrTenantMismatch, got %v", err)
		}
	})

	t.Run("accepts matching tenant_id", func(t *testing.T) {
		input := &dynamodb.PutItemInput{
			TableName: aws.String("test-table"),
			Item: map[string]types.AttributeValue{
				"id":        &types.AttributeValueMemberS{Value: "item-1"},
				"tenant_id": &types.AttributeValueMemberS{Value: tenantID},
			},
		}

		_, err := tenantDB.PutItem(context.Background(), input)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestTenantScopedDB_Query(t *testing.T) {
	tenantID := "org-123"
	var capturedParams *dynamodb.QueryInput

	mock := &mockDynamoDBClient{
		queryFunc: func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
			capturedParams = params
			return &dynamodb.QueryOutput{}, nil
		},
	}

	tenantDB, _ := TenantDBFromID(tenantID, mock)

	t.Run("adds tenant_id to key condition", func(t *testing.T) {
		input := &dynamodb.QueryInput{
			TableName:              aws.String("test-table"),
			KeyConditionExpression: aws.String("pk = :pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: "pk-value"},
			},
		}

		_, err := tenantDB.Query(context.Background(), input)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Check tenant_id was added to key condition
		if capturedParams.KeyConditionExpression == nil {
			t.Error("key condition expression is nil")
		} else if *capturedParams.KeyConditionExpression != "(tenant_id = :tenant_id) AND (pk = :pk)" {
			t.Errorf("unexpected key condition: %s", *capturedParams.KeyConditionExpression)
		}

		// Check tenant_id value was added
		if val, ok := capturedParams.ExpressionAttributeValues[":tenant_id"]; !ok {
			t.Error(":tenant_id was not added to expression values")
		} else if s, ok := val.(*types.AttributeValueMemberS); !ok || s.Value != tenantID {
			t.Errorf("expected tenant_id %s, got %v", tenantID, val)
		}
	})
}

func TestTenantScopedDB_Scan(t *testing.T) {
	tenantID := "org-123"
	var capturedParams *dynamodb.ScanInput

	mock := &mockDynamoDBClient{
		scanFunc: func(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
			capturedParams = params
			return &dynamodb.ScanOutput{}, nil
		},
	}

	tenantDB, _ := TenantDBFromID(tenantID, mock)

	input := &dynamodb.ScanInput{
		TableName:        aws.String("test-table"),
		FilterExpression: aws.String("status = :status"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: "active"},
		},
	}

	_, err := tenantDB.Scan(context.Background(), input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check tenant_id was added to filter
	if capturedParams.FilterExpression == nil {
		t.Error("filter expression is nil")
	} else if *capturedParams.FilterExpression != "(tenant_id = :tenant_id) AND (status = :status)" {
		t.Errorf("unexpected filter expression: %s", *capturedParams.FilterExpression)
	}
}

func TestTenantScopedDB_UpdateItem(t *testing.T) {
	tenantID := "org-123"
	var capturedParams *dynamodb.UpdateItemInput

	mock := &mockDynamoDBClient{
		updateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
			capturedParams = params
			return &dynamodb.UpdateItemOutput{}, nil
		},
	}

	tenantDB, _ := TenantDBFromID(tenantID, mock)

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String("test-table"),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: "item-1"},
		},
		UpdateExpression: aws.String("SET #name = :name"),
	}

	_, err := tenantDB.UpdateItem(context.Background(), input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check tenant_id was added to key
	if val, ok := capturedParams.Key[TenantDBAttributeName]; !ok {
		t.Error("tenant_id was not added to key")
	} else if s, ok := val.(*types.AttributeValueMemberS); !ok || s.Value != tenantID {
		t.Errorf("expected tenant_id %s, got %v", tenantID, val)
	}

	// Check condition was added
	if capturedParams.ConditionExpression == nil {
		t.Error("condition expression was not added")
	}
}

func TestTenantScopedDB_DeleteItem(t *testing.T) {
	tenantID := "org-123"
	var capturedParams *dynamodb.DeleteItemInput

	mock := &mockDynamoDBClient{
		deleteItemFunc: func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
			capturedParams = params
			return &dynamodb.DeleteItemOutput{}, nil
		},
	}

	tenantDB, _ := TenantDBFromID(tenantID, mock)

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String("test-table"),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: "item-1"},
		},
	}

	_, err := tenantDB.DeleteItem(context.Background(), input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check tenant_id was added to key
	if val, ok := capturedParams.Key[TenantDBAttributeName]; !ok {
		t.Error("tenant_id was not added to key")
	} else if s, ok := val.(*types.AttributeValueMemberS); !ok || s.Value != tenantID {
		t.Errorf("expected tenant_id %s, got %v", tenantID, val)
	}

	// Check condition was added
	if capturedParams.ConditionExpression == nil {
		t.Error("condition expression was not added")
	}
}

func TestTenantScopedDB_BatchWriteItem(t *testing.T) {
	tenantID := "org-123"
	var capturedParams *dynamodb.BatchWriteItemInput

	mock := &mockDynamoDBClient{
		batchWriteItemFunc: func(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
			capturedParams = params
			return &dynamodb.BatchWriteItemOutput{}, nil
		},
	}

	tenantDB, _ := TenantDBFromID(tenantID, mock)

	t.Run("stamps tenant_id on all puts", func(t *testing.T) {
		input := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				"test-table": {
					{
						PutRequest: &types.PutRequest{
							Item: map[string]types.AttributeValue{
								"id": &types.AttributeValueMemberS{Value: "item-1"},
							},
						},
					},
					{
						PutRequest: &types.PutRequest{
							Item: map[string]types.AttributeValue{
								"id": &types.AttributeValueMemberS{Value: "item-2"},
							},
						},
					},
				},
			},
		}

		_, err := tenantDB.BatchWriteItem(context.Background(), input)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Check all items have tenant_id
		for _, req := range capturedParams.RequestItems["test-table"] {
			if req.PutRequest != nil {
				if val, ok := req.PutRequest.Item[TenantDBAttributeName]; !ok {
					t.Error("tenant_id was not stamped on item")
				} else if s, ok := val.(*types.AttributeValueMemberS); !ok || s.Value != tenantID {
					t.Errorf("expected tenant_id %s, got %v", tenantID, val)
				}
			}
		}
	})

	t.Run("rejects mismatched tenant_id", func(t *testing.T) {
		input := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				"test-table": {
					{
						PutRequest: &types.PutRequest{
							Item: map[string]types.AttributeValue{
								"id":        &types.AttributeValueMemberS{Value: "item-1"},
								"tenant_id": &types.AttributeValueMemberS{Value: "other-org"},
							},
						},
					},
				},
			},
		}

		_, err := tenantDB.BatchWriteItem(context.Background(), input)
		if !errors.Is(err, ErrTenantMismatch) {
			t.Errorf("expected ErrTenantMismatch, got %v", err)
		}
	})
}

func TestTenantScopedDB_Unwrap(t *testing.T) {
	mock := &mockDynamoDBClient{}
	tenantDB, _ := TenantDBFromID("org-123", mock)

	unwrapped := tenantDB.Unwrap()
	if unwrapped != mock {
		t.Error("Unwrap() did not return the underlying client")
	}
}

func TestTenantScopedDB_TransactWriteItems(t *testing.T) {
	tenantID := "org-123"
	var capturedParams *dynamodb.TransactWriteItemsInput

	mock := &mockDynamoDBClient{
		transactWriteItemsFunc: func(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
			capturedParams = params
			return &dynamodb.TransactWriteItemsOutput{}, nil
		},
	}

	tenantDB, _ := TenantDBFromID(tenantID, mock)

	input := &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{
				Put: &types.Put{
					TableName: aws.String("test-table"),
					Item: map[string]types.AttributeValue{
						"id": &types.AttributeValueMemberS{Value: "item-1"},
					},
				},
			},
			{
				Update: &types.Update{
					TableName: aws.String("test-table"),
					Key: map[string]types.AttributeValue{
						"id": &types.AttributeValueMemberS{Value: "item-2"},
					},
					UpdateExpression: aws.String("SET #name = :name"),
				},
			},
		},
	}

	_, err := tenantDB.TransactWriteItems(context.Background(), input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check Put has tenant_id
	if val, ok := capturedParams.TransactItems[0].Put.Item[TenantDBAttributeName]; !ok {
		t.Error("tenant_id was not stamped on Put item")
	} else if s, ok := val.(*types.AttributeValueMemberS); !ok || s.Value != tenantID {
		t.Errorf("expected tenant_id %s, got %v", tenantID, val)
	}

	// Check Update has tenant_id in key
	if val, ok := capturedParams.TransactItems[1].Update.Key[TenantDBAttributeName]; !ok {
		t.Error("tenant_id was not added to Update key")
	} else if s, ok := val.(*types.AttributeValueMemberS); !ok || s.Value != tenantID {
		t.Errorf("expected tenant_id %s, got %v", tenantID, val)
	}
}
