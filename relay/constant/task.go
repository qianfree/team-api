package constant

import "strings"

// TaskPlatform 异步任务平台类型
type TaskPlatform string

const (
	TaskPlatformSora       TaskPlatform = "sora"
	TaskPlatformKling      TaskPlatform = "kling"
	TaskPlatformSuno       TaskPlatform = "suno"
	TaskPlatformMidjourney TaskPlatform = "midjourney"
	TaskPlatformVolcengine TaskPlatform = "volcengine"
	TaskPlatformAli        TaskPlatform = "ali"
	TaskPlatformGemini     TaskPlatform = "gemini"

	// TaskPlatformSyncImage 同步图片厂商异步化专用平台标记。
	// 注意：故意不加入 ProviderTypeToTaskPlatform —— 它由 sync_image worker 池执行，
	// 无上游任务 ID 可轮询，polling 轮询分支需跳过它。
	TaskPlatformSyncImage TaskPlatform = "sync_image"
)

// TaskAction 异步任务动作类型
type TaskAction string

const (
	TaskActionGenerate TaskAction = "generate" // 视频生成
	TaskActionMusic    TaskAction = "music"    // 音乐生成
	TaskActionLyrics   TaskAction = "lyrics"   // 歌词生成

	// Midjourney 动作
	TaskActionImagine       TaskAction = "imagine"
	TaskActionDescribe      TaskAction = "describe"
	TaskActionBlend         TaskAction = "blend"
	TaskActionUpscale       TaskAction = "upscale"
	TaskActionVariation     TaskAction = "variation"
	TaskActionReroll        TaskAction = "reroll"
	TaskActionInpaint       TaskAction = "inpaint"
	TaskActionModal         TaskAction = "modal"
	TaskActionZoom          TaskAction = "zoom"
	TaskActionCustomZoom    TaskAction = "custom_zoom"
	TaskActionShorten       TaskAction = "shorten"
	TaskActionHighVariation TaskAction = "high_variation"
	TaskActionLowVariation  TaskAction = "low_variation"
	TaskActionPan           TaskAction = "pan"
	TaskActionSwapFace      TaskAction = "swap_face"
	TaskActionUpload        TaskAction = "upload"
	TaskActionVideo         TaskAction = "video"
	TaskActionEdits         TaskAction = "edits"
)

// MjActions 合法的 Midjourney 动作集合
var MjActions = map[TaskAction]bool{
	TaskActionImagine: true, TaskActionDescribe: true, TaskActionBlend: true,
	TaskActionUpscale: true, TaskActionVariation: true, TaskActionReroll: true,
	TaskActionInpaint: true, TaskActionModal: true, TaskActionZoom: true,
	TaskActionCustomZoom: true, TaskActionShorten: true,
	TaskActionHighVariation: true, TaskActionLowVariation: true,
	TaskActionPan: true, TaskActionSwapFace: true, TaskActionUpload: true,
	TaskActionVideo: true, TaskActionEdits: true,
}

// TaskStatus 异步任务状态
type TaskStatus string

const (
	TaskStatusNotStart   TaskStatus = "NOT_START"
	TaskStatusSubmitted  TaskStatus = "SUBMITTED"
	TaskStatusQueued     TaskStatus = "QUEUED"
	TaskStatusInProgress TaskStatus = "IN_PROGRESS"
	TaskStatusSuccess    TaskStatus = "SUCCESS"
	TaskStatusFailure    TaskStatus = "FAILURE"
)

// IsTerminal 判断任务状态是否为终态
func (s TaskStatus) IsTerminal() bool {
	return s == TaskStatusSuccess || s == TaskStatusFailure
}

// ProviderTypeToTaskPlatform 将供应商类型映射到异步任务平台
func ProviderTypeToTaskPlatform(p ProviderType) (TaskPlatform, bool) {
	switch p {
	case ProviderSora:
		return TaskPlatformSora, true
	case ProviderKling:
		return TaskPlatformKling, true
	case ProviderSuno:
		return TaskPlatformSuno, true
	case ProviderMidjourney:
		return TaskPlatformMidjourney, true
	case ProviderVolcengine:
		return TaskPlatformVolcengine, true
	case ProviderAli:
		return TaskPlatformAli, true
	case ProviderGemini:
		return TaskPlatformGemini, true
	default:
		return "", false
	}
}

// IsAsyncImageModel 判断某 provider + 模型的图片生成（RelayModeImagesGenerations）是否为
// 异步任务式（提交拿 task_id → 轮询取图）。此类模型的同步 /v1/images/generations 端点无法
// 一次性返回，必须走 /v1/images/generations/async 提交 + 轮询。
//
// 这是「图片必须走异步」的唯一判定源：同步端点拦截 gate（relay_handler）与租户模型列表的
// async_image 标记都调用它，保证在线体验示例与后端实际行为一致。
//
// 目前仅阿里云 DashScope 图片模型分两族：
//   - 异步族（wanx*、qwen-image、qwen-image-plus、flux* 等）：text2image/image-synthesis + 轮询
//   - 同步 multimodal 族（qwen-image-2.x 系列）：multimodal-generation/generation，一次性同步返回
//
// 故对 Ali 默认异步，qwen-image-2.x 系列除外。其他 provider 图片均为同步（含 Gemini Imagen）。
func IsAsyncImageModel(p ProviderType, model string) bool {
	if p != ProviderAli {
		return false
	}
	return !IsAliSyncMultimodalImageModel(model)
}

// IsAliSyncMultimodalImageModel 判断是否为阿里云 DashScope「multimodal messages」协议图片模型。
// 这类模型走同步 /api/v1/services/aigc/multimodal-generation/generation 端点，请求体为
// input.messages 格式、响应从 output.choices[].message.content[].image 取图；不走旧版
// text2image/image-synthesis（input.prompt）端点。本项目对它们统一用同步 multimodal 处理
// （同步端点直出 / 异步端点交 sync_image worker 池同步执行）。目前包含：
//   - qwen-image-2.x 系列（qwen-image-2.0 / qwen-image-2.0-pro 等）
//   - z-image 系列（z-image-turbo 等）
//   - 万相新版协议图片模型：wan2.6-t2i、wan2.7-image / wan2.7-image-pro
//
// 注意：万相旧版图片模型（wan2.5-t2i-preview、wan2.2-t2i-*、wanx2.x-t2i-*）走旧版
// image-synthesis（input.prompt）异步协议，不在此列，由 taskchannel 的 buildImageRequest 处理。
//
// 该判定是「Ali multimodal 图片」的唯一真相源——同步端点拦截 gate、canPassThrough、
// isMultimodalImageMode、异步端点 worker 池路由、租户模型列表 async_image 标记均调用它，
// 改这一处即全链路生效。
func IsAliSyncMultimodalImageModel(model string) bool {
	m := strings.ToLower(model)
	return strings.HasPrefix(m, "qwen-image-2") ||
		strings.HasPrefix(m, "z-image") ||
		strings.HasPrefix(m, "wan2.6-t2i") ||
		strings.HasPrefix(m, "wan2.7-image")
}

// RelayModeToTaskAction 将 RelayMode 映射到异步任务动作
func RelayModeToTaskAction(mode RelayMode) (TaskAction, bool) {
	switch mode {
	case RelayModeVideoGenerations:
		return TaskActionGenerate, true
	case RelayModeSunoSubmit:
		return TaskActionMusic, true
	case RelayModeMjSubmit:
		return TaskActionImagine, true // 默认 imagine，具体由 URL action 覆盖
	default:
		return "", false
	}
}
