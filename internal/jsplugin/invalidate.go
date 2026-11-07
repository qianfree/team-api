// invalidate.go JS 插件 generation 的跨实例失效广播。
// 生命周期写操作（激活/启停/删除/配置更新）只重建本进程 generation，多实例部署下
// 其余实例无从得知——经 Redis pub/sub 广播，收到通知的实例各自 rebuildGeneration，
// 与 dispatchadapter 的 catalog:invalidate 同款架构。Redis 未配置（单实例部署常见）
// 或不可用时静默降级为「仅本实例生效」，探测走 relay/common 的可选 Redis 封装。
package jsplugin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/os/glog"

	relaycommon "github.com/qianfree/team-api/relay/common"
)

// jsPluginInvalidateChan 失效广播频道。
const jsPluginInvalidateChan = "jsplugin:invalidate"

// invalidateRebuildTimeout 订阅侧重建的时限：冷 compileCache 下含 esbuild 编译，
// 比渠道 catalog 的纯数据快照慢，取宽。
const invalidateRebuildTimeout = 60 * time.Second

// invalidateNodeID 本实例标识：广播消息携带来源实例 ID，来源实例已在管理操作内
// 同步重建，收到自己的消息直接跳过（避免整建整拆带来的无谓池 churn）。
var invalidateNodeID = randomNodeID()

var (
	invalidateMu   sync.Mutex
	invalidateStop chan struct{}
)

// randomNodeID 生成进程级随机标识（16 hex；熵源失败退化为时间戳，仅用于自识别）。
func randomNodeID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return time.Now().Format("150405.000000000")
	}
	return hex.EncodeToString(b)
}

// shouldRebuildInvalidate 判断收到的消息是否需要本实例重建：来源非本实例即重建
// （空载荷与 "*" 是手工运维广播，同样触发；仅来源等于本实例 ID 时跳过）。
func shouldRebuildInvalidate(payload string) bool {
	return payload != invalidateNodeID
}

// PublishInvalidate 广播跨实例 generation 失效（生命周期写操作成功重建后调用）。
// best-effort：失败仅记警告，管理操作本身不受影响；漏发的后果是其余实例沿用旧
// generation 直到下一次生命周期操作。
func (r *Registry) PublishInvalidate(ctx context.Context) {
	client := relaycommon.RedisClientOrNil()
	if client == nil {
		return // Redis 未配置：无多实例广播需求，静默降级
	}
	if _, err := client.Publish(ctx, jsPluginInvalidateChan, invalidateNodeID); err != nil {
		glog.Warningf(ctx, "jsplugin invalidate 广播失败（其余实例将延迟到下一次生命周期操作才收敛）: %v", err)
	}
}

// StartInvalidateSubscriber 启动跨实例失效订阅循环（Bootstrap 成功后调用一次）。
// Redis 未配置时不启动。不提供周期性兜底重建：与渠道 catalog 的纯数据快照不同，
// 插件重建是整建整拆（关旧池重建实例），周期性重建等于持续 churn；pub/sub 断连
// 窗口秒级、插件生命周期操作低频，漏收一次的后果是该实例沿用旧 generation 到
// 下一次操作（或手工向频道发布 "*"），可接受。
func (r *Registry) StartInvalidateSubscriber() {
	if relaycommon.RedisClientOrNil() == nil {
		return
	}
	invalidateMu.Lock()
	if invalidateStop == nil {
		invalidateStop = make(chan struct{})
	}
	stop := invalidateStop
	invalidateMu.Unlock()
	go r.subscribeInvalidate(stop)
}

// stopInvalidateSubscriber 停止订阅循环（Shutdown 调用；支持进程内再次启动）。
func stopInvalidateSubscriber() {
	invalidateMu.Lock()
	defer invalidateMu.Unlock()
	if invalidateStop != nil {
		close(invalidateStop)
		invalidateStop = nil
	}
}

// subscribeInvalidate 订阅-重建循环：断线重连；消息来源非本实例时 rebuildGeneration。
func (r *Registry) subscribeInvalidate(stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
		}
		conn, _, err := relaycommon.RedisClientOrNil().Subscribe(context.Background(), jsPluginInvalidateChan)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		for {
			v, err := conn.Receive(context.Background())
			if err != nil {
				break // 断线，外层重连
			}
			msg, ok := v.Val().(*gredis.Message)
			if ok && shouldRebuildInvalidate(msg.Payload) {
				r.rebuildRemote()
			}
		}
		conn.Close(context.Background())
		select {
		case <-stop:
			return
		case <-time.After(time.Second):
		}
	}
}

// rebuildRemote 订阅侧重建：失败仅记日志等下一次触发，绝不向上抛（循环不能退出）。
func (r *Registry) rebuildRemote() {
	ctx, cancel := context.WithTimeout(context.Background(), invalidateRebuildTimeout)
	defer cancel()
	failed, err := r.rebuildGeneration(ctx)
	if err != nil {
		glog.Warningf(ctx, "jsplugin 跨实例失效重建失败: %v", err)
		return
	}
	// 单插件装载失败沿用 rebuildGeneration 的跳过策略，此处只留痕
	for key, loadErr := range failed {
		glog.Warningf(ctx, "jsplugin 跨实例重建：插件 %s 装载失败（已跳过）: %v", key, loadErr)
	}
}
