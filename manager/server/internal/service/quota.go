// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"
	"go-vnet/common/quota"
	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/model/entity"
	"time"
)

type (
	IQuota interface {
		SubmitUsage(ctx context.Context, id int, target string, targetType string, usage int64, startTime time.Time)
		LoadUsage(ctx context.Context, id int, target string, targetType string) (int64, error)
		GetById(ctx context.Context, id int) (q *model.Quota, err error)
		List(ctx context.Context) (quotas []*entity.Quota, err error)
		Create(ctx context.Context, quota *entity.Quota) (err error)
		Update(ctx context.Context, id int, fields map[string]any) (err error)
		Delete(ctx context.Context, id int) (err error)
		GetQuotaAdaptor(ctx context.Context, id int, target string, targetType string) (a quota.Adaptor, err error)
	}
)

var (
	localQuota IQuota
)

func Quota() IQuota {
	if localQuota == nil {
		panic("implement not found for interface IQuota, forgot register?")
	}
	return localQuota
}

func RegisterQuota(i IQuota) {
	localQuota = i
}
