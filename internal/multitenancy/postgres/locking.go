package postgres

import (
	"errors"

	"github.com/avast/retry-go/v5"
	"gorm.io/gorm"

	"github.com/openkcm/cmk/internal/multitenancy/migrator"
)

// pg_try_advisory_xact_lock ( key bigint ) → boolean.
// Obtains the lock immediately and returns true, or returns false without waiting if the
// lock cannot be acquired immediately.
// https://www.postgresql.org/docs/16/functions-admin.html#FUNCTIONS-ADVISORY-LOCKS
const sqlTryAdvisoryXactLock = "SELECT pg_try_advisory_xact_lock(?)"

type lock struct {
	tx  *gorm.DB
	key string
}

func (p *lock) acquire() error {
	key := migrator.GenerateLockKey(p.key)
	var ok bool
	if err := p.tx.Raw(sqlTryAdvisoryXactLock, key).Scan(&ok).Error; err != nil || !ok {
		return errors.Join(
			migrator.ErrAcquireLock,
			migrator.ErrExecSQL,
			err,
		)
	}
	return nil
}

// acquireXact acquires a PostgreSQL transaction-level advisory lock, retrying per the given
// retry options. The caller is responsible for ensuring that a transaction is active, and
// that the lock is released after use.
func acquireXact(tx *gorm.DB, lockKey string, opts ...retry.Option) error {
	l := &lock{tx: tx, key: lockKey}
	if len(opts) == 0 {
		return l.acquire()
	}
	return retry.New(opts...).Do(l.acquire)
}
