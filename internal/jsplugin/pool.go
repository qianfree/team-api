package jsplugin

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// DefaultPoolConcurrency 每插件实例池默认并发闸。
const DefaultPoolConcurrency = 20

// DefaultAdmissionTimeout 池满时的调用准入等待上限。
const DefaultAdmissionTimeout = 3 * time.Second

// PluginPool 单个插件（按 key）的实例池：并发闸 + 实例复用 + 损坏重建。
type PluginPool struct {
	compiled *CompiledPlugin
	engine   Engine
	opts     InstanceOptions

	sem  chan struct{} // 并发闸
	idle chan Instance // 健康可复用实例

	mu        sync.Mutex
	live      int // 存活实例数（含使用中）
	closed    bool
	lastError error
}

// NewPluginPool 创建插件实例池。allowedHosts 应已包含渠道 baseUrl host 与 manifest.allowedHosts。
func NewPluginPool(engine Engine, compiled *CompiledPlugin, opts InstanceOptions) *PluginPool {
	concurrency := DefaultPoolConcurrency
	if opts.Concurrency > 0 {
		concurrency = opts.Concurrency
	}
	return &PluginPool{
		compiled: compiled,
		engine:   engine,
		opts:     opts,
		sem:      make(chan struct{}, concurrency),
		idle:     make(chan Instance, concurrency),
	}
}

// release 归还实例到空闲队列。池已关闭（rebuild 整建整拆的窗口期）时直接丢弃：
// Close 只排空当时的 idle，之后归还进 channel 的实例会滞留到进程结束（channel
// 持有引用阻 GC），live 计数也不再回零。
func (p *PluginPool) release(inst Instance) {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		inst.Close()
		p.mu.Lock()
		p.live--
		p.mu.Unlock()
		return
	}
	select {
	case p.idle <- inst:
	default:
		inst.Close()
		p.mu.Lock()
		p.live--
		p.mu.Unlock()
	}
}

// Call 经池调用钩子：准入 → 取/建实例 → 调用 → 归还或丢弃。
func (p *PluginPool) Call(ctx context.Context, hook string, args json.RawMessage) (json.RawMessage, error) {
	// 准入（池满等待，超时拒绝）
	admCtx, cancel := context.WithTimeout(ctx, DefaultAdmissionTimeout)
	defer cancel()
	select {
	case p.sem <- struct{}{}:
	case <-admCtx.Done():
		return nil, gerror.Newf("jsplugin %s 调用准入超时（并发闸 %d 已满）", p.compiled.Key, cap(p.sem))
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-p.sem }()

	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return nil, gerror.Newf("jsplugin %s 池已关闭", p.compiled.Key)
	}

	inst := p.acquire()
	if inst == nil {
		var err error
		inst, err = p.engine.NewInstance(p.compiled, p.opts)
		if err != nil {
			return nil, gerror.Wrapf(err, "jsplugin %s 创建实例失败", p.compiled.Key)
		}
		p.mu.Lock()
		p.live++
		p.mu.Unlock()
	}

	started := time.Now()
	result, callErr := inst.Call(ctx, hook, args)
	RecordHookCall(ctx, p.compiled.Key, hook, args, result, time.Since(started), callErr)

	if callErr != nil {
		// 调用失败一律丢弃实例（重建便宜且隔离故障状态）
		inst.Close()
		p.mu.Lock()
		p.live--
		p.mu.Unlock()
		return nil, callErr
	}

	// 归还或丢弃（池已关闭时丢弃，避免滞留已关闭池的 channel）
	p.release(inst)
	return result, nil
}

// HasHook 报告插件是否导出钩子（任取一存活实例探测；无实例时临时建一个）。
func (p *PluginPool) HasHook(hook string) bool {
	inst := p.acquire()
	if inst == nil {
		var err error
		inst, err = p.engine.NewInstance(p.compiled, p.opts)
		if err != nil {
			return false
		}
		p.mu.Lock()
		p.live++
		p.mu.Unlock()
	}
	has := inst.HasHook(hook)
	p.release(inst)
	return has
}

// acquire 从空闲队列取实例（非阻塞）。
func (p *PluginPool) acquire() Instance {
	select {
	case inst := <-p.idle:
		return inst
	default:
		return nil
	}
}

// LiveInstances 返回存活实例数（观测用）。
func (p *PluginPool) LiveInstances() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.live
}

// Close 关闭池：清空空闲实例。
func (p *PluginPool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()
	for {
		select {
		case inst := <-p.idle:
			inst.Close()
			p.mu.Lock()
			p.live--
			p.mu.Unlock()
		default:
			return
		}
	}
}
