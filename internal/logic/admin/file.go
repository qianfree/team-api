package admin

import (
	"context"
	"strings"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
)

// deriveFileCategory 由存储路径与 MIME 类型推导文件分类：export / image / other。
func deriveFileCategory(storagePath, mimeType string) string {
	if strings.HasPrefix(storagePath, "exports/") {
		return "export"
	}
	if strings.HasPrefix(mimeType, "image/") {
		return "image"
	}
	return "other"
}

// adminFileService 从数据库配置构造 FileService；对象存储未配置时返回业务错误。
func adminFileService(ctx context.Context) (*common.FileService, error) {
	svc, err := common.NewFileServiceFromConfig(ctx)
	if err != nil {
		return nil, common.NewBadRequestError("对象存储未配置，请先在系统设置中配置存储")
	}
	return svc, nil
}

// FileList 文件列表（分页 + 筛选）。
func (s *sAdmin) FileList(ctx context.Context, req *v1.FileListReq) (*v1.FileListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	query := dao.FilFiles.Ctx(ctx)
	if req.TenantId > 0 {
		query = query.Where("tenant_id", req.TenantId)
	}
	if req.UserId > 0 {
		query = query.Where("user_id", req.UserId)
	}
	if req.Provider != "" {
		query = query.Where("storage_provider", req.Provider)
	}
	switch req.Category {
	case "export":
		query = query.Where("storage_path LIKE ?", "exports/%")
	case "image":
		query = query.Where("mime_type LIKE ?", "image/%").Where("storage_path NOT LIKE ?", "exports/%")
	case "other":
		query = query.Where("COALESCE(mime_type,'') NOT LIKE ?", "image/%").Where("storage_path NOT LIKE ?", "exports/%")
	}
	if req.Keyword != "" {
		kw := "%" + strings.TrimSpace(req.Keyword) + "%"
		query = query.Where("(original_name LIKE ? OR storage_path LIKE ?)", kw, kw)
	}
	if req.StartDate != "" {
		query = query.Where("created_at >= ?", req.StartDate+" 00:00:00")
	}
	if req.EndDate != "" {
		query = query.Where("created_at <= ?", req.EndDate+" 23:59:59")
	}

	var total int
	rows := make([]*v1.FileItem, 0)
	err := query.OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&rows, &total, false)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		r.Category = deriveFileCategory(r.StoragePath, r.MimeType)
	}

	return &v1.FileListRes{List: rows, Total: total, Page: page, PageSize: pageSize}, nil
}

// FileStats 存储占用统计（KPI）。
func (s *sAdmin) FileStats(ctx context.Context, req *v1.FileStatsReq) (*v1.FileStatsRes, error) {
	res := &v1.FileStatsRes{
		ByProvider: make([]v1.FileProviderStat, 0),
		ByCategory: make([]v1.FileCategoryStat, 0),
		TopTenants: make([]v1.FileTenantStat, 0),
	}

	// 总量
	var totals struct {
		Count int64 `json:"count"`
		Bytes int64 `json:"bytes"`
	}
	if err := dao.FilFiles.Ctx(ctx).
		Fields("COUNT(*) AS count, COALESCE(SUM(size),0) AS bytes").
		Scan(&totals); err != nil {
		return nil, err
	}
	res.TotalCount = totals.Count
	res.TotalBytes = totals.Bytes

	// 按供应商
	if err := dao.FilFiles.Ctx(ctx).
		Fields("storage_provider AS provider, COUNT(*) AS count, COALESCE(SUM(size),0) AS bytes").
		Group("storage_provider").
		Scan(&res.ByProvider); err != nil {
		return nil, err
	}

	// 按分类（export / image / other）
	for _, cat := range []string{"export", "image", "other"} {
		count, bytes, err := s.fileCategoryStat(ctx, cat)
		if err != nil {
			return nil, err
		}
		res.ByCategory = append(res.ByCategory, v1.FileCategoryStat{Category: cat, Count: count, Bytes: bytes})
	}

	// Top 租户占用
	topN := req.TopN
	if topN <= 0 {
		topN = 10
	}
	if err := dao.FilFiles.Ctx(ctx).
		Fields("tenant_id, COUNT(*) AS count, COALESCE(SUM(size),0) AS bytes").
		Group("tenant_id").
		Order("bytes DESC").
		Limit(topN).
		Scan(&res.TopTenants); err != nil {
		return nil, err
	}

	return res, nil
}

// fileCategoryStat 统计单个分类的文件数与字节数。
func (s *sAdmin) fileCategoryStat(ctx context.Context, category string) (int64, int64, error) {
	m := dao.FilFiles.Ctx(ctx)
	switch category {
	case "export":
		m = m.Where("storage_path LIKE ?", "exports/%")
	case "image":
		m = m.Where("mime_type LIKE ?", "image/%").Where("storage_path NOT LIKE ?", "exports/%")
	default: // other
		m = m.Where("COALESCE(mime_type,'') NOT LIKE ?", "image/%").Where("storage_path NOT LIKE ?", "exports/%")
	}
	var r struct {
		Count int64 `json:"count"`
		Bytes int64 `json:"bytes"`
	}
	if err := m.Fields("COUNT(*) AS count, COALESCE(SUM(size),0) AS bytes").Scan(&r); err != nil {
		return 0, 0, err
	}
	return r.Count, r.Bytes, nil
}

// FileDownload 生成临时预览下载链接。
func (s *sAdmin) FileDownload(ctx context.Context, req *v1.FileDownloadReq) (*v1.FileDownloadRes, error) {
	count, err := dao.FilFiles.Ctx(ctx).Where("id", req.Id).Count()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, common.NewNotFoundError("文件")
	}

	svc, err := adminFileService(ctx)
	if err != nil {
		return nil, err
	}
	var url string
	if req.Variant == "thumb" {
		url, err = svc.GetThumbnailURL(ctx, req.Id, req.Width)
	} else {
		url, err = svc.GetDownloadURL(ctx, req.Id)
	}
	if err != nil {
		return nil, err
	}
	return &v1.FileDownloadRes{Url: url}, nil
}

// FileDelete 删除单个文件（存储对象 + 记录）。
func (s *sAdmin) FileDelete(ctx context.Context, req *v1.FileDeleteReq) (*v1.FileDeleteRes, error) {
	count, err := dao.FilFiles.Ctx(ctx).Where("id", req.Id).Count()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, common.NewNotFoundError("文件")
	}

	svc, err := adminFileService(ctx)
	if err != nil {
		return nil, err
	}
	if err := svc.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &v1.FileDeleteRes{}, nil
}

// FileCleanup 手动触发一次保留期清理（导出 + 图片）。
func (s *sAdmin) FileCleanup(ctx context.Context, req *v1.FileCleanupReq) (*v1.FileCleanupRes, error) {
	result, err := RunFileRetentionNow(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.FileCleanupRes{
		ExportsDeleted: result.ExportsDeleted,
		ImagesDeleted:  result.ImagesDeleted,
	}, nil
}
