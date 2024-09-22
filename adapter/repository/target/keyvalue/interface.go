package keyvalue

import (
	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/google/uuid"
)

type Repository interface {
	TargetType() string
	Create(in *entity.Aggregate) error
	Read(id uuid.UUID) (*entity.Aggregate, error)
	Update(in *entity.Aggregate) error
	Delete(id uuid.UUID) error
}
