// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// AudAiAnalysisJobs is the golang structure for table aud_ai_analysis_jobs.
type AudAiAnalysisJobs struct {
	Id           int64       `json:"id"             orm:"id"             description:""`                                    //
	ReportId     int64       `json:"report_id"      orm:"report_id"      description:"完成后回填的报告ID"`                          // 完成后回填的报告ID
	TenantId     int64       `json:"tenant_id"      orm:"tenant_id"      description:"分析目标租户ID"`                            // 分析目标租户ID
	TargetUserId int64       `json:"target_user_id" orm:"target_user_id" description:"分析目标用户ID（0=租户全员）"`                    // 分析目标用户ID（0=租户全员）
	Source       string      `json:"source"         orm:"source"         description:"任务来源：manual=手动 / patrol=定时巡查"`        // 任务来源：manual=手动 / patrol=定时巡查
	TimeStart    *gtime.Time `json:"time_start"     orm:"time_start"     description:"分析窗口起点"`                              // 分析窗口起点
	TimeEnd      *gtime.Time `json:"time_end"       orm:"time_end"       description:"分析窗口终点"`                              // 分析窗口终点
	Status       string      `json:"status"         orm:"status"         description:"状态：pending/running/completed/failed"` // 状态：pending/running/completed/failed
	Progress     int         `json:"progress"       orm:"progress"       description:"进度百分比（已处理单元批次/总批次）"`                  // 进度百分比（已处理单元批次/总批次）
	ErrorMsg     string      `json:"error_msg"      orm:"error_msg"      description:"失败原因"`                                // 失败原因
	CreatedBy    int64       `json:"created_by"     orm:"created_by"     description:"触发人管理员ID（patrol 为 0）"`                // 触发人管理员ID（patrol 为 0）
	CreatedAt    *gtime.Time `json:"created_at"     orm:"created_at"     description:""`                                    //
	UpdatedAt    *gtime.Time `json:"updated_at"     orm:"updated_at"     description:""`                                    //
}
