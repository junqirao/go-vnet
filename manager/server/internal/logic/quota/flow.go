package quota

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/junqirao/gocomponents/response"

	"go-vnet/common/quota"
	"go-vnet/manager/server/internal/dao"
	"go-vnet/manager/server/internal/model/entity"
)

// usageCacheItem represents an in-memory cache entry for quota usage
type usageCacheItem struct {
	usage     atomic.Int64
	startTime time.Time
}

// cacheKey is the composite key for usage cache
type cacheKey struct {
	id         int
	target     string
	targetType string
}

// Global in-memory cache for quota usage
var (
	usageCache sync.Map // key: cacheKey, value: *usageCacheItem
)

func (k cacheKey) String() string {
	return fmt.Sprintf("%d:%s:%s", k.id, k.target, k.targetType)
}

func (s *sQuota) SubmitUsage(ctx context.Context, id int, target string, targetType string, usage int64, startTime time.Time) {
	// Get or create cache item for this quota/target
	key := cacheKey{id: id, target: target, targetType: targetType}

	// If startTime is zero, use current time
	currentTime := startTime
	if startTime.IsZero() {
		currentTime = time.Now()
	}

	// Try to load existing cache item
	if actual, ok := usageCache.Load(key); ok {
		// Existing cache item, just add usage
		item := actual.(*usageCacheItem)
		item.usage.Add(usage)
		return
	}

	// Create new cache item
	item := &usageCacheItem{
		startTime: currentTime,
	}
	item.usage.Store(usage)

	// Try to store, handle race condition
	if actual, loaded := usageCache.LoadOrStore(key, item); loaded {
		// Another goroutine stored it first, use existing item
		existingItem := actual.(*usageCacheItem)
		existingItem.usage.Add(usage)
	}

	return
}

func (s *sQuota) LoadUsage(ctx context.Context, id int, target string, targetType string) (int64, error) {
	// Get quota configuration to determine period type
	v, err := dao.Quota.Ctx(ctx).One(dao.Quota.Columns().Id, id)
	if err != nil {
		if errors.Is(gerror.Cause(err), sql.ErrNoRows) {
			return 0, response.CodeNotFound
		}
		return 0, err
	}

	var q entity.Quota
	if err = v.Struct(&q); err != nil {
		return 0, err
	}

	// If no period specified, use no limit (return 0)
	if q.Period == "" {
		return 0, nil
	}

	// Determine time range based on period type
	now := time.Now()
	var startTime, endTime time.Time

	switch q.Period {
	case quota.PeriodTypeDay:
		// Current day: from 00:00:00 to 23:59:59
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endTime = startTime.Add(24 * time.Hour).Add(-time.Second)
	case quota.PeriodTypeMonth:
		// Current month: from 1st day 00:00:00 to last day 23:59:59
		startTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		endTime = startTime.AddDate(0, 1, 0).Add(-time.Second)
	default:
		// Unknown period type, use no limit
		return 0, nil
	}

	// Query quota_flow records within the time range
	results, err := dao.QuotaFlow.Ctx(ctx).
		Where(g.Map{
			dao.QuotaFlow.Columns().Quota:      id,
			dao.QuotaFlow.Columns().Target:     target,
			dao.QuotaFlow.Columns().TargetType: targetType,
		}).
		Where(dao.QuotaFlow.Columns().RecordEnd+">=", startTime).
		Where(dao.QuotaFlow.Columns().RecordEnd+"<=", endTime).
		All()
	if err != nil {
		return 0, err
	}

	// Sum up all usage values from database
	var totalUsage int64 = 0
	for _, record := range results {
		var flow entity.QuotaFlow
		if err = record.Struct(&flow); err != nil {
			return 0, err
		}
		totalUsage += int64(flow.Usage)
	}

	// Add in-memory cache usage if within time range
	key := cacheKey{id: id, target: target, targetType: targetType}
	if actual, ok := usageCache.Load(key); ok {
		item := actual.(*usageCacheItem)
		// Check if cache item's start time is within the period range
		if item.startTime.After(startTime) && item.startTime.Before(endTime) {
			totalUsage += item.usage.Load()
		}
	}

	return totalUsage, nil
}

// SubmitFlowToDatabase submits all in-memory cached usage to database and clears cache
func (s *sQuota) SubmitFlowToDatabase(ctx context.Context) error {
	var errorsList []error

	// Iterate through all cache items and submit to database
	usageCache.Range(func(key, value interface{}) bool {
		cacheKey := key.(cacheKey)
		item := value.(*usageCacheItem)

		// Atomically read and reset usage to avoid race conditions with concurrent SubmitUsage
		// This ensures we don't lose any usage data during submission
		for {
			currentUsage := item.usage.Load()
			if currentUsage == 0 {
				// No usage to submit, skip this item
				return true
			}

			// Try to reset to 0 atomically
			if item.usage.CompareAndSwap(currentUsage, 0) {
				// Successfully swapped, now submit the currentUsage to database
				err := s.submitToDatabase(ctx, cacheKey.id, cacheKey.target, cacheKey.targetType, currentUsage, item.startTime)
				if err != nil {
					// Failed to submit, restore the usage value back
					item.usage.Add(currentUsage)
					errorsList = append(errorsList, fmt.Errorf("failed to submit cache key %s: %w", cacheKey.String(), err))
					return true // Continue with other items even if one fails
				}

				// Successfully submitted and usage is already 0, remove item from cache
				usageCache.Delete(key)
				return true
			}
			// CAS failed, another thread modified the usage, retry
		}
	})

	// Return combined error if any occurred
	if len(errorsList) > 0 {
		return fmt.Errorf("failed to submit %d items: %v", len(errorsList), errorsList)
	}

	return nil
}

// submitToDatabase submits a single usage record to database (internal helper)
func (s *sQuota) submitToDatabase(ctx context.Context, id int, target string, targetType string, usage int64, startTime time.Time) error {
	// Query the latest quota_flow record by quota id, target and target_type
	record, err := dao.QuotaFlow.Ctx(ctx).
		Where(g.Map{
			dao.QuotaFlow.Columns().Quota:      id,
			dao.QuotaFlow.Columns().Target:     target,
			dao.QuotaFlow.Columns().TargetType: targetType,
		}).
		OrderDesc(dao.QuotaFlow.Columns().RecordEnd).
		One()
	if err != nil && !errors.Is(gerror.Cause(err), sql.ErrNoRows) {
		return err
	}

	// If startTime is zero, use current time
	currentTime := startTime
	if startTime.IsZero() {
		currentTime = time.Now()
	}

	curr := gtime.NewFromTime(currentTime)

	// Check if there is a recent record and the time difference is within 1 hour
	if record != nil {
		var flow entity.QuotaFlow
		if err = record.Struct(&flow); err != nil {
			return err
		}

		if flow.RecordEnd != nil {
			timeDiff := currentTime.Sub(flow.RecordEnd.Time)
			// Merge if time difference is within 1 hour
			if timeDiff.Abs() < time.Hour {
				// Update the existing record: add usage and update record_end
				_, err = dao.QuotaFlow.Ctx(ctx).Where(dao.QuotaFlow.Columns().Id, flow.Id).Update(g.Map{
					dao.QuotaFlow.Columns().Usage:     flow.Usage + int(usage),
					dao.QuotaFlow.Columns().RecordEnd: curr,
				})
				return err
			}
		}
	}

	// Cannot merge or no record exists, create a new flow record
	_, err = dao.QuotaFlow.Ctx(ctx).Insert(&entity.QuotaFlow{
		Quota:       id,
		Usage:       int(usage),
		Target:      target,
		TargetType:  targetType,
		RecordStart: curr,
		RecordEnd:   gtime.Now(),
	})
	return err
}
