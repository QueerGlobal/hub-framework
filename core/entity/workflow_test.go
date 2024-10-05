package entity_test

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"testing"

	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/stretchr/testify/assert"
)

// MockTask for testing
type MockTask struct {
	applyFunc func(ctx context.Context, req entity.ServiceRequest) error
	name      string
}

func (m *MockTask) Apply(ctx context.Context, req entity.ServiceRequest) error {
	return m.applyFunc(ctx, req)
}

func (m *MockTask) Name() string {
	return m.name
}

// mockStep creates a mock WorkflowStep with predefined task and precedence.
func mockStep(precedence int) *entity.WorkflowStep {
	return &entity.WorkflowStep{
		Name:       "MockStep",
		Precedence: precedence,
		TaskType:   "MockType",
		Task: &MockTask{
			applyFunc: func(ctx context.Context, req entity.ServiceRequest) error {
				body := append(req.GetBody(), []byte("+mockstep"+strconv.Itoa(precedence))...)
				req.SetBody(body)
				return nil
			},
			name: "MockTask",
		},
	}
}

func mockServiceRequest() *entity.HTTPServiceRequest {
	mockURL, _ := url.Parse("https://example.com")

	someServiceRequest := entity.HTTPServiceRequest{
		ApiName:     "TestAPI",
		ServiceName: "TestService",
		Method:      "POST",
		URL:         mockURL,
		Body:        []byte("testbody"),
		Header:      make(map[string][]string),
	}

	return &someServiceRequest
}

func TestNewWorkflow(t *testing.T) {
	step1 := mockStep(1)
	step2 := mockStep(2)

	wf := entity.NewWorkflowTasks([]*entity.WorkflowStep{step1, step2}...)

	rqst := mockServiceRequest()

	err := wf.Apply(context.Background(), rqst)
	assert.NoError(t, err)

	assert.Equal(t, []byte("testbody+mockstep1+mockstep2"), rqst.Body)

	assert.Equal(t, 2, len(wf.Steps))
}

func TestWorkflow_Apply(t *testing.T) {
	step1 := mockStep(1)
	step2 := mockStep(2)
	step3 := mockStep(3)

	wf := entity.NewWorkflowTasks([]*entity.WorkflowStep{step1, step2, step3}...)

	req := entity.HTTPServiceRequest{
		Body: []byte("TESTSR"),
	}
	err := wf.Apply(context.Background(), &req)
	assert.NoError(t, err)
	assert.Equal(t, []byte("TESTSR+mockstep1+mockstep2+mockstep3"), req.Body)
}

func TestWorkflow_Apply_TaskError(t *testing.T) {
	step1 := &entity.WorkflowStep{
		Name:       "Step1",
		Precedence: 1,
		TaskType:   "MockType",
		Task: &MockTask{
			applyFunc: func(ctx context.Context, req entity.ServiceRequest) error {
				body := append(req.GetBody(), []byte("+mockstep1")...)
				req.SetBody(body)
				return nil
			},
			name: "MockTask",
		},
	}

	step2 := &entity.WorkflowStep{
		Name:       "Step2",
		Precedence: 2,
		TaskType:   "MockType",
		Task: &MockTask{
			applyFunc: func(ctx context.Context, req entity.ServiceRequest) error {
				return errors.New("task error")
			},
			name: "MockTask2",
		},
	}

	wf := entity.NewWorkflowTasks(step1, step2)

	req := entity.HTTPServiceRequest{Body: []byte("TEST")}
	err := wf.Apply(context.Background(), &req)
	assert.EqualError(t, err, "task error")
}
