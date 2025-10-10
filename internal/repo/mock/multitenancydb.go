package mock

import (
	"sync"
)

type InMemoryMultitenancyDB struct {
	databases map[string]*InMemoryDB
	mu        sync.Mutex
}

func NewInMemoryMultitenancyDB() *InMemoryMultitenancyDB {
	return &InMemoryMultitenancyDB{
		databases: make(map[string]*InMemoryDB),
	}
}

func (mt *InMemoryMultitenancyDB) CreateDB(tenantID string) (*InMemoryDB, error) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if _, ok := mt.databases[tenantID]; ok {
		return nil, ErrDbAlreadyExists
	}

	mt.databases[tenantID] = NewInMemoryDB()

	return mt.databases[tenantID], nil
}

func (mt *InMemoryMultitenancyDB) GetDB(tenantID string) *InMemoryDB {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if _, ok := mt.databases[tenantID]; !ok {
		mt.databases[tenantID] = NewInMemoryDB()
	}

	return mt.databases[tenantID]
}
