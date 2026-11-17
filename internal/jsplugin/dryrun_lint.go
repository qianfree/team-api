package jsplugin

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// 试运行静态检查：覆盖样例执行走不到的分支。只做能精确归因到具体代码的判定
// （报出变量名/调用点），不做模糊的风格推断。

// hostAPIMembers utils 宿主能力的合法成员（与 hostfunc.go buildObject 一致）。
// 值为子成员集合；nil 表示叶子成员（函数），非 nil 表示命名空间对象。
var hostAPIMembers = map[string]map[string]struct{}{
	"fetch":  nil,
	"now":    nil,
	"uuid":   nil,
	"log":    {"info": {}, "warn": {}, "error": {}},
	"config": {"get": {}},
	"crypto": {"hmacSHA256": {}, "jwtSignHS256": {}, "base64": {}, "base64URL": {}, "base64URLDecode": {}},
}

var (
	lintBlockCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)
	// 行注释：// 前不能是冒号（排除字符串里的 https://）
	lintLineCommentRe = regexp.MustCompile(`(?m)(^|[^:])//.*$`)
	// utils.a 或 utils.a.b（成员访问链，允许点号两侧空白）
	lintUtilsMemberRe = regexp.MustCompile(`\butils\s*\.\s*([A-Za-z_$][\w$]*)(?:\s*\.\s*([A-Za-z_$][\w$]*))?`)
	// utils.fetch 首参为字符串/模板字面量：fetch(url, options) 的错误心智模型
	lintFetchStringArgRe = regexp.MustCompile("\\butils\\s*\\.\\s*fetch\\s*\\(\\s*[\"'`]")
	// utils.fetch(标识符, ...)：两参形态（合法写法是单个对象参数）
	lintFetchTwoArgRe = regexp.MustCompile(`\butils\s*\.\s*fetch\s*\(\s*[A-Za-z_$][\w$.]*\s*,`)
	// 顶层 let/var 声明（行首无缩进；IIFE 打包后成为实例常驻的闭包变量）
	lintModuleVarRe = regexp.MustCompile(`(?m)^(?:export\s+)?(?:let|var)\s+([A-Za-z_$][\w$]*)`)
	// 顶层空容器常量（典型缓存形态：const cache = {} / [] / new Map()）
	lintModuleContainerRe = regexp.MustCompile(`(?m)^(?:export\s+)?const\s+([A-Za-z_$][\w$]*)\s*=\s*(?:\{\s*\}|\[\s*\]|new\s+(?:Map|Set|WeakMap)\b)`)
)

// stripJSComments 去除 JS 注释（近似实现：字符串内的 /* 可能误吞代码，只会导致漏报不会误报）。
func stripJSComments(source string) string {
	out := lintBlockCommentRe.ReplaceAllString(source, "")
	return lintLineCommentRe.ReplaceAllString(out, "$1")
}

// dryRunLintHostAPI 宿主 API 用法检查：调用不存在的 utils 成员、utils.log(...) 直接调用、
// utils.fetch 非单对象参数——这些写法运行到即抛错，命中即 fail。
func dryRunLintHostAPI(source string) DryRunCheck {
	code := stripJSComments(source)
	var problems []string
	seen := map[string]bool{}
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			problems = append(problems, p)
		}
	}

	for _, m := range lintUtilsMemberRe.FindAllStringSubmatchIndex(code, -1) {
		member := code[m[2]:m[3]]
		sub := ""
		if m[4] >= 0 {
			sub = code[m[4]:m[5]]
		}
		children, ok := hostAPIMembers[member]
		if !ok {
			add(fmt.Sprintf("utils.%s 不存在（可用：fetch/log/config/now/uuid/crypto）", member))
			continue
		}
		if children == nil {
			continue // 叶子函数：utils.fetch / utils.now / utils.uuid
		}
		if sub == "" {
			// 命名空间被直接调用：utils.log(...) / utils.config(...)
			rest := strings.TrimLeft(code[m[1]:], " \t\r\n")
			if strings.HasPrefix(rest, "(") {
				add(fmt.Sprintf("utils.%s 是对象不能直接调用（应为 utils.%s.%s(...)）", member, member, firstMember(children)))
			}
			continue
		}
		if _, ok := children[sub]; !ok {
			add(fmt.Sprintf("utils.%s.%s 不存在（可用：%s）", member, sub, strings.Join(sortedMembers(children), "/")))
		}
	}
	if lintFetchStringArgRe.MatchString(code) || lintFetchTwoArgRe.MatchString(code) {
		add("utils.fetch 参数须为单个对象 utils.fetch({url, method, headers, body})，不支持 fetch(url, options) 形态")
	}

	if len(problems) == 0 {
		return DryRunCheck{Name: "lint-host-api", Status: dryRunPass, Detail: "宿主 API（utils.*）用法符合契约"}
	}
	return DryRunCheck{Name: "lint-host-api", Status: dryRunFail,
		Detail: "宿主 API 用法错误（执行到即抛错）：" + strings.Join(problems, "；")}
}

// dryRunLintModuleState 模块作用域可变状态检查：插件实例跨租户复用，顶层 let/var 与
// 空容器常量在实例生命周期内常驻，存放请求级数据会泄漏给其他租户。命中仅 warn——
// 只读常量用 let 声明也会命中，由作者确认。
func dryRunLintModuleState(source string) DryRunCheck {
	code := stripJSComments(source)
	var names []string
	for _, m := range lintModuleVarRe.FindAllStringSubmatch(code, -1) {
		names = append(names, m[1])
	}
	for _, m := range lintModuleContainerRe.FindAllStringSubmatch(code, -1) {
		names = append(names, m[1])
	}
	if len(names) == 0 {
		return DryRunCheck{Name: "lint-module-state", Status: dryRunPass, Detail: "模块作用域无可变状态"}
	}
	return DryRunCheck{Name: "lint-module-state", Status: dryRunWarn,
		Detail: fmt.Sprintf("模块作用域存在可变变量 %s：插件实例跨租户复用，禁止在其中保存请求级/租户级数据"+
			"（会泄漏给其他租户的请求），请求级状态请用 state 透传；若仅为只读常量请改用 const",
			strings.Join(names, "、"))}
}

func sortedMembers(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func firstMember(set map[string]struct{}) string {
	if _, ok := set["info"]; ok {
		return "info"
	}
	return sortedMembers(set)[0]
}
