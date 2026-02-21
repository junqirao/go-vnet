package quota

import (
	"context"
	"errors"
	"sync/atomic"
)

const (
	ValueNoLimit = -1
)

var (
	// ErrQuotaExceeded is returned when usage exceeds the quota limit.
	ErrQuotaExceeded = errors.New("quota exceeded")
	// ErrQuotaClosed is returned when quota is already closed and cannot be committed again.
	ErrQuotaClosed = errors.New("quota is already closed")
)

type (
	// Quota represents a quota counter for tracking usage locally.
	// It provides basic functionality to update and commit usage values
	// without coupling to any external systems like databases.
	Quota struct {
		ctx      context.Context
		adaptor  Adaptor
		used     int64
		maxLimit int64        // maximum allowed usage value (quota limit)
		usage    atomic.Int64 // current usage value in bytes
	}
	// Adaptor defines the interface for quota persistence.
	// Implementations are responsible for loading and persisting quota values.
	Adaptor interface {
		// LoadUsage loads the current used quota value from database.
		LoadUsage(ctx context.Context) (int64, error)
		// GetQuotaMaxUsage returns the maximum quota limit value.
		// Returns -1 for unlimited quota.
		GetQuotaMaxUsage() int64
		// SubmitUsage submits the total usage (maxLimit + current counter) to storage.
		SubmitUsage(ctx context.Context, usage int64)
	}
)

// New creates a new Quota instance with the given adaptor.
// The loaded value is used as the maximum limit, and the counter starts at 0.
func New(ctx context.Context, adaptor Adaptor) (q *Quota, err error) {
	maxLimit := adaptor.GetQuotaMaxUsage()
	used, err := adaptor.LoadUsage(ctx)
	if err != nil {
		return
	}
	q = &Quota{
		ctx:      ctx,
		adaptor:  adaptor,
		used:     used,
		maxLimit: maxLimit,
		usage:    atomic.Int64{},
	}
	q.usage.Store(0)
	return
}

// AddUsage adds the given value to the local usage counter.
// This operation is thread-safe and can be called concurrently.
//
// If the addition exceeded the quota limit, it:
// 1. Immediately commits the local counter to storage
// 2. Marks the quota as closed to prevent further commits
// 3. Returns ErrQuotaExceeded
//
// Parameters:
//   - value: the usage value to add (in bytes). Can be negative to reduce usage.
//
// Returns:
//   - error: nil on success, ErrQuotaExceeded if quota is exceeded
func (q *Quota) AddUsage(value int64) error {
	if value == 0 {
		return nil
	}

	if q.isExceeded() {
		q.CommitUsage()
		return ErrQuotaExceeded
	}

	q.usage.Add(value)
	return nil
}

func (q *Quota) IsExceeded() bool {
	return q.isExceeded()
}

func (q *Quota) isExceeded() bool {
	return q.usage.Load()+q.used > q.maxLimit && q.maxLimit != ValueNoLimit
}

// CommitUsage commits the local counter usage to the adaptor.
// Once committed, the quota is marked as closed and cannot be committed again.
//
// Returns:
//   - int64: the committed usage value (current counter)
//   - error: ErrQuotaClosed if already committed, or error from adaptor submit operation
func (q *Quota) CommitUsage() int64 {
	if q.usage.Load() == 0 {
		return 0
	}
	currentUsage := q.usage.Swap(0)
	q.adaptor.SubmitUsage(q.ctx, currentUsage)
	return currentUsage
}

// GetUsage returns the current local counter value without resetting it.
// This is useful for monitoring or checking the current usage state.
//
// Returns:
//   - int64: the current counter value
func (q *Quota) GetUsage() int64 {
	return q.usage.Load()
}

// GetMaxLimit returns the maximum allowed usage value.
//
// Returns:
//   - int64: the maximum limit
func (q *Quota) GetMaxLimit() int64 {
	return q.maxLimit
}

// Reset resets the local counter value to zero.
// This does not affect the closed state or the max limit.
func (q *Quota) Reset() {
	q.usage.Store(0)
}

func (q *Quota) Adaptor() Adaptor {
	return q.adaptor
}
