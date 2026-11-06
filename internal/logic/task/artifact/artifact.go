// Package artifact 实现 JS 任务插件的产物转存流水线（设计方案 §七）：
//
//	插件声明（key + taskData 取值路径）→ 宿主下载/解码（SSRF 防护 + 体积上限）
//	→ FileService 上传对象存储 → 改写 task.Data 中的 URL → 元数据落表
//
// 边界纪律：JS 永不接触产物字节。worker 池有界队列 + sweep 周期重试，
// 失败回落原上游 URL（客户端仍可用，只是未经转存）。
package artifact

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

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
)

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

// EnqueueTransfer 任务终态成功后入队转存：元数据先行落表（pending），队列满时
// 留给 sweep 重试。非阻塞，绝不拖慢轮询循环。
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
	for _, art := range artifacts {
		insertArtifactRow(ctx, job, art)
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
func SweepPending(ctx context.Context) {
	mu.Lock()
	q := queue
	mu.Unlock()
	if q == nil {
		return
	}
	// 先取去重任务 ID（PG 严格模式下列必须都在 GROUP BY，两段查询规避）
	taskIDs, err := g.DB().Model(artifactTable).Ctx(ctx).
		Where("status IN (?)", g.Slice{"pending", "failed"}).
		Where("retry_count < ?", maxRetryCount).
		Distinct().Fields("task_id").Limit(100).Array()
	if err != nil {
		g.Log().Warningf(ctx, "js artifact sweep query: %v", err)
		return
	}
	for _, taskIDVal := range taskIDs {
		taskID := taskIDVal.Int64()
		var rows []struct {
			PublicTask  string `json:"public_task_id"`
			PluginKey   string `json:"plugin_key"`
			ArtifactKey string `json:"artifact_key"`
			SourceType  string `json:"source_type"`
			SourceUrl   string `json:"source_url"`
		}
		if err := g.DB().Model(artifactTable).Ctx(ctx).
			Where("task_id", taskID).
			Where("status IN (?)", g.Slice{"pending", "failed"}).Scan(&rows); err != nil || len(rows) == 0 {
			continue
		}
		job := &transferJob{TaskID: taskID, PublicTaskID: rows[0].PublicTask, PluginKey: rows[0].PluginKey}
		for _, ar := range rows {
			job.Artifacts = append(job.Artifacts, rcommon.TaskArtifact{
				Key: ar.ArtifactKey, SourceType: ar.SourceType, SourcePath: ar.SourceUrl,
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
		url, storedErr := transferOne(ctx, fs, job, art, data)
		if storedErr != nil {
			markArtifactFailed(ctx, job.TaskID, art.Key, storedErr)
			continue
		}
		if url == "" {
			continue // 已 stored（重试幂等）
		}
		if !setJSONPath(data, art.SourcePath, url) {
			markArtifactFailed(ctx, job.TaskID, art.Key, fmt.Errorf("改写路径 %s 失败", art.SourcePath))
			continue
		}
		rewritten = true
	}

	if rewritten {
		newData, err := json.Marshal(data)
		if err == nil {
			_, _ = g.DB().Model("tsk_model_tasks").Ctx(ctx).
				Where("id", job.TaskID).Data(g.Map{"data": newData, "updated_at": time.Now()}).Update()
		}
	}
}

// transferOne 转存单个产物，返回应改写进去的 URL（空串 = 无需改写）。
func transferOne(ctx context.Context, fs *lcommon.FileService, job *transferJob, art rcommon.TaskArtifact, data map[string]any) (string, error) {
	// 幂等：已 stored 直接跳过
	if stored := getJSONPath(data, art.SourcePath); stored != nil {
		if rowStatus := artifactStoredStatus(ctx, job.TaskID, art.Key); rowStatus == "stored" {
			return "", nil
		}
	}

	var (
		content     []byte
		contentType string
	)
	switch art.SourceType {
	case "inline":
		v := getJSONPath(data, art.SourcePath)
		s, ok := v.(string)
		if !ok || s == "" {
			return "", fmt.Errorf("inline 路径 %s 取值非字符串", art.SourcePath)
		}
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return "", fmt.Errorf("inline base64 解码失败: %v", err)
		}
		content = decoded
		contentType = art.MimeType
	case "url":
		v := getJSONPath(data, art.SourcePath)
		rawURL, ok := v.(string)
		if !ok || rawURL == "" {
			return "", fmt.Errorf("url 路径 %s 取值非字符串", art.SourcePath)
		}
		// 相对 URL（/path 形态）：按渠道 baseURL 解析（需鉴权产物端点的常见形态）
		if strings.HasPrefix(rawURL, "/") {
			baseURL, key, chErr := loadChannelBase(ctx, job.ChannelID)
			if chErr != nil {
				return "", fmt.Errorf("解析相对 URL 失败（渠道不可用）: %v", chErr)
			}
			rawURL = strings.TrimRight(baseURL, "/") + rawURL
			if art.SourceAuth == "" {
				art.SourceAuth = "channel-bearer" // 相对 URL 隐含渠道上下文
			}
			_ = key
		}
		if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
			return "", fmt.Errorf("url 取值不是 http(s)：%s", rawURL)
		}
		// SSRF 预检仅对绝对外链 URL（渠道相对路径已限定在渠道内）
		if !strings.Contains(rawURL, "@relative") {
			if err := ssrf.ValidateURL(rawURL); err != nil {
				return "", fmt.Errorf("SSRF 预检失败: %v", err)
			}
		}
		body, ct, dErr := downloadArtifact(ctx, rawURL, job, art)
		if dErr != nil {
			return "", fmt.Errorf("下载失败: %v", dErr)
		}
		content = body
		contentType = firstNonEmpty(art.MimeType, ct)
		markArtifactSourceURL(ctx, job.TaskID, art.Key, rawURL)
	default:
		return "", fmt.Errorf("未知 source type %q", art.SourceType)
	}

	if len(content) == 0 {
		return "", fmt.Errorf("产物内容为空")
	}

	key := storageKeyPrefix + job.PublicTaskID + "/" + art.Key + extOf(firstNonEmpty(contentType, "application/octet-stream"))
	rec, err := fs.Upload(ctx, &lcommon.FileUpload{
		Reader:      bytes.NewReader(content),
		Filename:    art.Key + extOf(firstNonEmpty(contentType, "bin")),
		ContentType: firstNonEmpty(contentType, "application/octet-stream"),
		Size:        int64(len(content)),
		TenantID:    job.TenantID,
		UserID:      job.UserID,
	})
	if err != nil {
		return "", fmt.Errorf("上传对象存储失败: %v", err)
	}
	url, err := fs.GetDownloadURL(ctx, rec.ID, job.TenantID)
	if err != nil {
		return "", fmt.Errorf("生成下载 URL 失败: %v", err)
	}
	markArtifactStored(ctx, job.TaskID, art.Key, key, fs.ProviderName(), firstNonEmpty(contentType, "application/octet-stream"), int64(len(content)))
	return url, nil
}

// downloadArtifact 下载上游产物（限体积；art.SourceAuth=channel-bearer 时注入渠道鉴权）。
func downloadArtifact(ctx context.Context, rawURL string, job *transferJob, art rcommon.TaskArtifact) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	if art.SourceAuth == "channel-bearer" {
		apiKey, kErr := loadChannelKey(ctx, job.ChannelID)
		if kErr != nil {
			return nil, "", fmt.Errorf("渠道密钥不可用: %w", kErr)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("上游返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxArtifactBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(body) > maxArtifactBytes {
		return nil, "", fmt.Errorf("产物超过 %dMB 上限", maxArtifactBytes>>20)
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// ---------------------------------------------------------------------------
// 元数据行
// ---------------------------------------------------------------------------

func insertArtifactRow(ctx context.Context, job *transferJob, art rcommon.TaskArtifact) {
	_, err := g.DB().Model(artifactTable).Ctx(ctx).Data(g.Map{
		"task_id":        job.TaskID,
		"public_task_id": job.PublicTaskID,
		"plugin_key":     job.PluginKey,
		"artifact_key":   art.Key,
		"source_type":    art.SourceType,
		"source_url":     art.SourcePath, // 声明期存取值路径，url 型转存时更新为真实 URL
		"status":         "pending",
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

// loadChannelBase 渠道 base_url（相对 URL 解析用；key 供鉴权场景单独加载）。
func loadChannelBase(ctx context.Context, channelID int64) (baseURL, apiKey string, err error) {
	if channelID <= 0 {
		return "", "", fmt.Errorf("task 无渠道归属")
	}
	val, err := g.DB().Model("chn_channels").Ctx(ctx).
		Where("id", channelID).Fields("base_url").Value()
	if err != nil {
		return "", "", err
	}
	return val.String(), "", nil
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
// JSON path（"data.videos[0].url"）
// ---------------------------------------------------------------------------

// getJSONPath 按点分 + [n] 路径取值。
func getJSONPath(data map[string]any, path string) any {
	cur := any(data)
	for _, seg := range splitPath(path) {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[seg.key]
			if !ok {
				return nil
			}
			cur = v
		case []any:
			if seg.index < 0 || seg.index >= len(node) {
				return nil
			}
			cur = node[seg.index]
		default:
			return nil
		}
	}
	return cur
}

// setJSONPath 按路径改写值（路径必须已存在——产物声明指向 taskData 已有字段）。
func setJSONPath(data map[string]any, path string, value any) bool {
	segs := splitPath(path)
	if len(segs) == 0 {
		return false
	}
	parent := any(data)
	for i := 0; i < len(segs)-1; i++ {
		switch node := parent.(type) {
		case map[string]any:
			v, ok := node[segs[i].key]
			if !ok {
				return false
			}
			parent = v
		case []any:
			if segs[i].index < 0 || segs[i].index >= len(node) {
				return false
			}
			parent = node[segs[i].index]
		default:
			return false
		}
	}
	last := segs[len(segs)-1]
	switch node := parent.(type) {
	case map[string]any:
		if _, ok := node[last.key]; !ok {
			return false
		}
		node[last.key] = value
		return true
	case []any:
		if last.index < 0 || last.index >= len(node) {
			return false
		}
		node[last.index] = value
		return true
	default:
		return false
	}
}

type pathSeg struct {
	key   string
	index int
}

func splitPath(path string) []pathSeg {
	var segs []pathSeg
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			continue
		}
		// 处理 "videos[0]" 形态
		bracket := strings.Index(part, "[")
		if bracket < 0 {
			segs = append(segs, pathSeg{key: part, index: -1})
			continue
		}
		if bracket > 0 {
			segs = append(segs, pathSeg{key: part[:bracket], index: -1})
		}
		for bracket >= 0 {
			end := strings.Index(part[bracket:], "]")
			if end < 0 {
				return segs
			}
			idx := 0
			numStr := part[bracket+1 : bracket+end]
			for _, c := range numStr {
				if c < '0' || c > '9' {
					return segs
				}
				idx = idx*10 + int(c-'0')
			}
			segs = append(segs, pathSeg{key: "", index: idx})
			next := strings.Index(part[bracket+end:], "[")
			if next < 0 {
				break
			}
			bracket = bracket + end + next
		}
	}
	return segs
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
