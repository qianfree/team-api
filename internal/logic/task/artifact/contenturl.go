package artifact

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
	uc "github.com/qianfree/team-api/internal/utility/crypto"
	rcommon "github.com/qianfree/team-api/relay/common"
)

// 统一内容链接：{base}/v1/tasks/{public_task_id}/content[/{artifact_key}]?expires=&signature=
// HMAC-SHA256 签名（密钥由 crypto.encryptionKey 经域分离派生），免 API Key 直接打开
// （浏览器 / <video> 标签 / 下游拉取），有效期由 task_content_url_ttl_minutes 控制。
// 每次渲染任务都签发新链接，链接形态与产物是否转存 OSS 无关。

const (
	contentURLPrefix     = "/v1/tasks/"
	contentSignDomain    = "team-api/task-content-url/v1"
	defaultContentTTLMin = 1440
)

// ErrContentURLExpired / ErrContentURLInvalid 签名校验失败分类（调用方映射 HTTP 状态）。
var (
	ErrContentURLExpired = errors.New("content url expired")
	ErrContentURLInvalid = errors.New("content url signature invalid")
)

// signingKeyFn 签名密钥（测试可替换）。
var signingKeyFn = func(ctx context.Context) ([]byte, error) {
	master, err := uc.GetEncryptionKey(g.Cfg().MustGet(ctx, "crypto.encryptionKey").String())
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, master)
	mac.Write([]byte(contentSignDomain))
	return mac.Sum(nil), nil
}

// nowFn 当前时间（测试可替换）。
var nowFn = time.Now

// publicBaseFn 对外基址（测试可替换）。
var publicBaseFn = lcommon.PublicBaseURL

// contentTTLFn 链接有效期（测试可替换）。
var contentTTLFn = contentTTL

func contentTTL(ctx context.Context) time.Duration {
	minutes := lcommon.Config().GetInt(ctx, "task_content_url_ttl_minutes")
	if minutes <= 0 {
		minutes = defaultContentTTLMin
	}
	if minutes < 5 {
		minutes = 5
	}
	return time.Duration(minutes) * time.Minute
}

func contentSignature(key []byte, publicTaskID, artifactKey string, expires int64) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(publicTaskID + "\n" + artifactKey + "\n" + strconv.FormatInt(expires, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// ContentPath 内容链接路径（artifactKey 为空 = 主产物）。
func ContentPath(publicTaskID, artifactKey string) string {
	p := contentURLPrefix + url.PathEscape(publicTaskID) + "/content"
	if artifactKey != "" {
		p += "/" + url.PathEscape(artifactKey)
	}
	return p
}

// SignContentURL 签发统一内容链接（基址取 PublicBaseURL；签名失败返回空串）。
func SignContentURL(ctx context.Context, publicTaskID, artifactKey string) string {
	key, err := signingKeyFn(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "task content url: signing key unavailable: %v", err)
		return ""
	}
	expires := nowFn().Add(contentTTLFn(ctx)).Unix()
	q := url.Values{}
	q.Set("expires", strconv.FormatInt(expires, 10))
	q.Set("signature", contentSignature(key, publicTaskID, artifactKey, expires))
	return publicBaseFn(ctx) + ContentPath(publicTaskID, artifactKey) + "?" + q.Encode()
}

// VerifyContentURL 校验签名与有效期。
func VerifyContentURL(ctx context.Context, publicTaskID, artifactKey, expiresStr, signature string) error {
	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil || signature == "" {
		return ErrContentURLInvalid
	}
	key, err := signingKeyFn(ctx)
	if err != nil {
		return err
	}
	want := contentSignature(key, publicTaskID, artifactKey, expires)
	if !hmac.Equal([]byte(want), []byte(signature)) {
		return ErrContentURLInvalid
	}
	if nowFn().Unix() > expires {
		return ErrContentURLExpired
	}
	return nil
}

// deliveryWaitLimit 交付等待上限：任务完成后转存迟迟未成功（队列积压 / 反复失败），
// 超过该时长即按回源交付放行完成态——覆盖 sweep 全部重试窗口（5 分钟 × 5 次），
// 避免客户端永远停在进行中。
const deliveryWaitLimit = 30 * time.Minute

// deliveryPending 产物是否仍在转存途中：pending/running，或 failed 但仍在重试次数内。
// direct（插件关闭转存）与 stored 均视为就绪；重试耗尽的 failed 走回源交付。
func deliveryPending(task *rcommon.AsyncTask, rows []*artifactRow) bool {
	inFlight := false
	for _, row := range rows {
		switch row.Status {
		case statusPending, "running":
			inFlight = true
		case "failed":
			if row.RetryCount < maxRetryCount {
				inFlight = true
			}
		}
	}
	if !inFlight {
		return false
	}
	if task.FinishTime != nil && nowFn().Sub(*task.FinishTime) > deliveryWaitLimit {
		return false
	}
	return true
}

// ResolveDelivery 任务对外交付视图：成功任务的产物仍在转存 OSS 途中时 Pending=true
// （调用方对外呈现进行中、不给链接）；就绪时给主产物统一内容链接。
// 非 JS 插件任务没有产物登记，有 ResultURL 即就绪（链接经平台回源代理）。
func ResolveDelivery(ctx context.Context, task *rcommon.AsyncTask) rcommon.TaskDelivery {
	view, _ := resolveDelivery(ctx, task)
	return view
}

func resolveDelivery(ctx context.Context, task *rcommon.AsyncTask) (rcommon.TaskDelivery, []*artifactRow) {
	if task == nil || task.Status != string(rcommon.TaskStatusSuccess) {
		return rcommon.TaskDelivery{}, nil
	}
	var rows []*artifactRow
	if strings.HasPrefix(task.Platform, "js:") && task.ID > 0 {
		loaded, err := loadArtifactRows(ctx, task.ID, task.TenantID)
		if err != nil {
			// 登记不可读时不阻断完成态：链接访问时由内容供给自行回落
			g.Log().Warningf(ctx, "task delivery: load artifacts of %s failed: %v", task.PublicTaskID, err)
		}
		rows = loaded
	}
	if deliveryPending(task, rows) {
		return rcommon.TaskDelivery{Pending: true}, rows
	}
	if len(rows) == 0 && task.ResultURL == "" {
		return rcommon.TaskDelivery{}, rows
	}
	return rcommon.TaskDelivery{ContentURL: SignContentURL(ctx, task.PublicTaskID, "")}, rows
}

// RenderData 插件渲染视图：成功任务的登记产物声明路径替换为统一内容链接（不落库），
// 插件 renderTask 读原字段（如 _video_url）即自动呈现统一链接；同时返回交付视图。
// 转存途中（Pending）把声明路径置空——不向客户端泄出上游直链，调用方按进行中呈现。
func RenderData(ctx context.Context, task *rcommon.AsyncTask) (json.RawMessage, rcommon.TaskDelivery) {
	if task == nil {
		return nil, rcommon.TaskDelivery{}
	}
	view, rows := resolveDelivery(ctx, task)
	if len(rows) == 0 || len(task.Data) == 0 {
		return task.Data, view
	}
	var data map[string]any
	if json.Unmarshal(task.Data, &data) != nil || data == nil {
		return task.Data, view
	}
	primary := pickArtifactRow(rows, "")
	changed := false
	for _, row := range rows {
		path := row.declPath()
		if path == "" || rcommon.GetJSONPath(data, path) == nil {
			continue
		}
		link := ""
		if !view.Pending {
			link = view.ContentURL
			if row != primary {
				link = SignContentURL(ctx, task.PublicTaskID, row.ArtifactKey)
			}
		}
		if rcommon.SetJSONPath(data, path, link) {
			changed = true
		}
	}
	if !changed {
		return task.Data, view
	}
	out, err := json.Marshal(data)
	if err != nil {
		return task.Data, view
	}
	return out, view
}
