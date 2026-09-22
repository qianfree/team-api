package ali

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/taskchannel"
)

func init() {
	taskchannel.Register(constant.ProviderAli, func() common.TaskAdaptor {
		return &AliAdaptor{}
	})
}

// ==================== 请求/响应结构体 ====================

// wan3MaxDurationSeconds wan3.0 智能时长模式（duration=-1）的输出上限（秒）。
// 智能时长下输出长度不可预知，预扣按该上限冻结，结算按上游 usage.output_video_duration 多退少补
const wan3MaxDurationSeconds = 30.0

// dashScopeVideoRequest DashScope 视频生成请求
type dashScopeVideoRequest struct {
	Model      string                `json:"model"`
	Input      dashScopeVideoInput   `json:"input"`
	Parameters *dashScopeVideoParams `json:"parameters,omitempty"`
}

type dashScopeVideoInput struct {
	Prompt         string `json:"prompt,omitempty"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
	AudioURL       string `json:"audio_url,omitempty"`
	// Media wan3.0 全能参考模式的媒体素材数组（first_frame/last_frame/reference_image/
	// reference_video/reference_audio/file/link）；wan2.x 旧格式不填
	Media []dashScopeMedia `json:"media,omitempty"`
}

// dashScopeMedia wan3.0 input.media 数组元素
type dashScopeMedia struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type dashScopeVideoParams struct {
	Resolution   string `json:"resolution,omitempty"`
	Ratio        string `json:"ratio,omitempty"`
	Size         string `json:"size,omitempty"`
	Duration     *int   `json:"duration,omitempty"`
	PromptExtend *bool  `json:"prompt_extend,omitempty"`
	Watermark    *bool  `json:"watermark,omitempty"`
	Seed         *int   `json:"seed,omitempty"`
	// Audio wan3.0 输出视频是否包含音频（开关价格相同）
	Audio *bool `json:"audio,omitempty"`
}

// aliMetadata Ali DashScope metadata 参数结构体（用于 UnmarshalMetadata 映射）
type aliMetadata struct {
	NegativePrompt string `json:"negative_prompt,omitempty"`
	AudioURL       string `json:"audio_url,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
	Ratio          string `json:"ratio,omitempty"`
	Size           string `json:"size,omitempty"`
	Duration       *int   `json:"duration,omitempty"`
	PromptExtend   *bool  `json:"prompt_extend,omitempty"`
	Watermark      *bool  `json:"watermark,omitempty"`
	Seed           *int   `json:"seed,omitempty"`
	// Audio wan3.0 输出音频开关（"on"/"off" 通用词汇由 buildVideoRequest 归一，此处不承载）
}

// dashScopeImageRequest DashScope 图片生成请求
type dashScopeImageRequest struct {
	Model      string              `json:"model"`
	Input      dashScopeImageInput `json:"input"`
	Parameters map[string]any      `json:"parameters,omitempty"`
}

type dashScopeImageInput struct {
	Prompt         string `json:"prompt,omitempty"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
}

// dashScopeSubmitResponse DashScope 异步提交响应
type dashScopeSubmitResponse struct {
	Output struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
		Code       string `json:"code"`
		Message    string `json:"message"`
	} `json:"output"`
	RequestID string `json:"request_id"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// dashScopeTaskResponse DashScope 异步任务查询响应
type dashScopeTaskResponse struct {
	Output struct {
		TaskID     string            `json:"task_id"`
		TaskStatus string            `json:"task_status"`
		VideoURL   string            `json:"video_url"`
		Results    []dashScopeResult `json:"results"`
		Code       string            `json:"code"`
		Message    string            `json:"message"`
	} `json:"output"`
	Usage struct {
		Duration            float64 `json:"duration"`
		VideoCount          int     `json:"video_count"`
		OutputVideoDuration float64 `json:"output_video_duration"`
		SR                  int     `json:"SR"`
		Ratio               string  `json:"ratio"`
		ImageCount          int     `json:"image_count"`
	} `json:"usage"`
	RequestID string `json:"request_id"`
}

type dashScopeResult struct {
	URL string `json:"url"`
}

// ==================== Adaptor 实现 ====================

type AliAdaptor struct {
	info    *common.RelayInfo
	isVideo bool // 是否视频生成（否则图片生成）
}

func (a *AliAdaptor) Init(info *common.RelayInfo) {
	a.info = info
}

// detectVideo 根据模型名判断是否为视频生成。
// 注意：万相图片模型（-t2i 文生图、-i2i 图像编辑）即使以 wan2./wan3. 开头也不是视频，必须先排除，
// 否则会被误判为视频而 POST 到 video-synthesis 端点，被上游以 "url error" 拒绝。
func (a *AliAdaptor) detectVideo(modelName string) bool {
	m := strings.ToLower(modelName)
	if strings.Contains(m, "-t2i") || strings.Contains(m, "-i2i") {
		return false
	}
	return strings.HasPrefix(m, "wan2.") || strings.HasPrefix(m, "wan3.") || strings.HasPrefix(m, "wanx2.1-t2v")
}

func (a *AliAdaptor) ValidateRequest(_ context.Context, _ *common.RelayInfo, body []byte) *common.TaskError {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body"}
	}
	if _, ok := req["model"]; !ok {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "model is required"}
	}
	return nil
}

// EstimateBilling 估算任务费用（提交前）。
// 注意：此处自行从 body 提取模型名判定视频/图片，不依赖 BuildRequestBody 的 a.isVideo 副作用——
// 管线中 EstimateBilling（第 6 步）先于 BuildRequestBody（第 7 步）执行，旧实现读 a.isVideo
// 恒为 false，视频时长信号从未上报（预扣恒按引擎默认 5s 估），本次修复。
func (a *AliAdaptor) EstimateBilling(_ context.Context, _ *common.RelayInfo, body []byte) map[string]any {
	ratios := map[string]any{"base": 1.0}
	var req map[string]any
	if json.Unmarshal(body, &req) != nil {
		return ratios
	}

	modelName, _ := req["model"].(string)
	if a.detectVideo(modelName) {
		metadata := taskchannel.ExtractMetadata(req)
		var meta aliMetadata
		if taskchannel.UnmarshalMetadata(metadata, &meta) == nil {
			// 分辨率事实值：per_second 矩阵查价键（480P/720P/1080P 原值）
			if meta.Resolution != "" {
				ratios["spec.resolution"] = meta.Resolution
			}
			// 时长事实值：duration>0 用请求值；duration=-1（wan3.0 智能时长）按输出上限冻结，
			// 结算以 usage.output_video_duration 多退少补
			if meta.Duration != nil {
				d := float64(*meta.Duration)
				if d == -1 {
					d = wan3MaxDurationSeconds
				}
				if d > 0 {
					ratios["duration"] = d
					ratios["spec.duration"] = d
				}
			}
		}
		// 顶层 seconds（OpenAI Videos 通用词汇）优先于 metadata
		if seconds, ok := req["seconds"].(string); ok {
			if d, err := parseInt(seconds); err == nil && d > 0 {
				ratios["duration"] = float64(d)
				ratios["spec.duration"] = float64(d)
			}
		}
	} else {
		// 图片数量
		if n, ok := req["n"].(float64); ok && n > 1 {
			ratios["count"] = n
		}
	}

	return ratios
}

func (a *AliAdaptor) AdjustBillingOnSubmit(_ *common.RelayInfo, _ []byte) map[string]any {
	return nil
}

func (a *AliAdaptor) BuildRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelMeta.BaseURL, "/")
	if a.isVideo {
		return baseURL + "/api/v1/services/aigc/video-generation/video-synthesis", nil
	}
	return baseURL + "/api/v1/services/aigc/text2image/image-synthesis", nil
}

func (a *AliAdaptor) BuildRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	header.Set("X-DashScope-Async", "enable")
	return nil
}

func (a *AliAdaptor) BuildRequestBody(_ context.Context, info *common.RelayInfo, body []byte) (io.Reader, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return strings.NewReader(string(body)), nil
	}

	// 确定模型名，同时判断视频/图片
	modelName := ""
	if info.ChannelMeta.IsModelMapped && info.ChannelMeta.UpstreamModelName != "" {
		modelName = info.ChannelMeta.UpstreamModelName
	} else if m, ok := req["model"].(string); ok {
		modelName = m
	}
	a.isVideo = a.detectVideo(modelName)

	if a.isVideo {
		return a.buildVideoRequest(info, req, modelName)
	}
	return a.buildImageRequest(info, req, modelName)
}

func (a *AliAdaptor) buildVideoRequest(info *common.RelayInfo, req map[string]any, modelName string) (io.Reader, error) {
	dsReq := dashScopeVideoRequest{
		Input: dashScopeVideoInput{},
	}
	dsReq.Model = modelName

	if v, ok := req["prompt"].(string); ok {
		dsReq.Input.Prompt = v
	}

	params := &dashScopeVideoParams{}

	// 从 metadata 提取参数
	metadata := taskchannel.ExtractMetadata(req)
	var meta aliMetadata
	if err := taskchannel.UnmarshalMetadata(metadata, &meta); err != nil {
		return nil, fmt.Errorf("parse ali metadata: %w", err)
	}
	dsReq.Input.NegativePrompt = meta.NegativePrompt
	dsReq.Input.AudioURL = meta.AudioURL
	params.Resolution = meta.Resolution
	params.Ratio = meta.Ratio
	if meta.Size != "" {
		params.Size = strings.ReplaceAll(meta.Size, "x", "*")
	}
	params.Duration = meta.Duration
	params.PromptExtend = meta.PromptExtend
	params.Watermark = meta.Watermark
	params.Seed = meta.Seed

	if seconds, ok := req["seconds"].(string); ok {
		if d, err := parseInt(seconds); err == nil && d > 0 {
			params.Duration = &d
		}
	}

	// wan3.0 专属适配：媒体素材数组 + 音频开关 + size 档位映射。
	// wan2.x 保持旧格式不动（其 size 是原生尺寸语义，不可参与档位映射）
	if strings.HasPrefix(strings.ToLower(modelName), "wan3.") {
		dsReq.Input.Media = buildWan3Media(req, metadata)
		if sound, ok := metadata["sound"].(string); ok && sound != "" {
			audio := sound == "on"
			params.Audio = &audio
		}
		if params.Resolution == "" && meta.Size != "" {
			params.Resolution = resolutionFromSize(meta.Size)
		}
		// ratio 兜底：通用词汇 aspect_ratio（OpenAI Videos 双写键，部分链路只写该键）
		if params.Ratio == "" {
			if ar, ok := metadata["aspect_ratio"].(string); ok {
				params.Ratio = ar
			}
		}
	}

	if params.Resolution != "" || params.Ratio != "" || params.Size != "" ||
		params.Duration != nil || params.PromptExtend != nil || params.Watermark != nil ||
		params.Seed != nil || params.Audio != nil {
		dsReq.Parameters = params
	}

	data, err := json.Marshal(dsReq)
	if err != nil {
		return nil, fmt.Errorf("marshal dashscope video request: %w", err)
	}
	return strings.NewReader(string(data)), nil
}

// buildWan3Media 构建 wan3.0 input.media 数组（全能参考模式）。
// 优先级：顶层 media（阿里原生入站原样透传）> metadata.image（OpenAI input_reference，
// 首帧语义对齐 Sora）> 顶层 images[]（通用多图词汇 → 参考图）。均无则返回 nil（纯文生视频）
func buildWan3Media(req map[string]any, metadata map[string]any) []dashScopeMedia {
	if raw, ok := req["media"].([]any); ok && len(raw) > 0 {
		media := make([]dashScopeMedia, 0, len(raw))
		for _, item := range raw {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			t, _ := m["type"].(string)
			u, _ := m["url"].(string)
			if t == "" || u == "" {
				continue
			}
			media = append(media, dashScopeMedia{Type: t, URL: u})
		}
		if len(media) > 0 {
			return media
		}
	}
	if img, ok := metadata["image"].(string); ok && img != "" {
		return []dashScopeMedia{{Type: "first_frame", URL: img}}
	}
	if imgs, ok := req["images"].([]any); ok && len(imgs) > 0 {
		media := make([]dashScopeMedia, 0, len(imgs))
		for _, item := range imgs {
			if u, ok := item.(string); ok && u != "" {
				media = append(media, dashScopeMedia{Type: "reference_image", URL: u})
			}
		}
		if len(media) > 0 {
			return media
		}
	}
	return nil
}

// resolutionFromSize 从尺寸字符串（如 "1280x720"）按短边映射 wan3.0 分辨率档位。
// 竖版尺寸（720x1280）同样按短边归档；解析失败返回空串（上游取默认 1080P）
func resolutionFromSize(size string) string {
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(size)), "x", 2)
	if len(parts) != 2 {
		return ""
	}
	w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return ""
	}
	short := w
	if h < short {
		short = h
	}
	switch {
	case short <= 500:
		return "480P"
	case short <= 800:
		return "720P"
	default:
		return "1080P"
	}
}

func (a *AliAdaptor) buildImageRequest(info *common.RelayInfo, req map[string]any, modelName string) (io.Reader, error) {
	dsReq := dashScopeImageRequest{
		Input:      dashScopeImageInput{},
		Parameters: make(map[string]any),
	}
	dsReq.Model = modelName

	if v, ok := req["prompt"].(string); ok {
		dsReq.Input.Prompt = v
	}
	if v, ok := req["negative_prompt"].(string); ok {
		dsReq.Input.NegativePrompt = v
	}

	// size: 1024x1024 → 1024*1024
	if v, ok := req["size"].(string); ok && v != "" {
		dsReq.Parameters["size"] = strings.ReplaceAll(v, "x", "*")
	}
	if v, ok := req["n"]; ok {
		dsReq.Parameters["n"] = v
	}
	if v, ok := req["seed"]; ok {
		dsReq.Parameters["seed"] = v
	}
	if v, ok := req["style"]; ok {
		dsReq.Parameters["style"] = v
	}
	if v, ok := req["ref_strength"]; ok {
		dsReq.Parameters["ref_strength"] = v
	}
	if v, ok := req["ref_img"]; ok {
		dsReq.Parameters["ref_img"] = v
	}

	if len(dsReq.Parameters) == 0 {
		dsReq.Parameters = nil
	}

	data, err := json.Marshal(dsReq)
	if err != nil {
		return nil, fmt.Errorf("marshal dashscope image request: %w", err)
	}
	return strings.NewReader(string(data)), nil
}

func (a *AliAdaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	url, err := a.BuildRequestURL(info)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if err := a.BuildRequestHeader(req.Header, info); err != nil {
		return nil, fmt.Errorf("setup header: %w", err)
	}
	client := common.NewPooledClient(120, info.ChannelMeta.Settings.UseProxy)
	return client.Do(req)
}

func (a *AliAdaptor) DoResponse(_ context.Context, resp *http.Response, _ *common.RelayInfo) (string, []byte, *common.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "read response failed"}
	}

	// DashScope 错误时可能在顶层返回 code + message
	var errResp dashScopeSubmitResponse
	if json.Unmarshal(body, &errResp) == nil && errResp.Code != "" {
		return "", body, &common.TaskError{
			StatusCode: resp.StatusCode,
			Message:    errResp.Message,
			ErrCode:    errResp.Code,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return "", body, &common.TaskError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	var result dashScopeSubmitResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", body, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "parse response failed"}
	}

	if result.Output.TaskID == "" {
		if result.Output.Code != "" {
			return "", body, &common.TaskError{
				StatusCode: resp.StatusCode,
				Message:    result.Output.Message,
				ErrCode:    result.Output.Code,
			}
		}
		return "", body, &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "upstream returned empty task id"}
	}

	return result.Output.TaskID, body, nil
}

func (a *AliAdaptor) FetchTask(ctx context.Context, baseURL, apiKey string, taskData []byte) (*http.Response, error) {
	var data struct {
		TaskID   string `json:"task_id"`
		UseProxy bool   `json:"use_proxy"`
	}
	if err := json.Unmarshal(taskData, &data); err != nil {
		return nil, fmt.Errorf("ali: invalid task data: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/tasks/%s", strings.TrimRight(baseURL, "/"), data.TaskID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := common.NewPooledClient(30, data.UseProxy)
	return client.Do(req)
}

func (a *AliAdaptor) ParseTaskResult(body []byte) (*common.TaskInfo, error) {
	var resp dashScopeTaskResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("ali: parse task result: %w", err)
	}

	info := &common.TaskInfo{Data: body}

	switch resp.Output.TaskStatus {
	case "PENDING":
		info.Status = common.TaskStatusSubmitted
		info.Progress = "10%"
	case "RUNNING":
		info.Status = common.TaskStatusInProgress
		info.Progress = "50%"
	case "SUCCEEDED":
		info.Status = common.TaskStatusSuccess
		info.Progress = "100%"
		// 视频结果
		if resp.Output.VideoURL != "" {
			info.ResultURL = resp.Output.VideoURL
		}
		// 素材计量（wan3.0 按秒计费）：实际输出秒数 + 实际分辨率，
		// 供 generic per_second 结算按「实际秒数 × 实际档位单价」多退少补（方案 A）
		if resp.Usage.OutputVideoDuration > 0 {
			info.MaterialUsage = &common.TaskMaterialUsage{
				OutputSeconds: resp.Usage.OutputVideoDuration,
				Resolution:    resolutionFromSR(resp.Usage.SR),
			}
		}
		// 图片结果
		if len(resp.Output.Results) > 0 {
			info.ResultURL = resp.Output.Results[0].URL
			for i, r := range resp.Output.Results {
				info.SubTasks = append(info.SubTasks, common.SubTask{
					Index:     i,
					ResultURL: r.URL,
				})
			}
		}
	case "FAILED":
		info.Status = common.TaskStatusFailure
		info.FailReason = resp.Output.Message
	case "CANCELED", "UNKNOWN":
		info.Status = common.TaskStatusFailure
		info.FailReason = fmt.Sprintf("task status: %s", resp.Output.TaskStatus)
	default:
		info.Status = common.TaskStatusSubmitted
		info.Progress = "10%"
	}

	return info, nil
}

func (a *AliAdaptor) GetModelList() []string {
	return ModelList
}

func (a *AliAdaptor) GetChannelName() string {
	return channelName
}

// ==================== 辅助函数 ====================

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// resolutionFromSR 上游 usage.SR（分辨率短边整数，如 720）→ per_second 矩阵键（"720P"）。
// 0/异常值返回空串（结算查价回退 ratios 的 spec.resolution）
func resolutionFromSR(sr int) string {
	if sr <= 0 {
		return ""
	}
	return strconv.Itoa(sr) + "P"
}
