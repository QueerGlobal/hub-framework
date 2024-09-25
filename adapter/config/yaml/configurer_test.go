package yaml

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QueerGlobal/hub-framework/adapter/handler/requesthandler"
	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockTask struct {
	Config  map[string]any
	handler func(*entity.ServiceRequest)
}

func NewMockTask(config map[string]any) (entity.Task, error) {
	mockTask := &MockTask{Config: config}
	mockTask.handler = func(req *entity.ServiceRequest) {
		req.Response = &entity.ServiceResponse{
			ResponseMeta: entity.ResponseMeta{
				StatusCode: http.StatusOK,
			},
			Body: []byte("MockTask"),
		}
	}
	return mockTask, nil
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
	entity.RegisterTaskType("MockTask", entity.TaskConstructorFunc(NewMockTask))
}

func registerMockTargets() {
	entity.RegisterTargetType("MockTarget", entity.TargetConstructorFunc(NewMockTarget))
}

type MockTarget struct {
	Config map[string]any
}

func NewMockTarget(config map[string]any) (entity.Target, error) {
	return &MockTarget{Config: config}, nil
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

func TestNewConfigurer(t *testing.T) {
	c := NewConfigurer("/tmp/test/dir")
	assert.Equal(t, "/tmp/test/dir", c.applicationDirectory)
}

func TestConfigureHub(t *testing.T) {
	// register mock tasks and targets
	registerMockTasks()
	registerMockTargets()
	// Setup test directory
	testDir := setupTestDirectory(t)
	defer os.RemoveAll(testDir)

	c := NewConfigurer(testDir)

	// Create a logger for testing
	logger := zerolog.New(os.Stdout)

	// Create a new hub with the logger and a test application name
	hub, err := entity.NewHub(&logger, "TestApp")
	require.NoError(t, err)

	err = c.ConfigureHub(hub)
	require.NoError(t, err)

	// Create a new RequestHandler
	port := 8080
	requestHandler := requesthandler.NewRequestHandler(port, hub)

	// Create an HTTP POST request with a body
	body := strings.NewReader(`{"key": "value"}`)
	req, err := http.NewRequest("POST", "/testapp/testaggregate", body)
	require.NoError(t, err)

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Pass the request to the RequestHandler's ServeHTTP method
	requestHandler.ServeHTTP(rr, req)

	// Check the response status code
	assert.Equal(t, http.StatusOK, rr.Code)

	// Check the response body
	assert.NotEmpty(t, rr.Body.String(), "Response body should not be empty")
	assert.Equal(t, "MockTask", string(rr.Body.Bytes()))
}

func TestReadHubSpec(t *testing.T) {
	// register mock tasks and targets
	registerMockTasks()
	registerMockTargets()
	// Setup test directory
	testDir := setupTestDirectory(t)
	defer os.RemoveAll(testDir)

	c := NewConfigurer(testDir)
	spec, err := c.readHubSpec()
	require.NoError(t, err)
	assert.Equal(t, "1.0", spec.APIVersion)
	assert.Equal(t, "TestApp", spec.Spec.ApplicationName)
}

func TestReadAggregateSpecs(t *testing.T) {
	// register mock tasks and targets
	registerMockTasks()
	registerMockTargets()
	// Setup test directory
	testDir := setupTestDirectory(t)
	defer os.RemoveAll(testDir)

	c := NewConfigurer(testDir)
	specs, err := c.readAggregateSpecs()
	require.NoError(t, err)
	assert.Len(t, *specs, 1)
	assert.Contains(t, *specs, "test_aggregate.yaml")
}

func TestReadSchemas(t *testing.T) {
	// register mock tasks and targets
	registerMockTasks()
	registerMockTargets()
	// Setup test directory
	testDir := setupTestDirectory(t)
	defer os.RemoveAll(testDir)

	c := NewConfigurer(testDir)
	schemas, err := c.readSchemas()
	require.NoError(t, err)
	assert.Len(t, *schemas, 1)
	assert.Contains(t, *schemas, "test_schema.yaml")
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
