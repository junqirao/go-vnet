package quota

import (
	"context"
	"time"

	"go-vnet/manager/server/internal/model/entity"
)

type (
	adaptor struct {
		ref         *sQuota
		quota       *entity.Quota
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

func (a *adaptor) SubmitUsage(ctx context.Context, usage int64) error {
	// Delegate to sQuota.SubmitUsage with quota id, target, target type, usage and start time
	err := a.ref.SubmitUsage(ctx, a.quota.Id, a.target, a.targetType, usage, a.startTime)
	if err != nil {
		return err
	}

	// Update adaptor state after successful submission
	a.lastEndTime = time.Now()
	return nil
}
