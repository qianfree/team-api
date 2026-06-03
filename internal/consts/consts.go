package consts

import (
	"os"
	"strings"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
)

// Version can be set at build time via -ldflags "-X github.com/qianfree/team-api/internal/consts.Version=x.y.z".
// Falls back to reading the VERSION file in the project root, or "dev" if unavailable.
var Version = ""

func init() {
	if Version != "" {
		return
	}
	data, err := os.ReadFile("VERSION")
	if err != nil {
		Version = "dev"
		return
	}
	if v := strings.TrimSpace(string(data)); v != "" {
		Version = v
	} else {
		Version = "dev"
	}
}

// Business error variables (using gerror for full stack traces)
var (
	ErrUnauthorized       = gerror.NewCode(gcode.New(CodeUnauthorized, MsgUnauthorized, nil), MsgUnauthorized)
	ErrKeyExpired         = gerror.NewCode(gcode.New(CodeKeyExpired, MsgKeyExpired, nil), MsgKeyExpired)
	ErrKeyDisabled        = gerror.NewCode(gcode.New(CodeKeyDisabled, "API Key 已禁用", nil), "API Key 已禁用")
	ErrTenantSuspended    = gerror.NewCode(gcode.New(CodeTenantSuspended, MsgTenantSuspended, nil), MsgTenantSuspended)
	ErrChannelUnavailable = gerror.NewCode(gcode.New(CodeChannelUnavailable, MsgChannelUnavailable, nil), MsgChannelUnavailable)
	ErrModelDisabled      = gerror.NewCode(gcode.New(CodeModelDisabled, MsgModelDisabled, nil), MsgModelDisabled)
	ErrTotpRequired       = gerror.NewCode(gcode.New(CodeTotpRequired, MsgTotpRequired, nil), MsgTotpRequired)
	ErrTotpInvalid        = gerror.NewCode(gcode.New(CodeTotpInvalid, MsgTotpInvalid, nil), MsgTotpInvalid)
	ErrTurnstileFailed    = gerror.NewCode(gcode.New(CodeTurnstileFailed, MsgTurnstileFailed, nil), MsgTurnstileFailed)
	ErrIpRestricted       = gerror.NewCode(gcode.New(CodeIpRestricted, MsgIpRestricted, nil), MsgIpRestricted)
	ErrCaptchaRequired    = gerror.NewCode(gcode.New(CodeCaptchaRequired, MsgCaptchaRequired, nil), MsgCaptchaRequired)
	ErrCaptchaFailed      = gerror.NewCode(gcode.New(CodeCaptchaFailed, MsgCaptchaFailed, nil), MsgCaptchaFailed)
)

// Error codes for business logic
const (
	BusinessErrorCodePrefix = 10000

	// Common errors
	CodeSuccess             = 0
	CodeBadRequest          = 400
	CodeUnauthorized        = 401
	CodeForbidden           = 403
	CodeNotFound            = 404
	CodeTooManyRequests     = 429
	CodeInternalServerError = 500

	// Business errors (10000+)
	CodeInsufficientBalance = 10001
	CodeQuotaExceeded       = 10002
	CodeChannelUnavailable  = 10003
	CodeModelDisabled       = 10004
	CodeTenantSuspended     = 10005
	CodeKeyExpired          = 10006
	CodeKeyScopeDenied      = 10007
	CodeInvitationExpired   = 10008
	CodeInvitationUsed      = 10009
	CodeAccountLocked       = 10010
	CodeInvalidCredentials  = 10011
	CodeEmailVerifyExpired  = 10012
	CodeEmailVerifyUsed     = 10013
	CodeRateLimitExceeded   = 10014
	CodePasswordTooWeak     = 10015
	CodeUsernameExists      = 10016
	CodeTenantCodeExists    = 10017
	CodeSessionExpired      = 10018
	CodeTokenExpired        = 10019
	CodeTokenRevoked        = 10020
	CodeMemberLimitReached  = 10021
	CodeVerifyCodeInvalid   = 10022
	CodeOldPasswordWrong    = 10023

	// Payment errors (10024+)
	CodePaymentChannelNotFound = 10024
	CodePaymentChannelDisabled = 10025
	CodePaymentCreateFailed    = 10026
	CodePaymentCallbackFailed  = 10027
	CodePaymentInvalidConfig   = 10028
	CodePaymentBelowMinTopup   = 10029
	CodeOrderAlreadyPaid       = 10030

	// Project errors (10031+)
	CodeProjectNotFound     = 10031
	CodeProjectNotActive    = 10032
	CodeProjectKeyForbidden = 10033
	CodeConsentRequired     = 10034

	// Data governance errors (10035+)
	CodeExportAlreadyPending    = 10035
	CodeExportNotFound          = 10036
	CodeExportExpired           = 10037
	CodeDeletionRequestExists   = 10038
	CodeDeletionRequestNotFound = 10039
	CodeTaskNotRetryable        = 10040
	CodeOnboardingCompleted     = 10041

	// Status page errors (10042+)
	CodeComponentNotFound   = 10042
	CodeIncidentNotFound    = 10043
	CodeMaintenanceNotFound = 10044
	CodeSubscriptionExists  = 10045
	CodeInvalidConfirmToken = 10046

	// Security errors (10047+)
	CodeTotpRequired         = 10047
	CodeTotpInvalid          = 10048
	CodeTotpAlreadyEnabled   = 10049
	CodeTotpNotEnabled       = 10050
	CodeBackupCodeInvalid    = 10051
	CodeTurnstileFailed      = 10052
	CodeIpRestricted         = 10053
	CodeHighRisk2FARequired  = 10054
	CodeNoAvailableApiKey    = 10055
	CodeSandboxQuotaExceeded = 10056
	CodeCaptchaRequired      = 10057
	CodeCaptchaFailed        = 10058

	// OAuth errors (10059+)
	CodeOAuthDisabled      = 10059
	CodeOAuthInvalidCode   = 10060
	CodeOAuthTokenFailed   = 10061
	CodeOAuthAlreadyLinked = 10062

	// Feedback & Changelog errors (10063+)
	CodeFeedbackNotFound  = 10063
	CodeChangelogNotFound = 10064
	CodeKeyDisabled       = 10065

	// Response cache errors (10068+)
	CodeCacheDisabled = 10068
	CodeCacheFull     = 10069

	// Help Center errors (10073+)
	CodeHelpCategoryNotFound   = 10073
	CodeHelpCategorySlugExists = 10074
	CodeHelpArticleNotFound    = 10075
	CodeHelpArticleSlugExists  = 10076

	// Registration errors
	CodeRegistrationDisabled      = 10077
	CodeEmailVerificationDisabled = 10082

	// Setup errors
	CodeSetupCompleted        = 10078
	CodeSetupInvalidUsername  = 10079
	CodeSetupPasswordMismatch = 10080
	CodeSetupNotInitialized   = 10081

	// Model errors
	CodeModelNameExists = 10083

	// Model group errors
	CodeModelGroupNotFound   = 10084
	CodeModelGroupCodeExists = 10085
	CodeModelGroupHasTenants = 10086

	// Model import errors
	CodeModelImportInvalidFile = 10087
	CodeModelImportBadVersion  = 10088

	// Demo mode
	CodeDemoModeRestricted = 10403

	// Plugin errors (10404+)
	CodePluginNotFound          = 10404
	CodePluginAlreadyInstalled  = 10405
	CodePluginNotInstalled      = 10406
	CodePluginNotEnabled        = 10407
	CodePluginAlreadyEnabled    = 10408
	CodePluginDependencyMissing = 10409
	CodePluginInstallFailed     = 10410
)

// Common error messages
const (
	MsgSuccess             = "OK"
	MsgBadRequest          = "请求参数错误"
	MsgUnauthorized        = "未授权"
	MsgForbidden           = "无权限访问"
	MsgNotFound            = "资源不存在"
	MsgTooManyRequests     = "请求过于频繁"
	MsgInternalServerError = "服务器内部错误"

	MsgInsufficientBalance = "余额不足"
	MsgQuotaExceeded       = "额度已用完"
	MsgChannelUnavailable  = "没有可用的渠道"
	MsgModelDisabled       = "模型已禁用"
	MsgTenantSuspended     = "租户已被暂停"
	MsgKeyExpired          = "API Key 已过期"
	MsgKeyScopeDenied      = "API Key 无权访问此接口"
	MsgInvitationExpired   = "邀请链接已过期"
	MsgInvitationUsed      = "邀请链接已被使用"
	MsgAccountLocked       = "账号已被锁定"
	MsgInvalidCredentials  = "用户名或密码错误"
	MsgEmailVerifyExpired  = "验证码已过期"
	MsgEmailVerifyUsed     = "验证码已使用"
	MsgRateLimitExceeded   = "请求频率超限"
	MsgPasswordTooWeak     = "密码不符合策略"
	MsgUsernameExists      = "用户名已存在"
	MsgTenantCodeExists    = "租户代码已存在"
	MsgSessionExpired      = "会话已过期"
	MsgTokenExpired        = "Token 已过期"
	MsgTokenRevoked        = "Token 已被撤销"
	MsgMemberLimitReached  = "成员数已达上限"
	MsgVerifyCodeInvalid   = "验证码错误"
	MsgOldPasswordWrong    = "原密码错误"
	// Payment errors
	MsgPaymentChannelNotFound = "支付渠道不存在"
	MsgPaymentChannelDisabled = "支付渠道已禁用"
	MsgPaymentCreateFailed    = "创建支付失败"
	MsgPaymentCallbackFailed  = "支付回调处理失败"
	MsgPaymentInvalidConfig   = "支付渠道配置无效"
	MsgPaymentBelowMinTopup   = "充值金额低于最低限额"
	MsgOrderAlreadyPaid       = "订单已支付"
	// Project errors
	MsgProjectNotFound     = "项目不存在"
	MsgProjectNotActive    = "项目状态不可用"
	MsgProjectKeyForbidden = "仅管理员可管理项目密钥"
	MsgConsentRequired     = "需要同意法律条款后才能继续"
	// Data governance errors
	MsgExportAlreadyPending    = "已有导出任务进行中"
	MsgExportNotFound          = "导出任务不存在"
	MsgExportExpired           = "导出文件已过期"
	MsgDeletionRequestExists   = "已有删除请求进行中"
	MsgDeletionRequestNotFound = "删除请求不存在"
	MsgTaskNotRetryable        = "任务不可重试"
	MsgOnboardingCompleted     = "引导流程已完成"
	// Status page errors
	MsgComponentNotFound   = "组件不存在"
	MsgIncidentNotFound    = "事件不存在"
	MsgMaintenanceNotFound = "维护计划不存在"
	MsgSubscriptionExists  = "该邮箱已订阅"
	MsgInvalidConfirmToken = "确认令牌无效"
	// Security errors
	MsgTotpRequired        = "需要双因素认证"
	MsgTotpInvalid         = "验证码错误"
	MsgTotpAlreadyEnabled  = "双因素认证已启用"
	MsgTotpNotEnabled      = "未启用双因素认证"
	MsgBackupCodeInvalid   = "恢复码无效"
	MsgTurnstileFailed     = "人机验证失败"
	MsgIpRestricted        = "IP 地址不在白名单中"
	MsgHighRisk2FARequired = "高风险操作需要二次验证"
	MsgCaptchaRequired     = "请完成滑块验证"
	MsgCaptchaFailed       = "滑块验证失败"
	// OAuth errors
	MsgOAuthDisabled      = "该 OAuth 供应商未启用"
	MsgOAuthInvalidCode   = "OAuth 授权码无效"
	MsgOAuthTokenFailed   = "获取 OAuth 令牌失败"
	MsgOAuthAlreadyLinked = "该 OAuth 账号已绑定其他用户"
	// Feedback & Changelog errors
	MsgFeedbackNotFound  = "反馈不存在"
	MsgChangelogNotFound = "更新日志不存在"
	// Response cache errors
	MsgCacheDisabled = "响应缓存未启用"
	MsgCacheFull     = "缓存空间已满"
	// Help Center errors
	MsgHelpCategoryNotFound   = "帮助分类不存在"
	MsgHelpCategorySlugExists = "分类标识已存在"
	MsgHelpArticleNotFound    = "帮助文章不存在"
	MsgHelpArticleSlugExists  = "文章标识已存在"
	// Registration errors
	MsgRegistrationDisabled      = "注册功能已关闭"
	MsgEmailVerificationDisabled = "注册邮箱验证未启用"

	// Setup errors
	MsgSetupCompleted        = "系统已完成初始化"
	MsgSetupInvalidUsername  = "用户名格式无效，仅支持字母、数字和下划线，长度3-20"
	MsgSetupPasswordMismatch = "两次输入的密码不一致"
	MsgSetupNotInitialized   = "系统未初始化，请先完成设置"

	// Model errors
	MsgModelNameExists = "模型名称已存在"

	// Model group errors
	MsgModelGroupNotFound   = "模型分组不存在"
	MsgModelGroupCodeExists = "模型分组标识已存在"
	MsgModelGroupHasTenants = "该分组下存在关联租户，无法删除"
	// Model import errors
	MsgModelImportInvalidFile = "导入文件格式无效"
	MsgModelImportBadVersion  = "导入文件版本不兼容"
)

// Demo mode messages
const (
	MsgDemoModeRestricted = "演示环境，数据不可修改"
	// Plugin errors
	MsgPluginNotFound          = "插件不存在"
	MsgPluginAlreadyInstalled  = "插件已安装"
	MsgPluginNotInstalled      = "插件未安装"
	MsgPluginNotEnabled        = "插件未启用"
	MsgPluginAlreadyEnabled    = "插件已启用"
	MsgPluginDependencyMissing = "插件依赖未满足"
	MsgPluginInstallFailed     = "插件安装失败"
)
