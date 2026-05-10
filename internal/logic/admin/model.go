package admin

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/relay"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/utility/export"
)

// ListModels 获取模型列表
func (s *sAdmin) ListModels(ctx context.Context, req *v1.ModelListReq) (*v1.ModelListRes, error) {
	query := dao.MdlModels.Ctx(ctx)

	if req.Category != "" {
		query = query.Where("category", req.Category)
	}
	if req.Status != "" {
		query = query.Where("status", req.Status)
	}
	if req.Search != "" {
		query = query.Where("model_id LIKE ? OR model_name LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
	}

	var total int
	var models []struct {
		ID               int64       `json:"id"`
		ModelId          string      `json:"model_id"`
		ModelName        string      `json:"model_name"`
		Category         string      `json:"category"`
		Status           string      `json:"status"`
		MaxContextTokens int         `json:"max_context_tokens"`
		MaxOutputTokens  int         `json:"max_output_tokens"`
		Capabilities     string      `json:"capabilities"`
		Description      string      `json:"description"`
		Tags             string      `json:"tags"`
		CreatedAt        *gtime.Time `json:"created_at"`
		UpdatedAt        *gtime.Time `json:"updated_at"`
		DeprecatedAt     *gtime.Time `json:"deprecated_at"`
		SunsetDate       *gtime.Time `json:"sunset_date"`
		ReplacementModel string      `json:"replacement_model"`
	}

	err := query.Fields("id, model_id, model_name, category, status, max_context_tokens, max_output_tokens, capabilities, description, tags, created_at, updated_at, deprecated_at, sunset_date, replacement_model").
		OrderDesc("id").
		Page(req.Page, req.PageSize).
		ScanAndCount(&models, &total, false)
	if err != nil {
		return nil, err
	}

	list := make([]v1.ModelItem, 0, len(models))
	for _, m := range models {
		item := v1.ModelItem{
			ID:           m.ID,
			ModelId:      m.ModelId,
			ModelName:    m.ModelName,
			Category:     m.Category,
			Status:       m.Status,
			MaxContext:   m.MaxContextTokens,
			MaxOutput:    m.MaxOutputTokens,
			Capabilities: parseCapabilities(m.Capabilities),
			Description:  m.Description,
			Tags:         parseTagsArray(m.Tags),
			CreatedAt:    m.CreatedAt.String(),
			UpdatedAt:    m.UpdatedAt.String(),
		}
		if m.DeprecatedAt != nil {
			s := m.DeprecatedAt.String()
			item.DeprecatedAt = &s
		}
		if m.SunsetDate != nil {
			s := m.SunsetDate.Format("Y-m-d")
			item.SunsetDate = &s
		}
		item.ReplacementModel = m.ReplacementModel
		list = append(list, item)
	}

	return &v1.ModelListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// CreateModel 创建模型（自动创建默认 token 定价记录）
func (s *sAdmin) CreateModel(ctx context.Context, req *v1.ModelCreateReq) (*v1.ModelCreateRes, error) {
	count, err := dao.MdlModels.Ctx(ctx).Where(do.MdlModels{ModelId: req.ModelId}).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, gerror.NewCode(gcode.New(consts.CodeModelNameExists, consts.MsgModelNameExists, nil), consts.MsgModelNameExists)
	}

	insertData := g.Map{
		"model_id":           req.ModelId,
		"model_name":         req.ModelName,
		"category":           req.Category,
		"max_context_tokens": req.MaxContext,
		"max_output_tokens":  req.MaxOutput,
		"description":        req.Description,
		"tags":               req.Tags,
		"status":             "active",
	}
	if req.Capabilities != nil {
		capJson, _ := json.Marshal(req.Capabilities)
		insertData["capabilities"] = string(capJson)
	}

	id, err := dao.MdlModels.Ctx(ctx).InsertAndGetId(insertData)
	if err != nil {
		return nil, err
	}

	_, err = dao.MdlPricing.Ctx(ctx).Insert(do.MdlPricing{
		ModelId:     id,
		BillingMode: "token",
		MinTokens:   0,
		MaxTokens:   nil,
		InputPrice:  0,
		OutputPrice: 0,
	})
	if err != nil {
		return nil, err
	}

	return &v1.ModelCreateRes{ID: id}, nil
}

// UpdateModel 更新模型（含弃用状态管理）
func (s *sAdmin) UpdateModel(ctx context.Context, req *v1.ModelUpdateReq) (*v1.ModelUpdateRes, error) {
	var oldModel struct {
		ModelId string `json:"model_id"`
		Status  string `json:"status"`
	}
	err := dao.MdlModels.Ctx(ctx).Where("id", req.ID).Fields("model_id, status").Scan(&oldModel)
	if err != nil || oldModel.ModelId == "" {
		return nil, common.NewBusinessError(404, "模型不存在")
	}

	data := do.MdlModels{}
	hasUpdate := false
	var capUpdate *string // capabilities 需要单独处理

	if req.ModelName != "" {
		data.ModelName = req.ModelName
		hasUpdate = true
	}
	if req.Category != "" {
		data.Category = req.Category
		hasUpdate = true
	}
	data.MaxContextTokens = req.MaxContext
	data.MaxOutputTokens = req.MaxOutput
	hasUpdate = true
	if req.Capabilities != nil {
		capJson, _ := json.Marshal(req.Capabilities)
		s := string(capJson)
		capUpdate = &s
		hasUpdate = true
	}
	if req.Description != "" {
		data.Description = req.Description
		hasUpdate = true
	}
	if req.Tags != nil {
		data.Tags = req.Tags
		hasUpdate = true
	}

	statusChanged := req.Status != "" && req.Status != oldModel.Status

	if req.Status == "deprecated" {
		data.Status = "deprecated"
		data.DeprecatedAt = gtime.Now()
		hasUpdate = true
		if req.SunsetDate != nil && *req.SunsetDate != "" {
			data.SunsetDate = gtime.NewFromStr(*req.SunsetDate)
			hasUpdate = true
		}
		if req.ReplacementModel != "" {
			data.ReplacementModel = req.ReplacementModel
			hasUpdate = true
		}
	} else if req.Status == "active" {
		data.Status = "active"
		hasUpdate = true
	} else if req.Status == "offline" {
		data.Status = "offline"
		hasUpdate = true
	} else if req.Status != "" {
		data.Status = req.Status
		hasUpdate = true
	}

	if !hasUpdate {
		return nil, nil
	}

	if capUpdate != nil {
		_, err = dao.MdlModels.Ctx(ctx).Where("id", req.ID).Data(g.Map{
			"capabilities": *capUpdate,
		}).Update()
		if err != nil {
			return nil, err
		}
	}

	_, err = dao.MdlModels.Ctx(ctx).Where("id", req.ID).Data(data).Update()
	if err != nil {
		return nil, err
	}

	// 状态从 active → deprecated 时发送通知并清除缓存
	if statusChanged && req.Status == "deprecated" && oldModel.Status == "active" {
		go func() {
			bgCtx := context.Background()
			variables := g.Map{
				"model_name":        oldModel.ModelId,
				"sunset_date":       "",
				"replacement_model": req.ReplacementModel,
			}
			if req.SunsetDate != nil {
				variables["sunset_date"] = *req.SunsetDate
			}
			engine := common.NewNotificationEngine()
			if err := engine.SendToAllTenants(bgCtx, "model_deprecated", variables, ""); err != nil {
				g.Log().Errorf(bgCtx, "[ModelDeprecation] send notification for %s failed: %v", oldModel.ModelId, err)
			}
		}()
		relay.NewDataProvider().InvalidateModelCache(oldModel.ModelId)
	}

	// 状态从 deprecated → active 时清除缓存
	if statusChanged && req.Status == "active" && oldModel.Status == "deprecated" {
		relay.NewDataProvider().InvalidateModelCache(oldModel.ModelId)
	}

	return nil, nil
}

// DeleteModel 删除模型（同时删除定价记录）
func (s *sAdmin) DeleteModel(ctx context.Context, req *v1.ModelDeleteReq) (*v1.ModelDeleteRes, error) {
	_, _ = dao.MdlPricing.Ctx(ctx).Where("model_id", req.ID).Delete()
	_, err := dao.MdlModels.Ctx(ctx).Where("id", req.ID).Delete()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func parseTagsArray(tags string) []string {
	if tags == "" {
		return nil
	}
	return nil
}

func parseCapabilities(raw string) map[string]bool {
	if raw == "" || raw == "{}" {
		return nil
	}
	var caps map[string]bool
	if err := json.Unmarshal([]byte(raw), &caps); err != nil {
		return nil
	}
	return caps
}

// ListModelPricing 模型定价列表（模型定价页面专用）
func (s *sAdmin) ListModelPricing(ctx context.Context, req *v1.PricingListReq) (*v1.PricingListRes, error) {
	query := dao.MdlModels.Ctx(ctx).Where("status", "active")

	if req.Category != "" {
		query = query.Where("category", req.Category)
	}
	if req.Search != "" {
		query = query.Where("model_id LIKE ? OR model_name LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
	}

	var total int
	var models []struct {
		ID        int64  `json:"id"`
		ModelId   string `json:"model_id"`
		ModelName string `json:"model_name"`
		Category  string `json:"category"`
	}

	err := query.Fields("id, model_id, model_name, category").
		OrderAsc("category").OrderAsc("model_id").
		Page(req.Page, req.PageSize).
		ScanAndCount(&models, &total, false)
	if err != nil {
		return nil, err
	}

	modelIDs := make([]int64, 0, len(models))
	for _, m := range models {
		modelIDs = append(modelIDs, m.ID)
	}

	type pricingRow struct {
		ModelId         int64    `json:"model_id"`
		BillingMode     string   `json:"billing_mode"`
		InputPrice      float64  `json:"input_price"`
		OutputPrice     float64  `json:"output_price"`
		PerRequestPrice *float64 `json:"per_request_price"`
	}
	var pricingRows []pricingRow
	if len(modelIDs) > 0 {
		err = dao.MdlPricing.Ctx(ctx).
			Fields("model_id, billing_mode, input_price, output_price, per_request_price").
			Where("model_id IN (?) AND min_tokens = 0", modelIDs).
			Scan(&pricingRows)
		if err != nil {
			return nil, err
		}
	}
	pricingMap := make(map[int64]pricingRow, len(pricingRows))
	for _, p := range pricingRows {
		pricingMap[p.ModelId] = p
	}

	list := make([]v1.PricingListItem, 0, len(models))
	for _, m := range models {
		item := v1.PricingListItem{
			ID:        m.ID,
			ModelId:   m.ModelId,
			ModelName: m.ModelName,
			Category:  m.Category,
		}
		if p, ok := pricingMap[m.ID]; ok {
			item.PricingMode = p.BillingMode
			item.InputPrice = p.InputPrice
			item.OutputPrice = p.OutputPrice
			if p.PerRequestPrice != nil {
				item.PerRequestPrice = *p.PerRequestPrice
			}
		}
		list = append(list, item)
	}

	return &v1.PricingListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetModelPricing 获取模型定价
func (s *sAdmin) GetModelPricing(ctx context.Context, req *v1.PricingGetReq) (*v1.PricingGetRes, error) {
	var rows []struct {
		BillingMode        string   `json:"billing_mode"`
		MinTokens          int64    `json:"min_tokens"`
		MaxTokens          *int64   `json:"max_tokens"`
		InputPrice         float64  `json:"input_price"`
		OutputPrice        float64  `json:"output_price"`
		PerRequestPrice    *float64 `json:"per_request_price"`
		CacheReadPrice     float64  `json:"cache_read_price"`
		CacheCreationPrice float64  `json:"cache_creation_price"`
	}

	err := dao.MdlPricing.Ctx(ctx).
		Where("model_id", req.ModelID).
		OrderAsc("min_tokens").
		Scan(&rows)
	if err != nil {
		return nil, err
	}

	result := make([]v1.PricingItem, 0, len(rows))
	for _, r := range rows {
		result = append(result, v1.PricingItem{
			BillingMode:        r.BillingMode,
			MinTokens:          r.MinTokens,
			MaxTokens:          r.MaxTokens,
			InputPrice:         r.InputPrice,
			OutputPrice:        r.OutputPrice,
			PerRequestPrice:    r.PerRequestPrice,
			CacheReadPrice:     r.CacheReadPrice,
			CacheCreationPrice: r.CacheCreationPrice,
		})
	}
	return &v1.PricingGetRes{List: result}, nil
}

// SetModelPricing 设置模型定价（全量替换）
func (s *sAdmin) SetModelPricing(ctx context.Context, req *v1.PricingSetReq) (*v1.PricingSetRes, error) {
	_, err := dao.MdlPricing.Ctx(ctx).Where("model_id", req.ModelID).Delete()
	if err != nil {
		return nil, err
	}

	for _, item := range req.Items {
		_, err := dao.MdlPricing.Ctx(ctx).Insert(do.MdlPricing{
			ModelId:            req.ModelID,
			BillingMode:        item.BillingMode,
			MinTokens:          item.MinTokens,
			MaxTokens:          item.MaxTokens,
			InputPrice:         item.InputPrice,
			OutputPrice:        item.OutputPrice,
			PerRequestPrice:    item.PerRequestPrice,
			CacheReadPrice:     item.CacheReadPrice,
			CacheCreationPrice: item.CacheCreationPrice,
		})
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}

// FetchOfficialPricing 拉取模型官方定价（来自 LiteLLM + models.dev 双数据源）
func (s *sAdmin) FetchOfficialPricing(ctx context.Context, req *v1.PricingFetchOfficialReq) (*v1.PricingFetchOfficialRes, error) {
	var model struct {
		ModelId string `json:"model_id"`
	}
	err := dao.MdlModels.Ctx(ctx).Where("id", req.ModelID).Fields("model_id").Scan(&model)
	if err != nil || model.ModelId == "" {
		return nil, common.NewBusinessError(404, "模型不存在")
	}

	sources := make([]*v1.OfficialPricingSource, 0, 2)

	// Source 1: LiteLLM
	litellmSource := fetchLiteLLMSource(ctx, model.ModelId)
	sources = append(sources, litellmSource)

	// Source 2: models.dev
	modelsDevSource := fetchModelsDevSource(ctx, model.ModelId)
	sources = append(sources, modelsDevSource)

	return &v1.PricingFetchOfficialRes{
		ModelName: model.ModelId,
		Sources:   sources,
	}, nil
}

func fetchLiteLLMSource(ctx context.Context, modelName string) *v1.OfficialPricingSource {
	source := &v1.OfficialPricingSource{Source: "litellm"}

	data, err := common.FetchLiteLLMPricing(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[FetchOfficialPricing] litellm fetch failed for %s: %v", modelName, err)
		return source
	}

	matchedName, entry := common.FindLiteLLMModel(data, modelName)
	if entry == nil {
		return source
	}

	inputPrice := entry.InputCostPerToken * 1_000_000
	outputPrice := entry.OutputCostPerToken * 1_000_000
	cacheReadPrice := entry.CacheReadInputTokenCost * 1_000_000
	cacheCreationPrice := entry.CacheCreationInputTokenCost * 1_000_000

	billingMode := "token"
	if entry.Mode == "image_generation" && entry.OutputCostPerImage > 0 && inputPrice == 0 && outputPrice == 0 {
		billingMode = "per_request"
	}

	source.Found = true
	source.Provider = entry.LitellmProvider
	source.Mode = entry.Mode
	source.MaxContext = entry.MaxInputTokens
	source.MaxOutput = entry.MaxOutputTokens
	source.Pricing = &v1.OfficialPricingItem{
		InputPrice:         roundTo4(inputPrice),
		OutputPrice:        roundTo4(outputPrice),
		CacheReadPrice:     roundTo4(cacheReadPrice),
		CacheCreationPrice: roundTo4(cacheCreationPrice),
		BillingMode:        billingMode,
	}

	g.Log().Debugf(ctx, "[FetchOfficialPricing] litellm matched %s -> %s", modelName, matchedName)
	return source
}

func fetchModelsDevSource(ctx context.Context, modelName string) *v1.OfficialPricingSource {
	source := &v1.OfficialPricingSource{Source: "models.dev"}

	data, err := common.FetchModelsDevPricing(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[FetchOfficialPricing] models.dev fetch failed for %s: %v", modelName, err)
		return source
	}

	matchedName, entry := common.FindModelsDevModel(data, modelName)
	if entry == nil {
		return source
	}

	cacheReadPrice := 0.0
	if entry.CacheRead != nil {
		cacheReadPrice = *entry.CacheRead
	}

	source.Found = true
	source.Provider = entry.Provider
	source.Pricing = &v1.OfficialPricingItem{
		InputPrice: roundTo4(entry.Input),
		OutputPrice: roundTo4(func() float64 {
			if entry.Output != nil {
				return *entry.Output
			}
			return 0
		}()),
		CacheReadPrice: roundTo4(cacheReadPrice),
		BillingMode:    "token",
	}

	g.Log().Debugf(ctx, "[FetchOfficialPricing] models.dev matched %s -> %s", modelName, matchedName)
	return source
}

func roundTo4(v float64) float64 {
	return float64(int(v*10000+0.5)) / 10000
}

// FetchOfficialModelInfo 按模型名称拉取官方模型信息（上下文长度+能力特性）
func (s *sAdmin) FetchOfficialModelInfo(ctx context.Context, req *v1.ModelFetchOfficialInfoReq) (*v1.ModelFetchOfficialInfoRes, error) {
	data, err := common.FetchLiteLLMPricing(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[FetchOfficialModelInfo] litellm fetch failed for %s: %v", req.ModelName, err)
		return &v1.ModelFetchOfficialInfoRes{Found: false}, nil
	}

	_, entry := common.FindLiteLLMModel(data, req.ModelName)
	if entry == nil {
		return &v1.ModelFetchOfficialInfoRes{Found: false}, nil
	}

	capabilities := map[string]bool{
		"vision":                    entry.SupportsVision,
		"function_calling":          entry.SupportsFunctionCalling,
		"parallel_function_calling": entry.SupportsParallelFuncCalling,
		"tool_choice":               entry.SupportsToolChoice,
		"response_schema":           entry.SupportsResponseSchema,
		"system_messages":           entry.SupportsSystemMessages,
		"prompt_caching":            entry.SupportsPromptCaching,
		"audio_input":               entry.SupportsAudioInput,
		"audio_output":              entry.SupportsAudioOutput,
		"pdf_input":                 entry.SupportsPdfInput,
		"embedding_image":           entry.SupportsEmbeddingImage,
		"reasoning":                 entry.SupportsReasoning,
		"web_search":                entry.SupportsWebSearch,
	}

	return &v1.ModelFetchOfficialInfoRes{
		Found:            true,
		Provider:         entry.LitellmProvider,
		MaxContextTokens: entry.MaxInputTokens,
		MaxOutputTokens:  entry.MaxOutputTokens,
		Capabilities:     capabilities,
	}, nil
}

// ExportModels exports model list to CSV or Excel.
func (s *sAdmin) ExportModels(ctx context.Context, req *v1.ModelExportReq) (*v1.ModelExportRes, error) {
	r := g.RequestFromCtx(ctx)
	format := req.Format
	if format == "" {
		format = "csv"
	}
	if format == "excel" {
		format = "xlsx"
	}

	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "model_id", Header: "模型标识"},
		{Field: "model_name", Header: "显示名称"},
		{Field: "category", Header: "分类"},
		{Field: "status", Header: "状态"},
		{Field: "max_context_tokens", Header: "最大上下文"},
		{Field: "max_output_tokens", Header: "最大输出"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   format,
		Filename: "模型_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	modelFields := "id, model_id, model_name, category, status, max_context_tokens, max_output_tokens, created_at"

	if format == "xlsx" {
		query := dao.MdlModels.Ctx(ctx)
		if req.Category != "" {
			query = query.Where("category", req.Category)
		}
		if req.Status != "" {
			query = query.Where("status", req.Status)
		}
		if req.Search != "" {
			query = query.Where("model_id LIKE ? OR model_name LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
		}
		var models []struct {
			ID               int64       `json:"id"`
			ModelId          string      `json:"model_id"`
			ModelName        string      `json:"model_name"`
			Category         string      `json:"category"`
			Status           string      `json:"status"`
			MaxContextTokens int         `json:"max_context_tokens"`
			MaxOutputTokens  int         `json:"max_output_tokens"`
			CreatedAt        *gtime.Time `json:"created_at"`
		}
		if err := query.Fields(modelFields).OrderDesc("id").Scan(&models); err != nil {
			return nil, err
		}
		data := make([]map[string]any, len(models))
		for i, m := range models {
			data[i] = map[string]any{
				"id":                 m.ID,
				"model_id":           m.ModelId,
				"model_name":         m.ModelName,
				"category":           m.Category,
				"status":             m.Status,
				"max_context_tokens": m.MaxContextTokens,
				"max_output_tokens":  m.MaxOutputTokens,
				"created_at":         m.CreatedAt.String(),
			}
		}
		return nil, export.WriteExcel(r, config, data)
	}

	return nil, export.StreamCSV(r, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			query := dao.MdlModels.Ctx(ctx)
			if req.Category != "" {
				query = query.Where("category", req.Category)
			}
			if req.Status != "" {
				query = query.Where("status", req.Status)
			}
			if req.Search != "" {
				query = query.Where("model_id LIKE ? OR model_name LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
			}
			var batch []struct {
				ID               int64       `json:"id"`
				ModelId          string      `json:"model_id"`
				ModelName        string      `json:"model_name"`
				Category         string      `json:"category"`
				Status           string      `json:"status"`
				MaxContextTokens int         `json:"max_context_tokens"`
				MaxOutputTokens  int         `json:"max_output_tokens"`
				CreatedAt        *gtime.Time `json:"created_at"`
			}
			if err := query.Fields(modelFields).OrderDesc("id").Limit(1000).Offset(offset).Scan(&batch); err != nil {
				return
			}
			for _, m := range batch {
				if !yield(map[string]any{
					"id":                 m.ID,
					"model_id":           m.ModelId,
					"model_name":         m.ModelName,
					"category":           m.Category,
					"status":             m.Status,
					"max_context_tokens": m.MaxContextTokens,
					"max_output_tokens":  m.MaxOutputTokens,
					"created_at":         m.CreatedAt.String(),
				}) {
					return
				}
			}
			if len(batch) < 1000 {
				break
			}
			offset += 1000
		}
	})
}
