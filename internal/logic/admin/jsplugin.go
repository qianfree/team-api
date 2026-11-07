package admin

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/jsplugin"
	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// JS 插件管理：logic 薄转发到 jsplugin 注册中心（生命周期与 generation 见 registry.go）。

func (s *sAdmin) JSPluginList(ctx context.Context, _ *v1.JSPluginListReq) (*v1.JSPluginListRes, error) {
	summaries, err := jsplugin.DefaultRegistry().List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]v1.JSPluginItem, 0, len(summaries))
	for _, sm := range summaries {
		usageSchema := make([]v1.JSPluginUsageField, 0, len(sm.UsageSchema))
		for _, f := range sm.UsageSchema {
			usageSchema = append(usageSchema, v1.JSPluginUsageField{
				Key: f.Key, Type: f.Type, Unit: f.Unit, Description: f.Description,
			})
		}
		item := v1.JSPluginItem{
			Key:           sm.Key,
			Name:          sm.Name,
			Icon:          sm.Icon,
			HasImage:      sm.HasImage,
			Description:   sm.Description,
			Author:        sm.Author,
			Website:       sm.Website,
			Origin:        sm.Origin,
			ActiveVersion: sm.ActiveVer,
			Enabled:       sm.Enabled,
			InService:     sm.InService,
			Models:        sm.Models,
			FetchMode:     sm.FetchMode,
			AllowedHosts:  sm.AllowedHosts,
			UsageSchema:   usageSchema,
			Manifest:      sm.Manifest,
			Versions:      make([]v1.JSPluginVersionItem, 0, len(sm.Versions)),
		}
		for _, ver := range sm.Versions {
			created := ""
			if ver.CreatedAt != nil {
				created = ver.CreatedAt.Format("2006-01-02 15:04:05")
			}
			item.Versions = append(item.Versions, v1.JSPluginVersionItem{
				ID:        ver.Id,
				Version:   ver.Version,
				Origin:    ver.Origin,
				Active:    ver.Active,
				Enabled:   ver.Enabled,
				Remark:    ver.Remark,
				CreatedAt: created,
			})
		}
		out = append(out, item)
	}
	return &v1.JSPluginListRes{List: out}, nil
}

func (s *sAdmin) JSPluginUpload(ctx context.Context, req *v1.JSPluginUploadReq) (*v1.JSPluginUploadRes, error) {
	manifest, err := jsplugin.DefaultRegistry().Upload(ctx, req.Source, req.Icon, req.Remark)
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(manifest)
	return &v1.JSPluginUploadRes{Manifest: data}, nil
}

// JSPluginDryRun 上传前试运行：编译 / 钩子齐备性 / 跨插件冲突预检 / 样例执行 /
// 静态 lint（见 jsplugin.DryRun）。不落库、不发布 generation、不发真实 HTTP，
// 所有失败都体现在检查项里，接口本身始终成功返回。
func (s *sAdmin) JSPluginDryRun(_ context.Context, req *v1.JSPluginDryRunReq) (*v1.JSPluginDryRunRes, error) {
	report := jsplugin.DryRun(req.Source)
	res := &v1.JSPluginDryRunRes{
		Checks: make([]v1.JSPluginDryRunCheck, 0, len(report.Checks)),
		Pass:   !report.HasFail(),
	}
	if report.Manifest != nil {
		m := report.Manifest
		res.Key = m.Key
		res.Name = m.Name
		res.Version = m.Version
		res.Models = m.Models
		res.FetchMode = m.EffectiveFetchMode()
		res.Routes = make([]v1.JSPluginRouteItem, 0, len(m.Routes))
		for _, route := range m.Routes {
			res.Routes = append(res.Routes, v1.JSPluginRouteItem{
				Method: route.Method, Path: route.Path, Type: string(route.Type),
			})
		}
	}
	for _, c := range report.Checks {
		res.Checks = append(res.Checks, v1.JSPluginDryRunCheck{Name: c.Name, Status: c.Status, Detail: c.Detail})
	}
	return res, nil
}

// JSPluginTemplate 模板下载：内置 golden 参考实现源码（作者抄它起步，删掉不需要的钩子）。
func (s *sAdmin) JSPluginTemplate(_ context.Context, _ *v1.JSPluginTemplateReq) (*v1.JSPluginTemplateRes, error) {
	return &v1.JSPluginTemplateRes{Source: jsplugin.TemplateSource(), FileName: "plugin.template.js"}, nil
}

func (s *sAdmin) JSPluginActivate(ctx context.Context, req *v1.JSPluginActivateReq) (*v1.JSPluginActivateRes, error) {
	if err := jsplugin.DefaultRegistry().Activate(ctx, req.Key, req.Version); err != nil {
		return nil, err
	}
	return &v1.JSPluginActivateRes{}, nil
}

func (s *sAdmin) JSPluginEnable(ctx context.Context, req *v1.JSPluginEnableReq) (*v1.JSPluginEnableRes, error) {
	if err := jsplugin.DefaultRegistry().Enable(ctx, req.Key, req.Enabled); err != nil {
		return nil, err
	}
	return &v1.JSPluginEnableRes{}, nil
}

func (s *sAdmin) JSPluginDelete(ctx context.Context, req *v1.JSPluginDeleteReq) (*v1.JSPluginDeleteRes, error) {
	if err := jsplugin.DefaultRegistry().DeleteVersion(ctx, req.Key, req.Version); err != nil {
		return nil, err
	}
	return &v1.JSPluginDeleteRes{}, nil
}

// JSPluginSource 查看指定版本源码（只读，内置/上传版本均可）。
func (s *sAdmin) JSPluginSource(ctx context.Context, req *v1.JSPluginSourceReq) (*v1.JSPluginSourceRes, error) {
	row, err := jsplugin.DefaultRegistry().VersionSource(ctx, req.Key, req.Version)
	if err != nil {
		return nil, err
	}
	created := ""
	if row.CreatedAt != nil {
		created = row.CreatedAt.Format("2006-01-02 15:04:05")
	}
	return &v1.JSPluginSourceRes{
		Key:        row.PluginKey,
		Version:    row.Version,
		Origin:     row.Origin,
		Source:     row.Source,
		SourceHash: row.SourceHash,
		Remark:     row.Remark,
		CreatedAt:  created,
	}, nil
}

// JSPluginRoutes 路由预览取激活版本 manifest（上传/内置 seed 时从 plugin.js 解析落库），
// 不依赖当前 generation——禁用状态下也要能看见端点信息（启用后按同一路径挂载）。
func (s *sAdmin) JSPluginRoutes(ctx context.Context, req *v1.JSPluginRoutesReq) (*v1.JSPluginRoutesRes, error) {
	manifest, err := jsplugin.DefaultRegistry().ActiveManifest(ctx, req.Key)
	if err != nil {
		// DB 错误如实上抛（区别于「插件不存在」——那不是错误，返回空列表）
		return nil, err
	}
	if manifest == nil {
		return &v1.JSPluginRoutesRes{List: []v1.JSPluginRouteItem{}}, nil
	}
	out := make([]v1.JSPluginRouteItem, 0, len(manifest.Routes))
	for _, route := range manifest.Routes {
		out = append(out, v1.JSPluginRouteItem{
			Method: route.Method,
			Path:   "/pjs/" + req.Key + contract.NormalizePathShape(route.Path),
			Type:   string(route.Type),
		})
	}
	return &v1.JSPluginRoutesRes{List: out}, nil
}

// JSPluginMetrics 运行指标合流：内存钩子指标（自进程启动）+ DB 业务统计
// （任务量按 platform=js:{key}、产物转存按 js_task_artifacts.plugin_key）。
func (s *sAdmin) JSPluginMetrics(ctx context.Context, _ *v1.JSPluginMetricsReq) (*v1.JSPluginMetricsRes, error) {
	snap := jsplugin.MetricsSnapshot()
	out := make([]v1.JSPluginMetricsItem, 0, len(snap))
	for _, m := range snap {
		item := v1.JSPluginMetricsItem{
			Key: m.Key, InService: m.InService, LiveInstances: m.LiveInstances,
			TotalCalls: m.TotalCalls, TotalErrors: m.TotalErrors, AvgMs: m.AvgMs,
			Hooks: make(map[string]*v1.JSPluginMetricsHook, len(m.Hooks)),
		}
		for name, h := range m.Hooks {
			item.Hooks[name] = &v1.JSPluginMetricsHook{
				Calls: h.Calls, Errors: h.Errors, ErrorRate: h.ErrorRate,
				AvgMs: h.AvgMs, LastErr: h.LastErr, LastErrAt: h.LastErrAt,
			}
		}
		// 任务量统计（platform 维度）——统计失败不阻断指标返回，但必须留痕
		platform := "js:" + m.Key
		taskRows, err := g.DB().Model("tsk_model_tasks").Ctx(ctx).
			Where("platform", platform).Fields("status, count(*) as cnt").Group("status").All()
		if err != nil {
			g.Log().Warningf(ctx, "jsplugin metrics task stats %s: %v", m.Key, err)
		} else {
			for _, row := range taskRows {
				cnt := row["cnt"].Int64()
				switch row["status"].String() {
				case "SUCCESS":
					item.TasksSuccess = cnt
				case "FAILURE":
					item.TasksFailed = cnt
				default:
					item.TasksActive += cnt
				}
				item.TasksTotal += cnt
			}
		}
		// 产物转存统计
		artRows, err := g.DB().Model("js_task_artifacts").Ctx(ctx).
			Where("plugin_key", m.Key).Fields("status, count(*) as cnt").Group("status").All()
		if err != nil {
			g.Log().Warningf(ctx, "jsplugin metrics artifact stats %s: %v", m.Key, err)
		} else {
			for _, row := range artRows {
				switch row["status"].String() {
				case "stored":
					item.ArtifactsStored = row["cnt"].Int64()
				case "failed":
					item.ArtifactsFailed = row["cnt"].Int64()
				}
			}
		}
		out = append(out, item)
	}
	return &v1.JSPluginMetricsRes{Since: jsplugin.MetricsSince().Format("2006-01-02 15:04:05"), List: out}, nil
}
