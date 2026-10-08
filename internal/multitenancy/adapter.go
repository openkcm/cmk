package multitenancy

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/multitenancy/gmterrors"
)

type (
	// Adapter defines an interface for enhancing [gorm.DB] instances with additional functionalities.
	Adapter interface {
		// AdaptDB enhances an existing [gorm.DB] instance with additional functionalities and returns
		// a new [DB] instance. The returned DB instance should be used by a single goroutine at a time
		// to ensure thread safety and prevent concurrent access issues.
		AdaptDB(ctx context.Context, db *gorm.DB) (*DB, error)
	}

	// adapterMux is a multiplexer that holds a map of driver names to their respective adapters.
	adapterMux struct {
		mu      sync.RWMutex       // Protects access to the drivers map.
		drivers map[string]Adapter // Maps driver names to their respective adapters.
	}
)

// Adapter registry errors.
var (
	ErrDriverAlreadyRegistered = errors.New("driver already registered")
	ErrNoRegisteredAdapter     = errors.New("no registered adapter for driver")
)

// Register adds a new adapter to the registry under the specified driver name.
// It panics if an Adapter for the given driver name is already registered.
func (mux *adapterMux) Register(driver string, adapter Adapter) {
	mux.mu.Lock()
	defer mux.mu.Unlock()
	if mux.drivers == nil {
		mux.drivers = make(map[string]Adapter)
	}
	if _, exists := mux.drivers[driver]; exists {
		panic(gmterrors.New(fmt.Errorf("%w: %s", ErrDriverAlreadyRegistered, driver)))
	}
	mux.drivers[driver] = adapter
}

// AdaptDB creates a new [DB] instance using the provided db instance and driver name.
// It returns an error if no adapter is registered for the given driver name.
func (mux *adapterMux) AdaptDB(ctx context.Context, db *gorm.DB) (*DB, error) {
	driverName := db.Name()
	mux.mu.RLock()
	adapter, ok := mux.drivers[driverName]
	mux.mu.RUnlock()
	if !ok {
		return nil, gmterrors.New(fmt.Errorf("%w: %s", ErrNoRegisteredAdapter, driverName))
	}
	return adapter.AdaptDB(ctx, db)
}

var defaultDriverMux = new(adapterMux)

// Register adds a new [Adapter] to the default registry under the specified driver name.
// It panics if an [Adapter] for the given driver name is already registered.
func Register(name string, adapter Adapter) {
	defaultDriverMux.Register(name, adapter)
}

// Open is a drop-in replacement for [gorm.Open]. It returns a new [DB] instance using
// the provided dialector (see the postgres subpackage) and options.
//
//	import (
//		gormpg "gorm.io/driver/postgres"
//
//		"github.com/openkcm/cmk/internal/multitenancy"
//		"github.com/openkcm/cmk/internal/multitenancy/postgres"
//	)
//
//	dsn := "postgres://user:password@localhost:5432/dbname?sslmode=disable"
//	db, err := multitenancy.Open(postgres.New(postgres.Config{Config: gormpg.Config{DSN: dsn}}))
//	if err != nil {
//		// handle err
//	}
func Open(dialector gorm.Dialector, opts ...gorm.Option) (*DB, error) {
	db, err := gorm.Open(dialector, opts...)
	if err != nil {
		return nil, gmterrors.New(fmt.Errorf("failed to open gorm database: %w", err))
	}
	return defaultDriverMux.AdaptDB(context.TODO(), db)
}
