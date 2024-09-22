package entity_test

import (
	"context"
	"errors"
	"testing"

	"github.com/QueerGlobal/hub-framework/core/entity"
	domainerr "github.com/QueerGlobal/hub-framework/core/entity/error"
	"github.com/stretchr/testify/assert"
)

// Mock Handler for testing
type MockHandler struct {
	inboundWorkflow  *MockWorkflow
	outboundWorkflow *MockWorkflow
	Target           *MockTarget
}

func (m *MockHandler) Apply(ctx context.Context, req *entity.ServiceRequest) (*entity.ServiceResponse, error) {
	if m.inboundWorkflow != nil {
		if err := m.inboundWorkflow.Apply(ctx, req); err != nil {
			return nil, err
		}
	}

	// Simulate target execution
	resp := &entity.ServiceResponse{}

	if m.outboundWorkflow != nil {
		if err := m.outboundWorkflow.Apply(ctx, req); err != nil {
			return nil, err
		}
	}

	return resp, nil
}

// Mock Workflow for testing
type MockWorkflow struct {
	applyFunc func(ctx context.Context, req *entity.ServiceRequest) error
}

func (m *MockWorkflow) Apply(ctx context.Context, req *entity.ServiceRequest) error {
	return m.applyFunc(ctx, req)
}

// Mock Target for testing
type MockTarget struct{}

func (m *MockTarget) Apply(ctx context.Context, req *entity.ServiceRequest) (*entity.ServiceResponse, error) {
	return &entity.ServiceResponse{}, nil
}

func TestNewService(t *testing.T) {
	service, err := entity.NewService("/test", "TestService", "TestSchema", "1.0", true)
	assert.NoError(t, err)
	assert.Equal(t, "TestService", service.Name)
	assert.Equal(t, "TestSchema", service.SchemaName)
	assert.Equal(t, "1.0", service.SchemaVersion)
	assert.Equal(t, "/test", service.APIName)
	assert.True(t, service.IsPublic)
}

func TestDoRequest_NilRequest(t *testing.T) {
	service, _ := entity.NewService("/test", "TestService", "TestSchema", "1.0", true)
	err := service.DoRequest(context.Background(), nil)
	assert.ErrorIs(t, err, domainerr.ErrEmptyInput)
}

func TestDoRequest_MethodNotFound(t *testing.T) {
	service, _ := entity.NewService("/test", "TestService", "TestSchema", "1.0", true)
	service.Methods = map[entity.HTTPMethod]*entity.Handler{}

	req := &entity.ServiceRequest{
		Method: entity.HTTPMethodPOST,
	}

	err := service.DoRequest(context.Background(), req)
	assert.ErrorIs(t, err, entity.ErrMethodNotConfigured)
}

func TestDoRequest_ApplyWorkflowError(t *testing.T) {
	mockInboundWorkflow := &MockWorkflow{
		applyFunc: func(ctx context.Context, req *entity.ServiceRequest) error {
			return errors.New("workflow error")
		},
	}

	service, _ := entity.NewService("/test", "TestService", "TestSchema", "1.0", true)
	service.Methods = map[entity.HTTPMethod]*entity.Handler{
		entity.HTTPMethodPOST: {
			InboundWorkflow: mockInboundWorkflow,
		},
	}

	req := &entity.ServiceRequest{
		Method: entity.HTTPMethodPOST,
	}

	err := service.DoRequest(context.Background(), req)
	assert.EqualError(t, err, "workflow error")
}

func TestDoRequest_Success(t *testing.T) {
	mockInboundWorkflow := &MockWorkflow{
		applyFunc: func(ctx context.Context, req *entity.ServiceRequest) error {
			return nil
		},
	}
	mockOutboundWorkflow := &MockWorkflow{
		applyFunc: func(ctx context.Context, req *entity.ServiceRequest) error {
			return nil
		},
	}

	service, _ := entity.NewService("/test", "TestService", "TestSchema", "1.0", true)
	service.Methods = map[entity.HTTPMethod]*entity.Handler{

		entity.HTTPMethodPOST: {
			InboundWorkflow:  mockInboundWorkflow,
			OutboundWorkflow: mockOutboundWorkflow,
			Target:           &MockTarget{},
		},
	}

	req := &entity.ServiceRequest{
		Method: entity.HTTPMethodPOST,
	}

	err := service.DoRequest(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, req.Response)
}
