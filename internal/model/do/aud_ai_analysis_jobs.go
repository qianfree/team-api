// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// AudAiAnalysisJobs is the golang structure of table aud_ai_analysis_jobs for DAO operations like Where/Data.
type AudAiAnalysisJobs struct {
	g.Meta       `orm:"table:aud_ai_analysis_jobs, do:true"`
	Id           any         //
	ReportId     any         // 完成后回填的报告ID
	TenantId     any         // 分析目标租户ID
	TargetUserId any         // 分析目标用户ID（0=租户全员）
	Source       any         // 任务来源：manual=手动 / patrol=定时巡查
	TimeStart    *gtime.Time // 分析窗口起点
	TimeEnd      *gtime.Time // 分析窗口终点
	Status       any         // 状态：pending/running/completed/failed
	Progress     any         // 进度百分比（已处理单元批次/总批次）
	ErrorMsg     any         // 失败原因
	CreatedBy    any         // 触发人管理员ID（patrol 为 0）
	CreatedAt    *gtime.Time //
	UpdatedAt    *gtime.Time //
}
