package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/consts"
)

// CreateChangelog 创建更新日志
func (s *sAdmin) CreateChangelog(ctx context.Context, req *v1.ChangelogCreateReq) (*v1.ChangelogCreateRes, error) {
	result, err := dao.ClgChangelogs.Ctx(ctx).Data(do.ClgChangelogs{
		Version:   req.Version,
		Title:     req.Title,
		Content:   req.Content,
		Type:      req.Type,
		Status:    "draft",
		CreatedBy: common.GetCtxUserID(ctx),
	}).Insert()
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &v1.ChangelogCreateRes{Id: id}, nil
}

// ListChangelogs 更新日志列表（管理后台，含草稿）
func (s *sAdmin) ListChangelogs(ctx context.Context, req *v1.ChangelogListReq) (*v1.ChangelogListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.ClgChangelogs.Ctx(ctx)
	if req.Status != "" {
		query = query.Where("status", req.Status)
	}
	if req.Type != "" {
		query = query.Where("type", req.Type)
	}

	var total int
	rows := make([]*v1.ChangelogItem, 0)
	err := query.OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&rows, &total, false)
	if err != nil {
		return nil, err
	}

	return &v1.ChangelogListRes{
		List:     rows,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// UpdateChangelog 更新更新日志
func (s *sAdmin) UpdateChangelog(ctx context.Context, req *v1.ChangelogUpdateReq) (*v1.ChangelogUpdateRes, error) {
	var cl *struct {
		Id int64 `json:"id"`
	}
	err := dao.ClgChangelogs.Ctx(ctx).Where("id", req.Id).Scan(&cl)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if cl == nil {
		return nil, common.NewBusinessError(consts.CodeChangelogNotFound, consts.MsgChangelogNotFound)
	}

	_, err = dao.ClgChangelogs.Ctx(ctx).
		Where("id", req.Id).
		Data(do.ClgChangelogs{
			Version: req.Version,
			Title:   req.Title,
			Content: req.Content,
			Type:    req.Type,
			Status:  req.Status,
		}).
		Update()
	if err != nil {
		return nil, err
	}

	return &v1.ChangelogUpdateRes{}, nil
}

// DeleteChangelog 删除更新日志
func (s *sAdmin) DeleteChangelog(ctx context.Context, req *v1.ChangelogDeleteReq) (*v1.ChangelogDeleteRes, error) {
	var cl *struct {
		Id int64 `json:"id"`
	}
	err := dao.ClgChangelogs.Ctx(ctx).Where("id", req.Id).Scan(&cl)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if cl == nil {
		return nil, common.NewBusinessError(consts.CodeChangelogNotFound, consts.MsgChangelogNotFound)
	}

	_, err = dao.ClgChangelogs.Ctx(ctx).Where("id", req.Id).Delete()
	if err != nil {
		return nil, err
	}

	return &v1.ChangelogDeleteRes{}, nil
}

// PublishChangelog 发布更新日志
func (s *sAdmin) PublishChangelog(ctx context.Context, req *v1.ChangelogPublishReq) (*v1.ChangelogPublishRes, error) {
	var cl *struct {
		Id     int64  `json:"id"`
		Status string `json:"status"`
	}
	err := dao.ClgChangelogs.Ctx(ctx).Where("id", req.Id).Scan(&cl)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if cl == nil {
		return nil, common.NewBusinessError(consts.CodeChangelogNotFound, consts.MsgChangelogNotFound)
	}
	if cl.Status == "published" {
		return &v1.ChangelogPublishRes{}, nil
	}

	_, err = dao.ClgChangelogs.Ctx(ctx).
		Where("id", req.Id).
		Data(do.ClgChangelogs{
			Status:      "published",
			PublishedAt: gtime.Now(),
		}).
		Update()
	if err != nil {
		return nil, err
	}

	return &v1.ChangelogPublishRes{}, nil
}
