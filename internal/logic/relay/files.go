// Package relay —— /v1/files（OpenAI 兼容素材上传）业务逻辑。
//
// 设计约束（见 docs/协议文档/OpenAI/OpenAI-Files-API文档.md）：
//   - 仅对象存储（s3/minio/r2/oss/cos）可用；local 模式上传直接报错，
//     不做降级——签名 URL 是该功能的核心出口，local 无法提供
//   - 对外 ID 是不可枚举的 ext_id（file-<32hex>），与内部自增 id 解耦，
//     所有读写按 tenant_id 隔离
//   - 签名 URL 按需现场签发（TTL 由 relay_files_url_ttl_minutes 控制），绝不落库
//   - 存储路径统一 uploads/ 前缀，被文件保留期清理豁免
package relay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/google/uuid"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/model/entity"
)

// /v1/files 相关哨兵错误。上层 handler 用 errors.Is 精确映射 HTTP 状态码与
// OpenAI error code；中转替换链路用 ErrRelayFileRefUnknown 区分「引用了不存在的
// 文件」（客户端错误）与存储故障（服务端错误）。
var (
	ErrRelayFileNotFound      = errors.New("relay file not found")
	ErrRelayFileQuotaExceeded = errors.New("relay file quota exceeded")
	ErrRelayFileBadPurpose    = errors.New("invalid file purpose")
	ErrRelayFileRefUnknown    = errors.New("unknown file reference")
	// ErrRelayFileTypeNotAllowed / ErrRelayFileSizeExceeded 上传内容策略拒绝。
	// 拒绝话术携带策略本身（允许的类型/大小上限）——这是面向租户公开的
	// 使用约束，与部署细节类信息不同，帮助合法用户自助修正。
	ErrRelayFileTypeNotAllowed = errors.New("file type not allowed")
	ErrRelayFileSizeExceeded   = errors.New("file too large")
)

// allowedRelayFilePurposes OpenAI 官方 purpose 取值（含 fine-tuning 别名，宽松放行）。
// purpose 仅作元数据与列表过滤，不驱动网关行为——下游引用能否生效由中转层决定。
var allowedRelayFilePurposes = map[string]struct{}{
	"assistants":  {},
	"batch":       {},
	"fine-tune":   {},
	"fine-tuning": {},
	"vision":      {},
	"user_data":   {},
	"evals":       {},
}

// cloudStorageProviders 支持签名 URL 的对象存储 provider 集合（local 不在其中）。
var cloudStorageProviders = map[string]struct{}{
	"s3": {}, "minio": {}, "r2": {}, "oss": {}, "cos": {},
}

// RelayFileObject OpenAI File 对象（含 url/url_expires_at 网关扩展字段）。
// url 在出口处现场签发；签名失败降级省略（文件本身可用，file_id 引用不受影响）。
type RelayFileObject struct {
	ID            string `json:"id"`
	Object        string `json:"object"`
	Bytes         int64  `json:"bytes"`
	CreatedAt     int64  `json:"created_at"`
	Filename      string `json:"filename"`
	Purpose       string `json:"purpose"`
	Status        string `json:"status"`
	StatusDetails any    `json:"status_details"`
	URL           string `json:"url,omitempty"`
	URLExpiresAt  int64  `json:"url_expires_at,omitempty"`
}

// RelayFilesUnavailableMsg 存储未配置/不可用时对外的统一话术。
// 不暴露供应商列表、local 模式、配置缺失等部署细节（可被用于指纹识别），
// 真实原因只进服务端日志。
const RelayFilesUnavailableMsg = "file uploads are temporarily unavailable on this server"

// cloudFileService 校验对象存储已配置并构造 FileService。
//
// 不能直接用 common.IsStorageConfigured：它只查 provider+bucket 非空，
// provider=local 且桶非空时同样返回 true，而 local 无签名 URL 能力。
// 未配置时返回包装的 common.ErrStorageNotConfigured，保留可识别语义。
func cloudFileService(ctx context.Context) (*common.FileService, error) {
	cfg := common.GetStorageConfig(ctx)
	if _, ok := cloudStorageProviders[cfg.Provider]; !ok || cfg.Bucket == "" {
		// 拒绝原因只进日志（含 provider 名与配置状态），便于管理员定位
		g.Log().Warningf(ctx, "[RelayFiles] unavailable: storage provider=%q bucket_set=%v",
			cfg.Provider, cfg.Bucket != "")
		return nil, gerror.Wrap(common.ErrStorageNotConfigured, RelayFilesUnavailableMsg)
	}
	// 门禁已排除空配置，此处的 local 降级分支不会命中
	svc, err := common.NewFileServiceFromConfig(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[RelayFiles] init file service failed: %v", err)
		return nil, gerror.Wrap(common.ErrStorageNotConfigured, RelayFilesUnavailableMsg)
	}
	if _, ok := cloudStorageProviders[svc.ProviderName()]; !ok {
		g.Log().Warningf(ctx, "[RelayFiles] unavailable: resolved provider=%q", svc.ProviderName())
		return nil, gerror.Wrap(common.ErrStorageNotConfigured, RelayFilesUnavailableMsg)
	}
	return svc, nil
}

// relayFileURLTTL 读取签名 URL 有效期配置（分钟），非法回退默认 60。
func relayFileURLTTL(ctx context.Context) time.Duration {
	minutes := common.Config().GetInt(ctx, "relay_files_url_ttl_minutes")
	if minutes <= 0 {
		minutes = 60
	}
	return time.Duration(minutes) * time.Minute
}

// relayFilesAllowedTypes 读取类型白名单配置（逗号分隔），空/未配置 = 不限制。
func relayFilesAllowedTypes(ctx context.Context) []string {
	raw := strings.TrimSpace(common.Config().GetString(ctx, "relay_files_allowed_types"))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	entries := make([]string, 0, len(parts))
	for _, p := range parts {
		if e := strings.TrimSpace(p); e != "" {
			entries = append(entries, e)
		}
	}
	return entries
}

// relayFilesMaxSizeBytes 读取单文件大小上限配置，0 = 不限制。
// 传输层另有全局 clientMaxBodySize（RelayBodyLimit 中间件 413），实际生效
// 上限为二者较小值；本配置提供独立于全局值的收紧旋钮。
func relayFilesMaxSizeBytes(ctx context.Context) int64 {
	mb := common.Config().GetInt(ctx, "relay_files_max_size_mb")
	if mb <= 0 {
		return 0
	}
	return int64(mb) * 1024 * 1024
}

// mimePrefixExts 常见 MIME 大类对应的扩展名（小写、不带点）。prefix/* 条目在
// 声明 MIME 缺失或失真（工具常发 application/octet-stream）时按扩展名兜底，
// 否则合法图片上传会被 Postman 等工具的默认 MIME 误伤。application/ 大类
// 过宽不设兜底表，需用完整 MIME 条目（如 application/pdf）精确匹配。
var mimePrefixExts = map[string][]string{
	"image/": {"jpg", "jpeg", "png", "gif", "webp", "bmp", "svg", "ico", "tif", "tiff", "heic", "heif", "avif"},
	"video/": {"mp4", "mov", "avi", "mkv", "webm", "flv", "wmv", "m4v"},
	"audio/": {"mp3", "wav", "ogg", "flac", "aac", "m4a", "wma", "opus"},
	"text/":  {"txt", "md", "csv", "html", "htm", "xml", "json"},
}

// matchRelayFileType 判断文件是否落在白名单内。
//
// 条目形态（大小写不敏感）：
//   - `*` / `*/*`：全部放行
//   - `image/*`：MIME 前缀命中，或扩展名落在该大类的兜底表内（见 mimePrefixExts）
//   - `application/pdf`：完整 MIME 精确匹配（声明 MIME 维度）
//   - `jpg` / `.jpg`：扩展名精确匹配
//
// 扩展名与声明 MIME 任一命中即放行。声明 MIME 与扩展名均由客户端提供、
// 不可信：白名单能拦截扩展名/MIME 都不在名单内的上传（如 .exe），但无法
// 识别「内容与扩展名不符」的伪装（exe 改名 a.png 会以 png 放行）——内容级
// 校验（magic bytes / 病毒扫描）为后续增强项，见协议文档已知限制。
func matchRelayFileType(filename, contentType string, allowed []string) bool {
	if len(allowed) == 0 {
		return true // 留空 = 不限制
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	mime := strings.ToLower(strings.TrimSpace(contentType))
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = mime[:i] // 去掉 "; charset=..." 后缀
	}
	for _, raw := range allowed {
		entry := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case entry == "*" || entry == "*/*":
			return true
		case strings.HasSuffix(entry, "/*"):
			prefix := strings.TrimSuffix(entry, "*")
			if strings.HasPrefix(mime, prefix) {
				return true
			}
			if exts, ok := mimePrefixExts[prefix]; ok && ext != "" {
				for _, e := range exts {
					if ext == e {
						return true
					}
				}
			}
		case strings.Contains(entry, "/"):
			if mime != "" && mime == entry {
				return true
			}
		default:
			if ext != "" && ext == strings.TrimPrefix(entry, ".") {
				return true
			}
		}
	}
	return false
}

// checkRelayFileSize 校验单文件大小：空文件与超限文件均拒绝。
func checkRelayFileSize(size, maxBytes int64) error {
	if size <= 0 {
		return gerror.Wrap(ErrRelayFileSizeExceeded, "uploaded file is empty")
	}
	if maxBytes > 0 && size > maxBytes {
		return gerror.Wrapf(ErrRelayFileSizeExceeded,
			"file size %.1f MB exceeds the %d MB limit", float64(size)/1024/1024, maxBytes/1024/1024)
	}
	return nil
}

// newRelayFileExtID 生成对外文件 ID：file- + 128bit 随机 hex（32 字符）。
// 不可枚举，杜绝跨租户按序号探测；唯一索引兜底碰撞（概率可忽略）。
func newRelayFileExtID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", gerror.Wrap(err, "generate ext id")
	}
	return "file-" + hex.EncodeToString(b), nil
}

// uploadsQuotaUsedBytes 统计租户 /v1/files 已用存储（uploads/ 前缀聚合）。
func uploadsQuotaUsedBytes(ctx context.Context, tenantID int64) (int64, error) {
	one, err := dao.FilFiles.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("storage_path LIKE ?", common.UploadsPathPrefix+"%").
		Fields("COALESCE(SUM(size), 0) AS used").
		One()
	if err != nil {
		return 0, gerror.Wrap(err, "sum uploads size")
	}
	return one["used"].Int64(), nil
}

// UploadRelayFile 处理 POST /v1/files：校验 purpose、门禁对象存储、检查租户配额，
// 经 uploads/{date}/{uuid}{ext} key 落存储并返回带签名 URL 的 File 对象。
func UploadRelayFile(ctx context.Context, tenantID, userID int64, reader io.Reader,
	filename, contentType string, size int64, purpose string) (*RelayFileObject, error) {

	if _, ok := allowedRelayFilePurposes[purpose]; !ok {
		return nil, gerror.Wrapf(ErrRelayFileBadPurpose,
			"invalid purpose '%s'; allowed: assistants, batch, fine-tune, vision, user_data, evals", purpose)
	}
	if tenantID <= 0 {
		return nil, gerror.New("tenant context missing")
	}

	// 内容策略（类型白名单 + 单文件大小）先于存储门禁：本地即可判定，
	// 拒绝时不消耗配额查询与存储初始化
	allowedTypes := relayFilesAllowedTypes(ctx)
	if !matchRelayFileType(filename, contentType, allowedTypes) {
		return nil, gerror.Wrapf(ErrRelayFileTypeNotAllowed,
			"file type not allowed: %q (allowed: %s)", filename, strings.Join(allowedTypes, ", "))
	}
	if err := checkRelayFileSize(size, relayFilesMaxSizeBytes(ctx)); err != nil {
		return nil, err
	}

	svc, err := cloudFileService(ctx)
	if err != nil {
		return nil, err
	}

	// 租户配额（0 = 不限制）。检查与插入非原子，超限边缘可少量超出，防滥用语义足够。
	if maxMB := common.Config().GetInt(ctx, "relay_files_max_tenant_mb"); maxMB > 0 {
		maxBytes := int64(maxMB) * 1024 * 1024
		used, err := uploadsQuotaUsedBytes(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		if used+size > maxBytes {
			return nil, gerror.Wrapf(ErrRelayFileQuotaExceeded,
				"uploads quota exceeded: used %.1f MB + %.1f MB > limit %d MB",
				float64(used)/1024/1024, float64(size)/1024/1024, maxMB)
		}
	}

	extID, err := newRelayFileExtID()
	if err != nil {
		return nil, err
	}

	ext := ""
	if idx := strings.LastIndex(filename, "."); idx >= 0 {
		ext = filename[idx:]
	}
	key := fmt.Sprintf("%s%s/%s%s", common.UploadsPathPrefix,
		time.Now().Format("2006-01-02"), uuid.New().String(), ext)

	record, err := svc.UploadWithKey(ctx, &common.FileUpload{
		Reader:      reader,
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
		TenantID:    tenantID,
		UserID:      userID,
		ExtID:       extID,
		Purpose:     purpose,
	}, key)
	if err != nil {
		return nil, err
	}

	return buildRelayFileObject(ctx, svc, record.StoragePath, record.ExtID, record.OriginalName,
		record.Purpose, record.Size, gtime.Now().Unix()), nil
}

// buildRelayFileObject 组装对外 File 对象并现场签发 URL。
// 签名失败降级：对象返回但缺 url 字段（file_id 引用与 content 端点不受影响）。
func buildRelayFileObject(ctx context.Context, svc *common.FileService,
	storagePath, extID, filename, purpose string, size int64, createdAt int64) *RelayFileObject {

	obj := &RelayFileObject{
		ID:        extID,
		Object:    "file",
		Bytes:     size,
		CreatedAt: createdAt,
		Filename:  filename,
		Purpose:   purpose,
		Status:    "processed",
	}
	ttl := relayFileURLTTL(ctx)
	url, err := svc.Provider().PresignedURL(ctx, storagePath, ttl)
	if err != nil {
		g.Log().Warningf(ctx, "[RelayFiles] presign file %s failed (url omitted): %v", extID, err)
		return obj
	}
	obj.URL = url
	obj.URLExpiresAt = time.Now().Add(ttl).Unix()
	return obj
}

// relayFileMeta 查询 /v1/files 文件记录（tenant 隔离；ext_id 非空 = 用户上传素材）。
func relayFileMeta(ctx context.Context, tenantID int64, extID string) (*entity.FilFiles, error) {
	var record *entity.FilFiles
	err := dao.FilFiles.Ctx(ctx).
		Where("ext_id", extID).
		Where("tenant_id", tenantID).
		Scan(&record)
	if err != nil {
		return nil, gerror.Wrapf(err, "query file %s", extID)
	}
	if record == nil {
		return nil, gerror.Wrapf(ErrRelayFileNotFound, "file %s", extID)
	}
	return record, nil
}

// GetRelayFile 返回单个文件对象（GET /v1/files/{id}）。
func GetRelayFile(ctx context.Context, tenantID int64, extID string) (*RelayFileObject, error) {
	record, err := relayFileMeta(ctx, tenantID, extID)
	if err != nil {
		return nil, err
	}
	svc, err := cloudFileService(ctx)
	if err != nil {
		return nil, err
	}
	var createdAt int64
	if record.CreatedAt != nil {
		createdAt = record.CreatedAt.Unix()
	}
	return buildRelayFileObject(ctx, svc, record.StoragePath, record.ExtId, record.OriginalName,
		record.Purpose, record.Size, createdAt), nil
}

// ListRelayFiles 列出当前租户的上传素材（GET /v1/files），created_at 倒序。
func ListRelayFiles(ctx context.Context, tenantID int64, purpose string, limit int) ([]*RelayFileObject, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	m := dao.FilFiles.Ctx(ctx).
		Where("tenant_id", tenantID).
		WhereNotNull("ext_id")
	if purpose != "" {
		m = m.Where("purpose", purpose)
	}

	var records []*entity.FilFiles
	if err := m.Order("created_at DESC").Order("id DESC").Limit(limit).Scan(&records); err != nil {
		return nil, gerror.Wrap(err, "list files")
	}

	svc, err := cloudFileService(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]*RelayFileObject, 0, len(records))
	for _, record := range records {
		var createdAt int64
		if record.CreatedAt != nil {
			createdAt = record.CreatedAt.Unix()
		}
		list = append(list, buildRelayFileObject(ctx, svc, record.StoragePath, record.ExtId,
			record.OriginalName, record.Purpose, record.Size, createdAt))
	}
	return list, nil
}

// OpenRelayFile 打开文件内容流（GET /v1/files/{id}/content）。
// 调用方负责 Close 返回的 reader。
func OpenRelayFile(ctx context.Context, tenantID int64, extID string) (*entity.FilFiles, io.ReadCloser, error) {
	record, err := relayFileMeta(ctx, tenantID, extID)
	if err != nil {
		return nil, nil, err
	}
	svc, err := cloudFileService(ctx)
	if err != nil {
		return nil, nil, err
	}
	reader, err := svc.Provider().Download(ctx, record.StoragePath)
	if err != nil {
		return nil, nil, gerror.Wrapf(err, "download file %s", extID)
	}
	return record, reader, nil
}

// DeleteRelayFile 删除文件对象与记录（DELETE /v1/files/{id}）。
func DeleteRelayFile(ctx context.Context, tenantID int64, extID string) error {
	record, err := relayFileMeta(ctx, tenantID, extID)
	if err != nil {
		return err
	}
	svc, err := cloudFileService(ctx)
	if err != nil {
		return err
	}
	// FileService.Delete 按（数字 id, tenantID）隔离删除，对象删除失败保留行待重试
	if err := svc.Delete(ctx, record.Id, tenantID); err != nil {
		return gerror.Wrapf(err, "delete file %s", extID)
	}
	return nil
}

// ResolveRelayFileURLs 批量把 file-xxx 引用解析为签名 URL（中转替换链路）。
// 任何一个引用无法解析都视为客户端错误——放行会把 file-xxx 原样送上游，
// 换来一个更难排查的上游报错。基础设施错误（DB/存储）只回笼统话术，
// 细节进服务端日志，避免经由中转错误响应外泄部署信息。
func ResolveRelayFileURLs(ctx context.Context, tenantID int64, extIDs []string) (map[string]string, error) {
	if len(extIDs) == 0 {
		return map[string]string{}, nil
	}

	var records []*entity.FilFiles
	if err := dao.FilFiles.Ctx(ctx).
		Where("tenant_id", tenantID).
		WhereIn("ext_id", extIDs).
		WhereNotNull("ext_id").
		Scan(&records); err != nil {
		g.Log().Warningf(ctx, "[RelayFiles] resolve refs query failed: %v", err)
		return nil, gerror.New("failed to resolve file references")
	}

	svc, err := cloudFileService(ctx)
	if err != nil {
		return nil, err
	}
	ttl := relayFileURLTTL(ctx)

	urls := make(map[string]string, len(records))
	for _, record := range records {
		url, err := svc.Provider().PresignedURL(ctx, record.StoragePath, ttl)
		if err != nil {
			g.Log().Warningf(ctx, "[RelayFiles] presign file %s failed: %v", record.ExtId, err)
			return nil, gerror.New("failed to resolve file references")
		}
		urls[record.ExtId] = url
	}

	for _, id := range extIDs {
		if _, ok := urls[id]; !ok {
			return nil, gerror.Wrapf(ErrRelayFileRefUnknown,
				"file %s not found for this tenant (deleted or never uploaded)", id)
		}
	}
	return urls, nil
}
