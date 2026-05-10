package constant

// TaskPlatform 异步任务平台类型
type TaskPlatform string

const (
	TaskPlatformSora       TaskPlatform = "sora"
	TaskPlatformKling      TaskPlatform = "kling"
	TaskPlatformSuno       TaskPlatform = "suno"
	TaskPlatformMidjourney TaskPlatform = "midjourney"
	TaskPlatformVolcengine TaskPlatform = "volcengine"
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
	default:
		return "", false
	}
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
