package builtin

type RequestBodyValidator interface {
}

type requestJSONValidator[T any] struct {
	fieldRules map[string][]string
}

func NewRequestValidator[T any](rules map[string][]string) RequestBodyValidator {
	return &requestJSONValidator[T]{
		fieldRules: rules,
	}
}

func (r *requestJSONValidator[T]) Validate(body []byte) error {
	return nil
}
