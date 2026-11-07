// Package jsplugin 实现 JS 任务插件基座：引擎抽象（goja + esbuild）、实例池、
// 宿主能力注入与注册中心。体系架构与调用链见 docs/JS插件体系架构与调用链分析.md。
package jsplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/gogf/gf/v2/errors/gerror"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// DefaultCallTimeout 单次钩子调用的默认硬超时（vm.Interrupt 兜底）。
const DefaultCallTimeout = 5 * time.Second

// MaxPluginSourceBytes 插件源码大小上限（512 KiB）。
const MaxPluginSourceBytes = 512 << 10

// globalName esbuild IIFE 产物挂载的全局变量名（bundle 执行后 exports 可经它取到）。
const globalName = "teamPlugin"

// CompiledPlugin 编译产物：源码经 esbuild 打包为 IIFE，manifest 已解析校验。
type CompiledPlugin struct {
	Key      string
	Version  string
	Source   string
	Bundle   string
	Manifest *contract.Manifest
}

// Engine 引擎抽象：编译源码、创建隔离实例。goja 为 v1 唯一实现，
// 未来可替换为 wasm 实现而不动上层（演进预留）。
type Engine interface {
	// Compile 编译插件源码：esbuild ESM→IIFE 打包（v1 仅接受零 import 的单文件插件，
	// 有 import 即在编译期报错——这是上传期安全检查的一部分）并解析校验 manifest。
	Compile(source string) (*CompiledPlugin, error)
	// NewInstance 创建隔离实例：独立 goja.Runtime + 注入 utils 宿主能力。
	NewInstance(compiled *CompiledPlugin, opts InstanceOptions) (Instance, error)
}

// InstanceOptions 实例创建选项。
type InstanceOptions struct {
	CallTimeout  time.Duration  // 钩子调用硬超时，0=DefaultCallTimeout
	AllowedHosts []string       // utils.fetch 白名单（已含渠道 baseUrl host），条目为 host 或 host:port
	Concurrency  int            // 池并发闸，0=DefaultPoolConcurrency
	ConfigValues map[string]any // 插件配置值（解密后），utils.config.get 读取；nil=插件无配置
}

// Instance 一个隔离的插件执行实例（goroutine 间串行复用）。
type Instance interface {
	// Call 调用钩子：入参/返回均走 JSON 边界，不暴露 Go 对象图。
	Call(ctx context.Context, hook string, args json.RawMessage) (json.RawMessage, error)
	// HasHook 报告插件是否导出了该钩子。
	HasHook(hook string) bool
	Close()
}

// HookError 钩子执行失败（JS 异常 / 校验前置失败 / 非法返回）。
type HookError struct {
	Plugin  string
	Hook    string
	Message string
}

func (e *HookError) Error() string {
	return fmt.Sprintf("jsplugin %s hook %s: %s", e.Plugin, e.Hook, e.Message)
}

// NewHookError 构造钩子错误。
func NewHookError(plugin, hook, format string, args ...any) *HookError {
	return &HookError{Plugin: plugin, Hook: hook, Message: fmt.Sprintf(format, args...)}
}

// ---------------------------------------------------------------------------
// goja 实现
// ---------------------------------------------------------------------------

// GojaEngine 基于 goja 的引擎实现（纯 Go，无 CGo）。
type GojaEngine struct {
	// probeTimeout Compile 期空 runtime 跑 bundle 求 manifest 的顶层执行上限。
	// 顶层代码理论上应是纯声明，超时即视为恶意/缺陷插件。测试可注入小值。
	probeTimeout time.Duration
}

// NewGojaEngine 创建引擎实例。无状态（除超时配置），可全局复用。
func NewGojaEngine() *GojaEngine { return &GojaEngine{probeTimeout: DefaultCallTimeout} }

// Compile 编译插件源码。
func (e *GojaEngine) Compile(source string) (*CompiledPlugin, error) {
	if strings.TrimSpace(source) == "" {
		return nil, gerror.New("插件源码为空")
	}
	if len(source) > MaxPluginSourceBytes {
		return nil, gerror.Newf("插件源码超过 %d 字节上限", MaxPluginSourceBytes)
	}
	result := api.Build(api.BuildOptions{
		Stdin: &api.StdinOptions{
			Contents:   source,
			Loader:     api.LoaderJS,
			ResolveDir: ".",
			Sourcefile: "plugin.js",
		},
		Bundle:     true,
		Format:     api.FormatIIFE,
		GlobalName: globalName,
		Target:     api.ES2017,
		Write:      false,
		LogLevel:   api.LogLevelSilent,
	})
	if len(result.Errors) > 0 {
		msgs := make([]string, 0, len(result.Errors))
		for _, es := range result.Errors {
			msgs = append(msgs, es.Text)
		}
		return nil, gerror.Newf("esbuild 编译失败：%s", strings.Join(msgs, "; "))
	}
	if len(result.OutputFiles) == 0 || len(result.OutputFiles[0].Contents) == 0 {
		return nil, gerror.New("esbuild 编译产物为空")
	}
	bundle := string(result.OutputFiles[0].Contents)

	// 静态求值 manifest：空 runtime 跑 bundle 取 meta（插件顶层不应有副作用，
	// utils 缺失时顶层访问会抛错——契约要求 meta 求值不依赖宿主能力）。
	// 顶层执行同样受超时保护——恶意/缺陷插件的死循环不能挂死上传或启动装载。
	probe := goja.New()
	err := runInterruptGuarded(probe, e.probeTimeout, func() error {
		_, runErr := probe.RunString(bundle)
		return runErr
	})
	if err != nil {
		if isInterruptedErr(err) {
			return nil, gerror.Newf("插件顶层执行超时（%s），meta 求值不允许死循环/耗时计算", e.probeTimeout)
		}
		return nil, gerror.Wrap(err, "插件顶层执行失败（meta 求值不允许依赖 utils）")
	}
	exports := probe.Get(globalName)
	obj, ok := exports.(*goja.Object)
	if !ok {
		return nil, gerror.New("插件未导出任何成员（缺少 export const meta 与钩子）")
	}
	metaVal := obj.Get("meta")
	if metaVal == nil || goja.IsUndefined(metaVal) || goja.IsNull(metaVal) {
		return nil, gerror.New("插件缺少 export const meta")
	}
	metaJSON, err := stringifyValue(probe, metaVal)
	if err != nil {
		return nil, gerror.Wrap(err, "meta 序列化失败")
	}
	var m contract.Manifest
	if err := json.Unmarshal(metaJSON, &m); err != nil {
		return nil, gerror.Wrap(err, "meta 不是合法的 manifest JSON")
	}
	if err := m.Validate(); err != nil {
		return nil, gerror.Wrap(err, "manifest 校验失败")
	}
	return &CompiledPlugin{
		Key:      m.Key,
		Version:  m.Version,
		Source:   source,
		Bundle:   bundle,
		Manifest: &m,
	}, nil
}

// NewInstance 创建隔离实例。
func (e *GojaEngine) NewInstance(compiled *CompiledPlugin, opts InstanceOptions) (Instance, error) {
	if opts.CallTimeout <= 0 {
		opts.CallTimeout = DefaultCallTimeout
	}
	inst := &gojaInstance{
		compiled:     compiled,
		callTimeout:  opts.CallTimeout,
		allowedHosts: normalizeHosts(opts.AllowedHosts),
		configValues: opts.ConfigValues,
	}
	if err := inst.init(); err != nil {
		return nil, err
	}
	return inst, nil
}

// gojaInstance 单个隔离实例：一个 goja.Runtime，互斥串行调用。
type gojaInstance struct {
	compiled     *CompiledPlugin
	callTimeout  time.Duration
	allowedHosts map[string]struct{} // host / host:port（已小写归一）
	configValues map[string]any      // 插件配置值（解密后），utils.config.get 读取

	rt      *goja.Runtime
	exports *goja.Object
	utils   *hostUtils

	mu       sync.Mutex
	closed   bool
	poisoned bool
}

func (i *gojaInstance) init() error {
	i.rt = goja.New()
	i.utils = newHostUtils(i)
	i.rt.Set("utils", i.utils.buildObject(i.rt))

	// 顶层执行受 CallTimeout 保护：Compile 的 probe 不注入 utils，实例注入——
	// 插件可借环境差异（如 typeof utils 判断）在实例侧埋死循环，挂死池调用与启动装载
	if err := runInterruptGuarded(i.rt, i.callTimeout, func() error {
		_, runErr := i.rt.RunString(i.compiled.Bundle)
		return runErr
	}); err != nil {
		if isInterruptedErr(err) {
			return gerror.Newf("插件顶层执行超时（%s），实例创建失败", i.callTimeout)
		}
		return gerror.Wrap(err, "插件加载失败")
	}
	exports, ok := i.rt.Get(globalName).(*goja.Object)
	if !ok {
		return gerror.New("插件导出不可用")
	}
	i.exports = exports
	return nil
}

// Call 串行调用钩子，JSON 边界出入。
func (i *gojaInstance) Call(ctx context.Context, hook string, args json.RawMessage) (json.RawMessage, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.closed {
		return nil, gerror.New("实例已关闭")
	}
	if i.poisoned {
		return nil, gerror.New("实例已损坏（前次调用 panic），需重建")
	}

	fn := i.exports.Get(hook)
	if fn == nil || goja.IsUndefined(fn) || goja.IsNull(fn) {
		return nil, NewHookError(i.compiled.Key, hook, "钩子未导出")
	}
	callable, ok := goja.AssertFunction(fn)
	if !ok {
		return nil, NewHookError(i.compiled.Key, hook, "导出成员不可调用")
	}

	// 每次调用重置宿主能力预算与上下文
	i.utils.beginCall(ctx)

	var argAny any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &argAny); err != nil {
			return nil, NewHookError(i.compiled.Key, hook, "入参序列化失败: %v", err)
		}
	}

	var result json.RawMessage
	if err := runInterruptGuarded(i.rt, i.callTimeout, func() error {
		var callErr error
		result, callErr = i.invoke(callable, argAny, hook)
		return callErr
	}); err != nil {
		return nil, err
	}
	return result, nil
}

// runInterruptGuarded 带取消闸的执行保护：fn 在 d 内未完成则 Interrupt 中断。
//
// 取消闸的必要性：goja 的 Interrupt 标志一旦置位不会自动清除（runtime 空闲时置位，
// 下次 Run*() 立即中断，须 ClearInterrupt 复位）。回调可能在 fn 已返回、defer 尚未
// 执行的微秒窗口内拿到锁并 Interrupt——残留标志会让归还池的实例下一次执行假超时。
// 互斥闸保证 cancelled 置位后回调不再 Interrupt；回调持锁置 interrupted 标志，
// defer 侧据此精确 ClearInterrupt（标志位在锁内与 Interrupt 同步置位，不存在
// 「ClearInterrupt 先于 Interrupt 执行」的乱序——Stop 返回值不可信，故不用它判断）。
func runInterruptGuarded(rt *goja.Runtime, d time.Duration, fn func() error) error {
	var mu sync.Mutex
	cancelled := false
	interrupted := false
	timer := time.AfterFunc(d, func() {
		mu.Lock()
		defer mu.Unlock()
		if !cancelled {
			interrupted = true
			rt.Interrupt("timeout")
		}
	})
	defer func() {
		mu.Lock()
		cancelled = true
		fired := interrupted
		mu.Unlock()
		timer.Stop()
		// 回调已 Interrupt 过：无论 fn 是否因中断失败，都清掉标志——
		// 成功路径的残留标志会让该 runtime 复用时假超时
		if fired {
			rt.ClearInterrupt()
		}
	}()
	return fn()
}

// isInterruptedErr 判断是否 goja 中断错误（超时/Interrupt 触发）。
func isInterruptedErr(err error) bool {
	var interrupted *goja.InterruptedError
	if gerror.As(err, &interrupted) {
		return true
	}
	return strings.Contains(err.Error(), "interrupted")
}

// invoke 执行 JS 调用并做 JSON 边界转换，含 panic 与中断恢复。
func (i *gojaInstance) invoke(callable goja.Callable, argAny any, hook string) (out json.RawMessage, err error) {
	defer func() {
		if r := recover(); r != nil {
			i.poisoned = true
			err = NewHookError(i.compiled.Key, hook, "引擎 panic：%v", r)
		}
	}()
	v, jsErr := callable(goja.Undefined(), i.rt.ToValue(argAny))
	if jsErr != nil {
		if isInterruptedErr(jsErr) {
			return nil, NewHookError(i.compiled.Key, hook, "执行超时（%s）或被中断", i.callTimeout)
		}
		return nil, NewHookError(i.compiled.Key, hook, "%s", sanitizeJSMessage(jsErr.Error()))
	}
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil, NewHookError(i.compiled.Key, hook, "返回为空（需返回契约定义的对象）")
	}
	out, err = stringifyValue(i.rt, v)
	if err != nil {
		return nil, NewHookError(i.compiled.Key, hook, "返回无法序列化为 JSON: %v", err)
	}
	return out, nil
}

// HasHook 报告钩子是否导出且可调用。
func (i *gojaInstance) HasHook(hook string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.closed || i.exports == nil {
		return false
	}
	fn := i.exports.Get(hook)
	if fn == nil || goja.IsUndefined(fn) || goja.IsNull(fn) {
		return false
	}
	_, ok := goja.AssertFunction(fn)
	return ok
}

// Close 关闭实例（归还资源的语义占位；goja Runtime 由 GC 回收）。
func (i *gojaInstance) Close() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.closed = true
}

// stringifyValue 在 JS 侧用 JSON.stringify 序列化（天然处理函数/undefined/循环引用）。
func stringifyValue(rt *goja.Runtime, v goja.Value) (json.RawMessage, error) {
	jsonObj := rt.Get("JSON").ToObject(rt)
	stringify, ok := goja.AssertFunction(jsonObj.Get("stringify"))
	if !ok {
		return nil, gerror.New("JSON.stringify 不可用")
	}
	sv, err := stringify(goja.Undefined(), v)
	if err != nil {
		return nil, err
	}
	s := sv.String()
	if s == "undefined" {
		return nil, gerror.New("顶层返回不可序列化（函数或 undefined）")
	}
	return json.RawMessage(s), nil
}

// sanitizeJSMessage 清理引擎错误前缀，得到可透出的消息。
func sanitizeJSMessage(msg string) string {
	// goja 异常形如 "ReferenceError: xxx at npm (<eval>:1:1)" —— 保留首段
	if idx := strings.Index(msg, " at "); idx > 0 {
		msg = msg[:idx]
	}
	return msg
}

// normalizeHosts 归一白名单：小写、去重复。
func normalizeHosts(entries []string) map[string]struct{} {
	set := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			set[e] = struct{}{}
		}
	}
	return set
}
