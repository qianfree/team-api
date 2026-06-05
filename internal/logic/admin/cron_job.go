package admin

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/logic/common"
)

// CronJobList returns all registered cron jobs with their last execution status.
func (s *sAdmin) CronJobList(ctx context.Context, _ *v1.CronJobListReq) (*v1.CronJobListRes, error) {
	jobs := common.GetCronScheduler().ListJobs()

	if len(jobs) == 0 {
		return &v1.CronJobListRes{List: []v1.CronJobItem{}}, nil
	}

	// Query sys_cron_jobs directly — one row per job, no aggregation needed
	rows, err := g.DB().Ctx(ctx).Query(ctx,
		`SELECT job_name, last_status, last_started_at, last_duration_ms,
		        last_error_message, total_runs, total_failures
		 FROM sys_cron_jobs`,
	)
	if err != nil {
		return nil, err
	}
	jobMap := make(map[string]map[string]string, len(rows))
	for _, row := range rows {
		m := make(map[string]string)
		for k, v := range row.Map() {
			m[k] = gconv.String(v)
		}
		jobMap[row["job_name"].String()] = m
	}

	items := make([]v1.CronJobItem, 0, len(jobs))
	for _, j := range jobs {
		item := v1.CronJobItem{
			Name:      j.Name,
			Schedule:  j.Schedule,
			IsRunning: j.IsRunning,
		}

		if data, ok := jobMap[j.Name]; ok {
			item.LastStatus = data["last_status"]
			item.LastStartedAt = data["last_started_at"]
			item.LastDurationMs = gconv.Int(data["last_duration_ms"])
			item.LastErrorMsg = data["last_error_message"]
			item.TotalExecs = gconv.Int(data["total_runs"])
			item.TotalFailures = gconv.Int(data["total_failures"])
		}

		items = append(items, item)
	}

	return &v1.CronJobListRes{List: items}, nil
}

// CronJobTrigger manually triggers a cron job.
func (s *sAdmin) CronJobTrigger(ctx context.Context, req *v1.CronJobTriggerReq) (*v1.CronJobTriggerRes, error) {
	err := common.GetCronScheduler().TriggerJob(ctx, req.Name)
	if err != nil {
		return nil, gerror.Wrapf(err, "触发定时任务失败")
	}
	return &v1.CronJobTriggerRes{Message: "任务已触发"}, nil
}
