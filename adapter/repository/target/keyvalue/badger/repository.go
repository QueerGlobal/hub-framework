// Package badger provides a simple repository implementation using the Badger database.
// This code is intended for testing and local development only.
// It handles basic CRUD operations with a UUID as the key in a BadgerDB instance.
//
// NOTE: This implementation is not optimized for production use.
//
//	Error handling could be more robust, and there are no performance optimizations.
//	Additionally, the database instance is not closed, so ensure that the repository
//	is not used in a long-running production environment without adding resource management.
package badger

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/QueerGlobal/hub-framework/core/entity"

	"github.com/QueerGlobal/hub-framework/adapter/repository/target/model"
	domainerr "github.com/QueerGlobal/hub-framework/core/entity/error"
	badger "github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
)

// Repository encapsulates the BadgerDB instance for managing stored values.
type Repository[T any] struct {
	db     *badger.DB
	dbPath string
}

// NewRepository initializes a new BadgerDB instance and returns a Repository.
// This function should only be used in testing and local development environments.
func NewRepository[T any](config *map[string]any) (*Repository[T], error) {
	repo := Repository[T]{
		dbPath: "/tmp/badgerdb",
	}

	if config != nil {
		cfgpath, ok := (*config)["path"]
		if ok {
			repo.dbPath = cfgpath.(string)
		}
	}

	db, err := badger.Open(badger.DefaultOptions("/tmp/badgerdb"))
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB: %w", err)
	}

	repo.db = db

	return &repo, nil
}

// Update updates an existing entry in the BadgerDB based on the provided StoredAggregate.
// The ID field of the StoredAggregate is used as the key.
func (r *Repository[T]) Update(in *entity.Aggregate) error {
	if in == nil {
		return domainerr.ErrEmptyInput
	}

	storedAggregate, err := model.AggregateToStoredValue[T](in)
	if err != nil {
		return fmt.Errorf("failed to build StoredAggregate: %w", err)
	}

	body, err := json.Marshal(storedAggregate)
	if err != nil {
		return fmt.Errorf("failed to marshal StoredAggregate: %w", err)
	}

	id := in.ID[:] // Assumes in.ID is a UUID represented as [16]byte

	err = r.db.Update(func(txn *badger.Txn) error {
		return txn.Set(id, body)
	})
	if err != nil {
		return fmt.Errorf("failed to update entry in BadgerDB: %w", err)
	}

	return nil
}

// CreateEntry creates a new entry in the BadgerDB with a generated UUID as the key.
// The UUID is generated and used internally; it is not set on the input StoredAggregate.
func (r *Repository[T]) Create(in *entity.Aggregate) error {
	if in == nil {
		return domainerr.ErrEmptyInput
	}

	storedAggregate, err := model.AggregateToStoredValue[T](in)
	if err != nil {
		return fmt.Errorf("failed to build StoredAggregate: %w", err)
	}

	body, err := json.Marshal(storedAggregate)
	if err != nil {
		return fmt.Errorf("failed to marshal StoredAggregate: %w", err)
	}

	key := in.ID[:]

	err = r.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, body)
	})
	if err != nil {
		return fmt.Errorf("failed to create entry in BadgerDB: %w", err)
	}

	return nil
}

// ReadEntry retrieves an entry from the BadgerDB based on the provided UUID key.
// The method returns the corresponding StoredAggregate object or an error if the entry is not found.
func (r *Repository[T]) Read(id uuid.UUID) (*entity.Aggregate, error) {
	var result model.StoredAggregate[T]

	err := r.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(id[:])
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return fmt.Errorf("entry not found for key %v: %w", id, err)
			}
			return fmt.Errorf("failed to get entry from BadgerDB: %w", err)
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &result)
		})
	})
	if err != nil {
		return nil, err
	}

	resultAgg, err := model.StoredValueToAggregate[T](&result)

	return resultAgg, nil
}

// DeleteEntry removes an entry from the BadgerDB based on the provided UUID key.
// The method returns an error if the entry could not be deleted.
func (r *Repository[T]) Delete(id uuid.UUID) error {
	err := r.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(id[:])
	})
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("entry not found for key %v: %w", id, err)
		}
		return fmt.Errorf("failed to delete entry from BadgerDB: %w", err)
	}

	return nil
}

// Close gracefully closes the BadgerDB instance.
// This function should be called when the repository is no longer needed to prevent resource leaks.
func (r *Repository[T]) Close() error {
	return r.db.Close()
}
