package yaml

import (
	"fmt"
	"os"
	"path/filepath"

	domainerr "github.com/QueerGlobal/hub-framework/core/entity/error"

	"gopkg.in/yaml.v2"

	"github.com/QueerGlobal/hub-framework/adapter/config/model"
	"github.com/QueerGlobal/hub-framework/core/entity"
)

type Configurer struct {
	applicationDirectory string
}

func NewConfigurer(applicationDirectory string) *Configurer {
	return &Configurer{
		applicationDirectory: applicationDirectory,
	}
}

func (c *Configurer) ConfigureHub(hub *entity.Hub) error {
	hubConfig, err := c.readHubSpec()
	if err != nil {
		return err
	}

	if err := c.applyHubSpec(hub, hubConfig); err != nil {
		return err
	}

	aggregateSpecs, err := c.readAggregateSpecs()
	if err != nil {
		return err
	}

	if err := c.applyAggregateSpecs(hub, aggregateSpecs); err != nil {
		return err
	}

	schemas, err := c.readSchemas()
	if err != nil {
		return err
	}

	if err := c.applySchemasSpec(hub, schemas); err != nil {
		return err
	}

	return nil
}

func (c *Configurer) readAggregateSpecs() (*map[string]*model.AggregateSpec, error) {
	aggregateMap := make(map[string]*model.AggregateSpec)
	aggregateDir := filepath.Join(c.applicationDirectory, "aggregates")

	files, err := os.ReadDir(aggregateDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(aggregateDir, file.Name())
		aggregateData, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		var aggregateSpec model.AggregateSpec
		err = yaml.Unmarshal(aggregateData, &aggregateSpec)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal aggregate spec %s: %v", file.Name(), err)
		}

		aggregateMap[file.Name()] = &aggregateSpec
	}

	return &aggregateMap, nil
}

func (c *Configurer) readSchemas() (*map[string]*model.SchemasSpec, error) {
	schemaMap := make(map[string]*model.SchemasSpec)
	schemaDir := filepath.Join(c.applicationDirectory, "schemas")

	files, err := os.ReadDir(schemaDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(schemaDir, file.Name())
		schemaData, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		var schemaSpec model.SchemasSpec
		err = yaml.Unmarshal(schemaData, &schemaSpec)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal schema spec %s: %v", file.Name(), err)
		}

		schemaMap[file.Name()] = &schemaSpec
	}

	return &schemaMap, nil
}

func (c *Configurer) readHubSpec() (*model.HubSpec, error) {
	hubFile := filepath.Join(c.applicationDirectory, "hub.yaml")

	hubData, err := os.ReadFile(hubFile)
	if err != nil {
		return nil, err
	}

	var hub model.HubSpec
	err = yaml.Unmarshal(hubData, &hub)
	if err != nil {
		return nil, err
	}

	return &hub, nil
}

func (c *Configurer) applyAggregateSpecs(hub *entity.Hub, specs *map[string]*model.AggregateSpec) error {
	if hub == nil || specs == nil {
		return domainerr.ErrEmptyInput
	}

	for _, aggregateSpec := range *specs {
		aggregate := &model.Aggregate{
			Name:          aggregateSpec.Spec.Name,
			APIName:       aggregateSpec.Spec.APIName,
			SchemaName:    aggregateSpec.Spec.SchemaName,
			SchemaVersion: aggregateSpec.Spec.SchemaVersion,
			IsPublic:      aggregateSpec.Spec.IsPublic,
			Handlers:      aggregateSpec.Spec.Handlers,
		}

		err := c.addOrConfigureAggregate(hub, aggregate)
		if err != nil {
			return fmt.Errorf("failed to configure aggregate %s: %w", aggregateSpec.Spec.Name, err)
		}
	}

	return nil
}

func (c *Configurer) addOrConfigureAggregate(hub *entity.Hub, aggregate *model.Aggregate) error {
	if hub == nil || aggregate == nil {
		return domainerr.ErrEmptyInput
	}

	var err error

	aggregateSvc, ok := hub.GetService(aggregate.APIName, aggregate.Name)
	if !ok {
		aggregateSvc, err = entity.NewService(aggregate.APIName, aggregate.Name, aggregate.SchemaName, aggregate.SchemaVersion, aggregate.IsPublic)
		if err != nil {
			return err
		}
	}

	aggregateSvc.IsPublic = aggregate.IsPublic
	aggregateSvc.SchemaName = aggregate.SchemaName
	aggregateSvc.SchemaVersion = aggregate.SchemaVersion
	err = c.buildHandlers(aggregateSvc, aggregate.Handlers)
	if err != nil {
		return err
	}

	hub.AddService(aggregateSvc)

	return nil
}

func (c *Configurer) buildHandlers(svc *entity.Service, handlers []model.Handler) error {
	if svc == nil || handlers == nil {
		return domainerr.ErrEmptyInput
	}

	for _, hndlr := range handlers {
		handler, err := c.buildHandler(&hndlr)
		if err != nil {
			return err
		}

		for _, method := range hndlr.Methods {
			httpMethod, err := entity.StringToHTTPMethod(method)
			if err != nil {
				return err
			}

			svc.SetHandler(httpMethod, handler)
		}

	}

	return nil
}

func (c *Configurer) buildHandler(handler *model.Handler) (*entity.Handler, error) {
	if handler == nil {
		return nil, domainerr.ErrEmptyInput
	}

	inboundWorkflow, err := c.buildWorkflow(handler.Inbound)
	if err != nil {
		return nil, fmt.Errorf("failed to build inbound workflow: %w", err)
	}

	outboundWorkflow, err := c.buildWorkflow(handler.Outbound)
	if err != nil {
		return nil, fmt.Errorf("failed to build outbound workflow: %w", err)
	}

	handlerTarget, err := c.buildTarget(&handler.Target)
	if err != nil {
		return nil, fmt.Errorf("failed to build handler target: %w", err)
	}

	entityHandler := &entity.Handler{
		InboundWorkflow:  inboundWorkflow,
		OutboundWorkflow: outboundWorkflow,
		Target:           handlerTarget,
	}

	return entityHandler, nil
}

func (c *Configurer) buildTarget(target *model.Target) (entity.Target, error) {
	if target == nil {
		return nil, domainerr.ErrEmptyInput
	}

	configuredTgt, err := entity.GetTarget(target.Type, target.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to get target: %w", err)
	}

	return configuredTgt, nil
}

func (c *Configurer) buildWorkflow(workflow []model.Task) (entity.Workflow, error) {
	if workflow == nil {
		return nil, domainerr.ErrEmptyInput
	}

	var steps []*entity.WorkflowStep
	for _, s := range workflow {
		task, err := entity.GetTask(s.Type, s.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to get task for step %s: %w", s.Name, err)
		}

		step := &entity.WorkflowStep{
			Name:          s.Name,
			Precedence:    s.Precedence,
			Description:   s.Description,
			TaskType:      s.Type,
			ExecutionType: s.ExecutionType,
			Task:          task,
		}
		steps = append(steps, step)
	}

	wkfl := entity.NewWorkflowTasks(steps...)
	return wkfl, nil
}

func (c *Configurer) applySchemasSpec(hub *entity.Hub, specs *map[string]*model.SchemasSpec) error {
	if hub == nil || specs == nil {
		return domainerr.ErrEmptyInput
	}

	for _, schemaSpec := range *specs {
		for _, schema := range schemaSpec.Spec.Schemas {
			filePath := filepath.Join(c.applicationDirectory, "schemas", schema.FileName)
			schemaData, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read schema file %s: %w", schema.FileName, err)
			}

			entity.RegisterSchema(schema.Name, schema.Version, schemaData)
		}
	}

	return nil
}

func (c *Configurer) applyHubSpec(hub *entity.Hub, s *model.HubSpec) error {
	hub.APIVersion = s.APIVersion
	hub.ApplicationName = s.Spec.ApplicationName

	return nil
}
