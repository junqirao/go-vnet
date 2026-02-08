package quota

import (
	"context"
	"time"

	"go-vnet/manager/server/internal/model"
)

type (
	adaptor struct {
		ref         *sQuota
		quota       *model.Quota
		target      string
		targetType  string
		startTime   time.Time // 记录 adaptor 创建时的时间
		lastEndTime time.Time
	}
)

func (a *adaptor) LoadUsage(ctx context.Context) (int64, error) {
	// Delegate to sQuota.LoadUsage with quota id, target and target type
	return a.ref.LoadUsage(ctx, a.quota.Id, a.target, a.targetType)
}

func (a *adaptor) GetQuotaMaxUsage() int64 {
	// Return the maximum quota limit value from the quota configuration
	// -1 represents unlimited quota
	return int64(a.quota.Value)
}

func (a *adaptor) SubmitUsage(ctx context.Context, usage int64) {
	// Delegate to sQuota.SubmitUsage with quota id, target, target type, usage and start time
	a.ref.SubmitUsage(ctx, a.quota.Id, a.target, a.targetType, usage, a.startTime)

	// Update adaptor state after successful submission
	a.lastEndTime = time.Now()
}
