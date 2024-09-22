package model

import (
	"fmt"
	"time"

	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/google/uuid"
)

type StoredAggregate[T any] struct {
	ID                uuid.UUID
	AggregateTypeName string
	SchemaVersion     string
	AggregateVersion  string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Aggregate         T
}

func AggregateToStoredValue[T any](aggregate *entity.Aggregate) (*StoredAggregate[T], error) {
	aggregateBody, ok := aggregate.Body.(T)
	if !ok {
		return nil, fmt.Errorf("failed to convert stored aggregate %s ", aggregate.AggregateName)
	}

	body := &StoredAggregate[T]{
		AggregateTypeName: aggregate.AggregateName,
		AggregateVersion:  aggregate.AggregateVersion,
		SchemaVersion:     aggregate.SchemaVersion,
		CreatedAt:         aggregate.CreatedAt,
		UpdatedAt:         aggregate.UpdatedAt,
		Aggregate:         aggregateBody,
	}

	return body, nil
}

func StoredValueToAggregate[T any](in *StoredAggregate[T]) (*entity.Aggregate, error) {
	obj := &entity.Aggregate{
		ID:               in.ID,
		AggregateName:    in.AggregateTypeName,
		AggregateVersion: in.AggregateVersion,
		SchemaVersion:    in.SchemaVersion,
		CreatedAt:        in.CreatedAt,
		UpdatedAt:        in.UpdatedAt,
		Body:             in.Aggregate,
	}

	return obj, nil
}
