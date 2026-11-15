// Package aianalysis AI请求分析：基于请求审计日志与会话关联，用独立配置的分析 AI
// （OpenAI 兼容端点，建议内网自部署）按会话单元分析用户 AI 使用行为的合规风险与
// 数据泄露风险。原 ai-req-analyze 插件（1.0.0）转为系统原生功能。
//
// 组成：sampler（单元采样）→ analyzer（物化/调 AI/归聚落库）→ jobrunner（异步任务）→
// patrol（定时巡查与保留期清理）。配置存 sys_options（ai_analysis_* 键，见
// common/settings_registry.go），API 门面在 internal/logic/admin/ai_analysis.go。
package aianalysis
