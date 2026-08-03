-- wrk 压测脚本：对 /v1/chat/completions（OpenAI 兼容代理端点）发压。
--
-- 用法：
--   RELAY_API_KEY=sk-xxx RELAY_MODEL=claude-3-5-sonnet \
--     wrk -t4 -c100 -d30s --latency -s scripts/wrk_relay.lua http://localhost:8080
--
-- 环境变量：
--   RELAY_API_KEY  必填，租户 API Key（Bearer）
--   RELAY_MODEL    目标模型名（默认 gpt-4）；切换模型即切换上游供应商，
--                  从而压测不同 relaykit 转换路径（claude/gemini/coze/dify/ollama）
--   RELAY_STREAM   设为 1 时启用流式（stream:true）
--
-- 对照 relaykit 新旧路径：分别在 manifest/config 的 relaykit.enabled=false / true
-- （并按需配置 providers 白名单）重启服务后各压一次，比较 QPS / 延迟 / 错误率。

local model    = os.getenv("RELAY_MODEL") or "gpt-4"
local apiKey   = os.getenv("RELAY_API_KEY") or "sk-test"
local stream   = os.getenv("RELAY_STREAM") == "1"

wrk.method  = "POST"
wrk.path    = "/v1/chat/completions"
wrk.headers["Content-Type"]  = "application/json"
wrk.headers["Authorization"] = "Bearer " .. apiKey

-- 固定一段短 prompt，控制单请求成本，聚焦转换层开销而非模型推理耗时。
local template = [[{"model":"%s","max_tokens":16,"stream":%s,"messages":[{"role":"user","content":"Reply with the single word: ok"}]}]]

request = function()
	wrk.body = string.format(template, model, stream and "true" or "false")
	return wrk.format()
end
