package admin

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/logic/common"
)

// GetSettingsCategories returns all available setting categories.
func (s *sAdmin) GetSettingsCategories(ctx context.Context, _ *v1.AdminSettingsCategoriesReq) (*v1.AdminSettingsCategoriesRes, error) {
	categories := common.Categories
	list := make([]v1.SettingCategoryItem, len(categories))
	for i, c := range categories {
		list[i] = v1.SettingCategoryItem{
			Key:   c.Key,
			Label: c.Label,
			Icon:  c.Icon,
			Order: c.Order,
		}
	}
	return &v1.AdminSettingsCategoriesRes{List: list}, nil
}

// GetSettings retrieves settings with schema for a given category.
func (s *sAdmin) GetSettings(ctx context.Context, req *v1.AdminSettingsGetReq) (*v1.AdminSettingsGetRes, error) {
	if !isValidCategory(req.Category) {
		return nil, common.NewBadRequestError("无效的设置分类")
	}

	items := common.Config().GetCategoryWithValues(ctx, req.Category)
	list := make([]v1.AdminSettingItem, len(items))
	for i, item := range items {
		list[i] = v1.AdminSettingItem{
			Key:         item.Key,
			Value:       common.TypedValue(item.Type, item.Value, item.Default),
			Type:        string(item.Type),
			Label:       item.Label,
			Description: item.Description,
			Sensitive:   item.Sensitive,
			Validation:  item.Validation,
			Default:     common.TypedValue(item.Type, "", item.Default),
		}
	}
	return &v1.AdminSettingsGetRes{List: list}, nil
}

// UpdateSettings batch-updates settings for a given category.
func (s *sAdmin) UpdateSettings(ctx context.Context, req *v1.AdminSettingsUpdateReq) (*v1.AdminSettingsUpdateRes, error) {
	if !isValidCategory(req.Category) {
		return nil, common.NewBadRequestError("无效的设置分类")
	}

	// Normalize interface{} values to strings for storage
	strValues := make(map[string]string, len(req.Settings))
	for key, val := range req.Settings {
		strValues[key] = common.NormalizeSettingValue(val)
	}

	if err := s.validateCrossFieldSettings(ctx, req.Category, strValues); err != nil {
		return nil, err
	}

	if err := common.Config().UpdateCategory(ctx, req.Category, strValues); err != nil {
		return nil, err
	}
	return nil, nil
}

// validateCrossFieldSettings validates interdependent settings within a category.
func (s *sAdmin) validateCrossFieldSettings(ctx context.Context, category string, values map[string]string) error {
	switch category {
	case "security":
		return s.validateSecuritySettings(ctx, values)
	case "channel":
		return s.validateChannelSettings(ctx, values)
	default:
		return nil
	}
}

// validateSecuritySettings 校验安全配置项之间的依赖：启用 Turnstile 前必须已配置密钥。
func (s *sAdmin) validateSecuritySettings(ctx context.Context, values map[string]string) error {
	enabled, ok := values["turnstile_enabled"]
	if !ok || (enabled != "true" && enabled != "1") {
		return nil
	}

	siteKey := values["turnstile_site_key"]
	secretKey := values["turnstile_secret_key"]

	// If keys not in the current request, read existing values from DB
	if siteKey == "" {
		siteKey = common.Config().GetString(ctx, "turnstile_site_key")
	}
	if secretKey == "" || secretKey == "******" {
		secretKey = common.Config().GetString(ctx, "turnstile_secret_key")
	}

	if strings.TrimSpace(siteKey) == "" || strings.TrimSpace(secretKey) == "" {
		return common.NewBadRequestError("启用 Turnstile 前必须先配置 Site Key 和 Secret Key")
	}

	return nil
}

// validateChannelSettings 校验渠道配置项之间的依赖：同步图片厂商异步化依赖对象存储保存生成
// 结果，开启前必须已在「存储配置」中配置对象存储（OSS/S3/COS），否则生成的图片无处保存。
func (s *sAdmin) validateChannelSettings(ctx context.Context, values map[string]string) error {
	enabled, ok := values["sync_image_async_enabled"]
	if !ok || (enabled != "true" && enabled != "1") {
		return nil
	}

	// 存储配置属于 storage 分类、独立持久化，此处直接读已保存的存储配置判断是否可用。
	if !common.IsStorageConfigured(ctx) {
		return common.NewBadRequestError("开启同步图片厂商异步化前，必须先在「存储配置」中配置对象存储（OSS/S3/COS）")
	}

	return nil
}

func isValidCategory(category string) bool {
	for _, c := range common.Categories {
		if c.Key == category {
			return true
		}
	}
	return false
}

// TestStorageConfig 测试对象存储配置连通性：用生成的测试图片走一次「上传 → 下载校验 → 删除」
// 完整往返，验证凭证 / 桶 / 端点 / 读写删权限是否正常。
//
// access_key_id / secret 为空或掩码（"******"）时回落到已保存的配置值——与 GET 接口对敏感字段
// 的掩码逻辑（config.GetCategoryWithValues）以及 Turnstile 校验的 "******" 处理保持一致，
// 从而支持管理员在「未点保存」的状态下直接测试表单中新填/改动的配置。
func (s *sAdmin) TestStorageConfig(ctx context.Context, req *v1.AdminStorageTestReq) (*v1.AdminStorageTestRes, error) {
	cfg := &common.StorageConfig{
		Provider:    strings.TrimSpace(req.Provider),
		Endpoint:    strings.TrimSpace(req.Endpoint),
		Region:      strings.TrimSpace(req.Region),
		Bucket:      strings.TrimSpace(req.Bucket),
		AccessKeyID: strings.TrimSpace(req.AccessKeyID),
		SecretKey:   req.SecretKey,
		UseSSL:      req.UseSSL,
		PathPrefix:  strings.TrimSpace(req.PathPrefix),
	}
	// 敏感字段掩码 / 留空 → 回落已保存值
	if cfg.AccessKeyID == "" || cfg.AccessKeyID == "******" {
		cfg.AccessKeyID = common.Config().GetString(ctx, "storage_access_key_id")
	}
	if cfg.SecretKey == "" || cfg.SecretKey == "******" {
		cfg.SecretKey = common.Config().GetString(ctx, "storage_access_key_secret")
	}
	if cfg.PathPrefix == "" {
		cfg.PathPrefix = "team-api" // 与 GetStorageConfig 的默认前缀保持一致
	}

	if cfg.Provider == "" || cfg.Bucket == "" {
		return nil, common.NewBadRequestError("请先填写存储供应商和存储桶名称")
	}

	provider, err := common.NewStorageProvider(cfg)
	if err != nil {
		return nil, common.NewBadRequestError("创建存储客户端失败：" + err.Error())
	}

	// 生成一张 1x1 PNG 作为测试图片（真实图片字节，验证二进制内容往返无损）
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 0x33, G: 0x99, B: 0xff, A: 0xff})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, common.NewBadRequestError("生成测试图片失败：" + err.Error())
	}
	payload := buf.Bytes()

	// 独立 _healthcheck 目录 + uuid，避免与业务对象冲突；provider 内部会自动加路径前缀
	key := fmt.Sprintf("_healthcheck/%s.png", uuid.New().String())

	// 整个往返设 20s 超时，端点不可达时不至于长时间挂起
	testCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	res := &v1.AdminStorageTestRes{}
	start := time.Now()

	// 1. 上传
	if _, err := provider.Upload(testCtx, bytes.NewReader(payload), key, "image/png"); err != nil {
		return nil, common.NewBadRequestError("上传测试图片失败：" + err.Error())
	}
	res.Uploaded = true
	// 兜底清理：覆盖下载失败等提前返回路径，避免遗留测试对象（Delete 幂等，成功路径重复删除无副作用）
	defer func() {
		if delErr := provider.Delete(context.WithoutCancel(testCtx), key); delErr != nil {
			g.Log().Warningf(ctx, "storage test: cleanup test object %s failed: %v", key, delErr)
		}
	}()

	// 2. 下载并校验内容一致
	rc, err := provider.Download(testCtx, key)
	if err != nil {
		return nil, common.NewBadRequestError("下载测试图片失败：" + err.Error())
	}
	got, readErr := io.ReadAll(rc)
	_ = rc.Close()
	if readErr != nil {
		return nil, common.NewBadRequestError("读取测试图片失败：" + readErr.Error())
	}
	if !bytes.Equal(got, payload) {
		return nil, common.NewBadRequestError("下载内容与上传不一致，存储未正确保存对象")
	}
	res.Downloaded = true

	// 3. 删除（校验删除权限；失败不致命，仅告警，兜底 defer 会再尝试一次）
	if err := provider.Delete(testCtx, key); err != nil {
		g.Log().Warningf(ctx, "storage test: delete test object %s failed: %v", key, err)
	} else {
		res.Deleted = true
	}

	res.ElapsedMs = time.Since(start).Milliseconds()
	res.Message = "对象存储配置正常：测试图片上传、下载校验通过"
	return res, nil
}
