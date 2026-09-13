package minimax

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/taskchannel"
)

// MiniMax 视频生成任务适配器，覆盖两代协议（按上游模型名分派，见 IsV2Model）：
//   - v2（MiniMax-H3 系列）：POST /v2/video_generation 提交（content[] 多模态）→
//     GET /v2/query/video_generation/{task_id} 轮询，错误为 OpenAI 风格 OaiError；
//   - v1（Hailuo 2.x 及旧型号）：POST /v1/video_generation 提交（扁平字段）→
//     GET /v1/query/video_generation?task_id= 轮询，错误为 base_resp 信封
//     （HTTP 200 且 status_code != 0 即业务失败），成功后需二跳
//     GET /v1/files/retrieve?file_id= 换取 download_url（FetchTask 内链式完成）。
//
// 请求体支持两种入站形态（由 BuildRequestBody 分流）：
//   - 原生形态：v2 顶层 content[] 数组 / v1 顶层 first_frame_image 等扁平字段（官方协议入站）；
//   - 通用形态：{model, prompt, seconds, images[], metadata{...}} 归一化任务体
//     （/v1/videos 与 /v1/video/generations 入站）。

func init() {
	taskchannel.Register(constant.ProviderMiniMax, func() common.TaskAdaptor {
		return &Adaptor{}
	})
}

// ==================== 请求/响应结构体 ====================

// submitResponse v2 提交任务响应（VideoGenerationV2Resp）
type submitResponse struct {
	TaskID string `json:"task_id"`
}

// baseResp v1 接口的业务状态信封（HTTP 200 且 status_code != 0 表示业务失败）
type baseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

// submitResponseV1 v1 提交任务响应（VideoGenerationResp）
type submitResponseV1 struct {
	TaskID   string    `json:"task_id"`
	BaseResp *baseResp `json:"base_resp"`
}

// taskQueryResponseV1 v1 任务查询响应（QueryVideoGenerationTaskResp）。
// DownloadURL 为 FetchTask 二跳 files/retrieve 后合并进来的字段（非上游查询接口原生字段）。
type taskQueryResponseV1 struct {
	TaskID      string    `json:"task_id"`
	Status      string    `json:"status"`
	FileID      string    `json:"file_id"`
	VideoWidth  int       `json:"video_width"`
	VideoHeight int       `json:"video_height"`
	DownloadURL string    `json:"download_url,omitempty"`
	BaseResp    *baseResp `json:"base_resp"`
}

// fileRetrieveResponse v1 文件查询响应（RetrieveFileResp）
type fileRetrieveResponse struct {
	File struct {
		FileID      int    `json:"file_id"`
		DownloadURL string `json:"download_url"`
	} `json:"file"`
	BaseResp *baseResp `json:"base_resp"`
}

// oaiErrorBody v2 接口错误结构（OpenAI 风格顶层 error 对象）
type oaiErrorBody struct {
	Error struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// IsV2Model 判断模型是否走 v2 H3 协议（content[] 多模态）。
// 其余模型（Hailuo 2.x 及 T2V/I2V/S2V 旧型号）一律走 v1 扁平协议——
// 官方 v2 接口明确「当前支持模型：MiniMax-H3」，未列型号默认按 v1 处理。
func IsV2Model(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "minimax-h3")
}

// ==================== Adaptor 实现 ====================

// Adaptor MiniMax H3 视频任务适配器
type Adaptor struct {
	info *common.RelayInfo
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
}

func (a *Adaptor) ValidateRequest(_ context.Context, _ *common.RelayInfo, body []byte) *common.TaskError {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body"}
	}
	// v2 原生形态：官方约束每次请求必须包含一个非空 text 项
	if content, ok := req["content"].([]any); ok {
		if !contentHasText(content) {
			return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "content must contain a non-empty text item"}
		}
		return nil
	}
	// v1 原生形态：文生视频 prompt 必填、图生视频 first_frame_image 必填、主体参考 subject_reference 必填（三选一）
	if hasAnyKey(req, "first_frame_image", "last_frame_image", "subject_reference") {
		if !hasAnyKey(req, "prompt", "first_frame_image", "subject_reference") {
			return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "one of prompt / first_frame_image / subject_reference is required"}
		}
		return nil
	}
	// 通用形态：prompt 必填（上游 text 项 / v1 prompt 的唯一来源）
	if prompt, ok := req["prompt"].(string); !ok || strings.TrimSpace(prompt) == "" {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "prompt is required"}
	}
	return nil
}

// EstimateBilling 估算计费上下文。
// 只写 spec.* 规格事实值（per_second 计费矩阵查价键 / 素材计费张数与输入视频存在性），
// 禁止附加 float 乘数键——ratios 中所有 float64 值会被计费引擎 applyRatioMultipliers 连乘，
// 混入非乘数键会改变费用（bool/字符串值不受影响，会被自动跳过）。
// resolveDuration/resolveResolution 与 BuildRequestBody 共用，保证计费规格与实际生成规格一致。
// spec.input_image_count 为提交可知的输入图片张数（素材计费预扣用；结算以官方 usage 为准）；
// spec.has_input_video 标记提交含输入视频素材（时长未知，预扣按生成长度上限估，见 minimax-material 方案）。
func (a *Adaptor) EstimateBilling(_ context.Context, _ *common.RelayInfo, body []byte) map[string]any {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		req = nil
	}
	return map[string]any{
		"spec.duration":          float64(resolveDuration(req)),
		"spec.resolution":        resolveResolution(req),
		"spec.input_image_count": float64(countInputImages(req)),
		"spec.has_input_video":   hasInputVideo(req),
	}
}

// countInputImages 统计提交请求中的输入图片张数（与 buildGenericContent 的素材归并同源）：
// v2 原生形态数 content[] 的 image_url 项；通用形态数 images[] + metadata.image +
// metadata.last_frame + metadata.content[] 的 image_url 项。图片张数不计音频/视频素材
// （视频存在性另见 hasInputVideo，时长不可知由结算补）。
func countInputImages(req map[string]any) int {
	if req == nil {
		return 0
	}
	// v2 原生形态
	if content, ok := req["content"].([]any); ok {
		count := 0
		for _, item := range content {
			if m, ok := item.(map[string]any); ok {
				if t, _ := m["type"].(string); t == "image_url" {
					count++
				}
			}
		}
		return count
	}
	// 通用形态：与 buildGenericContent 相同的归并口径
	count := 0
	if images, ok := req["images"].([]any); ok {
		count += len(images)
	}
	metadata := taskchannel.ExtractMetadata(req)
	if _, ok := metadata["image"].(string); ok {
		count++
	}
	if _, ok := metadata["last_frame"].(string); ok {
		count++
	}
	var meta struct {
		Content []map[string]any `json:"content"`
	}
	if taskchannel.UnmarshalMetadata(metadata, &meta) == nil {
		for _, item := range meta.Content {
			if t, _ := item["type"].(string); t == "image_url" {
				count++
			}
		}
	}
	return count
}

// hasInputVideo 判断提交请求是否含输入视频素材（与 buildGenericContent 的素材归并同源）：
// v2 原生形态查 content[] 的 video_url 项；通用形态查 metadata.content[] 的 video_url 项。
// 视频以 URL 提交、时长提交时不可知，素材计费据此在预扣时按生成长度上限冻结（15s，见 minimax-material 方案）。
func hasInputVideo(req map[string]any) bool {
	if req == nil {
		return false
	}
	// v2 原生形态
	if content, ok := req["content"].([]any); ok {
		for _, item := range content {
			if m, ok := item.(map[string]any); ok {
				if t, _ := m["type"].(string); t == "video_url" {
					return true
				}
			}
		}
		return false
	}
	// 通用形态：metadata.content[] 透传项（多模态参考，与 countInputImages 同口径）
	metadata := taskchannel.ExtractMetadata(req)
	var meta struct {
		Content []map[string]any `json:"content"`
	}
	if taskchannel.UnmarshalMetadata(metadata, &meta) == nil {
		for _, item := range meta.Content {
			if t, _ := item["type"].(string); t == "video_url" {
				return true
			}
		}
	}
	return false
}

func (a *Adaptor) AdjustBillingOnSubmit(_ *common.RelayInfo, _ []byte) map[string]any {
	return nil
}

func (a *Adaptor) BuildRequestURL(info *common.RelayInfo) (string, error) {
	base := strings.TrimRight(info.ChannelMeta.BaseURL, "/")
	if IsV2Model(dispatchModel(info)) {
		return base + "/v2/video_generation", nil
	}
	return base + "/v1/video_generation", nil
}

// dispatchModel 协议分派用的模型名（上游映射名优先，缺省回落请求模型名）
func dispatchModel(info *common.RelayInfo) string {
	if info.ChannelMeta.UpstreamModelName != "" {
		return info.ChannelMeta.UpstreamModelName
	}
	return info.OriginModelName
}

func (a *Adaptor) BuildRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	return nil
}

func (a *Adaptor) BuildRequestBody(_ context.Context, info *common.RelayInfo, body []byte) (io.Reader, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("minimax: invalid request body: %w", err)
	}

	modelName := resolveUpstreamModel(info, req)

	var out map[string]any
	if IsV2Model(modelName) {
		out = buildV2RequestBody(req, modelName)
	} else {
		out = buildV1RequestBody(req, modelName)
	}

	data, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal minimax request: %w", err)
	}
	return strings.NewReader(string(data)), nil
}

// buildV2RequestBody v2（H3 系列）请求体：content[] 多模态。
// 原生形态（顶层 content[]）原样透传；通用形态从 prompt/images/metadata 组装。
// 注意不透传 callback_url——上游回调携带上游 task_id，与网关公开 ID 对不上且会绕过网关轮询计费。
func buildV2RequestBody(req map[string]any, modelName string) map[string]any {
	out := map[string]any{
		"model":      modelName,
		"resolution": resolveResolution(req),
		"duration":   resolveDuration(req),
	}
	if content, ok := req["content"].([]any); ok && len(content) > 0 {
		out["content"] = content
		if ratio := resolveRatio(req); ratio != "" {
			out["ratio"] = ratio
		}
	} else {
		content := buildGenericContent(req)
		if len(content) == 0 {
			content = []map[string]any{{"type": "text", "text": ""}}
		}
		out["content"] = content
		// t2v（纯文本输入）时官方要求 ratio 必填且不能为 adaptive；含图片输入时上游恒按 adaptive 处理，显式传值无害
		ratio := resolveRatio(req)
		if ratio == "" {
			ratio = defaultTextOnlyRatio
		}
		out["ratio"] = ratio
	}
	if wm, ok := req["aigc_watermark"].(bool); ok {
		out["aigc_watermark"] = wm
	}
	return out
}

// buildV1RequestBody v1（Hailuo 系列）请求体：扁平字段。
// v1 原生入站的顶层字段（first_frame_image/last_frame_image/subject_reference/prompt_optimizer/
// fast_pretreatment）与通用词汇（images[0]/metadata.image/metadata.last_frame 等）统一在此归并，
// 白名单式拷贝保证 callback_url 等网关侧字段不会外泄到上游。
func buildV1RequestBody(req map[string]any, modelName string) map[string]any {
	out := map[string]any{
		"model":      modelName,
		"resolution": resolveResolution(req),
		"duration":   resolveDuration(req),
	}
	if prompt, ok := req["prompt"].(string); ok && strings.TrimSpace(prompt) != "" {
		out["prompt"] = prompt
	}

	// 首帧图：v1 原生顶层字段 > images[0]/metadata.image
	firstFrame := stringField(req, "first_frame_image")
	if firstFrame == "" {
		firstFrame = genericFirstImage(req)
	}
	if firstFrame != "" {
		out["first_frame_image"] = firstFrame
	}
	// 尾帧图：v1 原生顶层字段 > metadata.last_frame
	lastFrame := stringField(req, "last_frame_image")
	if lastFrame == "" {
		lastFrame = metaString(req, "last_frame")
	}
	if lastFrame != "" {
		out["last_frame_image"] = lastFrame
	}
	// 主体参考（S2V-01）：v1 原生顶层字段 > metadata.subject_reference
	if sr, ok := req["subject_reference"]; ok && sr != nil {
		out["subject_reference"] = sr
	} else if sr := metaAny(req, "subject_reference"); sr != nil {
		out["subject_reference"] = sr
	}
	// 可选参数白名单透传
	for _, key := range []string{"prompt_optimizer", "fast_pretreatment", "aigc_watermark"} {
		if v, ok := req[key]; ok {
			out[key] = v
		}
	}
	return out
}

func (a *Adaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	reqURL, err := a.BuildRequestURL(info)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if err := a.BuildRequestHeader(req.Header, info); err != nil {
		return nil, fmt.Errorf("setup header: %w", err)
	}
	client := common.NewPooledClient(120, info.ChannelMeta.Settings.UseProxy)
	return client.Do(req)
}

func (a *Adaptor) DoResponse(_ context.Context, resp *http.Response, info *common.RelayInfo) (string, []byte, *common.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "read response failed"}
	}

	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		// v2 用 OaiError、v1 用 base_resp，两种信封都尝试解析
		var errResp oaiErrorBody
		if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
			msg = errResp.Error.Message
		} else {
			var baseRespErr struct {
				BaseResp *baseResp `json:"base_resp"`
			}
			if json.Unmarshal(body, &baseRespErr) == nil && baseRespErr.BaseResp != nil && baseRespErr.BaseResp.StatusMsg != "" {
				msg = baseRespErr.BaseResp.StatusMsg
			}
		}
		if msg == "" {
			msg = fmt.Sprintf("upstream returned status %d", resp.StatusCode)
		}
		return "", body, &common.TaskError{StatusCode: resp.StatusCode, Message: msg}
	}

	if IsV2Model(dispatchModel(info)) {
		var result submitResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return "", body, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "parse response failed"}
		}
		if result.TaskID == "" {
			return "", body, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "upstream returned empty task id"}
		}
		return result.TaskID, body, nil
	}

	// v1：HTTP 200 仍需检查 base_resp 业务码（status_code != 0 即业务失败）
	var result submitResponseV1
	if err := json.Unmarshal(body, &result); err != nil {
		return "", body, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "parse response failed"}
	}
	if result.BaseResp != nil && result.BaseResp.StatusCode != 0 {
		msg := result.BaseResp.StatusMsg
		if msg == "" {
			msg = fmt.Sprintf("upstream business error %d", result.BaseResp.StatusCode)
		}
		return "", body, &common.TaskError{StatusCode: http.StatusBadRequest, Message: msg, ErrCode: fmt.Sprintf("%d", result.BaseResp.StatusCode)}
	}
	if result.TaskID == "" {
		return "", body, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "upstream returned empty task id"}
	}
	return result.TaskID, body, nil
}

func (a *Adaptor) FetchTask(ctx context.Context, baseURL, apiKey string, taskData []byte) (*http.Response, error) {
	var data struct {
		TaskID   string `json:"task_id"`
		UseProxy bool   `json:"use_proxy"`
		Model    string `json:"model"` // 轮询器注入的上游模型名，用于 v1/v2 协议分派
	}
	if err := json.Unmarshal(taskData, &data); err != nil {
		return nil, fmt.Errorf("minimax: invalid task data: %w", err)
	}

	base := strings.TrimRight(baseURL, "/")
	if IsV2Model(data.Model) {
		return fetchV2Task(ctx, base, apiKey, data.TaskID, data.UseProxy)
	}
	return fetchV1Task(ctx, base, apiKey, data.TaskID, data.UseProxy)
}

// fetchV2Task v2 单任务查询：GET /v2/query/video_generation/{task_id}
func fetchV2Task(ctx context.Context, base, apiKey, taskID string, useProxy bool) (*http.Response, error) {
	url := fmt.Sprintf("%s/v2/query/video_generation/%s", base, taskID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := common.NewPooledClient(30, useProxy)
	return client.Do(req)
}

// fetchV1Task v1 单任务查询：GET /v1/query/video_generation?task_id=...。
// v1 协议成功后只给 file_id，需二跳 GET /v1/files/retrieve?file_id=... 换取限时 download_url——
// 在此链式完成并把 download_url 合并进查询响应体（ParseTaskResult 与原生查询回放共用该字段）。
// 二跳失败不改变任务状态判定：任务仍按 Success 结算，仅缺下载直链（客户端可稍后重查）。
// 调试日志注意：两跳共用同一 ctx 捕获器，调试记录里 URL/headers 为二跳、body 为两跳拼接。
func fetchV1Task(ctx context.Context, base, apiKey, taskID string, useProxy bool) (*http.Response, error) {
	client := common.NewPooledClient(30, useProxy)

	queryURL := fmt.Sprintf("%s/v1/query/video_generation?task_id=%s", base, url.QueryEscape(taskID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 仅在成功且拿到 file_id 时二跳；其余状态原样返回
	var probe taskQueryResponseV1
	if resp.StatusCode != http.StatusOK || json.Unmarshal(body, &probe) != nil || probe.Status != "Success" || probe.FileID == "" {
		return rebuiltResponse(resp.StatusCode, body), nil
	}

	fileURL := fmt.Sprintf("%s/v1/files/retrieve?file_id=%s", base, url.QueryEscape(probe.FileID))
	fileReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return rebuiltResponse(resp.StatusCode, body), nil
	}
	fileReq.Header.Set("Authorization", "Bearer "+apiKey)

	fileResp, err := client.Do(fileReq)
	if err != nil {
		return rebuiltResponse(resp.StatusCode, body), nil
	}
	defer fileResp.Body.Close()
	fileBody, err := io.ReadAll(fileResp.Body)
	if err != nil {
		return rebuiltResponse(resp.StatusCode, body), nil
	}

	var fileResult fileRetrieveResponse
	if fileResp.StatusCode != http.StatusOK || json.Unmarshal(fileBody, &fileResult) != nil ||
		fileResult.File.DownloadURL == "" ||
		(fileResult.BaseResp != nil && fileResult.BaseResp.StatusCode != 0) {
		// 二跳失败：保留原查询响应（download_url 缺失），任务仍可按 Success 结算
		return rebuiltResponse(resp.StatusCode, body), nil
	}

	// 合并 download_url 进查询响应体
	var merged map[string]any
	if json.Unmarshal(body, &merged) == nil {
		merged["download_url"] = fileResult.File.DownloadURL
		if newBody, err := json.Marshal(merged); err == nil {
			return rebuiltResponse(resp.StatusCode, newBody), nil
		}
	}
	return rebuiltResponse(resp.StatusCode, body), nil
}

// rebuiltResponse 用已消费的 body 重建 http.Response（FetchTask 契约要求返回未读响应体）
func rebuiltResponse(statusCode int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

// ParseTaskResult 解析上游单任务查询响应。两代协议按响应形态自动区分：
// v2 为 {"task": {...}} 包装、v1 为扁平的 {task_id, status, file_id, ...}。
// 注意：不填 ActualCost——per_second 计费下 RecalculateByTokens 返回 0，结算保持预扣金额，
// 而 duration 必填=产物时长，预扣即终价；填入 ActualCost 反而会用上游口径覆盖平台定价。
func (a *Adaptor) ParseTaskResult(body []byte) (*common.TaskInfo, error) {
	var resp TaskQueryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("minimax: parse task result: %w", err)
	}
	if resp.Task != nil {
		return parseV2TaskResult(body, resp.Task)
	}
	return parseV1TaskResult(body)
}

// parseV2TaskResult v2（H3）查询响应解析
func parseV2TaskResult(body []byte, task *VideoTask) (*common.TaskInfo, error) {
	info := &common.TaskInfo{Data: body}

	// 上游返回的 token 用量仅作记录（per_second 计费不按 token 结算）
	if task.Usage != nil && task.Usage.TotalTokens > 0 {
		info.TotalTokens = task.Usage.TotalTokens
		info.CompletionTokens = task.Usage.CompletionTokens
		info.PromptTokens = task.Usage.PromptTokens
	}

	// 素材计量（input_seconds/output_seconds/input_image_count 等）：素材计费方案的结算依据。
	// 官方 usage 携带任一按秒计量或图片张数字段即透出（全零不出，避免无素材任务占用结算链路）
	if task.Usage != nil && (task.Usage.InputSeconds > 0 || task.Usage.OutputSeconds > 0 ||
		task.Usage.InputImageCount > 0 || task.Usage.InputAudioSeconds > 0 || task.Usage.TotalSeconds > 0) {
		info.MaterialUsage = &common.TaskMaterialUsage{
			InputVideoSeconds: float64(task.Usage.InputSeconds),
			InputImageCount:   task.Usage.InputImageCount,
			InputAudioSeconds: float64(task.Usage.InputAudioSeconds),
			OutputSeconds:     float64(task.Usage.OutputSeconds),
		}
	}

	switch task.Status {
	case "queued":
		info.Status = common.TaskStatusQueued
		info.Progress = "10%"
	case "running":
		info.Status = common.TaskStatusInProgress
		info.Progress = "50%"
	case "succeeded":
		info.Status = common.TaskStatusSuccess
		info.Progress = "100%"
		if task.Content != nil {
			info.ResultURL = task.Content.URL
		}
	case "failed":
		info.Status = common.TaskStatusFailure
		info.FailReason = "video generation failed"
		if task.Error != nil && task.Error.Message != "" {
			info.FailReason = task.Error.Message
		}
	case "cancelled":
		// 必须映射为终态 FAILURE：映射成非终态会让任务挂到超时兜底（30 分钟）才退款
		info.Status = common.TaskStatusFailure
		info.FailReason = "task cancelled"
	default:
		info.Status = common.TaskStatusInProgress
		info.Progress = "30%"
	}

	return info, nil
}

// parseV1TaskResult v1（Hailuo）查询响应解析（状态机 Preparing/Queueing/Processing/Success/Fail）。
// download_url 为 FetchTask 二跳合并字段，成功时作为 ResultURL。
func parseV1TaskResult(body []byte) (*common.TaskInfo, error) {
	var resp taskQueryResponseV1
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("minimax: parse v1 task result: %w", err)
	}

	info := &common.TaskInfo{Data: body}

	switch resp.Status {
	case "Preparing", "Queueing":
		info.Status = common.TaskStatusQueued
		info.Progress = "10%"
	case "Processing":
		info.Status = common.TaskStatusInProgress
		info.Progress = "50%"
	case "Success":
		info.Status = common.TaskStatusSuccess
		info.Progress = "100%"
		info.ResultURL = resp.DownloadURL
	case "Fail":
		info.Status = common.TaskStatusFailure
		info.FailReason = "video generation failed"
		if resp.BaseResp != nil && resp.BaseResp.StatusMsg != "" {
			info.FailReason = resp.BaseResp.StatusMsg
		}
	default:
		info.Status = common.TaskStatusInProgress
		info.Progress = "30%"
	}

	return info, nil
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return channelName
}

// ==================== 归一化辅助函数 ====================
// 以下函数被 EstimateBilling 与 BuildRequestBody 共用（计费一致性强约束）：
// per_second 矩阵键按 spec.resolution 原值逐字符匹配，两处归一化若不一致会导致
// 「按 768P 生成、按兜底价收费」的系统性少收。

// resolveDuration 提取时长秒数：顶层 duration（原生形态）> seconds > metadata.duration > 默认 6。
func resolveDuration(req map[string]any) int {
	if d, ok := parseIntAny(req["duration"]); ok && d > 0 {
		return d
	}
	if d, ok := parseIntAny(req["seconds"]); ok && d > 0 {
		return d
	}
	metadata := taskchannel.ExtractMetadata(req)
	var meta struct {
		Duration *int `json:"duration"`
	}
	if taskchannel.UnmarshalMetadata(metadata, &meta) == nil && meta.Duration != nil && *meta.Duration > 0 {
		return *meta.Duration
	}
	return defaultDurationSecs
}

// resolveResolution 提取分辨率：顶层 resolution > metadata.resolution > metadata.size > 按模型默认档。
// size 回退覆盖 /v1/videos 入站（该协议的分辨率就承载在 size 字段，原值透传，无换算）。
// 默认档跟随官方口径：H3 与 Hailuo 2.x 默认 768P，T2V/I2V/S2V 旧型号默认 720P。
func resolveResolution(req map[string]any) string {
	if r, ok := req["resolution"].(string); ok && strings.TrimSpace(r) != "" {
		return normalizeResolution(r)
	}
	metadata := taskchannel.ExtractMetadata(req)
	var meta struct {
		Resolution string `json:"resolution"`
		Size       string `json:"size"`
	}
	if taskchannel.UnmarshalMetadata(metadata, &meta) == nil {
		if strings.TrimSpace(meta.Resolution) != "" {
			return normalizeResolution(meta.Resolution)
		}
		if strings.TrimSpace(meta.Size) != "" {
			return normalizeResolution(meta.Size)
		}
	}
	model, _ := req["model"].(string)
	return defaultResolutionFor(model)
}

// defaultResolutionFor 按模型取分辨率默认档（H3/Hailuo 2.x → 768P，其余 → 720P）
func defaultResolutionFor(model string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	if strings.HasPrefix(m, "minimax-h3") || strings.HasPrefix(m, "minimax-hailuo-2") {
		return defaultResolution
	}
	return defaultResolutionV1
}

// resolveRatio 提取宽高比：顶层 ratio > metadata.ratio > metadata.aspect_ratio（kling 词汇）；空 = 未指定。
func resolveRatio(req map[string]any) string {
	if r, ok := req["ratio"].(string); ok && strings.TrimSpace(r) != "" {
		return r
	}
	metadata := taskchannel.ExtractMetadata(req)
	var meta struct {
		Ratio       string `json:"ratio"`
		AspectRatio string `json:"aspect_ratio"`
	}
	if taskchannel.UnmarshalMetadata(metadata, &meta) == nil {
		if meta.Ratio != "" {
			return meta.Ratio
		}
		if meta.AspectRatio != "" {
			return meta.AspectRatio
		}
	}
	return ""
}

// normalizeResolution 分辨率值规范：仅做两代协议官方档位的大小写规范（per_second 计费矩阵键
// 逐字符匹配，大小写差异会导致查价错档），**不做任何档位换算**——非官方档位的值
// 原样透传，由上游裁决（非法值上游 400，错误透传给客户端并全额退款）。
// 官方档位全集：v2 的 480P/768P/2K + v1 的 720P/768P/1080P/512P。
func normalizeResolution(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "480p":
		return "480P"
	case "720p":
		return "720P"
	case "768p":
		return "768P"
	case "1080p":
		return "1080P"
	case "512p":
		return "512P"
	case "2k":
		return "2K"
	}
	return strings.TrimSpace(v)
}

// parseIntAny 宽松解析整数（JSON number 解码为 float64，或字符串数字）
func parseIntAny(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		if n > 0 {
			return int(n), true
		}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil && i > 0 {
			return i, true
		}
	}
	return 0, false
}

// buildGenericContent 从归一化任务体组装 MiniMax content[]：
// prompt → text 项；images[0]/metadata.image → 首帧图；metadata.last_frame → 尾帧图；
// metadata.content[] → 追加透传（多模态参考，与 volcengine 适配器同词汇）。
func buildGenericContent(req map[string]any) []map[string]any {
	content := make([]map[string]any, 0, 4)

	if prompt, ok := req["prompt"].(string); ok && strings.TrimSpace(prompt) != "" {
		content = append(content, map[string]any{"type": "text", "text": prompt})
	}

	metadata := taskchannel.ExtractMetadata(req)
	var meta struct {
		Image     string           `json:"image"`
		LastFrame string           `json:"last_frame"`
		Content   []map[string]any `json:"content"`
	}
	_ = taskchannel.UnmarshalMetadata(metadata, &meta)

	firstFrame := meta.Image
	if firstFrame == "" {
		if images, ok := req["images"].([]any); ok && len(images) > 0 {
			if url, ok := images[0].(string); ok {
				firstFrame = url
			}
		}
	}
	if firstFrame != "" {
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": firstFrame},
			"role":      "first_frame",
		})
	}
	if meta.LastFrame != "" {
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": meta.LastFrame},
			"role":      "last_frame",
		})
	}
	content = append(content, meta.Content...)

	return content
}

// contentHasText 判断原生 content[] 是否含非空 text 项
func contentHasText(content []any) bool {
	for _, item := range content {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := m["type"].(string); t == "text" {
			if text, ok := m["text"].(string); ok && strings.TrimSpace(text) != "" {
				return true
			}
		}
	}
	return false
}

// resolveUpstreamModel 解析上游模型名（渠道模型映射优先）
func resolveUpstreamModel(info *common.RelayInfo, req map[string]any) string {
	if info.ChannelMeta.IsModelMapped && info.ChannelMeta.UpstreamModelName != "" {
		return info.ChannelMeta.UpstreamModelName
	}
	if m, ok := req["model"].(string); ok {
		return m
	}
	return ""
}

// ==================== v1 归并辅助 ====================

// hasAnyKey 判断请求 map 是否含任一指定键（值非空）
func hasAnyKey(req map[string]any, keys ...string) bool {
	for _, key := range keys {
		if v, ok := req[key]; ok && v != nil {
			if s, isStr := v.(string); !isStr || strings.TrimSpace(s) != "" {
				return true
			}
		}
	}
	return false
}

// stringField 取顶层字符串字段（非空）
func stringField(req map[string]any, key string) string {
	if s, ok := req[key].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

// metaString 取 metadata 内字符串字段（非空）
func metaString(req map[string]any, key string) string {
	if v, ok := taskchannel.ExtractMetadata(req)[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// metaAny 取 metadata 内任意类型字段（存在则返回）
func metaAny(req map[string]any, key string) any {
	metadata := taskchannel.ExtractMetadata(req)
	if v, ok := metadata[key]; ok {
		return v
	}
	return nil
}

// genericFirstImage 取通用形态的首帧图：images[0] > metadata.image
func genericFirstImage(req map[string]any) string {
	if images, ok := req["images"].([]any); ok && len(images) > 0 {
		if url, ok := images[0].(string); ok {
			return strings.TrimSpace(url)
		}
	}
	if v, ok := taskchannel.ExtractMetadata(req)["image"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
