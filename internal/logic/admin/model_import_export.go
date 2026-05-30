package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/relay"
	do "github.com/qianfree/team-api/internal/model/do"
)

// ExportModelsJson 导出模型配置为 JSON 文件（含定价数据，用于跨环境迁移）
func (s *sAdmin) ExportModelsJson(ctx context.Context, req *v1.ModelExportJsonReq) (*v1.ModelExportJsonRes, error) {
	var models []struct {
		ID               int64       `orm:"id" json:"id"`
		ModelId          string      `orm:"model_id" json:"model_id"`
		ModelName        string      `orm:"model_name" json:"model_name"`
		Category         string      `orm:"category" json:"category"`
		Status           string      `orm:"status" json:"status"`
		MaxContextTokens int         `orm:"max_context_tokens" json:"max_context_tokens"`
		MaxOutputTokens  int         `orm:"max_output_tokens" json:"max_output_tokens"`
		Description      string      `orm:"description" json:"description"`
		Tags             string      `orm:"tags" json:"tags"`
		Capabilities     string      `orm:"capabilities" json:"capabilities"`
		SunsetDate       *gtime.Time `orm:"sunset_date" json:"sunset_date"`
		ReplacementModel string      `orm:"replacement_model" json:"replacement_model"`
	}

	err := dao.MdlModels.Ctx(ctx).
		WhereIn("model_id", req.ModelIds).
		Scan(&models)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, gerror.New("未找到匹配的模型")
	}

	type exportPricing struct {
		BillingMode        string   `json:"billing_mode"`
		MinTokens          int64    `json:"min_tokens"`
		MaxTokens          *int64   `json:"max_tokens"`
		InputPrice         float64  `json:"input_price"`
		OutputPrice        float64  `json:"output_price"`
		PerRequestPrice    *float64 `json:"per_request_price"`
		CacheReadPrice     float64  `json:"cache_read_price"`
		CacheCreationPrice float64  `json:"cache_creation_price"`
	}

	type exportModel struct {
		ModelId          string          `json:"model_id"`
		ModelName        string          `json:"model_name"`
		Category         string          `json:"category"`
		Status           string          `json:"status"`
		MaxContextTokens int             `json:"max_context_tokens"`
		MaxOutputTokens  int             `json:"max_output_tokens"`
		Description      string          `json:"description"`
		Tags             []string        `json:"tags"`
		Capabilities     map[string]bool `json:"capabilities"`
		SunsetDate       string          `json:"sunset_date,omitempty"`
		ReplacementModel string          `json:"replacement_model,omitempty"`
		Pricing          []exportPricing `json:"pricing"`
	}

	result := make([]exportModel, 0, len(models))
	for _, m := range models {
		var pricingRows []struct {
			BillingMode        string   `orm:"billing_mode" json:"billing_mode"`
			MinTokens          int64    `orm:"min_tokens" json:"min_tokens"`
			MaxTokens          *int64   `orm:"max_tokens" json:"max_tokens"`
			InputPrice         float64  `orm:"input_price" json:"input_price"`
			OutputPrice        float64  `orm:"output_price" json:"output_price"`
			PerRequestPrice    *float64 `orm:"per_request_price" json:"per_request_price"`
			CacheReadPrice     float64  `orm:"cache_read_price" json:"cache_read_price"`
			CacheCreationPrice float64  `orm:"cache_creation_price" json:"cache_creation_price"`
		}
		err := dao.MdlPricing.Ctx(ctx).Where("model_id", m.ID).Scan(&pricingRows)
		if err != nil {
			return nil, err
		}

		pricing := make([]exportPricing, 0, len(pricingRows))
		for _, p := range pricingRows {
			pricing = append(pricing, exportPricing{
				BillingMode:        p.BillingMode,
				MinTokens:          p.MinTokens,
				MaxTokens:          p.MaxTokens,
				InputPrice:         p.InputPrice,
				OutputPrice:        p.OutputPrice,
				PerRequestPrice:    p.PerRequestPrice,
				CacheReadPrice:     p.CacheReadPrice,
				CacheCreationPrice: p.CacheCreationPrice,
			})
		}

		em := exportModel{
			ModelId:          m.ModelId,
			ModelName:        m.ModelName,
			Category:         m.Category,
			Status:           m.Status,
			MaxContextTokens: m.MaxContextTokens,
			MaxOutputTokens:  m.MaxOutputTokens,
			Description:      m.Description,
			Capabilities:     parseCapabilities(m.Capabilities),
			ReplacementModel: m.ReplacementModel,
			Pricing:          pricing,
		}
		if m.SunsetDate != nil {
			em.SunsetDate = m.SunsetDate.Format("Y-m-d")
		}
		if m.Tags != "" {
			em.Tags = parsePgArray(m.Tags)
		}
		result = append(result, em)
	}

	exportData := g.Map{
		"version":     "1.0",
		"exported_at": gtime.Now().Format("Y-m-d\\TH:i:sP"),
		"models":      result,
	}

	jsonBytes, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return nil, err
	}

	filename := "models_" + gtime.Now().Format("Ymd_His") + ".json"
	r := g.RequestFromCtx(ctx)
	r.Response.Header().Set("Content-Type", "application/json; charset=utf-8")
	r.Response.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.QueryEscape(filename))
	r.Response.Write(jsonBytes)

	return nil, nil
}

// ImportModelsPreview 导入模型预览（解析上传文件，检测冲突）
func (s *sAdmin) ImportModelsPreview(ctx context.Context, req *v1.ModelImportPreviewReq) (*v1.ModelImportPreviewRes, error) {
	var data []byte

	r := g.RequestFromCtx(ctx)

	// 优先通过标准文件上传读取
	file := r.GetUploadFile("file")
	if file != nil {
		f, err := file.Open()
		if err != nil {
			return nil, gerror.NewCode(gcode.New(consts.CodeModelImportInvalidFile, consts.MsgModelImportInvalidFile, nil), consts.MsgModelImportInvalidFile)
		}
		defer f.Close()
		data, err = io.ReadAll(f)
		if err != nil {
			return nil, gerror.NewCode(gcode.New(consts.CodeModelImportInvalidFile, consts.MsgModelImportInvalidFile, nil), consts.MsgModelImportInvalidFile)
		}
	}

	// 兜底：从已解析的 MultipartForm.File 中读取（文件在 File 而非 Value 中）
	if len(data) == 0 {
		mf := r.GetMultipartForm()
		if mf != nil {
			if files := mf.File["file"]; len(files) > 0 {
				f, err := files[0].Open()
				if err == nil {
					defer f.Close()
					data, _ = io.ReadAll(f)
				}
			}
		}
	}

	if len(data) == 0 {
		return nil, gerror.NewCode(gcode.New(consts.CodeModelImportInvalidFile, consts.MsgModelImportInvalidFile, nil), consts.MsgModelImportInvalidFile)
	}

	var exportData struct {
		Version string                      `json:"version"`
		Models  []v1.ModelImportPreviewItem `json:"models"`
	}
	// 使用 json.Decoder 而非 json.Unmarshal，只解析第一个完整 JSON 值，
	// 兼容旧版本导出文件末尾可能被中间件追加的标准响应体。
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&exportData); err != nil {
		return nil, gerror.NewCode(gcode.New(consts.CodeModelImportInvalidFile, consts.MsgModelImportInvalidFile, nil), consts.MsgModelImportInvalidFile)
	}

	if exportData.Version != "1.0" {
		return nil, gerror.NewCode(gcode.New(consts.CodeModelImportBadVersion, consts.MsgModelImportBadVersion, nil), consts.MsgModelImportBadVersion)
	}

	for i := range exportData.Models {
		count, _ := dao.MdlModels.Ctx(ctx).Where("model_id", exportData.Models[i].ModelId).Count()
		if count > 0 {
			exportData.Models[i].Conflict = "exists"
		}
	}

	return &v1.ModelImportPreviewRes{Models: exportData.Models}, nil
}

// ImportModels 确认导入模型（事务内执行）
func (s *sAdmin) ImportModels(ctx context.Context, req *v1.ModelImportReq) (*v1.ModelImportRes, error) {
	if len(req.Models) > 200 {
		return nil, gerror.New("单次导入不能超过 200 个模型")
	}

	res := &v1.ModelImportRes{}
	err := dao.MdlModels.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for _, item := range req.Models {
			var existing *struct {
				ID int64 `orm:"id"`
			}
			err := tx.Model("mdl_models").Ctx(ctx).Where("model_id", item.ModelId).Scan(&existing)
			if err != nil {
				return err
			}

			if existing != nil {
				if item.ConflictAction == "skip" {
					res.Skipped++
					continue
				}
				// overwrite: 更新模型 + 替换定价
				updateData := g.Map{
					"model_name":         item.ModelName,
					"category":           item.Category,
					"max_context_tokens": item.MaxContextTokens,
					"max_output_tokens":  item.MaxOutputTokens,
					"description":        item.Description,
				}
				if item.Tags != nil {
					updateData["tags"] = item.Tags
				}
				if item.Capabilities != nil {
					capJson, _ := json.Marshal(item.Capabilities)
					updateData["capabilities"] = string(capJson)
				}
				if item.SunsetDate != "" {
					updateData["sunset_date"] = item.SunsetDate
				}
				if item.ReplacementModel != "" {
					updateData["replacement_model"] = item.ReplacementModel
				}
				if item.Status != "" {
					updateData["status"] = item.Status
				}

				_, err = tx.Model("mdl_models").Ctx(ctx).Where("id", existing.ID).Data(updateData).Update()
				if err != nil {
					return err
				}

				_, err = tx.Model("mdl_pricing").Ctx(ctx).Where("model_id", existing.ID).Delete()
				if err != nil {
					return err
				}
				for _, p := range item.Pricing {
					_, err = tx.Model("mdl_pricing").Ctx(ctx).Insert(do.MdlPricing{
						ModelId:            existing.ID,
						BillingMode:        p.BillingMode,
						MinTokens:          p.MinTokens,
						MaxTokens:          p.MaxTokens,
						InputPrice:         p.InputPrice,
						OutputPrice:        p.OutputPrice,
						PerRequestPrice:    p.PerRequestPrice,
						CacheReadPrice:     p.CacheReadPrice,
						CacheCreationPrice: p.CacheCreationPrice,
					})
					if err != nil {
						return err
					}
				}

				relay.NewDataProvider().InvalidateModelCache(item.ModelId)
				res.Imported++
			} else {
				// 新建模型
				insertData := g.Map{
					"model_id":           item.ModelId,
					"model_name":         item.ModelName,
					"category":           item.Category,
					"status":             "active",
					"max_context_tokens": item.MaxContextTokens,
					"max_output_tokens":  item.MaxOutputTokens,
					"description":        item.Description,
				}
				if item.Tags != nil {
					insertData["tags"] = item.Tags
				} else {
					insertData["tags"] = []string{}
				}
				if item.Capabilities != nil {
					capJson, _ := json.Marshal(item.Capabilities)
					insertData["capabilities"] = string(capJson)
				}

				id, err := tx.Model("mdl_models").Ctx(ctx).InsertAndGetId(insertData)
				if err != nil {
					return err
				}

				for _, p := range item.Pricing {
					_, err = tx.Model("mdl_pricing").Ctx(ctx).Insert(do.MdlPricing{
						ModelId:            id,
						BillingMode:        p.BillingMode,
						MinTokens:          p.MinTokens,
						MaxTokens:          p.MaxTokens,
						InputPrice:         p.InputPrice,
						OutputPrice:        p.OutputPrice,
						PerRequestPrice:    p.PerRequestPrice,
						CacheReadPrice:     p.CacheReadPrice,
						CacheCreationPrice: p.CacheCreationPrice,
					})
					if err != nil {
						return err
					}
				}

				res.Imported++
			}
		}
		return nil
	})

	return res, err
}

// parsePgArray 解析 PostgreSQL 数组格式的字符串，如 {tag1,tag2}
func parsePgArray(raw string) []string {
	if raw == "" || raw == "{}" || raw == "NULL" {
		return nil
	}
	s := strings.Trim(raw, "{}")
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
