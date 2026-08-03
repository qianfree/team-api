package types

type FileType string

const (
	FileTypeImage FileType = "image" // 图片文件类型
	FileTypeAudio FileType = "audio" // 音频文件类型
	FileTypeVideo FileType = "video" // 视频文件类型
	FileTypeFile  FileType = "file"  // 通用文件类型
)

type TokenType string

const (
	TokenTypeTextNumber TokenType = "text_number" // 文本或数字 token
	TokenTypeTokenizer  TokenType = "tokenizer"   // Tokenizer token
	TokenTypeImage      TokenType = "image"       // 图片 token
)

type TokenCountMeta struct {
	TokenType     TokenType   `json:"token_type,omitempty"`     // 请求中使用的 token 类型
	CombineText   string      `json:"combine_text,omitempty"`   // 所有消息合并后的文本
	ToolsCount    int         `json:"tools_count,omitempty"`    // 使用的工具数量
	NameCount     int         `json:"name_count,omitempty"`     // 请求中 name 的数量
	MessagesCount int         `json:"messages_count,omitempty"` // 请求中的消息数量
	Files         []*FileMeta `json:"files,omitempty"`          // 文件列表，每个文件包含类型和内容
	MaxTokens     int         `json:"max_tokens,omitempty"`     // 请求允许的最大 token 数

	ImagePriceRatio float64            `json:"image_ratio,omitempty"`    // 图片尺寸的比率（如适用）
	BillingRatios   map[string]float64 `json:"billing_ratios,omitempty"` // 预扣计费使用的已校验请求乘数
	//IsStreaming   bool        `json:"is_streaming,omitempty"`   // Indicates if the request is streaming
}

type FileMeta struct {
	FileType
	Source FileSource // 统一的文件来源（URL 或 base64）
	Detail string     // 图片细节级别（low/high/auto）
}

// NewFileMeta 创建新的 FileMeta
func NewFileMeta(fileType FileType, source FileSource) *FileMeta {
	return &FileMeta{
		FileType: fileType,
		Source:   source,
	}
}

// NewImageFileMeta 创建图片类型的 FileMeta
func NewImageFileMeta(source FileSource, detail string) *FileMeta {
	return &FileMeta{
		FileType: FileTypeImage,
		Source:   source,
		Detail:   detail,
	}
}

// GetIdentifier 获取文件标识符（用于日志）
func (f *FileMeta) GetIdentifier() string {
	if f.Source != nil {
		return f.Source.GetIdentifier()
	}
	return "unknown"
}

// IsURL 判断是否是 URL 来源
func (f *FileMeta) IsURL() bool {
	return f.Source != nil && f.Source.IsURL()
}

// GetRawData 获取原始数据（兼容旧代码）
// Deprecated: 请使用 Source.GetRawData()
func (f *FileMeta) GetRawData() string {
	if f.Source != nil {
		return f.Source.GetRawData()
	}
	return ""
}

type RequestMeta struct {
	OriginalModelName string `json:"original_model_name"`
	UserUsingGroup    string `json:"user_using_group"`
	PromptTokens      int    `json:"prompt_tokens"`
	PreConsumedQuota  int    `json:"pre_consumed_quota"`
}
