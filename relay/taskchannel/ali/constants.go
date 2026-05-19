package ali

const channelName = "Ali"

// ModelList 阿里云 DashScope 支持异步生成的模型列表（图片 + 视频）
var ModelList = []string{
	// 视频生成
	"wan2.7-t2v-2026-04-25",
	"wan2.6-t2v",
	"wan2.6-t2v-us",
	"wan2.5-t2v-preview",
	"wan2.2-t2v-plus",
	"wanx2.1-t2v-turbo",
	"wanx2.1-t2v-plus",
	// 图片生成
	"wanx-v1",
	"wanx-v2",
	"wanx2.1-t2i-ediff",
	"wanx-sketch-to-image-v2",
	"wanx-image-inpainting",
	"wanx-style-repaint",
	"wanx2.1-image-edit",
	"wanx2.5-image-edit",
	"wanx-image-outpainting",
	"qwen-image-generation",
	"qwen-image-edit",
	"zimage",
	"flux-dev",
	"flux-schnell",
	"stable-diffusion-xl",
}
