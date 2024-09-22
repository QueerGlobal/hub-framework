package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QueerGlobal/hub-framework/api"
	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockTask struct {
	Config  map[string]any
	handler func(*entity.ServiceRequest)
}

func NewMockTask(config map[string]any) entity.Task {
	mockTask := &MockTask{Config: config}
	mockTask.handler = func(req *entity.ServiceRequest) {
		req.Response = &entity.ServiceResponse{
			ResponseMeta: entity.ResponseMeta{
				StatusCode: http.StatusOK,
			},
			Body: []byte("MockTask"),
		}
	}
	return mockTask
}

func (t *MockTask) Apply(ctx context.Context, req *entity.ServiceRequest) error {
	req.Response = &entity.ServiceResponse{}
	t.handler(req)
	return nil
}

func (t *MockTask) Name() string {
	return "MockTask"
}

func registerMockTasks() {
	api.RegisterTaskType("MockTask", entity.TaskConstructorFunc(NewMockTask))
}

func registerMockTargets() {
	api.RegisterTargetType("MockTarget", entity.TargetConstructorFunc(NewMockTarget))
}

type MockTarget struct {
	Config map[string]any
}

func NewMockTarget(config map[string]any) entity.Target {
	return &MockTarget{Config: config}
}

func (t *MockTarget) Apply(ctx context.Context, req *entity.ServiceRequest) (*entity.ServiceResponse, error) {
	return &entity.ServiceResponse{
		ResponseMeta: entity.ResponseMeta{
			StatusCode: http.StatusOK,
		},
		Body: []byte("MockTarget"),
	}, nil
}

func (t *MockTarget) Name() string {
	return "MockTarget"
}

func TestApplicationIntegrationNonNilRequest(t *testing.T) {
	// register mock tasks and targets
	registerMockTasks()
	registerMockTargets()
	// Setup test directory
	testDir := setupTestDirectory(t)
	defer os.RemoveAll(testDir)

	// Create a new application
	app := api.NewApplication("TestApp",
		api.WithPrivatePort(3542),
		api.WithPublicPort(3541),
		api.WithLogLevel(api.DebugLevel),
		api.WithApplicationHome(testDir),
	)

	// Start the application
	err := app.Start()
	if err != nil {
		t.Fatalf("Failed to start application: %v", err)
	}

	// Allow some time for the hub to start
	time.Sleep(2 * time.Second)

	// Create an HTTP POST request with a body
	body := strings.NewReader(`{"key": "value"}`)
	req, err := http.NewRequest("POST", "/testapp/testaggregate", body)
	require.NoError(t, err)

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Pass the request to the RequestHandler's ServeHTTP method
	app.PublicHandler.ServeHTTP(rr, req)

	// Check the response status code
	assert.Equal(t, http.StatusOK, rr.Code)

	// Check the response body
	assert.NotEmpty(t, rr.Body.String(), "Response body should not be empty")
	assert.Equal(t, "MockTask", (rr.Body.String()))

}

func setupTestDirectory(t *testing.T) string {
	testDir, err := os.MkdirTemp("", "configurer_test")
	require.NoError(t, err)

	// Create hub.yaml
	hubYAML := `
apiVersion: "1.0"
spec:
  applicationName: TestApp
`
	err = os.WriteFile(filepath.Join(testDir, "hub.yaml"), []byte(hubYAML), 0644)
	require.NoError(t, err)

	// Create aggregates directory and a test aggregate file
	aggregatesDir := filepath.Join(testDir, "aggregates")
	err = os.Mkdir(aggregatesDir, 0755)
	require.NoError(t, err)
	aggregateYAML := `
spec:
  name: testAggregate
  apiName: testApp
  isPublic: true
  schemaName: TestSchema
  schemaVersion: v0.0.1
  handlers:
    - methods: ["POST", "PUT", "DELETE"]
      inbound:
        - name: mocktask
          type: MockTask
          precedence: 1
          executionType: sync
          mustFinish: true
          onError: LogAndIgnore
          enabled: True
          config:
            value: value1
      outbound:
        - name: mockResponseTask
          type: MockTask
          precedence: 1
          executionType: sync
          onError: LogAndFail
          enabled: True
          config:
            value: value2
      target:
        name: persistTest
        type: MockTarget
`
	err = os.WriteFile(filepath.Join(aggregatesDir, "test_aggregate.yaml"), []byte(aggregateYAML), 0644)
	require.NoError(t, err)

	// Create schemas directory and a test schema file
	schemasDir := filepath.Join(testDir, "schemas")
	err = os.Mkdir(schemasDir, 0755)
	require.NoError(t, err)
	schemaYAML := `
spec:
  name: TestSchema
  version: "1.0"
`
	err = os.WriteFile(filepath.Join(schemasDir, "test_schema.yaml"), []byte(schemaYAML), 0644)
	require.NoError(t, err)

	// Create tasks directory and a test task file
	tasksDir := filepath.Join(testDir, "tasks")
	err = os.Mkdir(tasksDir, 0755)
	require.NoError(t, err)
	taskYAML := `
apiVersion: v1
specType: Tasks
namespace: builtin
spec:

`
	err = os.WriteFile(filepath.Join(tasksDir, "test_task.yaml"), []byte(taskYAML), 0644)
	require.NoError(t, err)

	return testDir
}
