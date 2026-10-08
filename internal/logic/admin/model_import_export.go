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
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/relay"
	do "github.com/qianfree/team-api/internal/model/do"
)

// exportPricing 导出文件中的定价行（与 v1.PricingItem 字段同构，仅控制 JSON 形态：
// per_second_prices 只在按秒/特殊方案矩阵模式出现）
type exportPricing struct {
	BillingMode        string             `json:"billing_mode"`
	MinTokens          int64              `json:"min_tokens"`
	MaxTokens          *int64             `json:"max_tokens"`
	InputPrice         float64            `json:"input_price"`
	OutputPrice        float64            `json:"output_price"`
	PerRequestPrice    *float64           `json:"per_request_price"`
	CacheReadPrice     float64            `json:"cache_read_price"`
	CacheCreationPrice float64            `json:"cache_creation_price"`
	PerSecondPrices    map[string]float64 `json:"per_second_prices,omitempty"` // 仅 per_second/special 模式（锚点行 pricing JSONB）
}

// toExportPricing API 定价项 → 导出行（tiered 逐档缓存价随行携带，导入侧经
// BuildPricingBlob 原样还原，不丢档内配置）
func toExportPricing(items []v1.PricingItem) []exportPricing {
	out := make([]exportPricing, 0, len(items))
	for _, it := range items {
		out = append(out, exportPricing{
			BillingMode:        it.BillingMode,
			MinTokens:          it.MinTokens,
			MaxTokens:          it.MaxTokens,
			InputPrice:         it.InputPrice,
			OutputPrice:        it.OutputPrice,
			PerRequestPrice:    it.PerRequestPrice,
			CacheReadPrice:     it.CacheReadPrice,
			CacheCreationPrice: it.CacheCreationPrice,
			PerSecondPrices:    it.PerSecondPrices,
		})
	}
	return out
}

// ExportModelsJson 导出模型配置为 JSON 文件（含定价数据，用于跨环境迁移）
func (s *sAdmin) ExportModelsJson(ctx context.Context, req *v1.ModelExportJsonReq) (*v1.ModelExportJsonRes, error) {
	var models []struct {
		ID               int64       `orm:"id" json:"id"`
		ModelId          string      `orm:"model_id" json:"model_id"`
		ModelName        string      `orm:"model_name" json:"model_name"`
		Category         string      `orm:"category" json:"category"`
		Vendor           string      `orm:"vendor" json:"vendor"`
		Status           string      `orm:"status" json:"status"`
		MaxContextTokens int         `orm:"max_context_tokens" json:"max_context_tokens"`
		MaxOutputTokens  int         `orm:"max_output_tokens" json:"max_output_tokens"`
		Description      string      `orm:"description" json:"description"`
		Tags             []string    `orm:"tags" json:"tags"`
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

	type exportModel struct {
		ModelId          string                   `json:"model_id"`
		ModelName        string                   `json:"model_name"`
		Category         string                   `json:"category"`
		Vendor           string                   `json:"vendor,omitempty"` // 研发厂商（空=未分类，旧文件导入兼容）
		Status           string                   `json:"status"`
		MaxContextTokens int                      `json:"max_context_tokens"`
		MaxOutputTokens  int                      `json:"max_output_tokens"`
		Description      string                   `json:"description"`
		Tags             []string                 `json:"tags"`
		Capabilities     map[string]bool          `json:"capabilities"`
		SunsetDate       string                   `json:"sunset_date,omitempty"`
		ReplacementModel string                   `json:"replacement_model,omitempty"`
		Pricing          []exportPricing          `json:"pricing"`
		TimeSegments     []v1.TimeSegmentItem     `json:"time_segments,omitempty"`
		ParamMultipliers []v1.ParamMultiplierItem `json:"param_multipliers,omitempty"`
		// 官方参考定价（official_pricing JSONB 还原）：未配置时为 nil（导出 null），
		// 导入侧 nil=不动库内人工核对值（旧导出文件缺字段同语义），非 nil=全量替换
		OfficialItems            []exportPricing          `json:"official_items"`
		OfficialTimeSegments     []v1.TimeSegmentItem     `json:"official_time_segments,omitempty"`
		OfficialParamMultipliers []v1.ParamMultiplierItem `json:"official_param_multipliers,omitempty"`
		// 定价行展示字段（全量替换语义：空串=清除；旧文件缺字段按空串导入=清除）
		PriceNote       string `json:"price_note"`
		DiscountLabel   string `json:"discount_label"`
		PriceChangeNote string `json:"price_change_note"`
		// 特殊计费方案声明（pricing JSONB 顶层 scheme/scheme_config，模型级语义）：
		// 随导出携带，导入侧透传还原，避免跨环境迁移后方案定价退化为通用模式
		Scheme       string          `json:"scheme,omitempty"`
		SchemeConfig json.RawMessage `json:"scheme_config,omitempty"`
	}

	// 批量查询所有模型的定价行（避免循环内逐模型查询导致 N+1），与 ListModels 的批量模式对齐。
	modelIDs := make([]int64, 0, len(models))
	for _, m := range models {
		modelIDs = append(modelIDs, m.ID)
	}
	type pricingRow struct {
		ModelId         int64   `orm:"model_id" json:"model_id"`
		BillingMode     string  `orm:"billing_mode" json:"billing_mode"`
		Pricing         string  `orm:"pricing" json:"pricing"`                     // pricing JSONB（计费定价唯一真相）
		OfficialPricing string  `orm:"official_pricing" json:"official_pricing"`   // 官方参考定价 JSONB（未配置=NULL）
		PriceNote       *string `orm:"price_note" json:"price_note"`               // 价格说明（仅锚点行，NULL=无）
		DiscountLabel   *string `orm:"discount_label" json:"discount_label"`       // 折扣标签（NULL=不展示）
		PriceChangeNote *string `orm:"price_change_note" json:"price_change_note"` // 价格调整说明（NULL=不展示）
	}
	var allPricingRows []pricingRow
	if len(modelIDs) > 0 {
		if err := dao.MdlPricing.Ctx(ctx).WhereIn("model_id", modelIDs).Scan(&allPricingRows); err != nil {
			return nil, err
		}
	}
	// 按 model_id 组装导出数据：pricing JSONB 为唯一真相，
	// 展开复用 expandPricingBlob（与 GetModelPricing 同一逻辑，含 tiered 逐档缓存价）
	pricingByModel := make(map[int64][]exportPricing, len(allPricingRows))
	segmentsByModel := make(map[int64][]v1.TimeSegmentItem, len(allPricingRows))
	paramRulesByModel := make(map[int64][]v1.ParamMultiplierItem, len(allPricingRows))
	schemeByModel := make(map[int64]string, len(allPricingRows))
	schemeConfigByModel := make(map[int64]json.RawMessage, len(allPricingRows))
	officialItemsByModel := make(map[int64][]exportPricing, len(allPricingRows))
	officialSegmentsByModel := make(map[int64][]v1.TimeSegmentItem, len(allPricingRows))
	officialRulesByModel := make(map[int64][]v1.ParamMultiplierItem, len(allPricingRows))
	priceNoteByModel := make(map[int64]string, len(allPricingRows))
	discountLabelByModel := make(map[int64]string, len(allPricingRows))
	priceChangeNoteByModel := make(map[int64]string, len(allPricingRows))
	for _, p := range allPricingRows {
		// 主定价：pricing JSONB → 行 + 时段 + 倍率（与 GetModelPricing 同一展开路径）
		blob := billing.ParsePricingBlob(p.Pricing)
		items, segs, rules := expandPricingBlob(p.BillingMode, blob)
		pricingByModel[p.ModelId] = toExportPricing(items)
		if len(segs) > 0 {
			segmentsByModel[p.ModelId] = segs
		}
		if len(rules) > 0 {
			paramRulesByModel[p.ModelId] = rules
		}
		if blob != nil && blob.Scheme != "" {
			schemeByModel[p.ModelId] = blob.Scheme
			schemeConfigByModel[p.ModelId] = blob.SchemeConfig
		}

		// 官方参考定价：official_pricing JSONB 同构展开；未配置（NULL/解析失败）时保持
		// nil——导出 null，导入侧 nil=不动库内人工核对值（旧导出文件缺字段同语义）
		if offMode, offBlob := officialBlobFromJSON(p.OfficialPricing); offBlob != nil {
			offItems, offSegs, offRules := expandPricingBlob(offMode, offBlob)
			officialItemsByModel[p.ModelId] = toExportPricing(offItems)
			if len(offSegs) > 0 {
				officialSegmentsByModel[p.ModelId] = offSegs
			}
			if len(offRules) > 0 {
				officialRulesByModel[p.ModelId] = offRules
			}
		}

		priceNoteByModel[p.ModelId] = derefStr(p.PriceNote)
		discountLabelByModel[p.ModelId] = derefStr(p.DiscountLabel)
		priceChangeNoteByModel[p.ModelId] = derefStr(p.PriceChangeNote)
	}

	result := make([]exportModel, 0, len(models))
	for _, m := range models {
		em := exportModel{
			ModelId:                  m.ModelId,
			ModelName:                m.ModelName,
			Category:                 m.Category,
			Vendor:                   m.Vendor,
			Status:                   m.Status,
			MaxContextTokens:         m.MaxContextTokens,
			MaxOutputTokens:          m.MaxOutputTokens,
			Description:              m.Description,
			Capabilities:             parseCapabilities(m.Capabilities),
			ReplacementModel:         m.ReplacementModel,
			Pricing:                  pricingByModel[m.ID],
			TimeSegments:             segmentsByModel[m.ID],
			ParamMultipliers:         paramRulesByModel[m.ID],
			OfficialItems:            officialItemsByModel[m.ID],
			OfficialTimeSegments:     officialSegmentsByModel[m.ID],
			OfficialParamMultipliers: officialRulesByModel[m.ID],
			PriceNote:                priceNoteByModel[m.ID],
			DiscountLabel:            discountLabelByModel[m.ID],
			PriceChangeNote:          priceChangeNoteByModel[m.ID],
			Scheme:                   schemeByModel[m.ID],
			SchemeConfig:             schemeConfigByModel[m.ID],
		}
		if em.Pricing == nil {
			em.Pricing = []exportPricing{}
		}
		if m.SunsetDate != nil {
			em.SunsetDate = m.SunsetDate.Format("Y-m-d")
		}
		em.Tags = m.Tags
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

// writeImportedPricing 导入路径统一写定价：主定价 + 时段 + 官方参考定价 + 展示字段，
// 新建与 overwrite 两分支共用，语义与 SetModelPricing 完全对齐：
//   - 官方定价 OfficialItems nil=导入文件未携带（旧导出文件），不动库内人工核对值；
//     非 nil=全量替换（三个官方数组全空=清除）。官方价非计费依据，不触发价格缓存失效
//   - 展示字段空串=清除（全量替换，与时段定价「导入文件无该字段=清除」同口径）
func writeImportedPricing(ctx context.Context, modelDBID int64, item *v1.ModelImportItem, paramRules []billing.ParamRule, scheme string) error {
	if err := writePricingForModel(ctx, modelDBID, item.Pricing, paramRules, scheme, item.SchemeConfig); err != nil {
		return gerror.Wrapf(err, "模型 %s 定价", item.ModelId)
	}
	if err := writeTimeSegmentsForModel(ctx, modelDBID, item.TimeSegments); err != nil {
		return gerror.Wrapf(err, "模型 %s 时段定价", item.ModelId)
	}
	if item.OfficialItems != nil {
		if err := writeOfficialPricingForModel(ctx, modelDBID, item.OfficialItems, item.OfficialTimeSegments, item.OfficialParamMultipliers); err != nil {
			return gerror.Wrapf(err, "模型 %s 官方参考定价", item.ModelId)
		}
	}
	if err := writePricingDisplayFieldsForModel(ctx, modelDBID, item.PriceNote, item.DiscountLabel, item.PriceChangeNote); err != nil {
		return gerror.Wrapf(err, "模型 %s 定价展示字段", item.ModelId)
	}
	return nil
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
	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for _, item := range req.Models {
			// 计费方案配对校验（与 SetModelPricing 同口径）：special ⇔ scheme 声明，
			// 且方案必须已注册——导入文件引用本环境不存在的方案时运行期会 fail-closed
			// 拒绝该模型请求，应在导入时报错而非留隐患
			scheme := strings.TrimSpace(item.Scheme)
			if len(item.Pricing) > 0 {
				isSpecial := item.Pricing[0].BillingMode == billing.BillingModeSpecial
				if isSpecial && scheme == "" {
					return gerror.Newf("模型 %s：特殊计费模式（special）缺少计费方案声明", item.ModelId)
				}
				if !isSpecial && scheme != "" {
					return gerror.Newf("模型 %s：计费方案模型必须使用特殊计费模式（billing_mode=special）", item.ModelId)
				}
			}
			if scheme != "" && !billing.SchemeRegistered(scheme) {
				return gerror.Newf("模型 %s：计费方案 %q 未注册", item.ModelId, scheme)
			}

			var existing *struct {
				ID int64 `orm:"id"`
			}
			err := dao.MdlModels.Ctx(ctx).Where("model_id", item.ModelId).Scan(&existing)
			if err != nil {
				return err
			}

			if existing != nil {
				if item.ConflictAction == "skip" {
					res.Skipped++
					continue
				}
				// overwrite: 更新模型 + 替换定价
				updateData := do.MdlModels{
					ModelName:        item.ModelName,
					Category:         item.Category,
					MaxContextTokens: item.MaxContextTokens,
					MaxOutputTokens:  item.MaxOutputTokens,
					Description:      item.Description,
				}
				// 旧导出文件无 vendor 字段（空串），不覆盖已有厂商配置
				if item.Vendor != "" {
					updateData.Vendor = item.Vendor
				}
				if item.Tags != nil {
					updateData.Tags = item.Tags
				}
				if item.Capabilities != nil {
					capJson, _ := json.Marshal(item.Capabilities)
					updateData.Capabilities = string(capJson)
				}
				if item.SunsetDate != "" {
					updateData.SunsetDate = gtime.NewFromStr(item.SunsetDate)
				}
				if item.ReplacementModel != "" {
					updateData.ReplacementModel = item.ReplacementModel
				}
				if item.Status != "" {
					updateData.Status = item.Status
				}

				_, err = dao.MdlModels.Ctx(ctx).Where("id", existing.ID).Data(updateData).Update()
				if err != nil {
					return err
				}

				paramRules, pErr := paramMultipliersFromAPI(item.ParamMultipliers)
				if pErr != nil {
					return gerror.Wrapf(pErr, "模型 %s 参数倍率", item.ModelId)
				}
				if err := writeImportedPricing(ctx, existing.ID, &item, paramRules, scheme); err != nil {
					return err
				}

				relay.NewDataProvider().InvalidateModelCache(item.ModelId)
				billing.ClearModelPriceCache(ctx, item.ModelId)
				res.Imported++
			} else {
				// 新建模型
				insertData := do.MdlModels{
					ModelId:          item.ModelId,
					ModelName:        item.ModelName,
					Category:         item.Category,
					Vendor:           item.Vendor,
					Status:           "active",
					MaxContextTokens: item.MaxContextTokens,
					MaxOutputTokens:  item.MaxOutputTokens,
					Description:      item.Description,
				}
				if item.Tags != nil {
					insertData.Tags = item.Tags
				} else {
					insertData.Tags = []string{}
				}
				if item.Capabilities != nil {
					capJson, _ := json.Marshal(item.Capabilities)
					insertData.Capabilities = string(capJson)
				}

				id, err := dao.MdlModels.Ctx(ctx).InsertAndGetId(insertData)
				if err != nil {
					return err
				}

				// 定价写入（pricing JSONB + 官方参考定价 + 展示字段，与 SetModelPricing 口径一致）
				paramRules, pErr := paramMultipliersFromAPI(item.ParamMultipliers)
				if pErr != nil {
					return gerror.Wrapf(pErr, "模型 %s 参数倍率", item.ModelId)
				}
				if err := writeImportedPricing(ctx, id, &item, paramRules, scheme); err != nil {
					return err
				}

				res.Imported++
			}
		}
		return nil
	})
	if err != nil {
		return res, err
	}

	// 导入含新建 / 更新模型，逐条失效太碎，事务提交后整体失效一次下拉缓存
	InvalidateModelOptionsCache(ctx)

	return res, nil
}
