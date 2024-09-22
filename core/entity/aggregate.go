package entity

import (
	"github.com/google/uuid"
	"time"
)

type Aggregate struct {
	ID               uuid.UUID
	AggregateName    string
	SchemaVersion    string
	AggregateVersion string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Body             any
}
