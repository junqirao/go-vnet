package quota

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/junqirao/gocomponents/response"

	"go-vnet/common/quota"
	"go-vnet/manager/server/internal/dao"
	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/model/entity"
	"go-vnet/manager/server/internal/service"
)

func init() {
	service.RegisterQuota(new(sQuota))
}

type (
	sQuota struct{}
)

func (s *sQuota) GetById(ctx context.Context, id int) (q *model.Quota, err error) {
	if id == quota.IdNoLimit {
		return &model.Quota{
			Name:  "unlimited",
			Value: quota.ValueNoLimit,
		}, nil
	}
	record, err := dao.Quota.Ctx(ctx).One(dao.Quota.Columns().Id, id)
	if err != nil {
		return
	}
	eq := &entity.Quota{}
	if err = record.Struct(&eq); err != nil {
		if errors.Is(gerror.Cause(err), sql.ErrNoRows) {
			err = response.CodeNotFound
			return
		}
	}
	q = &model.Quota{
		Id:     eq.Id,
		Name:   eq.Name,
		Type:   eq.Type,
		Value:  eq.Value,
		Period: eq.Period,
	}
	return
}

func (s *sQuota) List(ctx context.Context) (quotas []*entity.Quota, err error) {
	result, err := dao.Quota.Ctx(ctx).All()
	if err != nil {
		return
	}
	quotas = make([]*entity.Quota, 0)
	for _, record := range result {
		q := &entity.Quota{}
		if err = record.Struct(&q); err != nil {
			return
		}
		quotas = append(quotas, q)
	}
	return
}

func (s *sQuota) Create(ctx context.Context, quota *entity.Quota) (err error) {
	_, err = dao.Quota.Ctx(ctx).Insert(quota)
	return
}

func (s *sQuota) Update(ctx context.Context, id int, fields map[string]any) (err error) {
	updateMap := g.Map{}
	canUpdateFields := []string{
		dao.Quota.Columns().Name,
		dao.Quota.Columns().Value,
	}
	for _, f := range canUpdateFields {
		if v, ok := fields[f]; ok && v != nil {
			updateMap[f] = v
		}
	}
	if len(updateMap) == 0 {
		return nil
	}
	_, err = dao.Quota.Ctx(ctx).Where(dao.Quota.Columns().Id, id).Update(updateMap)
	return
}

func (s *sQuota) Delete(ctx context.Context, id int) (err error) {
	q, err := s.GetById(ctx, id)
	if err != nil || q.Id == quota.IdNoLimit {
		return
	}

	return dao.Quota.Ctx(ctx).Transaction(ctx, func(ctx context.Context, tx gdb.TX) (err error) {
		// Reset the quota value of the associated object to no limit
		// device
		if err = s.resetQuotaValueOfAssociatedObject(dao.NetworkDevice.Ctx(ctx), dao.NetworkDevice.Columns().Id, id); err != nil {
			return
		}
		if err = s.resetQuotaValueOfAssociatedObject(dao.Network.Ctx(ctx), dao.Network.Columns().Id, id); err != nil {
			return
		}
		_, err = dao.Quota.Ctx(ctx).Delete(dao.Quota.Columns().Id, id)
		return
	})
}

func (s *sQuota) resetQuotaValueOfAssociatedObject(model *gdb.Model, idField string, id int) (err error) {
	var res gdb.Result
	res, err = model.Where("quota", id).All()
	if err != nil {
		return
	}
	ids := make([]int, 0)
	for _, v := range res {
		ids = append(ids, gconv.Int(v.Map()[idField]))
	}
	_, err = model.WhereIn(idField, ids).Update(g.Map{
		"quota": quota.IdNoLimit,
	})
	return
}

func (s *sQuota) GetQuotaAdaptor(ctx context.Context, id int, target string, targetType string) (a quota.Adaptor, err error) {
	q, err := s.GetById(ctx, id)
	if err != nil {
		return
	}

	a = &adaptor{
		ref:        s,
		quota:      q,
		target:     target,
		targetType: targetType,
		startTime:  time.Now(),
	}
	return
}
