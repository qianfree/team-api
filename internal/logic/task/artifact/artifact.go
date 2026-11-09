// Package artifact 实现 JS 任务插件的产物转存流水线：
//
//	插件声明（key + taskData 取值路径）→ 元数据登记 → [转存开启] 宿主下载/解码
//	（SSRF 防护 + 体积上限）→ FileService 上传对象存储
//
// 对外交付统一走签名内容链接（contenturl.go）：已转存的从对象存储供给，未转存
// （插件关闭转存 / 转存未完成或失败）的按声明实时回源代理（content.go）。
// task.Data 保留上游原值（inline 产物转存后清空 base64 释放存储），渲染时由
// RenderData 把声明路径替换为统一链接。
//
// 边界纪律：JS 永不接触产物字节。worker 池有界队列 + sweep 周期重试。
package artifact

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/jsplugin"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	uc "github.com/qianfree/team-api/internal/utility/crypto"
	"github.com/qianfree/team-api/internal/utility/ssrf"
	rcommon "github.com/qianfree/team-api/relay/common"
)

const (
	artifactTable    = "js_task_artifacts"
	workerCount      = 8
	queueSize        = 1000
	maxArtifactBytes = 64 << 20 // 64 MB（对齐 MAX_FILE_DOWNLOAD_MB 口径）
	downloadTimeout  = 120 * time.Second
	maxRetryCount    = 5
	sweepCronName    = "js_artifact_sweep"
	sweepCronExpr    = "*/5 * * * *" // 每 5 分钟重试 pending/failed
	storageKeyPrefix = "jsartifacts/"

	statusPending = "pending"
	statusStored  = "stored"
	statusDirect  = "direct" // 插件关闭转存：仅登记声明，内容链接回源代理
)

// storageEnabled 插件产物转存开关（宿主内置配置项 artifact_storage；测试可替换）。
var storageEnabled = jsplugin.ArtifactStorageEnabled

// transferJob 单任务的转存作业。
type transferJob struct {
	TaskID       int64
	PublicTaskID string
	TenantID     int64
	UserID       int64
	PluginKey    string
	ChannelID    int64 // 相对 URL 解析与 channel-bearer 鉴权下载用
	Artifacts    []rcommon.TaskArtifact
}

// acquireFileSvc 存储服务获取（测试可替换）。
var acquireFileSvc = func(ctx context.Context) (*lcommon.FileService, error) {
	return lcommon.NewFileServiceFromConfig(ctx)
}

// downloadClient 产物下载专用 client：SSRF 防护 transport（dial 层拦截内网/DNS rebinding）。
var downloadClient = &http.Client{
	Timeout:   downloadTimeout,
	Transport: ssrf.NewTransport(),
}

// channelDownloadClient 渠道相对 URL 下载专用 client：渠道目标与主转发链路同信任级
// （管理员配置的 baseURL 允许内网中转形态），不做 SSRF 拦截，仅保留超时与体积上限。
var channelDownloadClient = &http.Client{
	Timeout: downloadTimeout,
}

var (
	mu      sync.Mutex
	started bool
	queue   chan *transferJob
	wg      sync.WaitGroup
	stop    chan struct{}
)

// Start 启动 worker 池与 sweep 周期（cmd.go 调用，幂等）。
func Start(ctx context.Context) {
	mu.Lock()
	defer mu.Unlock()
	if started {
		return
	}
	started = true
	queue = make(chan *transferJob, queueSize)
	stop = make(chan struct{})
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go workerLoop()
	}
	// sweep：重试 pending/failed（进程重启后的恢复网 + 队列溢出兜底）
	if cs := lcommon.GetCronScheduler(); cs != nil {
		cs.Register(sweepCronName, "JS插件产物转存重试", sweepCronExpr, func(ctx context.Context) error {
			SweepPending(ctx)
			return nil
		})
	}
	g.Log().Info(ctx, "js artifact pipeline started")
}

// Stop 停止 worker 池。
func Stop() {
	mu.Lock()
	if !started {
		mu.Unlock()
		return
	}
	started = false
	close(stop)
	mu.Unlock()
	wg.Wait()
}

// EnqueueTransfer 任务终态成功后登记产物：元数据先行落表；插件开启转存时入队
// （pending，队列满时由 sweep 兜底），关闭转存时登记为 direct（不下载，内容链接回源代理）。
func EnqueueTransfer(ctx context.Context, task *rcommon.AsyncTask, artifacts []rcommon.TaskArtifact) {
	if len(artifacts) == 0 {
		return
	}
	job := &transferJob{
		TaskID:       task.ID,
		PublicTaskID: task.PublicTaskID,
		TenantID:     task.TenantID,
		UserID:       task.UserID,
		PluginKey:    strings.TrimPrefix(task.Platform, "js:"),
		ChannelID:    task.ChannelID,
		Artifacts:    artifacts,
	}
	status := statusPending
	if !storageEnabled(ctx, job.PluginKey) {
		status = statusDirect
	}
	for _, art := range artifacts {
		insertArtifactRow(ctx, job, art, status)
	}
	if status == statusDirect {
		return
	}
	mu.Lock()
	q := queue
	mu.Unlock()
	if q == nil {
		return // 尚未 Start（测试/极端时序）：行已落表，Start 后 sweep 会接手
	}
	select {
	case q <- job:
	default:
		// 队列满：行保持 pending，sweep 兜底
		g.Log().Warningf(ctx, "js artifact queue full, task %s deferred to sweep", task.PublicTaskID)
	}
}

// SweepPending 重试未完成的产物转存（最多一批）。
// 进程重启后的恢复网：行里带齐任务归属（tenant/user/channel，000032 起落库），
// 重建的 job 与 EnqueueTransfer 等价——相对 URL 产物能按 channel_id 解析渠道，
// fil_files 落正确租户，不会再产生 tenant_id=0 的孤儿文件记录。
func SweepPending(ctx context.Context) {
	mu.Lock()
	q := queue
	mu.Unlock()
	if q == nil {
		return
	}
	// 卡死的 running 行回收：下载前置 running 后进程崩溃的行，sweep 原本只捞
	// pending/failed 会永久漏掉——超时未更新的 running 归位为 failed 交还重试
	if _, err := g.DB().Model(artifactTable).Ctx(ctx).
		Where("status = ?", "running").
		Where("updated_at < now() - interval '10 minutes'").
		Data(g.Map{"status": "failed", "last_error": "running 状态超时回收（进程重启或 worker 崩溃）", "updated_at": time.Now()}).
		Update(); err != nil {
		g.Log().Warningf(ctx, "js artifact sweep running-recycle: %v", err)
	}
	// 先取去重任务 ID（PG 严格模式下列必须都在 GROUP BY，两段查询规避）；
	// 按 task_id 升序保证批次稳定，pending 超 100 时不至于随机饥饿
	taskIDs, err := g.DB().Model(artifactTable).Ctx(ctx).
		Where("status IN (?)", g.Slice{"pending", "failed"}).
		Where("retry_count < ?", maxRetryCount).
		Order("task_id ASC").Distinct().Fields("task_id").Limit(100).Array()
	if err != nil {
		g.Log().Warningf(ctx, "js artifact sweep query: %v", err)
		return
	}
	for _, taskIDVal := range taskIDs {
		taskID := taskIDVal.Int64()
		var rows []struct {
			PublicTask  string `json:"public_task_id"`
			PluginKey   string `json:"plugin_key"`
			TenantID    int64  `json:"tenant_id"`
			UserID      int64  `json:"user_id"`
			ChannelID   int64  `json:"channel_id"`
			ArtifactKey string `json:"artifact_key"`
			SourceType  string `json:"source_type"`
			SourceUrl   string `json:"source_url"`
			SourcePath  string `json:"source_path"`
			SourceAuth  string `json:"source_auth"`
			MimeType    string `json:"mime_type"`
		}
		if err := g.DB().Model(artifactTable).Ctx(ctx).
			Where("task_id", taskID).
			Where("status IN (?)", g.Slice{"pending", "failed"}).Scan(&rows); err != nil || len(rows) == 0 {
			continue
		}
		job := &transferJob{
			TaskID: taskID, PublicTaskID: rows[0].PublicTask, PluginKey: rows[0].PluginKey,
			TenantID: rows[0].TenantID, UserID: rows[0].UserID, ChannelID: rows[0].ChannelID,
		}
		for _, ar := range rows {
			// source_path 为声明原样路径；存量行（000033 之前）回落 source_url
			path := firstNonEmpty(ar.SourcePath, ar.SourceUrl)
			job.Artifacts = append(job.Artifacts, rcommon.TaskArtifact{
				Key: ar.ArtifactKey, SourceType: ar.SourceType, SourcePath: path,
				SourceAuth: ar.SourceAuth, MimeType: ar.MimeType,
			})
		}
		select {
		case q <- job:
		default:
			return
		}
	}
}

// ---------------------------------------------------------------------------
// worker
// ---------------------------------------------------------------------------

func workerLoop() {
	defer wg.Done()
	for {
		select {
		case <-stop:
			return
		case job := <-queue:
			processJob(context.Background(), job)
		}
	}
}

// processJob 处理一个任务的全部产物：逐个转存，成功的改写 Data，失败的保留原值等重试。
func processJob(ctx context.Context, job *transferJob) {
	task, err := loadTaskData(ctx, job.TaskID)
	if err != nil || task == nil {
		g.Log().Warningf(ctx, "js artifact: task %d not loadable: %v", job.TaskID, err)
		return
	}
	if task.Status != string(rcommon.TaskStatusSuccess) {
		return // 仅成功任务转存（sweep 可能捞起已回滚的行）
	}

	fs, err := acquireFileSvc(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "js artifact: storage unavailable for task %s: %v", job.PublicTaskID, err)
		return
	}

	data := map[string]any{}
	_ = json.Unmarshal(task.Data, &data)
	rewritten := false

	for _, art := range job.Artifacts {
		stored, storedErr := transferOne(ctx, fs, job, art, data)
		if storedErr != nil {
			markArtifactFailed(ctx, job.TaskID, art.Key, storedErr)
			continue
		}
		// url 型保留上游原值（回源兜底的依据；对外链接由 RenderData 渲染期替换）。
		// inline 型转存成功后清空 base64：大字段不再常驻任务表，内容改由对象存储供给
		if stored && art.SourceType == "inline" && rcommon.SetJSONPath(data, art.SourcePath, "") {
			rewritten = true
		}
	}

	if rewritten {
		newData, err := json.Marshal(data)
		if err != nil {
			g.Log().Warningf(ctx, "js artifact: marshal rewritten data for task %s: %v", job.PublicTaskID, err)
			return
		}
		if _, uErr := g.DB().Model("tsk_model_tasks").Ctx(ctx).
			Where("id", job.TaskID).Data(g.Map{"data": newData, "updated_at": time.Now()}).Update(); uErr != nil {
			g.Log().Warningf(ctx, "js artifact: persist rewritten data for task %s: %v", job.PublicTaskID, uErr)
		}
	}
}

// transferOne 转存单个产物，返回本次是否新完成转存（已 stored 的幂等跳过返回 false）。
func transferOne(ctx context.Context, fs *lcommon.FileService, job *transferJob, art rcommon.TaskArtifact, data map[string]any) (bool, error) {
	// 幂等：已 stored 直接跳过
	if stored := rcommon.GetJSONPath(data, art.SourcePath); stored != nil {
		if rowStatus := artifactStoredStatus(ctx, job.TaskID, art.Key); rowStatus == statusStored {
			return false, nil
		}
	}

	var (
		content     []byte
		contentType string
		fromChannel bool // 相对 URL 已按渠道 baseURL 解析——目标天然限定在渠道内
	)
	switch art.SourceType {
	case "inline":
		v := rcommon.GetJSONPath(data, art.SourcePath)
		s, ok := v.(string)
		if !ok || s == "" {
			return false, fmt.Errorf("inline 路径 %s 取值非字符串", art.SourcePath)
		}
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return false, fmt.Errorf("inline base64 解码失败: %v", err)
		}
		content = decoded
		contentType = art.MimeType
	case "url":
		v := rcommon.GetJSONPath(data, art.SourcePath)
		rawURL, ok := v.(string)
		if !ok || rawURL == "" {
			return false, fmt.Errorf("url 路径 %s 取值非字符串", art.SourcePath)
		}
		target, rErr := resolveSource(ctx, job.ChannelID, rawURL, art.SourceAuth)
		if rErr != nil {
			return false, rErr
		}
		rawURL, art.SourceAuth, fromChannel = target.URL, target.Auth, target.FromChannel
		body, ct, dErr := downloadArtifact(ctx, rawURL, job, art, fromChannel)
		if dErr != nil {
			return false, fmt.Errorf("下载失败: %v", dErr)
		}
		content = body
		contentType = firstNonEmpty(art.MimeType, ct)
		markArtifactSourceURL(ctx, job.TaskID, art.Key, rawURL)
	default:
		return false, fmt.Errorf("未知 source type %q", art.SourceType)
	}

	if len(content) == 0 {
		return false, fmt.Errorf("产物内容为空")
	}

	key := storageKeyPrefix + job.PublicTaskID + "/" + art.Key + extOf(firstNonEmpty(contentType, "application/octet-stream"))
	// 必须用 UploadWithKey：Upload 内部自动生成 "{日期}/{uuid}" key，构造的 key 传不进去，
	// 落库 stored_key 会指向从未上传过的路径——观测/对账/对象清理全部对不上
	_, err := fs.UploadWithKey(ctx, &lcommon.FileUpload{
		Reader:      bytes.NewReader(content),
		Filename:    art.Key + extOf(firstNonEmpty(contentType, "bin")),
		ContentType: firstNonEmpty(contentType, "application/octet-stream"),
		Size:        int64(len(content)),
		TenantID:    job.TenantID,
		UserID:      job.UserID,
	}, key)
	if err != nil {
		return false, fmt.Errorf("上传对象存储失败: %v", err)
	}
	markArtifactStored(ctx, job.TaskID, art.Key, key, fs.ProviderName(), firstNonEmpty(contentType, "application/octet-stream"), int64(len(content)))
	return true, nil
}

// downloadArtifact 下载上游产物（限体积；art.SourceAuth=channel-bearer 时注入渠道鉴权）。
// fromChannel=true 表示目标来自渠道相对 URL 的解析——渠道目标与主转发链路同信任级
// （渠道 baseURL 由管理员配置，允许内网中转形态），走无 SSRF transport 的专用 client；
// 插件声明的绝对外链 URL 保持 downloadClient（预检 + dial 层双重拦截）。
func downloadArtifact(ctx context.Context, rawURL string, job *transferJob, art rcommon.TaskArtifact, fromChannel bool) ([]byte, string, error) {
	client := downloadClient
	if fromChannel {
		client = channelDownloadClient
	}
	target := sourceTarget{URL: rawURL, Auth: art.SourceAuth, FromChannel: fromChannel}
	resp, err := doSourceRequest(ctx, client, target, job.ChannelID)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxArtifactBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(body) > maxArtifactBytes {
		return nil, "", fmt.Errorf("产物超过 %dMB 上限", maxArtifactBytes>>20)
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// sourceTarget 解析后的产物下载目标。
type sourceTarget struct {
	URL         string
	Auth        string // channel-bearer = 注入渠道密钥
	FromChannel bool   // 相对 URL 按渠道 baseURL 解析而来（与主转发链路同信任级）
}

// resolveSource 产物 URL 解析与安全校验（转存下载与回源代理共用）：
//   - 相对 URL（/path）按渠道 baseURL 解析，隐含 channel-bearer
//   - 绝对外链做 SSRF 预检；声明 channel-bearer 的绝对 URL 必须与渠道同源（防密钥外泄）
func resolveSource(ctx context.Context, channelID int64, rawURL, auth string) (sourceTarget, error) {
	target := sourceTarget{URL: rawURL, Auth: auth}
	if strings.HasPrefix(rawURL, "/") {
		baseURL, err := loadChannelBase(ctx, channelID)
		if err != nil {
			return target, fmt.Errorf("解析相对 URL 失败（渠道不可用）: %v", err)
		}
		target.URL = strings.TrimRight(baseURL, "/") + rawURL
		if target.Auth == "" {
			target.Auth = "channel-bearer" // 相对 URL 隐含渠道上下文
		}
		target.FromChannel = true
	}
	if !strings.HasPrefix(target.URL, "http://") && !strings.HasPrefix(target.URL, "https://") {
		return target, fmt.Errorf("url 取值不是 http(s)：%s", target.URL)
	}
	// SSRF 预检仅对插件声明的绝对外链 URL——渠道相对路径解析出的绝对 URL
	// 指向渠道自身（自建中转指内网的部署形态合法），预检会误杀。
	// 公网出口的兜底防线在 dial 层（SafeDialTransport 对私网目标二次拦截）。
	if !target.FromChannel {
		if err := ssrf.ValidateURL(target.URL); err != nil {
			return target, fmt.Errorf("SSRF 预检失败: %v", err)
		}
	}
	// channel-bearer 鉴权仅允许渠道同源目标：插件声明的绝对 URL 若携带渠道
	// Bearer 密钥发往任意公网 host（SSRF 预检只拦内网），即是渠道密钥外泄通道。
	if target.Auth == "channel-bearer" && !target.FromChannel {
		baseURL, err := loadChannelBase(ctx, channelID)
		if err != nil {
			return target, fmt.Errorf("channel-bearer 产物校验渠道失败: %v", err)
		}
		if !sameChannelHost(target.URL, baseURL) {
			return target, fmt.Errorf("channel-bearer 产物仅允许渠道同源 URL：%s", target.URL)
		}
	}
	return target, nil
}

// doSourceRequest 发起产物 GET（按需注入渠道鉴权），非 200 关闭响应体并报错。
func doSourceRequest(ctx context.Context, client *http.Client, target sourceTarget, channelID int64) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	if err != nil {
		return nil, err
	}
	if target.Auth == "channel-bearer" {
		apiKey, kErr := loadChannelKey(ctx, channelID)
		if kErr != nil {
			return nil, fmt.Errorf("渠道密钥不可用: %w", kErr)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("上游返回 %d", resp.StatusCode)
	}
	return resp, nil
}

// sameChannelHost 判断产物 URL 与渠道 baseURL 是否同源（host 一致 + 端口归一：
// 显式默认端口 443/80 与缺省端口视为相同，避免 api.vendor.com 与
// api.vendor.com:443 误判不同源）。channel-bearer 鉴权下载的边界判定。
func sameChannelHost(rawURL, baseURL string) bool {
	u1, err1 := url.Parse(rawURL)
	u2, err2 := url.Parse(baseURL)
	if err1 != nil || err2 != nil || u1.Hostname() == "" || u2.Hostname() == "" {
		return false
	}
	return strings.EqualFold(u1.Hostname(), u2.Hostname()) && normURLPort(u1) == normURLPort(u2)
}

// normURLPort 归一 URL 端口：缺省按 scheme 补默认值（https=443，其余=80）。
func normURLPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	return "80"
}

// ---------------------------------------------------------------------------
// 元数据行
// ---------------------------------------------------------------------------

func insertArtifactRow(ctx context.Context, job *transferJob, art rcommon.TaskArtifact, status string) {
	_, err := g.DB().Model(artifactTable).Ctx(ctx).Data(g.Map{
		"task_id":        job.TaskID,
		"public_task_id": job.PublicTaskID,
		"plugin_key":     job.PluginKey,
		"tenant_id":      job.TenantID,
		"user_id":        job.UserID,
		"channel_id":     job.ChannelID,
		"artifact_key":   art.Key,
		"source_type":    art.SourceType,
		"source_path":    art.SourcePath, // 声明原样路径（不随转存改写）
		"source_auth":    art.SourceAuth,
		"source_url":     "", // url 型下载时写入真实 URL
		"mime_type":      art.MimeType,
		"status":         status,
	}).OnConflict("task_id,artifact_key").Save()
	if err != nil {
		g.Log().Warningf(ctx, "js artifact insert row: %v", err)
	}
}

func markArtifactSourceURL(ctx context.Context, taskID int64, key, rawURL string) {
	_, _ = g.DB().Model(artifactTable).Ctx(ctx).
		Where("task_id", taskID).Where("artifact_key", key).
		Data(g.Map{"source_url": rawURL, "status": "running", "updated_at": time.Now()}).Update()
}

func markArtifactStored(ctx context.Context, taskID int64, key, storedKey, backend, mimeType string, size int64) {
	_, _ = g.DB().Model(artifactTable).Ctx(ctx).
		Where("task_id", taskID).Where("artifact_key", key).
		Data(g.Map{
			"stored_key": storedKey, "backend": backend, "mime_type": mimeType,
			"size": size, "status": "stored", "last_error": "", "updated_at": time.Now(),
		}).Update()
}

func markArtifactFailed(ctx context.Context, taskID int64, key string, err error) {
	_, _ = g.DB().Model(artifactTable).Ctx(ctx).
		Where("task_id", taskID).Where("artifact_key", key).
		Data(g.Map{
			"status": "failed", "last_error": err.Error(),
			"retry_count": gdb.Raw("retry_count + 1"), "updated_at": time.Now(),
		}).Update()
}

func artifactStoredStatus(ctx context.Context, taskID int64, key string) string {
	val, err := g.DB().Model(artifactTable).Ctx(ctx).
		Where("task_id", taskID).Where("artifact_key", key).
		Fields("status").Value()
	if err != nil {
		return ""
	}
	return val.String()
}

func loadTaskData(ctx context.Context, taskID int64) (*struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
}, error) {
	var task struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	err := g.DB().Model("tsk_model_tasks").Ctx(ctx).Where("id", taskID).Scan(&task)
	if err != nil {
		return nil, err
	}
	if task.Status == "" {
		return nil, nil
	}
	return &task, nil
}

// loadChannelBase 渠道 base_url（相对 URL 解析用；鉴权密钥走 loadChannelKey 单独加载）。
func loadChannelBase(ctx context.Context, channelID int64) (string, error) {
	if channelID <= 0 {
		return "", fmt.Errorf("task 无渠道归属")
	}
	val, err := g.DB().Model("chn_channels").Ctx(ctx).
		Where("id", channelID).Fields("base_url").Value()
	if err != nil {
		return "", err
	}
	return val.String(), nil
}

// loadChannelKey 渠道首个可用密钥（AES 解密，口径对齐 async_provider.getChannelApiKey）。
func loadChannelKey(ctx context.Context, channelID int64) (string, error) {
	enc, err := g.DB().Model("chn_channel_keys").Ctx(ctx).
		Where("channel_id", channelID).Where("status", "active").
		Order("id").Fields("encrypted_key").Value()
	if err != nil {
		return "", err
	}
	cipher := enc.String()
	if cipher == "" {
		return "", fmt.Errorf("渠道无可用密钥")
	}
	hexKey := g.Cfg().MustGet(ctx, "crypto.encryptionKey").String()
	key, err := uc.GetEncryptionKey(hexKey)
	if err != nil {
		return "", err
	}
	plain, err := uc.DecryptString(key, cipher)
	if err != nil {
		return "", err
	}
	return plain, nil
}

// ---------------------------------------------------------------------------
// 辅助
// ---------------------------------------------------------------------------

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func extOf(mimeType string) string {
	switch {
	case strings.Contains(mimeType, "mp4"):
		return ".mp4"
	case strings.Contains(mimeType, "webm"):
		return ".webm"
	case strings.Contains(mimeType, "png"):
		return ".png"
	case strings.Contains(mimeType, "jpeg"), strings.Contains(mimeType, "jpg"):
		return ".jpg"
	case strings.Contains(mimeType, "webp"):
		return ".webp"
	case strings.Contains(mimeType, "gif"):
		return ".gif"
	case strings.Contains(mimeType, "mpeg"), strings.Contains(mimeType, "mp3"):
		return ".mp3"
	case strings.Contains(mimeType, "wav"):
		return ".wav"
	case strings.Contains(mimeType, "zip"):
		return ".zip"
	default:
		return ".bin"
	}
}
