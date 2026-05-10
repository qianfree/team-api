package admin

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/logic/common"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
)

// CronJobList returns all registered cron jobs with their last execution status.
func (s *sAdmin) CronJobList(ctx context.Context, _ *v1.CronJobListReq) (*v1.CronJobListRes, error) {
	jobs := common.GetCronScheduler().ListJobs()

	if len(jobs) == 0 {
		return &v1.CronJobListRes{List: []v1.CronJobItem{}}, nil
	}

	// Get latest execution per job
	latestRows, err := g.DB().Ctx(ctx).Query(ctx, `
		SELECT DISTINCT ON (job_name)
			job_name, status AS last_status, started_at AS last_started_at,
			duration_ms AS last_duration_ms, error_message AS last_error_msg
		FROM sys_cron_job_executions
		ORDER BY job_name, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	latestMap := make(map[string]map[string]string, len(latestRows))
	for _, row := range latestRows {
		m := make(map[string]string)
		for k, v := range row.Map() {
			m[k] = gconv.String(v)
		}
		latestMap[row["job_name"].String()] = m
	}

	// Get counts per job
	countRows, err := g.DB().Ctx(ctx).Query(ctx, `
		SELECT job_name,
			COUNT(*) AS total_execs,
			COUNT(*) FILTER (WHERE status = 'failed') AS total_failures
		FROM sys_cron_job_executions
		GROUP BY job_name
	`)
	if err != nil {
		return nil, err
	}
	countMap := make(map[string]map[string]int, len(countRows))
	for _, row := range countRows {
		m := make(map[string]int)
		for k, v := range row.Map() {
			m[k] = gconv.Int(v)
		}
		countMap[row["job_name"].String()] = m
	}

	items := make([]v1.CronJobItem, 0, len(jobs))
	for _, j := range jobs {
		item := v1.CronJobItem{
			Name:      j.Name,
			Schedule:  j.Schedule,
			IsRunning: j.IsRunning,
		}

		if latest, ok := latestMap[j.Name]; ok {
			item.LastStatus = latest["last_status"]
			item.LastStartedAt = latest["last_started_at"]
			item.LastDurationMs = gconv.Int(latest["last_duration_ms"])
			item.LastErrorMsg = latest["last_error_msg"]
		}

		if counts, ok := countMap[j.Name]; ok {
			item.TotalExecs = counts["total_execs"]
			item.TotalFailures = counts["total_failures"]
		}

		items = append(items, item)
	}

	return &v1.CronJobListRes{List: items}, nil
}

// CronJobExecutions returns paginated execution history for a specific job.
func (s *sAdmin) CronJobExecutions(ctx context.Context, req *v1.CronJobExecutionsReq) (*v1.CronJobExecutionsRes, error) {
	var conditions []string
	var args []any

	conditions = append(conditions, "job_name = ?")
	args = append(args, req.Name)

	if req.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, req.Status)
	}

	where := strings.Join(conditions, " AND ")
	list, total, err := queryPage(ctx,
		"sys_cron_job_executions",
		"id, job_name, status, started_at, finished_at, duration_ms, error_message, triggered_by, created_at",
		where, "created_at DESC",
		req.Page, req.PageSize, args...)
	if err != nil {
		return nil, err
	}

	return &v1.CronJobExecutionsRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// CronJobTrigger manually triggers a cron job.
func (s *sAdmin) CronJobTrigger(ctx context.Context, req *v1.CronJobTriggerReq) (*v1.CronJobTriggerRes, error) {
	err := common.GetCronScheduler().TriggerJob(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("触发任务失败: %v", err)
	}
	return &v1.CronJobTriggerRes{Message: "任务已触发"}, nil
}
