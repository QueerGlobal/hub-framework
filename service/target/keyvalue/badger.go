package keyvalue

/*

import (
	"context"

	"github.com/QueerGlobal/hub-framework/core/adapter/target/keyvalue/badger"
	"github.com/QueerGlobal/hub-framework/core/entity"
	"github.com/dgraph-io/badger/v4"
)

type Badger[T any] struct {
	repo *badger.Repository[T]
}

func NewBadger[T any](config map[string]interface{}) (entity.Target, error) {
	repo, err := badger.NewRepository[T](config)
	if err != nil {
		return nil, err
	}

	return &Badger[T]{repo: repo}, nil

}

func (b *Badger[T]) Apply(ctx context.Context, req entity.ServiceRequest) (entity.ServiceResponse, error) {
	var aggregate T
	body := req.GetBody()
	err := json.Unmarshal(body, &aggregate)
	if err != nil {
		return entity.ServiceResponse{}, fmt.Errorf("failed to unmarshal request body: %w", err)
	}
	req.SetAggregate(&aggregate)

	switch req.GetMethod() {
	case "POST":

		err := b.repo.Create(req.GetAggregate())
		if err != nil {
			return entity.ServiceResponse{}, err
		}
		return entity.ServiceResponse{StatusCode: 201}, nil
	case "PUT":
		err := b.repo.Update(req.GetAggregate())
		if err != nil {
			return entity.ServiceResponse{}, err
		}
		return entity.ServiceResponse{StatusCode: 200}, nil
	case "DELETE":
		err := b.repo.Delete(req.GetAggregate().GetID())
		if err != nil {
			return entity.ServiceResponse{}, err
		}
		return entity.ServiceResponse{StatusCode: 204}, nil
	case "GET":
		aggregate, err := b.repo.Get(req.Aggregate.ID)
		if err != nil {
			return entity.ServiceResponse{}, err
		}
		return entity.ServiceResponse{
			StatusCode: 200,
			Aggregate:  aggregate,
		}, nil
	default:
		return entity.ServiceResponse{}, entity.ErrUnsupportedHTTPMethod
	}

}
*/
