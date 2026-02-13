package quota

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/common/quota"
	"go-vnet/manager/server/internal/dao"
	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/model/entity"
)

func (s *sQuota) SubmitUsage(ctx context.Context, id int, target string, targetType string, usage int64, startTime time.Time) {
	// If startTime is zero, use current time
	currentTime := startTime
	if startTime.IsZero() {
		currentTime = time.Now()
	}

	err := s.submitToDatabase(ctx, id, target, targetType, usage, currentTime)
	if err != nil {
		g.Log().Errorf(ctx, "SubmitUsage error: %v", err)
		return
	}

	g.Log().Infof(ctx, "submit usage success: id=%d,target=%s, target_type=%s, value=%v", id, target, targetType, usage)

	return
}

func (s *sQuota) LoadUsage(ctx context.Context, id int, target string, targetType string) (usage int64, flow []*model.QuotaFlowBrief, err error) {
	q, err := s.GetById(ctx, id)
	if err != nil {
		usage = -1
		return
	}

	// Determine time range based on period type
	now := time.Now()
	var startTime, endTime time.Time

	switch q.Period {
	case quota.PeriodTypeDay:
		// Current day: from 00:00:00 to 23:59:59
		startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endTime = startTime.Add(24 * time.Hour).Add(-time.Second)
	default:
		// default quota.PeriodTypeMonth
		// Current month: from 1st day 00:00:00 to last day 23:59:59
		startTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		endTime = startTime.AddDate(0, 1, 0).Add(-time.Second)
	}

	// Query quota_flow records within the time range
	results, err := dao.QuotaFlow.Ctx(ctx).
		Where(g.Map{
			dao.QuotaFlow.Columns().Quota:      id,
			dao.QuotaFlow.Columns().Target:     target,
			dao.QuotaFlow.Columns().TargetType: targetType,
		}).
		WhereGTE(dao.QuotaFlow.Columns().RecordEnd, startTime.Unix()).
		WhereLTE(dao.QuotaFlow.Columns().RecordEnd, endTime.Unix()).
		All()
	if err != nil {
		return
	}

	// Sum up all usage values from database
	for _, record := range results {
		var f entity.QuotaFlow
		if err = record.Struct(&f); err != nil {
			return
		}
		usage += f.Usage
		flow = append(flow, &model.QuotaFlowBrief{
			Usage:       f.Usage,
			RecordStart: f.RecordStart,
			RecordEnd:   f.RecordEnd,
		})
	}

	return
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
	if err != nil {
		return err
	}

	// If startTime is zero, use current time
	if startTime.IsZero() {
		startTime = time.Now()
	}

	curr := startTime.Unix()

	var flow entity.QuotaFlow
	err = record.Struct(&flow)
	if err == nil {
		// Check if there is a recent record and the time difference is within 1 hour
		if flow.RecordEnd > 0 {
			timeDiff := time.Duration(curr-flow.RecordEnd) * time.Second
			// Merge if time difference is within 1 hour
			if timeDiff.Abs() < time.Hour {
				// Update the existing record: add usage and update record_end
				_, err = dao.QuotaFlow.Ctx(ctx).Where(dao.QuotaFlow.Columns().Id, flow.Id).Update(g.Map{
					dao.QuotaFlow.Columns().Usage:     flow.Usage + usage,
					dao.QuotaFlow.Columns().RecordEnd: curr,
				})
				return err
			}
		}
	} else if !errors.Is(gerror.Cause(err), sql.ErrNoRows) {
		return err
	}
	// Cannot merge or no record exists, create a new flow record
	_, err = dao.QuotaFlow.Ctx(ctx).Insert(&entity.QuotaFlow{
		Quota:       id,
		Usage:       usage,
		Target:      target,
		TargetType:  targetType,
		RecordStart: curr,
		RecordEnd:   time.Now().Unix(),
	})
	return err
}
