package tasks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/QueerGlobal/hub-framework/api"
)

type ExampleTaskGolang struct {
	message string
}

func NewExampleTaskGolang(config map[string]interface{}) *ExampleTaskGolang {
	message := "Love"
	if msg, ok := config["message"].(string); ok {
		message = msg
	}
	return &ExampleTaskGolang{message: message}
}

type Recipe struct {
	Name        string    `json:"name"`
	Ingredients []string  `json:"ingredients"`
	Steps       []string  `json:"steps"`
	Comments    []Comment `json:"comments"`
}

type Comment struct {
	Text string `json:"text"`
}

func (e *ExampleTaskGolang) Name() string {
	return "exampleTaskGolang"
}

func (e *ExampleTaskGolang) Apply(ctx context.Context, request api.ServiceRequest) error {
	// Read the body from the ServiceRequest
	if request == nil || request.GetBody() == nil {
		return fmt.Errorf("invalid request or empty body")
	}

	// Apply the changes using the execute method
	result, err := e.execute(request.GetBody())
	if err != nil {
		return fmt.Errorf("failed to execute task: %w", err)
	}

	// Update the request body with the result
	request.SetBody(result)

	return nil
}

func (e *ExampleTaskGolang) execute(input []byte) ([]byte, error) {
	// Deserialize recipe
	var recipe Recipe
	err := json.Unmarshal(input, &recipe)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal recipe: %w", err)
	}

	// Add the configured message as an ingredient
	recipe.Ingredients = append(recipe.Ingredients, e.message)

	// Serialize the updated recipe
	updatedRecipeJSON, err := json.Marshal(recipe)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated recipe: %w", err)
	}

	return updatedRecipeJSON, nil
}
