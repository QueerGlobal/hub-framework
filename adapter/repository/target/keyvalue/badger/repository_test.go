package badger_test

import (
	"github.com/QueerGlobal/qg-hub/adapter/repository/target/keyvalue/badger"
	"github.com/QueerGlobal/qg-hub/core/entity"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type TestType struct {
	Field string
}

func TestRepository_CRUD(t *testing.T) {
	repo, err := badger.NewRepository[TestType](nil)
	assert.NoError(t, err)
	defer repo.Close()

	testObj := TestType{
		Field: "value",
	}

	// Test CreateEntry
	stored := &entity.Aggregate{
		ID:               uuid.New(),
		AggregateName:    "TestAggregate",
		SchemaVersion:    "1.0",
		AggregateVersion: "1",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		Body:             testObj,
	}

	err = repo.Create(stored)
	assert.NoError(t, err)

	// Test ReadEntry
	read, err := repo.Read(stored.ID)
	assert.NoError(t, err)
	assert.Equal(t, stored.AggregateName, read.AggregateName)
	assert.Equal(t, stored.SchemaVersion, read.SchemaVersion)
	assert.Equal(t, stored.AggregateVersion, read.AggregateVersion)
	t.Log(read)

	sag := stored.Body
	rag := read.Body
	t.Log(sag)
	t.Log(rag)

	assert.Equal(t, sag, rag)
	assert.Equal(t, stored.Body, read.Body)

	// Test Update
	stored.AggregateVersion = "2"
	stored.UpdatedAt = time.Now()
	err = repo.Update(stored)
	assert.NoError(t, err)

	read, err = repo.Read(stored.ID)
	assert.NoError(t, err)
	assert.Equal(t, "2", read.AggregateVersion)

	// Test DeleteEntry
	err = repo.Delete(stored.ID)
	assert.NoError(t, err)

	read, err = repo.Read(stored.ID)
	assert.Error(t, err)
	assert.Nil(t, read)
}

func TestRepository_DeleteNonExistingEntry(t *testing.T) {
	repo, err := badger.NewRepository[TestType](nil)
	assert.NoError(t, err)
	defer repo.Close()

	nonExistentID := uuid.New()

	// Try deleting a non-existing entry
	err = repo.Delete(nonExistentID)
	assert.NoError(t, err) // TODO: Decide what we want the behavior on delete nonexistent key
}

func TestRepository_ReadNonExistingEntry(t *testing.T) {
	repo, err := badger.NewRepository[TestType](nil)
	assert.NoError(t, err)
	defer repo.Close()

	nonExistentID := uuid.New()

	// Try reading a non-existing entry
	read, err := repo.Read(nonExistentID)
	assert.Error(t, err)
	assert.Nil(t, read)
}

func TestRepository_CreateInvalidEntry(t *testing.T) {
	repo, err := badger.NewRepository[TestType](nil)
	assert.NoError(t, err)
	defer repo.Close()

	entityAgg := entity.Aggregate{
		ID:               uuid.New(),
		AggregateName:    "testAggregate",
		SchemaVersion:    "v0.1.1",
		AggregateVersion: "asdf",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		Body:             make(chan int),
	}

	err = repo.Create(&entityAgg)
	assert.Error(t, err)
}

func TestRepository_UpdateInvalidEntry(t *testing.T) {
	repo, err := badger.NewRepository[TestType](nil)
	assert.NoError(t, err)
	defer repo.Close()

	entityAgg := entity.Aggregate{
		ID:               uuid.New(),
		AggregateName:    "testAggregate",
		SchemaVersion:    "v0.1.1",
		AggregateVersion: "asdf",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		Body:             make(chan int),
	}

	err = repo.Update(&entityAgg)
	assert.Error(t, err)
}
